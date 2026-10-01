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
