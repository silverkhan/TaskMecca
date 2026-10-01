package backlog

import (
    "strings"
    "testing"
)

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


func TestSemanticSectionSummaryIsOptionalAndRemovedFromBody(t *testing.T) {
    text := `# A-37 Semantic Summary

## 핵심 요약
- 목적: 상세 내용을 빠르게 판단한다.

## 요건 정의서

### 배경 및 문제

> 요약: 접힌 본문 도입부 대신 이 섹션 전체의 문제를 설명한다.

첫 번째 배경 문장이다.
두 번째 배경 문장이다.

### 목표

짧고 명확한 목표다.

### 요구사항

> Summary: Long requirements are summarized semantically.
> The summary can continue on the next quote line.

- 첫 번째 요구사항
- 두 번째 요구사항

### 범위

> 요약: 포함·제외 범위의 핵심 경계를 설명한다.

#### 포함
- Web 상세 화면

#### 제외
- 별도 번역 엔진

### 수용 기준

- [ ] 요약 없는 짧은 기준은 바로 보인다.

### 제약 및 보존 조건

-

## 작업 노트

짧은 진행 기록.

## 결과

> 요약: 구현 결과 전체를 한 문장으로 설명한다.

상세 결과 1
상세 결과 2

## 검증

검증은 짧다.
`
    doc:=documentModel(text,parseFields(text))
    summaries,ok:=doc["section_summaries"].(map[string]string)
    if !ok { t.Fatalf("section_summaries type=%T value=%v",doc["section_summaries"],doc["section_summaries"]) }
    if got:=summaries["background"]; got!="접힌 본문 도입부 대신 이 섹션 전체의 문제를 설명한다." {
        t.Fatalf("background summary=%q",got)
    }
    if got:=summaries["requirements"]; got!="Long requirements are summarized semantically. The summary can continue on the next quote line." {
        t.Fatalf("requirements summary=%q",got)
    }
    if got:=summaries["scope"]; got!="포함·제외 범위의 핵심 경계를 설명한다." {
        t.Fatalf("scope summary=%q",got)
    }
    if got:=summaries["acceptance"]; got!="" {
        t.Fatalf("acceptance should not have a forced summary: %q",got)
    }
    if got:=summaries["progress_result"]; got!="구현 결과 전체를 한 문장으로 설명한다." {
        t.Fatalf("progress summary=%q",got)
    }
    if got:=summaries["verification"]; got!="" {
        t.Fatalf("verification should stay unsummarized: %q",got)
    }

    req:=doc["requirements"].(map[string]any)
    background:=req["background"].(string)
    if strings.Contains(background,"> 요약:") || !strings.Contains(background,"첫 번째 배경 문장이다.") {
        t.Fatalf("background body=%q",background)
    }
    if got:=req["goal"].(string); got!="짧고 명확한 목표다." {
        t.Fatalf("goal changed without summary: %q",got)
    }
    if got:=doc["result"].(string); strings.Contains(got,"> 요약:") || !strings.Contains(got,"상세 결과 2") {
        t.Fatalf("result body=%q",got)
    }
    if got:=doc["verification"].(string); got!="검증은 짧다." {
        t.Fatalf("verification body changed=%q",got)
    }
}

func TestSplitSemanticSummaryLeavesOrdinaryBlockquoteAlone(t *testing.T) {
    value := "> 참고: 이것은 요약 표식이 아니다.\n\n본문"
    summary,body:=splitSemanticSummary(value)
    if summary!="" || body!=value {
        t.Fatalf("ordinary blockquote changed: summary=%q body=%q",summary,body)
    }
}
