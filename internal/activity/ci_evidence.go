package activity

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/projectguard"
)

// CI evidence is deliberately separate from agent Activity intervals. CI jobs
// may overlap with agent work and must never increase agent-time or the
// observed Activity wall-time total.
type CIJob struct {
    TaskID string `json:"task_id"`
    Repository string `json:"repository"`
    RunID int64 `json:"run_id"`
    RunAttempt int `json:"run_attempt"`
    JobID int64 `json:"job_id"`
    JobName string `json:"job_name"`
    Workflow string `json:"workflow"`
    CommitSHA string `json:"commit_sha"`
    Scope string `json:"scope,omitempty"`
    Type Type `json:"type,omitempty"`
    Conclusion string `json:"conclusion"`
    StartedAt time.Time `json:"started_at"`
    CompletedAt time.Time `json:"completed_at"`
    URL string `json:"url"`
    Source string `json:"source"`
}
type CIRunLink struct {
    JobID int64 `json:"job_id"`
    LocalRunIDs []string `json:"local_run_ids"`
}
type CIReport struct {
    JobCount int `json:"job_count"`
    FullRegressionCount int `json:"full_regression_count"`
    SumJobMillis int64 `json:"sum_job_ms"`
    FullRegressionMillis int64 `json:"full_regression_ms"`
    MatchedJobs int `json:"matched_jobs"`
    Links []CIRunLink `json:"links"`
    Jobs []CIJob `json:"jobs"`
}
var ciTaskRE=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var ciRepoRE=regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var ciSHA40=regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
var ErrCIConflict=errors.New("CI evidence identity conflict")

func (j CIJob) Validate()error{
    if !ciTaskRE.MatchString(j.TaskID){return fmt.Errorf("%w: invalid CI task ID",ErrInvalidEvent)}
    if !ciRepoRE.MatchString(j.Repository){return fmt.Errorf("%w: invalid CI repository",ErrInvalidEvent)}
    if !ciSHA40.MatchString(j.CommitSHA){return fmt.Errorf("%w: invalid CI commit SHA",ErrInvalidEvent)}
    if j.RunID<=0||j.RunAttempt<=0||j.JobID<=0{return fmt.Errorf("%w: invalid CI run or job ID",ErrInvalidEvent)}
    if strings.TrimSpace(j.JobName)=="" || strings.TrimSpace(j.Workflow)==""{
        return fmt.Errorf("%w: missing CI job or workflow name",ErrInvalidEvent)
    }
    if !j.StartedAt.Before(j.CompletedAt){return fmt.Errorf("%w: CI job must have positive completed duration",ErrInvalidEvent)}
    switch j.Conclusion {
    case "success","failure","cancelled","timed_out","neutral","skipped","action_required":
    default:return fmt.Errorf("%w: unsupported CI conclusion",ErrInvalidEvent)
    }
    if j.Type!="" && j.Type!=TargetedTest && j.Type!=FullRegression && j.Type!=BuildCheck{
        return fmt.Errorf("%w: invalid CI test type",ErrInvalidEvent)
    }
    if j.Type!="" && j.Scope==""{return fmt.Errorf("%w: classified CI job requires scope",ErrInvalidEvent)}
    if j.Source!="github_actions_api" {return fmt.Errorf("%w: unverified CI evidence source",ErrInvalidEvent)}
    expected:="https://github.com/"+j.Repository+"/actions/runs/"
    if !strings.HasPrefix(j.URL,expected){return fmt.Errorf("%w: invalid CI job URL",ErrInvalidEvent)}
    return nil
}

type CIJournal struct{Project string}
func (s CIJournal) directory(task string)(string,error){
    if !ciTaskRE.MatchString(task)||strings.TrimSpace(s.Project)==""{
        return "",fmt.Errorf("%w: invalid CI task or project",ErrInvalidEvent)
    }
    return filepath.Join(s.Project,"_task_mecca","data","activity","ci",task),nil
}
func ciIdentity(j CIJob)string{
    sum:=sha256.Sum256([]byte(j.Repository+"\x00"+fmt.Sprint(j.RunID)+"\x00"+fmt.Sprint(j.RunAttempt)+"\x00"+fmt.Sprint(j.JobID)))
    return "ci-"+hex.EncodeToString(sum[:12])
}
func (s CIJournal) List(task string)([]CIJob,error){
    dir,err:=s.directory(task);if err!=nil{return nil,err}
    entries,err:=os.ReadDir(dir)
    if errors.Is(err,os.ErrNotExist){return []CIJob{},nil}
    if err!=nil{return nil,err}
    rows:=make([]CIJob,0,len(entries))
    for _,ent:=range entries{
        if ent.IsDir()||!strings.HasSuffix(ent.Name(),".json"){continue}
        bytes,err:=os.ReadFile(filepath.Join(dir,ent.Name()))
        if err!=nil{return nil,err}
        var job CIJob
        if err:=json.Unmarshal(bytes,&job);err!=nil{return nil,fmt.Errorf("invalid CI record %s: %w",ent.Name(),err)}
        if err:=job.Validate();err!=nil{return nil,err}
        if ciIdentity(job)+".json"!=ent.Name()||job.TaskID!=task{
            return nil,fmt.Errorf("CI job identity mismatch: %s",ent.Name())
        }
        rows=append(rows,job)
    }
    sort.Slice(rows,func(i,k int)bool{
        if rows[i].StartedAt.Equal(rows[k].StartedAt){return rows[i].JobID<rows[k].JobID}
        return rows[i].StartedAt.Before(rows[k].StartedAt)
    })
    return rows,nil
}
// AppendBatch holds the canonical project guard and the Activity exclusive
// lock once per CI import, rather than locking for each CI job.
func (s CIJournal) AppendBatch(jobs []CIJob)(int,error){
    if len(jobs)==0{return 0,nil}
    task:=jobs[0].TaskID
    dir,err:=s.directory(task);if err!=nil{return 0,err}
    for _,job:=range jobs{
        if job.TaskID!=task{return 0,fmt.Errorf("%w: cross-task CI batch",ErrInvalidEvent)}
        if err:=job.Validate();err!=nil{return 0,err}
    }
    release,err:=projectguard.AcquireWrite(s.Project)
    if err!=nil{return 0,err}
    defer release()
    unlock,err:=acquireActivityLock(s.Project)
    if err!=nil{return 0,err}
    defer unlock()
    if err:=os.MkdirAll(dir,0700);err!=nil{return 0,err}
    // Preflight all conflicts so a re-sync never partially overwrites a run.
    unique:=map[string][]byte{}
    order:=make([]string,0,len(jobs))
    for _,job:=range jobs{
        name:=ciIdentity(job)+".json"
        body,err:=json.MarshalIndent(job,"","  ");if err!=nil{return 0,err}
        body=append(body,'\n')
        if prev,exists:=unique[name];exists{
            if string(prev)!=string(body){return 0,ErrCIConflict}
            continue
        }
        unique[name]=body;order=append(order,name)
        old,err:=os.ReadFile(filepath.Join(dir,name))
        if err==nil&&string(old)!=string(body){return 0,ErrCIConflict}
        if err!=nil&&!errors.Is(err,os.ErrNotExist){return 0,err}
    }
    inserted:=0
    for _,name:=range order{
        dest:=filepath.Join(dir,name)
        temp,err:=os.CreateTemp(dir,".ci-*.tmp")
        if err!=nil{return inserted,err}
        if err:=temp.Chmod(0600);err!=nil{temp.Close();os.Remove(temp.Name());return inserted,err}
        if _,err:=temp.Write(unique[name]);err!=nil{temp.Close();os.Remove(temp.Name());return inserted,err}
        if err:=temp.Sync();err!=nil{temp.Close();os.Remove(temp.Name());return inserted,err}
        if err:=temp.Close();err!=nil{os.Remove(temp.Name());return inserted,err}
        if err:=os.Link(temp.Name(),dest);err!=nil&&!errors.Is(err,os.ErrExist){
            os.Remove(temp.Name());return inserted,err
        }else if err==nil{inserted++}
        _=os.Remove(temp.Name())
    }
    return inserted,nil
}
func SummarizeCI(jobs []CIJob,local []TestRun)CIReport{
    out:=CIReport{Jobs:jobs,Links:[]CIRunLink{},JobCount:len(jobs)}
    matches:=map[string][]string{}
    for _,run:=range local{
        evidence:=run.RunEvidence
        if evidence.RunID==""||evidence.GitSHA==""||evidence.Scope==""||run.Type==""{continue}
        key:=strings.ToLower(evidence.GitSHA)+"\x00"+string(run.Type)+"\x00"+evidence.Scope
        matches[key]=append(matches[key],evidence.RunID)
    }
    for _,job:=range jobs{
        ms:=job.CompletedAt.Sub(job.StartedAt).Milliseconds()
        out.SumJobMillis+=ms
        if job.Type==FullRegression{out.FullRegressionCount++;out.FullRegressionMillis+=ms}
        if job.Type!=""&&job.Scope!=""{
            key:=strings.ToLower(job.CommitSHA)+"\x00"+string(job.Type)+"\x00"+job.Scope
            if ids:=matches[key];len(ids)>0{
                out.MatchedJobs++;out.Links=append(out.Links,CIRunLink{JobID:job.JobID,LocalRunIDs:append([]string(nil),ids...)})
            }
        }
    }
    return out
}
