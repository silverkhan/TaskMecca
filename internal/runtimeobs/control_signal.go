package runtimeobs

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"
)

type ControlSignal struct {
    ID string `json:"id"`
    Kind string `json:"kind"`
    ObservedAt string `json:"observed_at"`
    Provider string `json:"provider"`
    SessionID string `json:"session_id,omitempty"`
    TurnID string `json:"turn_id,omitempty"`
    RuntimeAgentID string `json:"runtime_agent_id,omitempty"`
    AttemptID string `json:"attempt_id,omitempty"`
    TaskID string `json:"task_id,omitempty"`
    AgentPath string `json:"agent_path,omitempty"`
    ToolName string `json:"tool_name,omitempty"`
    Message string `json:"message,omitempty"`
}

var controlSignalMu sync.Mutex

func ControlSignalPath(project string) string {
    return filepath.Join(project,"_task_mecca",".runtime","control_signals.jsonl")
}

func controlSignalID(signal ControlSignal) string {
    raw:=strings.Join([]string{
        signal.Kind,signal.Provider,signal.SessionID,signal.TurnID,
        signal.RuntimeAgentID,signal.AttemptID,signal.TaskID,signal.ToolName,
        signal.ObservedAt,
    },"\x00")
    sum:=sha256.Sum256([]byte(raw))
    return hex.EncodeToString(sum[:16])
}

func AppendControlSignal(project string,signal ControlSignal) error {
    controlSignalMu.Lock()
    defer controlSignalMu.Unlock()
    if strings.TrimSpace(signal.Kind)=="" { return errors.New("control signal requires kind") }
    if strings.TrimSpace(signal.ObservedAt)=="" { signal.ObservedAt=time.Now().UTC().Format(time.RFC3339Nano) }
    signal.Provider=strings.ToLower(strings.TrimSpace(signal.Provider))
    if signal.ID=="" { signal.ID=controlSignalID(signal) }
    path:=ControlSignalPath(project)
    if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
    payload,err:=json.Marshal(signal);if err!=nil{return err}
    payload=append(payload,'\n')
    f,err:=os.OpenFile(path,os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600);if err!=nil{return err}
    _,writeErr:=f.Write(payload);closeErr:=f.Close()
    if writeErr!=nil{return writeErr}
    return closeErr
}

func ReadControlSignals(project string,limit int) ([]ControlSignal,error) {
    controlSignalMu.Lock()
    defer controlSignalMu.Unlock()
    path:=ControlSignalPath(project)
    file,err:=os.Open(path)
    if errors.Is(err,os.ErrNotExist){return []ControlSignal{},nil}
    if err!=nil{return nil,err}
    defer file.Close()
    rows:=[]ControlSignal{}
    seen:=map[string]bool{}
    scanner:=bufio.NewScanner(file)
    scanner.Buffer(make([]byte,64*1024),2*1024*1024)
    for scanner.Scan(){
        raw:=strings.TrimSpace(scanner.Text());if raw==""{continue}
        var signal ControlSignal
        if json.Unmarshal([]byte(raw),&signal)!=nil{continue}
        if signal.ID==""{signal.ID=controlSignalID(signal)}
        if seen[signal.ID]{continue}
        seen[signal.ID]=true
        rows=append(rows,signal)
    }
    if err:=scanner.Err();err!=nil{return nil,err}
    sort.SliceStable(rows,func(i,j int)bool{
        a,aErr:=time.Parse(time.RFC3339Nano,rows[i].ObservedAt)
        b,bErr:=time.Parse(time.RFC3339Nano,rows[j].ObservedAt)
        if aErr==nil&&bErr==nil&&!a.Equal(b){return a.Before(b)}
        if rows[i].ObservedAt!=rows[j].ObservedAt{return rows[i].ObservedAt<rows[j].ObservedAt}
        return rows[i].ID<rows[j].ID
    })
    if limit>0&&len(rows)>limit{rows=append([]ControlSignal{},rows[len(rows)-limit:]...)}
    return rows,nil
}
