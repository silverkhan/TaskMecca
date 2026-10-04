//go:build windows

package webui

import "testing"

func TestWindowsRollbackScriptStopsFailedProcessBeforeRestore(t *testing.T) {
    script:=windowsRollbackScript(`C:\Program Files\Task Mecca\task-mecca.exe`,[]string{"web","--port","18765"},`C:\Work\My Project`,`C:\Program Files\Task Mecca\task-mecca.exe.previous`,18765)
    for _,needle:=range []string{"Start-Process -FilePath $exe","-PassThru","/api/health","Stop-Process -Id $p.Id -Force","$p.WaitForExit()","Copy-Item -Force","Restore-Previous",".upgrade.state","$handoff -eq 'rolled_back'","catch { Restore-Previous 'Upgraded Web could not start","Start-Process -FilePath $exe"} {
        if !contains(script,needle) { t.Fatalf("Windows rollback helper missing %q",needle) }
    }
}

func TestPowerShellQuoteHandlesApostrophe(t *testing.T) {
    got:=psQuote(`C:\Users\O'Brien\Task Mecca`)
    if got!=`'C:\Users\O''Brien\Task Mecca'` { t.Fatalf("psQuote=%q",got) }
}


