package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/activity"
)

func runActivityCI(args []string,out,errOut io.Writer)int{
    if len(args)==0||args[0]!="sync"{
        fmt.Fprintln(errOut,"usage: task-mecca activity ci sync --project DIR --task B-123 --repo OWNER/REPO --run-id ID --sha FULL_SHA [--job-name EXACT_JOB --type full_regression --scope all-go]");return 2
    }
    fs:=flag.NewFlagSet("activity ci sync",flag.ContinueOnError)
    fs.SetOutput(errOut)
    project:=fs.String("project",".","canonical project root")
    task:=fs.String("task","","backlog ID receiving CI evidence")
    repository:=fs.String("repo","","GitHub owner/repository")
    runID:=fs.Int64("run-id",0,"completed GitHub Actions run ID")
    sha:=fs.String("sha","","exact 40-character commit SHA")
    jobName:=fs.String("job-name","","optional exact CI job name filter")
    kind:=fs.String("type","","targeted_test|full_regression|build_check for selected job")
    scope:=fs.String("scope","","classified test scope matching local run scope")
    jsonOutput:=fs.Bool("json",false,"print JSON summary")
    if err:=fs.Parse(args[1:]);err!=nil{return 2}
    if fs.NArg()!=0{fmt.Fprintln(errOut,"unexpected positional arguments");return 2}
    root,err:=filepath.Abs(*project)
    if err!=nil{fmt.Fprintln(errOut,err);return 2}
    token:=strings.TrimSpace(os.Getenv("TASK_MECCA_GITHUB_TOKEN"))
    if token==""{token=strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))}
    ctx,cancel:=context.WithTimeout(context.Background(),25*time.Second)
    defer cancel()
    jobs,err:=(activity.GitHubCIClient{Token:token}).FetchGitHubCI(
        ctx,*task,*repository,*runID,*sha,*jobName,activity.Type(*kind),*scope)
    if err!=nil{fmt.Fprintln(errOut,"CI evidence not imported:",err);return 1}
    if len(jobs)==0{
        fmt.Fprintln(errOut,"no completed CI jobs matched; nothing was imported")
        return 2
    }
    inserted,err:=(activity.CIJournal{Project:root}).AppendBatch(jobs)
    if err!=nil{fmt.Fprintln(errOut,"CI evidence not imported:",err);return 1}
    response:=map[string]any{
        "ok":true,"task_id":*task,"repository":*repository,
        "run_id":*runID,"commit_sha":strings.ToLower(*sha),
        "jobs_found":len(jobs),"jobs_imported":inserted,
        "jobs_unchanged":len(jobs)-inserted,
        "agent_time_included":false,
    }
    if *jsonOutput{
        if err:=json.NewEncoder(out).Encode(response);err!=nil{fmt.Fprintln(errOut,err);return 1}
    }else{
        fmt.Fprintf(out,"CI evidence for %s: %d jobs imported, %d unchanged (not counted as Agent time)\n",
            *task,inserted,len(jobs)-inserted)
    }
    return 0
}
