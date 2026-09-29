package backlog

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDocumentModelAndSearch(t *testing.T) {
    root:=t.TempDir()
    folder:=filepath.Join(root,"_task_mecca","data","backlog")
    if err:=os.MkdirAll(folder,0755); err!=nil { t.Fatal(err) }
    text:=`# A-1 Searchable Task

## 작업 개요
- 등록자: test
- Agent: /root/controller/pairi
- 변경범위: internal/backlog
- 선행: -
- 연관: -
- 설명: Go parser migration

## 작업 정의
### 목표
Port the backlog parser.
### 수용 기준
- [x] Search can find this task.
- [ ] Keep JSON parity.

## 실행 정보
- RuntimeProvider: codex
- Dispatch상태: completed
- 실행근거: test
- Fallback근거: -

## 작업 노트
- 대기: -
- 대기유형: -
- 재개조건: -
- 대기근거: -
- 메모: parser note

## 결과
Parser complete.

## 검증
Search parity.
`
    path:=filepath.Join(folder,"000001.A-1.searchable.todo.md")
    if err:=os.WriteFile(path,[]byte(text),0644); err!=nil { t.Fatal(err) }

    rows,err:=Catalog(root,"")
    if err!=nil { t.Fatal(err) }
    if len(rows)!=1 { t.Fatalf("rows=%d",len(rows)) }
    row:=rows[0]
    if row.Document["schema"]!="simple-v2" { t.Fatalf("document=%+v",row.Document) }
    if row.Fields["설명"]!="Go parser migration" { t.Fatalf("fields=%+v",row.Fields) }
    if row.RuntimeMetadata["runtime_provider"]!="codex" { t.Fatalf("runtime=%+v",row.RuntimeMetadata) }

    report,err:=Search(root,"","parser",10)
    if err!=nil { t.Fatal(err) }
    if len(report.Results)!=1 || report.Results[0].ID!="A-1" {
        t.Fatalf("report=%+v",report)
    }
    if report.Results[0].Score<=0 { t.Fatalf("score=%d",report.Results[0].Score) }
}
