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
