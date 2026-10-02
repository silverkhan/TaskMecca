package webui

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "testing"

    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestRuntimeHookOverviewFlagsOnlyProviderActuallyInUse(t *testing.T) {
    root:=t.TempDir()
    backlogDir:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(backlogDir,0755); err!=nil { t.Fatal(err) }
    body:=[]byte("# A-1 Active Codex work\n\n## 작업 개요\n\n- Agent: /root/controller/kkobugi\n- 변경범위: internal/*\n- 선행: -\n- 연관: -\n\n## 실행 정보\n\n- RuntimeProvider: codex\n- Dispatch상태: running\n- 실행근거: test\n- Fallback근거: -\n")
    if err:=os.WriteFile(filepath.Join(backlogDir,"000001.A-1.active-codex.doing.md"),body,0644); err!=nil { t.Fatal(err) }
    if _,err:=runtimeobs.EnsureHooks(root,"claude"); err!=nil { t.Fatal(err) }

    handler,err:=Handler(root,"","test")
    if err!=nil { t.Fatal(err) }
    req:=httptest.NewRequest(http.MethodGet,"/api/runtime/hooks",nil)
    rec:=httptest.NewRecorder()
    handler.ServeHTTP(rec,req)
    if rec.Code!=http.StatusOK { t.Fatalf("status=%d body=%s",rec.Code,rec.Body.String()) }

    payload:=map[string]any{}
    if err:=json.Unmarshal(rec.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
    rows,ok:=payload["hooks"].([]any)
    if !ok || len(rows)!=2 { t.Fatalf("hooks=%T %+v",payload["hooks"],payload["hooks"]) }
    seen:=map[string]map[string]any{}
    for _,raw:=range rows {
        row,ok:=raw.(map[string]any); if !ok { continue }
        provider,_:=row["provider"].(string)
        seen[provider]=row
    }
    codex:=seen["codex"]
    if codex["in_use"]!=true || codex["configured"]!=false || codex["needs_attention"]!=true {
        t.Fatalf("codex status=%+v",codex)
    }
    if codex["applies_from"]!="new_root_session" { t.Fatalf("codex applies_from=%v",codex["applies_from"]) }
    claude:=seen["claude"]
    if claude["in_use"]!=false || claude["configured"]!=true || claude["needs_attention"]!=false {
        t.Fatalf("claude status=%+v",claude)
    }
}
