package preview

import (
    "context"
    "errors"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"
)

// githubCLIToken optionally reuses the user's existing GitHub CLI login.
// It never persists the token or exposes it to a Preview child process.
// GitHub CLI is optional and is not a Task Mecca installation dependency.
func githubCLIToken(ctx context.Context) (string,error) {
    executable,err:=exec.LookPath("gh")
    if err!=nil {
        // macOS launch agents and packaged apps often omit Homebrew paths.
        // An installed gh CLI should still work without requiring PATH edits.
        paths:=[]string{"/opt/homebrew/bin/gh","/usr/local/bin/gh","/usr/bin/gh"}
        if runtime.GOOS=="windows" {
            for _,base:=range []string{os.Getenv("ProgramFiles"),os.Getenv("ProgramW6432")} {
                if base!=""{paths=append(paths,filepath.Join(base,"GitHub CLI","gh.exe"))}
            }
        }
        for _,candidate:=range paths {
            if info,statErr:=os.Stat(candidate);statErr==nil&&!info.IsDir(){
                executable=candidate
                break
            }
        }
    }
    if executable=="" {return "",errors.New("GitHub CLI is not installed")}
    commandCtx,cancel:=context.WithTimeout(ctx,4*time.Second)
    defer cancel()
    cmd:=exec.CommandContext(commandCtx,executable,"auth","token","--hostname","github.com")
    output,err:=cmd.Output()
    if err!=nil{return "",errors.New("GitHub CLI is not authenticated")}
    token:=strings.TrimSpace(string(output))
    if token==""||strings.ContainsAny(token,"\r\n\t ") {
        return "",errors.New("GitHub CLI returned an invalid token")
    }
    return token,nil
}
