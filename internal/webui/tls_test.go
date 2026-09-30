package webui

import (
    "os"
    "path/filepath"
    "testing"
)

func TestTailscaleCLIPathUsesExplicitOverride(t *testing.T) {
    dir:=t.TempDir()
    path:=filepath.Join(dir,"tailscale-test")
    if err:=os.WriteFile(path,[]byte("test"),0755); err!=nil {
        t.Fatal(err)
    }
    t.Setenv("TASK_MECCA_TAILSCALE_CLI",path)

    got,err:=tailscaleCLIPath()
    if err!=nil {
        t.Fatal(err)
    }
    if got!=path {
        t.Fatalf("tailscaleCLIPath=%q want %q",got,path)
    }
}

func TestTailscaleCLIPathRejectsInvalidOverride(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"missing-tailscale")
    t.Setenv("TASK_MECCA_TAILSCALE_CLI",path)

    if _,err:=tailscaleCLIPath(); err==nil {
        t.Fatal("expected invalid override error")
    }
}
