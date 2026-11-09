package orchestration

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
 "strings"
 "time"
)

const ConfigSchemaVersion = 1
var groupIDPattern = regexp.MustCompile(`^D[0-9]{4}_[0-9]{2,}$`)
var workerNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,39}$`)

type ModeChange struct {
 At string `json:"at"`
 From Mode `json:"from"`
 To Mode `json:"to"`
 Reason string `json:"reason"`
}

type Group struct {
 SchemaVersion int `json:"schema_version"`
 GroupID string `json:"group_id"`
 DisplayPrefix string `json:"display_prefix"`
 Mode Mode `json:"mode"`
 Provider string `json:"provider"`
 RootSessionID string `json:"root_session_id"`
 CreatedAt string `json:"created_at"`
 UpdatedAt string `json:"updated_at"`
 ModeHistory []ModeChange `json:"mode_history"`
}

// All group records are project-owned durable data, never under .runtime.
// The provider-native session ID is a transport identity, not a display name.
func groupDir(project string) string {
 return filepath.Join(project,"_task_mecca","data","orchestration","groups")
}

func readGroup(path string) (Group,error) {
 data,err:=os.ReadFile(path);if err!=nil{return Group{},err}
 var g Group
 if err:=json.Unmarshal(data,&g);err!=nil{return Group{},err}
 if g.SchemaVersion!=ConfigSchemaVersion || !groupIDPattern.MatchString(g.GroupID) || g.DisplayPrefix!=g.GroupID+"_" {
  return Group{},fmt.Errorf("invalid orchestration group record: %s",path)
 }
 if _,ok:=ModeByCode(g.Mode);!ok{return Group{},fmt.Errorf("invalid saved mode: %s",g.Mode)}
 if strings.TrimSpace(g.RootSessionID)=="" || strings.TrimSpace(g.Provider)=="" {return Group{},errors.New("missing immutable root identity")}
 return g,nil
}

func groups(project string) ([]Group,error) {
 entries,err:=os.ReadDir(groupDir(project))
 if errors.Is(err,os.ErrNotExist){return []Group{},nil}
 if err!=nil{return nil,err}
 out:=[]Group{}
 for _,entry:=range entries {
  if entry.IsDir() || !strings.HasSuffix(entry.Name(),".json"){continue}
  g,e:=readGroup(filepath.Join(groupDir(project),entry.Name()))
  if e!=nil{return nil,e}
  if entry.Name()!=g.GroupID+".json"{return nil,fmt.Errorf("group filename mismatch: %s",entry.Name())}
  out=append(out,g)
 }
 return out,nil
}

func FindGroup(project,provider,rootSessionID string) (Group,bool,error) {
 provider=strings.ToLower(strings.TrimSpace(provider))
 rootSessionID=strings.TrimSpace(rootSessionID)
 if provider=="" || rootSessionID=="" {return Group{},false,errors.New("provider and exact root session ID required")}
 all,err:=groups(project);if err!=nil{return Group{},false,err}
 for _,g:=range all {if g.Provider==provider&&g.RootSessionID==rootSessionID{return g,true,nil}}
 return Group{},false,nil
}

func FindGroupByID(project,groupID string)(Group,error) {
 if !groupIDPattern.MatchString(groupID){return Group{},errors.New("invalid group ID")}
 g,err:=readGroup(filepath.Join(groupDir(project),groupID+".json"))
 if err!=nil{return Group{},err}
 if g.GroupID!=groupID{return Group{},errors.New("group ID mismatch")}
 return g,nil
}

// Configure is intentionally explicit. Changing a saved mode requires a
// confirmed user choice, never an inferred provider capability change.
func Configure(project,provider,rootSessionID string, mode Mode, caps Capabilities, changeMode bool, reason string, now time.Time) (Group,error) {
 provider=strings.ToLower(strings.TrimSpace(provider))
 rootSessionID=strings.TrimSpace(rootSessionID)
 if provider==""||rootSessionID==""{return Group{},errors.New("provider and exact root session ID required")}
 if strings.ContainsAny(provider,"/\\") {return Group{},errors.New("invalid provider")}
 d,err:=Decide(Request{Mode:mode,Event:"root_initialized",Capabilities:caps})
 if err!=nil{return Group{},err}
 if !d.Allowed{return Group{},fmt.Errorf("mode %s is not available: %s",mode,strings.Join(d.Reasons,","))}
 dir:=groupDir(project)
 if err:=os.MkdirAll(dir,0700);err!=nil{return Group{},err}
 // mkdir is an atomic portable lock, including on Windows. A stranded lock
 // must be inspected explicitly; never steal it and risk dual writers.
 lock:=filepath.Join(dir,".configure.lock")
 acquired:=false
 for i:=0;i<40;i++ {
  err=os.Mkdir(lock,0700)
  if err==nil{acquired=true;break}
  if !errors.Is(err,os.ErrExist){return Group{},err}
  time.Sleep(25*time.Millisecond)
 }
 if !acquired{return Group{},errors.New("orchestration configuration is locked; inspect .configure.lock before recovery")}
 defer os.Remove(lock)
 all,err:=groups(project);if err!=nil{return Group{},err}
 for _,g:=range all {
  if g.Provider!=provider||g.RootSessionID!=rootSessionID {continue}
  if g.Mode==mode {return g,nil}
  if !changeMode {return Group{},fmt.Errorf("root group %s already uses mode %s: explicit --change-mode required",g.GroupID,g.Mode)}
  if strings.TrimSpace(reason)==""{return Group{},errors.New("mode change requires explicit user decision evidence")}
  g.ModeHistory=append(g.ModeHistory,ModeChange{At:now.UTC().Format(time.RFC3339Nano),From:g.Mode,To:mode,Reason:reason})
  g.Mode=mode;g.UpdatedAt=now.UTC().Format(time.RFC3339Nano)
  return g,writeGroup(dir,g)
 }
 datePrefix:="D"+now.Format("0102")+"_"
 used:=map[string]bool{}
 for _,g:=range all {used[g.GroupID]=true}
 for n:=1;n<=9999;n++ {
  id:=fmt.Sprintf("%s%02d",datePrefix,n)
  if used[id] {continue}
  g:=Group{SchemaVersion:ConfigSchemaVersion,GroupID:id,DisplayPrefix:id+"_",Mode:mode,
   Provider:provider,RootSessionID:rootSessionID,CreatedAt:now.UTC().Format(time.RFC3339Nano),
   UpdatedAt:now.UTC().Format(time.RFC3339Nano),ModeHistory:[]ModeChange{}}
  data,err:=json.MarshalIndent(g,"","  ");if err!=nil{return Group{},err}
  path:=filepath.Join(dir,id+".json")
  f,err:=os.OpenFile(path,os.O_CREATE|os.O_EXCL|os.O_WRONLY,0600)
  if errors.Is(err,os.ErrExist){continue}
  if err!=nil{return Group{},err}
  data=append(data,'\n')
  _,writeErr:=f.Write(data);closeErr:=f.Close()
  if writeErr!=nil || closeErr!=nil {_=os.Remove(path);return Group{},errors.Join(writeErr,closeErr)}
  return g,nil
 }
 return Group{},errors.New("no group numbers available for date")
}

func writeGroup(dir string,g Group) error {
 data,err:=json.MarshalIndent(g,"","  ");if err!=nil{return err}
 final:=filepath.Join(dir,g.GroupID+".json")
 tmp,err:=os.CreateTemp(dir,".group-*.tmp")
 if err!=nil{return err}
 name:=tmp.Name()
 defer os.Remove(name)
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if _,err:=tmp.Write(append(data,'\n'));err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 // Retain original on replace failure (Windows cannot overwrite with Rename).
 // A mode change may fail safely without changing the persisted selection.
 if err:=os.Rename(name,final);err!=nil{return err}
 return nil
}

func SessionName(g Group,role,worker string,generation int)(string,error) {
 if !groupIDPattern.MatchString(g.GroupID)||g.DisplayPrefix!=g.GroupID+"_" {return "",errors.New("invalid group")}
 switch strings.ToLower(strings.TrimSpace(role)){
 case "root":return g.DisplayPrefix+"Root",nil
 case "registrar":return g.DisplayPrefix+"Registrar",nil
 case "controller":return g.DisplayPrefix+"Controller",nil
 case "worker":
  if !workerNamePattern.MatchString(worker)||generation<1{return "",errors.New("worker name and generation >= 1 are required")}
  return fmt.Sprintf("%sWorker_%s_G%d",g.DisplayPrefix,worker,generation),nil
 default:return "",fmt.Errorf("unknown role %q",role)
 }
}

func StableRootKey(provider,sessionID string)string{
 h:=sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(provider))+"\x00"+strings.TrimSpace(sessionID)))
 return hex.EncodeToString(h[:12])
}
