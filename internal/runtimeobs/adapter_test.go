package runtimeobs

import (
    "strings"
    "testing"
    "time"
)

func TestObserveHookReusesUniqueLiveAttemptWhenSessionMissing(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)

    _,err:=ObserveHook(project,"claude",strings.NewReader(`{"session_id":"session-1","hook_event_name":"SubagentStart","agent_id":"agent-1"}`),base)
    if err!=nil { t.Fatal(err) }

    activity,err:=ObserveHook(project,"claude",strings.NewReader(`{"hook_event_name":"PreToolUse","agent_id":"agent-1","tool_name":"Bash","tool_use_id":"tool-1"}`),base.Add(time.Second))
    if err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,base.Add(2*time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=1 { t.Fatalf("attempts=%+v",ledger.Attempts) }
    if activity.AttemptID!=ledger.Attempts[0].AttemptID { t.Fatalf("activity=%s attempt=%s",activity.AttemptID,ledger.Attempts[0].AttemptID) }
    if ledger.Attempts[0].ActivityCount!=1 { t.Fatalf("activity_count=%d",ledger.Attempts[0].ActivityCount) }
}

func TestObserveHookDoesNotGuessAcrossAmbiguousLiveAttempts(t *testing.T) {
    project:=t.TempDir()
    base:=time.Date(2026,10,1,10,0,0,0,time.UTC)
    for i,session:=range []string{"session-1","session-2"} {
        _,err:=ObserveHook(project,"claude",strings.NewReader("{\"session_id\":\""+session+"\",\"hook_event_name\":\"SubagentStart\",\"agent_id\":\"agent-1\"}"),base.Add(time.Duration(i)*time.Second))
        if err!=nil { t.Fatal(err) }
    }
    activity,err:=ObserveHook(project,"claude",strings.NewReader(`{"hook_event_name":"PreToolUse","agent_id":"agent-1","tool_name":"Bash","tool_use_id":"tool-x"}`),base.Add(3*time.Second))
    if err!=nil { t.Fatal(err) }

    ledger,err:=BuildLedger(project,10,base.Add(4*time.Second))
    if err!=nil { t.Fatal(err) }
    if len(ledger.Attempts)!=3 { t.Fatalf("ambiguous missing-session activity must remain separate: %+v",ledger.Attempts) }
    if activity.SessionID!="" { t.Fatalf("unexpected session attribution: %+v",activity) }
}
