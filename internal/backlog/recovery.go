package backlog

import (
    "os/exec"
    "path/filepath"
    "sort"
    "strings"
)

func workingTreeChanges(project string) ([]string,error) {
    cmd:=exec.Command("git","status","--porcelain=v1","-z","--untracked-files=all","--",".")
    cmd.Dir=project
    out,err:=cmd.Output()
    if err!=nil { return nil,err }

    seen:=map[string]bool{}
    parts:=strings.Split(string(out),"\x00")
    for i:=0;i<len(parts);i++ {
        entry:=parts[i]
        if len(entry)<4 { continue }
        code:=entry[:2]
        path:=filepath.ToSlash(entry[3:])
        if path!="" { seen[path]=true }

        if (strings.Contains(code,"R") || strings.Contains(code,"C")) && i+1<len(parts) {
            other:=filepath.ToSlash(parts[i+1])
            if other!="" { seen[other]=true }
            i++
        }
    }

    paths:=make([]string,0,len(seen))
    for path:=range seen { paths=append(paths,path) }
    sort.Strings(paths)
    return paths,nil
}

func scopedWorkingTreeChanges(changes []string,scope string) []string {
    scopes:=scopeTokens(scope)
    if len(scopes)==0 || len(changes)==0 { return []string{} }

    matched:=[]string{}
    for _,change:=range changes {
        normalized:=strings.ToLower(strings.TrimPrefix(filepath.ToSlash(change),"./"))
        for _,token:=range scopes {
            if scopeOverlap(token,normalized) {
                matched=append(matched,change)
                break
            }
        }
    }
    sort.Strings(matched)
    return matched
}

func continuityHealthConsumesWorkerSlot(health string) bool {
    switch health {
    case "awaiting_finalize","worker_missing","needs_user":
        return false
    default:
        return true
    }
}
