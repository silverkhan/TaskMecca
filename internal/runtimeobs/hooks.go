package runtimeobs

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

var lifecycleHookEvents=[]string{"SubagentStart","SubagentStop","PreToolUse","PostToolUse"}

func hookEventsForProvider(provider string) []string {
    events:=append([]string{},lifecycleHookEvents...)
    // Claude Code officially exposes an explicit/custom session_title only on
    // SessionStart. Codex SessionStart has no supported name/title field.
    if strings.EqualFold(strings.TrimSpace(provider),"claude") {
        events=append([]string{"SessionStart"},events...)
    }
    return events
}

type HookSetup struct {
    Provider string `json:"provider"`
    Path string `json:"path"`
    Installed bool `json:"installed"`
    Changed bool `json:"changed"`
    Events map[string]bool `json:"events"`
}

func HookConfigPath(project,provider string) (string,error) {
    switch strings.ToLower(strings.TrimSpace(provider)) {
    case "codex": return filepath.Join(project,".codex","hooks.json"),nil
    case "claude": return filepath.Join(project,".claude","settings.json"),nil
    default: return "",fmt.Errorf("unsupported provider %q",provider)
    }
}

func HookStatus(project,provider string) (HookSetup,error) {
    provider=strings.ToLower(strings.TrimSpace(provider))
    path,err:=HookConfigPath(project,provider); if err!=nil { return HookSetup{},err }
    out:=HookSetup{Provider:provider,Path:path,Events:map[string]bool{}}
    doc,exists,err:=readHookDocument(path); if err!=nil { return out,err }
    if !exists { return out,nil }
    hooks,_:=doc["hooks"].(map[string]any)
    for _,event:=range hookEventsForProvider(provider) { out.Events[event]=hasTaskMeccaHook(hooks[event],provider) }
    out.Installed=true
    for _,event:=range hookEventsForProvider(provider) { if !out.Events[event] { out.Installed=false } }
    return out,nil
}

func EnsureHooks(project,provider string) (HookSetup,error) {
    provider=strings.ToLower(strings.TrimSpace(provider))
    path,err:=HookConfigPath(project,provider); if err!=nil { return HookSetup{},err }
    doc,_,err:=readHookDocument(path); if err!=nil { return HookSetup{},err }
    hooks,ok:=doc["hooks"].(map[string]any)
    if !ok || hooks==nil { hooks=map[string]any{}; doc["hooks"]=hooks }

    changed:=false
    for _,event:=range hookEventsForProvider(provider) {
        if hasTaskMeccaHook(hooks[event],provider) { continue }
        current:=asSlice(hooks[event])
        current=append(current,desiredHook(event,provider))
        hooks[event]=current
        changed=true
    }
    if changed {
        data,err:=json.MarshalIndent(doc,"","  "); if err!=nil { return HookSetup{},err }
        data=append(data,'\n')
        if err:=os.MkdirAll(filepath.Dir(path),0755); err!=nil { return HookSetup{},err }
        mode:=os.FileMode(0644)
        if info,statErr:=os.Stat(path); statErr==nil { mode=info.Mode().Perm() }
        if err:=os.WriteFile(path,data,mode); err!=nil { return HookSetup{},err }
    }
    status,err:=HookStatus(project,provider); if err!=nil { return HookSetup{},err }
    status.Changed=changed
    return status,nil
}

func DisableHooks(project,provider string) (HookSetup,error) {
    provider=strings.ToLower(strings.TrimSpace(provider))
    path,err:=HookConfigPath(project,provider); if err!=nil { return HookSetup{},err }
    doc,exists,err:=readHookDocument(path); if err!=nil { return HookSetup{},err }
    if !exists {
        return HookStatus(project,provider)
    }
    hooks,ok:=doc["hooks"].(map[string]any)
    if !ok || hooks==nil {
        return HookStatus(project,provider)
    }

    changed:=false
    for _,event:=range hookEventsForProvider(provider) {
        entries:=asSlice(hooks[event])
        if len(entries)==0 { continue }
        filteredEntries:=make([]any,0,len(entries))
        for _,entry:=range entries {
            row,ok:=entry.(map[string]any)
            if !ok {
                filteredEntries=append(filteredEntries,entry)
                continue
            }
            commands:=asSlice(row["hooks"])
            filteredCommands:=make([]any,0,len(commands))
            removed:=false
            for _,hook:=range commands {
                item,ok:=hook.(map[string]any)
                if ok && strings.TrimSpace(fmt.Sprint(item["command"]))=="task-mecca runtime observe "+provider {
                    removed=true
                    changed=true
                    continue
                }
                filteredCommands=append(filteredCommands,hook)
            }
            if !removed || len(filteredCommands)>0 {
                if removed {
                    cloned:=map[string]any{}
                    for key,value:=range row { cloned[key]=value }
                    cloned["hooks"]=filteredCommands
                    filteredEntries=append(filteredEntries,cloned)
                } else {
                    filteredEntries=append(filteredEntries,row)
                }
            }
        }
        if len(filteredEntries)==0 {
            delete(hooks,event)
        } else {
            hooks[event]=filteredEntries
        }
    }

    if changed {
        if len(hooks)==0 { delete(doc,"hooks") }
        data,err:=json.MarshalIndent(doc,"","  "); if err!=nil { return HookSetup{},err }
        data=append(data,'\n')
        mode:=os.FileMode(0644)
        if info,statErr:=os.Stat(path); statErr==nil { mode=info.Mode().Perm() }
        if err:=os.WriteFile(path,data,mode); err!=nil { return HookSetup{},err }
    }
    status,err:=HookStatus(project,provider); if err!=nil { return HookSetup{},err }
    status.Changed=changed
    return status,nil
}

func readHookDocument(path string) (map[string]any,bool,error) {
    data,err:=os.ReadFile(path)
    if errors.Is(err,os.ErrNotExist) { return map[string]any{},false,nil }
    if err!=nil { return nil,false,err }
    if len(strings.TrimSpace(string(data)))==0 { return map[string]any{},true,nil }
    doc:=map[string]any{}
    if err:=json.Unmarshal(data,&doc); err!=nil { return nil,true,fmt.Errorf("invalid JSON in %s: %w",path,err) }
    return doc,true,nil
}

func desiredHook(event,provider string) map[string]any {
    command:=map[string]any{"type":"command","command":"task-mecca runtime observe "+provider,"timeout":3}
    entry:=map[string]any{"hooks":[]any{command}}
    if event=="PreToolUse" || event=="PostToolUse" {
        entry["matcher"]="*"
        command["async"]=true
    }
    return entry
}

func hasTaskMeccaHook(value any,provider string) bool {
    command:="task-mecca runtime observe "+provider
    for _,entry:=range asSlice(value) {
        row,ok:=entry.(map[string]any); if !ok { continue }
        for _,hook:=range asSlice(row["hooks"]) {
            item,ok:=hook.(map[string]any); if !ok { continue }
            if strings.TrimSpace(fmt.Sprint(item["command"]))==command { return true }
        }
    }
    return false
}

func asSlice(value any) []any {
    if value==nil { return []any{} }
    if rows,ok:=value.([]any); ok { return append([]any{},rows...) }
    return []any{value}
}
