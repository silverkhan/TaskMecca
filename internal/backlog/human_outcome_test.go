package backlog

import "testing"

func TestHumanOutcomeParsedWithoutPromotingTechnicalResult(t *testing.T) {
 raw := "# B-559 Test\n\n## 핵심 결과\n\n- 판정: 확인 필요\n- 성과1: 계산 요청 없이 결과가 표시된다.\n- 남은 사항: 독립 검산 미확인\n\n## 결과\n\ncommit abcdef123 및 job-123 세부 근거\n"
 d:=documentModel(raw,map[string]string{})
 got,ok:=d["result_summary"].(map[string]string)
 if !ok {t.Fatalf("missing outcome: %#v",d["result_summary"])}
 if got["verdict"]!="확인 필요" || got["achievement_1"]!="계산 요청 없이 결과가 표시된다." {t.Fatalf("unexpected outcome: %#v",got)}
 if d["result"]==nil {t.Fatal("detailed result lost")}
}

func TestHumanOutcomeAbsentDoesNotInferCompletion(t *testing.T) {
 raw := "# Task\n\n## 결과\n\n모든 작업 완료. commit 12345\n"
 d:=documentModel(raw,map[string]string{})
 got:=d["result_summary"].(map[string]string)
 if got["verdict"]!="" || got["achievement_1"]!="" {t.Fatalf("fabricated result: %#v",got)}
}
