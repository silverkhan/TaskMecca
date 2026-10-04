//go:build darwin

package webui

import (
    "strings"
    "testing"
)

func TestLaunchdRollbackScriptRestoresAndKickstarts(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME",t.TempDir())
    script:=launchdRollbackScript("/Applications/Task Mecca/task-mecca","/Applications/Task Mecca/task-mecca.previous",18765)
    for _,needle:=range []string{"/api/health","/bin/cp ","upgrade-recovery.json","rolled_back","launchctl kickstart -k","com.taskmecca.web"} {
        if !strings.Contains(script,needle) { t.Fatalf("macOS rollback helper missing %q",needle) }
    }
}
