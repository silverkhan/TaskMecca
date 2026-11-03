package runtimeobs

import (
    "strings"
    "testing"
    "time"
)

func TestHookEvidenceCacheInvalidatesAfterHookAppend(t *testing.T) {
    project:=t.TempDir()
    at:=time.Now().UTC().Add(-time.Minute)
    cutoff:=at.Format(time.RFC3339Nano)
    before,last,err:=HookEvidenceSince(project,"codex",cutoff)
    if err!=nil || before["start"] || last!="" {t.Fatalf("initial evidence: %v %q %v",before,last,err)}
    // A repeated empty read may be cached but a subsequently written Hook
    // event must invalidate it, even when no file existed at first.
    again,_,err:=HookEvidenceSince(project,"codex",cutoff)
    if err!=nil || again["start"] {t.Fatalf("cached empty proof: %v %v",again,err)}
    payload:=`{"session_id":"s","turn_id":"t","hook_event_name":"SubagentStart","agent_id":"worker"}`
    if _,err:=ObserveHook(project,"codex",strings.NewReader(payload),at.Add(time.Second));err!=nil{t.Fatal(err)}
    after,observed,err:=HookEvidenceSince(project,"codex",cutoff)
    if err!=nil || !after["start"] || observed=="" {t.Fatalf("stale cached evidence: %v %q %v",after,observed,err)}
    // Never return the cache's own map: callers can freely modify results.
    after["start"]=false
    again,_,err=HookEvidenceSince(project,"codex",cutoff)
    if err!=nil || !again["start"] {t.Fatalf("cached evidence map mutated: %v %v",again,err)}
    newerCutoff:=at.Add(10*time.Second).Format(time.RFC3339Nano)
    newer,_,err:=HookEvidenceSince(project,"codex",newerCutoff)
    if err!=nil || newer["start"] {t.Fatalf("old event trusted for new hook install: %v %v",newer,err)}
    second:=`{"session_id":"s","turn_id":"t","hook_event_name":"PreToolUse","agent_id":"worker","tool_use_id":"tool-2"}`
    if _,err:=ObserveHook(project,"codex",strings.NewReader(second),at.Add(20*time.Second));err!=nil{t.Fatal(err)}
    updated,_,err:=HookEvidenceSince(project,"codex",newerCutoff)
    if err!=nil || !updated["activity"] || updated["start"] {t.Fatalf("new setup source mismatch: %v %v",updated,err)}
}
