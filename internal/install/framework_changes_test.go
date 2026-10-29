package install

import (
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
)

func TestFrameworkChangesRequiredIgnoresVersionOnlyDrift(t *testing.T) {
 project:=t.TempDir()
 if err:=Init(project,"0.3.0-test");err!=nil {t.Fatal(err)}
 needed,err:=FrameworkChangesRequired(project)
 if err!=nil||needed {t.Fatalf("new framework: needed=%v err=%v",needed,err)}
 path:=filepath.Join(project,"_task_mecca","manifest.json")
 var m manifest
 raw,err:=os.ReadFile(path);if err!=nil {t.Fatal(err)}
 if err:=json.Unmarshal(raw,&m);err!=nil {t.Fatal(err)}
 m.Version="0.3.999-test"
 updated,err:=json.Marshal(m);if err!=nil {t.Fatal(err)}
 if err:=os.WriteFile(path,updated,0600);err!=nil {t.Fatal(err)}
 needed,err=FrameworkChangesRequired(project)
 if err!=nil||needed {t.Fatalf("version-only: needed=%v err=%v",needed,err)}
}

func TestFrameworkChangesRequiredMissingManagedFile(t *testing.T) {
 project:=t.TempDir()
 if err:=Init(project,"0.3.0-test");err!=nil {t.Fatal(err)}
 if err:=os.Remove(filepath.Join(project,"_task_mecca","ROOT_PROMPT.md"));err!=nil {t.Fatal(err)}
 needed,err:=FrameworkChangesRequired(project)
 if err!=nil||!needed {t.Fatalf("missing file: needed=%v err=%v",needed,err)}
}

func TestFrameworkChangesRequiredMissingManifest(t *testing.T) {
 project:=t.TempDir()
 if err:=os.MkdirAll(filepath.Join(project,"_task_mecca"),0700);err!=nil {t.Fatal(err)}
 needed,err:=FrameworkChangesRequired(project)
 if err!=nil||!needed {t.Fatalf("missing manifest: needed=%v err=%v",needed,err)}
}
