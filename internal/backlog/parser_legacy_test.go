package backlog

import "testing"

func TestParseFieldsKeepsLegacyRuntimeMetadataOutOfPreviousFields(t *testing.T) {
    text := `# B-311 legacy runtime metadata
- 변경범위: _task_mecca/measurements/B-311/**
- Runtime계약: 1
- 요청Provider: openai
- 요청Model: unknown
- 요청Effort: unknown
- 요청출처: 현재 사용자 대화
- 실효Model: unknown
- 실효Effort: unknown
- RuntimeProvider: openai
- Dispatch상태: completed
- 실행근거: approved
`
    fields:=parseFields(text)
    if got:=fields["변경범위"]; got!="_task_mecca/measurements/B-311/**" {
        t.Fatalf("change scope bled legacy metadata: %q",got)
    }
    if got:=fields["RuntimeProvider"]; got!="openai" {
        t.Fatalf("runtime provider bled legacy metadata: %q",got)
    }
    metadata:=runtimeFromFields(fields)
    if got:=metadata["runtime_provider"]; got!="openai" {
        t.Fatalf("runtime provider=%v",got)
    }
}

func TestRuntimeProviderFallsBackToLegacyProvider(t *testing.T) {
    fields:=map[string]string{"요청Provider":"openai"}
    metadata:=runtimeFromFields(fields)
    if got:=metadata["runtime_provider"]; got!="openai" {
        t.Fatalf("legacy provider fallback=%v",got)
    }
}


func TestHumanSummaryParsesCanonicalLabelsAndContinuation(t *testing.T) {
    text := `# A-24 사람이 읽기 편한 백로그
## 핵심 요약
- 목적: 처음 열었을 때 작업 의미를 빠르게 파악한다.
- 핵심 변경: 목록과 상세를 요약 중심으로 재구성한다.
  상세는 항목별로 선택해서 읽는다.
- 상태·결과: 진행 중이며 Web UI 구현이 남아 있다.
- 확인·후속: 모바일 화면도 확인한다.

## 작업 정의
### 목표
가독성을 높인다.
### 수용 기준
- [ ] 요약이 보인다.
`
    fields:=parseFields(text)
    doc:=documentModel(text,fields)
    summary,ok:=doc["summary"].(map[string]string)
    if !ok { t.Fatalf("summary type=%T value=%v",doc["summary"],doc["summary"]) }
    if got:=summary["purpose"]; got!="처음 열었을 때 작업 의미를 빠르게 파악한다." {
        t.Fatalf("purpose=%q",got)
    }
    if got:=summary["change"]; got!="목록과 상세를 요약 중심으로 재구성한다.\n  상세는 항목별로 선택해서 읽는다." {
        t.Fatalf("change=%q",got)
    }
    if got:=summary["status_result"]; got!="진행 중이며 Web UI 구현이 남아 있다." {
        t.Fatalf("status_result=%q",got)
    }
    if got:=summary["follow_up"]; got!="모바일 화면도 확인한다." {
        t.Fatalf("follow_up=%q",got)
    }
    if present,ok:=doc["summary_present"].(bool); !ok || !present {
        t.Fatalf("summary_present=%v",doc["summary_present"])
    }
}

func TestHumanSummaryIsOptionalForLegacyRecords(t *testing.T) {
    text := "# A-1 Legacy\n- 설명: 기존 설명\n"
    doc:=documentModel(text,parseFields(text))
    summary:=doc["summary"].(map[string]string)
    for key,value:=range summary {
        if value!="" { t.Fatalf("%s=%q",key,value) }
    }
    if present:=doc["summary_present"].(bool); present {
        t.Fatalf("legacy record unexpectedly has human summary")
    }
}
