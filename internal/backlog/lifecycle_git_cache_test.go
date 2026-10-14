package backlog

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestPersistedLifecycleGitHistoryCacheUsesExactHead(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"cache","lifecycle.json")
    persistLifecycleGitLog(path,"0123456789abcdef", "@@COLLAB@@2026-10-14T12:00:00Z\nA\t000001.B-1.task.todo.md\n")
    stat,err:=os.Lstat(path)
    if err!=nil {t.Fatal(err)}
    if !stat.Mode().IsRegular() || stat.Mode().Perm()&0077!=0 {
        t.Fatalf("Git history cache must remain a private regular file, got %v",stat.Mode())
    }
    content,ok:=loadPersistedLifecycleGitLog(path,"0123456789abcdef")
    if !ok||!strings.Contains(content,"B-1") {t.Fatalf("cache missing: ok=%t output=%q",ok,content)}
    if _,ok:=loadPersistedLifecycleGitLog(path,"different-commit");ok {
        t.Fatal("out-of-date history was returned after HEAD changed")
    }
}

func TestPersistedLifecycleGitHistoryCacheRejectsCorruptionAndSymlink(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"cache.json")
    if err:=os.WriteFile(path,[]byte("{corrupt"),0600);err!=nil {t.Fatal(err)}
    if _,ok:=loadPersistedLifecycleGitLog(path,"HEAD");ok{t.Fatal("invalid JSON accepted")}
    if err:=os.Remove(path);err!=nil {t.Fatal(err)}
    destination:=filepath.Join(t.TempDir(),"outside.json")
    if err:=os.WriteFile(destination,[]byte("{}"),0600);err!=nil {t.Fatal(err)}
    if err:=os.Symlink(destination,path);err!=nil{t.Skipf("symbolic links not supported: %v",err)}
    if _,ok:=loadPersistedLifecycleGitLog(path,"HEAD");ok{t.Fatal("symlink cache accepted")}
    persistLifecycleGitLog(path,"HEAD","untrusted")
    raw,err:=os.ReadFile(destination)
    if err!=nil||string(raw)!="{}" {t.Fatalf("symlink target overwritten: %s %v",raw,err)}
}
