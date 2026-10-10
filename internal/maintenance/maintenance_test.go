package maintenance

import (
    "os"
    "path/filepath"
    "testing"
)

func TestCompareVersions(t *testing.T) {
    cases:=[]struct{
        latest string
        current string
        want bool
    }{
        {"0.2.10","0.2.9",true},
        {"0.3.0","0.2.99",true},
        {"1.0.0","0.9.9",true},
        {"0.2.9","0.2.9",false},
        {"0.2.8","0.2.9",false},
        {"v0.2.10","0.2.9",true},
        {"0.2.38\\n","0.2.38",false},
        {"0.2.39\\n","0.2.38",true},
        {"0.2.39\\r\\n","0.2.39",false},
    }
    for _,tc:=range cases {
        if got:=newerVersion(tc.latest,tc.current); got!=tc.want {
            t.Fatalf("newerVersion(%q,%q)=%v want %v",tc.latest,tc.current,got,tc.want)
        }
    }
}


func TestNormalizeVersionRepairsLiteralEscapedNewlines(t *testing.T) {
    cases:=map[string]string{
        "0.2.39\\n":"0.2.39",
        "0.2.39\\r\\n":"0.2.39",
        "  v0.2.39\\n  ":"v0.2.39",
        "0.2.39\n":"0.2.39",
    }
    for raw,want:=range cases {
        if got:=normalizeVersion(raw); got!=want {
            t.Fatalf("normalizeVersion(%q)=%q want %q",raw,got,want)
        }
    }
}


func TestFrameworkVersionNormalizesLegacyLiteralNewline(t *testing.T) {
    project:=t.TempDir()
    dir:=filepath.Join(project,"_task_mecca")
    if err:=os.MkdirAll(dir,0755); err!=nil { t.Fatal(err) }
    if err:=os.WriteFile(filepath.Join(dir,"VERSION"),[]byte("0.2.39\\n"),0644); err!=nil { t.Fatal(err) }
    if got:=frameworkVersion(project); got!="0.2.39" {
        t.Fatalf("frameworkVersion=%q",got)
    }
}


func TestPreserveExecutableCreatesRollbackCopy(t *testing.T) {
    dir:=t.TempDir()
    exe:=filepath.Join(dir,"task-mecca")
    want:=[]byte("current-binary")
    if err:=os.WriteFile(exe,want,0755); err!=nil { t.Fatal(err) }
    backup,err:=preserveExecutable(exe)
    if err!=nil { t.Fatal(err) }
    got,err:=os.ReadFile(backup)
    if err!=nil { t.Fatal(err) }
    if string(got)!=string(want) { t.Fatalf("rollback copy=%q want %q",got,want) }
}

func TestValidateUpgradeBinaryRejectsCorruptDownload(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"task-mecca")
    if err:=os.WriteFile(path,[]byte("<html>gateway error</html>"),0755); err!=nil { t.Fatal(err) }
    if err:=validateUpgradeBinary(path); err==nil {
        t.Fatal("corrupt/non-Go upgrade binary must be rejected before replacement")
    }
}
