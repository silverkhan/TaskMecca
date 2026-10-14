package preview

import (
    "context"
    "errors"
    "os/exec"
    "strings"
    "time"
)

// githubCLIToken optionally reuses the user's existing GitHub CLI login.
// It never persists the token or exposes it to a Preview child process.
// GitHub CLI is optional and is not a Task Mecca installation dependency.
func githubCLIToken(ctx context.Context) (string, error) {
    commandCtx,cancel:=context.WithTimeout(ctx,4*time.Second)
    defer cancel()
    cmd:=exec.CommandContext(commandCtx,"gh","auth","token","--hostname","github.com")
    output,err:=cmd.Output()
    if err!=nil{return "",errors.New("GitHub CLI is not installed or is not authenticated")}
    token:=strings.TrimSpace(string(output))
    if token==""||strings.ContainsAny(token,"\r\n\t ") {
        return "",errors.New("GitHub CLI returned an invalid token")
    }
    return token,nil
}
