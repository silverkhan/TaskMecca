package runtimeobs

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
)

func TestEnsureHooksPreservesExistingConfigAndIsIdempotent(t *testing.T) {
    project:=t.TempDir()
    path:=filepath.Join(project,".claude","settings.json")
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { t.Fatal(err) }
    original:=`{
  "permissions": {"allow": ["Bash(git status)"]},
  "hooks": {
    "SubagentStart": [
      {"hooks": [{"type": "command", "command": "echo existing"}]}
    ]
  }
}`
    if err:=os.WriteFile(path,[]byte(original),0644); err!=nil { t.Fatal(err) }

    first,err:=EnsureHooks(project,"claude")
    if err!=nil { t.Fatal(err) }
    if !first.Installed || !first.Changed { t.Fatalf("first=%+v",first) }

    data,err:=os.ReadFile(path); if err!=nil { t.Fatal(err) }
    doc:=map[string]any{}; if err:=json.Unmarshal(data,&doc); err!=nil { t.Fatal(err) }
    if _,ok:=doc["permissions"]; !ok { t.Fatalf("existing config lost: %s",data) }

    second,err:=EnsureHooks(project,"claude")
    if err!=nil { t.Fatal(err) }
    if !second.Installed || second.Changed { t.Fatalf("second=%+v",second) }
}

func TestEnsureHooksUsesProviderSpecificPathsAndCommands(t *testing.T) {
    for _,provider:=range []string{"codex","claude"} {
        project:=t.TempDir()
        status,err:=EnsureHooks(project,provider)
        if err!=nil { t.Fatal(err) }
        if !status.Installed { t.Fatalf("%s status=%+v",provider,status) }
        data,err:=os.ReadFile(status.Path); if err!=nil { t.Fatal(err) }
        if provider=="codex" && filepath.Base(status.Path)!="hooks.json" { t.Fatalf("codex path=%s",status.Path) }
        if provider=="claude" && filepath.Base(status.Path)!="settings.json" { t.Fatalf("claude path=%s",status.Path) }
        if !hasTaskMeccaHookFromBytes(t,data,provider) { t.Fatalf("%s config=%s",provider,data) }
    }
}

func hasTaskMeccaHookFromBytes(t *testing.T,data []byte,provider string) bool {
    t.Helper()
    doc:=map[string]any{}; if err:=json.Unmarshal(data,&doc); err!=nil { t.Fatal(err) }
    hooks,_:=doc["hooks"].(map[string]any)
    for _,event:=range lifecycleHookEvents {
        if !hasTaskMeccaHook(hooks[event],provider) { return false }
    }
    return true
}


func TestDisableHooksRemovesOnlyTaskMeccaHandlers(t *testing.T) {
    project:=t.TempDir()
    path:=filepath.Join(project,".codex","hooks.json")
    if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { t.Fatal(err) }
    original:=`{
  "description": "keep me",
  "hooks": {
    "SubagentStart": [
      {"hooks": [
        {"type":"command","command":"echo existing"},
        {"type":"command","command":"task-mecca runtime observe codex","timeout":3}
      ]}
    ],
    "PreToolUse": [
      {"matcher":"*","hooks":[{"type":"command","command":"task-mecca runtime observe codex","async":true,"timeout":3}]}
    ]
  }
}`
    if err:=os.WriteFile(path,[]byte(original),0644); err!=nil { t.Fatal(err) }

    status,err:=DisableHooks(project,"codex")
    if err!=nil { t.Fatal(err) }
    if status.Installed { t.Fatalf("Task Mecca hooks should be disabled: %+v",status) }
    if !status.Changed { t.Fatalf("expected changed status: %+v",status) }

    data,err:=os.ReadFile(path); if err!=nil { t.Fatal(err) }
    doc:=map[string]any{}; if err:=json.Unmarshal(data,&doc); err!=nil { t.Fatal(err) }
    if doc["description"]!="keep me" { t.Fatalf("unrelated config lost: %s",data) }
    hooks,_:=doc["hooks"].(map[string]any)
    if hasTaskMeccaHook(hooks["SubagentStart"],"codex") || hasTaskMeccaHook(hooks["PreToolUse"],"codex") {
        t.Fatalf("Task Mecca hook remained: %s",data)
    }
    foundExisting:=false
    for _,entry:=range asSlice(hooks["SubagentStart"]) {
        row,ok:=entry.(map[string]any); if !ok { continue }
        for _,hook:=range asSlice(row["hooks"]) {
            item,ok:=hook.(map[string]any); if ok && item["command"]=="echo existing" { foundExisting=true }
        }
    }
    if !foundExisting { t.Fatalf("existing hook was removed: %s",data) }
}

func TestDisableHooksIsIdempotent(t *testing.T) {
    project:=t.TempDir()
    if _,err:=EnsureHooks(project,"claude"); err!=nil { t.Fatal(err) }
    first,err:=DisableHooks(project,"claude"); if err!=nil { t.Fatal(err) }
    if !first.Changed || first.Installed { t.Fatalf("first=%+v",first) }
    second,err:=DisableHooks(project,"claude"); if err!=nil { t.Fatal(err) }
    if second.Changed || second.Installed { t.Fatalf("second=%+v",second) }
}


func TestClaudeHooksIncludeSessionStartButCodexDoesNot(t *testing.T) {
    claudeProject:=t.TempDir()
    claude,err:=EnsureHooks(claudeProject,"claude")
    if err!=nil { t.Fatal(err) }
    claudeData,err:=os.ReadFile(claude.Path); if err!=nil { t.Fatal(err) }
    claudeDoc:=map[string]any{}; if err:=json.Unmarshal(claudeData,&claudeDoc); err!=nil { t.Fatal(err) }
    claudeHooks,_:=claudeDoc["hooks"].(map[string]any)
    if !hasTaskMeccaHook(claudeHooks["SessionStart"],"claude") {
        t.Fatalf("Claude SessionStart metadata hook missing: %s",claudeData)
    }

    codexProject:=t.TempDir()
    codex,err:=EnsureHooks(codexProject,"codex")
    if err!=nil { t.Fatal(err) }
    codexData,err:=os.ReadFile(codex.Path); if err!=nil { t.Fatal(err) }
    codexDoc:=map[string]any{}; if err:=json.Unmarshal(codexData,&codexDoc); err!=nil { t.Fatal(err) }
    codexHooks,_:=codexDoc["hooks"].(map[string]any)
    if hasTaskMeccaHook(codexHooks["SessionStart"],"codex") {
        t.Fatalf("Codex SessionStart must not be treated as a name source: %s",codexData)
    }
}
