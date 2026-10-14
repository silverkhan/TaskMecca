package backlog

import (
    "os"
    "os/exec"
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

func TestLifecycleCacheRevisionIgnoresUnrelatedRepoCommit(t *testing.T){
    if _,err:=exec.LookPath("git");err!=nil{t.Skip("git binary missing")}
    repo:=t.TempDir()
    git:=func(args ...string)string{
        t.Helper()
        cmd:=exec.Command("git",args...)
        cmd.Dir=repo
        raw,err:=cmd.CombinedOutput()
        if err!=nil{t.Fatalf("git %v failed: %v %s",args,err,raw)}
        return strings.TrimSpace(string(raw))
    }
    git("init","-q")
    git("config","user.email","test@localhost")
    git("config","user.name","Task Mecca tests")
    task:=filepath.Join(repo,"_task_mecca","data","backlog","000001.B-1.task.todo.md")
    if err:=os.MkdirAll(filepath.Dir(task),0700);err!=nil{t.Fatal(err)}
    if err:=os.WriteFile(task,[]byte("# B-1\n"),0600);err!=nil{t.Fatal(err)}
    git("add",".")
    git("commit","-qm","backlog commit")
    pathspec:=[]string{"_task_mecca/data/backlog"}
    prior:=git("log","-1","--format=%H","--",pathspec[0])
    // No need to recalculate git log -M when only release/source files
    // elsewhere in the same repository change.
    if err:=os.WriteFile(filepath.Join(repo,"README.md"),[]byte("unrelated"),0600);err!=nil{t.Fatal(err)}
    git("add","README.md")
    git("commit","-qm","unrelated source update")
    next:=git("log","-1","--format=%H","--",pathspec[0])
    if prior==""||prior!=next{t.Fatalf("unrelated commit invalidated backlog history: %s -> %s",prior,next)}
}
