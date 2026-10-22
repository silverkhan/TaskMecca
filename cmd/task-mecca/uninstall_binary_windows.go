//go:build windows

package main

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "os"
    "os/exec"
    "strings"
    "syscall"
)

// Windows locks a running executable. Launch a detached PowerShell helper
// that waits for this exact process ID to exit and verifies the executable
// hash before deleting it. A changed file is never removed.
func removeInstalledBinary(path string) (string,error) {
    data,err:=os.ReadFile(path)
    if err!=nil{return "",err}
    digest:=sha256.Sum256(data)
    expected:=hex.EncodeToString(digest[:])
    quote:=func(v string)string{return "'"+strings.ReplaceAll(v,"'","''")+"'"}
    helper,err:=os.CreateTemp("", "task-mecca-uninstall-*.ps1")
    if err!=nil{return "",err}
    helperPath:=helper.Name()
    body:=fmt.Sprintf(`$ErrorActionPreference='Stop'
$target=%s
$expected=%s
$sourcePID=%d
try {
  $gone=$false
  for($i=0; $i -lt 80; $i++) {
    if(-not (Get-Process -Id $sourcePID -ErrorAction SilentlyContinue)){ $gone=$true; break }
    Start-Sleep -Milliseconds 250
  }
  if(-not $gone) { throw 'Task Mecca is still running; executable retained' }
  if(Test-Path -LiteralPath $target -PathType Leaf) {
    $actual=(Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash.ToLowerInvariant()
    if($actual -ne $expected){ throw 'Executable changed; refusing to remove a different version' }
    Remove-Item -LiteralPath $target -Force -ErrorAction Stop
  }
} finally {
  Remove-Item -LiteralPath $PSCommandPath -Force -ErrorAction SilentlyContinue
}
`,quote(path),quote(expected),os.Getpid())
    if _,err=helper.WriteString(body);err!=nil{helper.Close();os.Remove(helperPath);return "",err}
    if err=helper.Close();err!=nil{os.Remove(helperPath);return "",err}
    cmd:=exec.Command("powershell.exe","-NoProfile","-NonInteractive","-ExecutionPolicy","Bypass","-File",helperPath)
    cmd.SysProcAttr=&syscall.SysProcAttr{CreationFlags:0x00000008|0x00000200,HideWindow:true}
    if err=cmd.Start();err!=nil{os.Remove(helperPath);return "",err}
    if err=cmd.Process.Release();err!=nil{return "",err}
    return "scheduled for removal after process exit",nil
}
