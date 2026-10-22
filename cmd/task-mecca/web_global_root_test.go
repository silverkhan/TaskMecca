package main

import (
    "path/filepath"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/webui"
)

func TestWebCLIUsesGlobalRootWithoutExplicitProject(t *testing.T) {
    t.Setenv("TASK_MECCA_HOME", filepath.Join(t.TempDir(), "global-home"))
    ordinaryCWD := filepath.Join(t.TempDir(), "someone-else-project")
    global := webui.GlobalWebRoot()
    if got:=webLaunchRoot(ordinaryCWD,false); got!=global {
        t.Fatalf("default Web launch depends on CWD: got %q want %q",got,global)
    }
    if got:=webLaunchRoot(ordinaryCWD,true); got!=ordinaryCWD {
        t.Fatalf("explicit compatibility --project ignored: %q",got)
    }
}
