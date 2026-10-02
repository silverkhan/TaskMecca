package runtimeobs

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os/exec"
    "strings"
    "time"
)

func ObserveHook(project,provider string,input io.Reader,now time.Time) (ExecutionEvent,error) {
    spike,err:=ParseHookEvent(provider,input,now)
    if err!=nil { return ExecutionEvent{},err }
    // SessionStart is session-level metadata, not an execution attempt. Keep it
    // in the same append-only journal so Root Session aggregation can consume it,
    // while BuildLedger deliberately excludes it from attempt accounting.
    if strings.EqualFold(spike.HookEventName,"SessionStart") {
        if strings.TrimSpace(spike.SessionID)=="" { return ExecutionEvent{},errors.New("SessionStart hook does not include session_id") }
        event:=ExecutionEvent{
            EventKind:"session_metadata",ObservedAt:spike.ObservedAt,
            Provider:strings.ToLower(strings.TrimSpace(spike.Provider)),SessionID:spike.SessionID,
            SessionName:spike.SessionName,SessionTitle:spike.SessionTitle,
            AttemptID:"rootmeta-"+rootSessionIDFor(spike.Provider,spike.SessionID),
            HookEventName:spike.HookEventName,Reason:fmt.Sprint(spike.Extra["source"]),EvidenceSource:EvidenceHook,
            ObservationQuality:QualityObserved,RawSHA256:spike.RawSHA256,
        }
        event.EventID=eventIDFor(event)
        if err:=AppendExecutionEvent(project,event); err!=nil { return ExecutionEvent{},err }
        return event,nil
    }

    event,err:=HookToExecutionEvent(spike)
    if err!=nil { return ExecutionEvent{},err }

    // Some provider/tool event variants may omit session_id. Reuse a unique
    // live runtime-agent match if one exists; never guess when multiple
    // candidates exist.
    if event.SessionID=="" && event.RuntimeAgentID!="" {
        if ledger,readErr:=BuildLedger(project,5,now); readErr==nil {
            matches:=[]Attempt{}
            for _,attempt:=range ledger.Attempts {
                if attempt.Provider==event.Provider && attempt.RuntimeAgentID==event.RuntimeAgentID && !attempt.Terminal {
                    matches=append(matches,attempt)
                }
            }
            if len(matches)==1 { event.AttemptID=matches[0].AttemptID; event.EventID=eventIDFor(event) }
        }
    }

    if err:=AppendExecutionEvent(project,event); err!=nil { return ExecutionEvent{},err }
    return event,nil
}

// Codex Root Session display names are enriched through the official App Server only.\n// The resolver is optional and must never become a liveness or identity source.\nfunc FilterLedger(ledger Ledger,provider string) Ledger {
    provider=strings.ToLower(strings.TrimSpace(provider))
    if provider=="" { return ledger }
    filtered:=ledger
    filtered.Attempts=nil
    filtered.Findings=nil
    allowed:=map[string]bool{}
    for _,attempt:=range ledger.Attempts {
        if attempt.Provider==provider { filtered.Attempts=append(filtered.Attempts,attempt); allowed[attempt.AttemptID]=true }
    }
    for _,finding:=range ledger.Findings {
        if finding.AttemptID=="" || allowed[finding.AttemptID] { filtered.Findings=append(filtered.Findings,finding) }
    }
    return filtered
}

type CodexThreadMetadata struct { Name string; Title string }
type codexRPCMessage struct { ID any `json:"id,omitempty"`; Result json.RawMessage `json:"result,omitempty"`; Error json.RawMessage `json:"error,omitempty"` }

func codexAppServerCommand(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx,"codex","app-server","--listen","stdio://") }

func readCodexRPC(reader *bufio.Reader,wantedID float64) (codexRPCMessage,error) {
    for {
        line,err:=reader.ReadBytes('\n'); if err!=nil { return codexRPCMessage{},err }
        var msg codexRPCMessage
        if json.Unmarshal(line,&msg)!=nil || msg.ID==nil { continue }
        id,ok:=msg.ID.(float64); if !ok || id!=wantedID { continue }
        if len(msg.Error)>0 && string(msg.Error)!="null" { return msg,fmt.Errorf("%s",strings.TrimSpace(string(msg.Error))) }
        return msg,nil
    }
}

func ResolveCodexThreadMetadata(ctx context.Context,sessionID string) (CodexThreadMetadata,error) {
    sessionID=strings.TrimSpace(sessionID)
    if sessionID=="" { return CodexThreadMetadata{},errors.New("empty Codex session_id") }
    lookupCtx,cancel:=context.WithTimeout(ctx,4*time.Second); defer cancel()
    cmd:=codexAppServerCommand(lookupCtx)
    stdin,err:=cmd.StdinPipe(); if err!=nil { return CodexThreadMetadata{},err }
    stdout,err:=cmd.StdoutPipe(); if err!=nil { return CodexThreadMetadata{},err }
    if err=cmd.Start(); err!=nil { return CodexThreadMetadata{},err }
    defer cmd.Wait()
    enc:=json.NewEncoder(stdin); reader:=bufio.NewReader(stdout)
    initRequest:=map[string]any{"jsonrpc":"2.0","id":1,"method":"initialize","params":map[string]any{"clientInfo":map[string]any{"name":"task_mecca","title":"Task Mecca","version":"0.1"}}}
    if err=enc.Encode(initRequest); err!=nil { return CodexThreadMetadata{},err }
    if _,err=readCodexRPC(reader,1); err!=nil { return CodexThreadMetadata{},fmt.Errorf("codex initialize: %w",err) }
    if err=enc.Encode(map[string]any{"jsonrpc":"2.0","method":"initialized","params":map[string]any{}}); err!=nil { return CodexThreadMetadata{},err }
    if err=enc.Encode(map[string]any{"jsonrpc":"2.0","id":2,"method":"thread/read","params":map[string]any{"threadId":sessionID}}); err!=nil { return CodexThreadMetadata{},err }
    msg,err:=readCodexRPC(reader,2); if err!=nil { return CodexThreadMetadata{},fmt.Errorf("codex thread/read: %w",err) }
    var result struct { Thread struct { ID string `json:"id"`; Name string `json:"name"`; Title string `json:"title"` } `json:"thread"` }
    if err=json.Unmarshal(msg.Result,&result); err!=nil { return CodexThreadMetadata{},err }
    return CodexThreadMetadata{Name:strings.TrimSpace(result.Thread.Name),Title:strings.TrimSpace(result.Thread.Title)},nil
}
