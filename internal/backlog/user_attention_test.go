package backlog

import (
	"os"
	"path/filepath"
	"testing"
)

const policyWaitFixture = "# A-14 텔레그램 알림에 프로젝트명 표시 및 백로그 ID 중복 제거\n## 작업 정의\n### 목표\n알림 표시 개선\n## 작업 노트\n- 대기: 같은 작업 ID가 든 프로젝트명과 링크 또는 여러 링크를 모두 보존할 때 평문 표시 1회 기준과 충돌하는 실제 입력의 처리 정책 확인 필요.\n- 대기유형: user\n- 재개조건: 프로젝트명·링크 ID를 표시 중복 예외로 둘지, 링크 표시 방식 확장을 허용할지 사용자 확정 후 계약과 대조한다.\n- 대기근거: Controller가 Root에 구체적인 충돌 예시와 선택지를 직접 전달했다.\n"

func TestCanonicalUserHoldRemainsVisibleWithoutHoldReview(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	writeWebTask(t, folder, "000014.A-14.policy.hold.md", policyWaitFixture)
	rows, err := Catalog(project, folder)
	if err != nil {
		t.Fatal(err)
	}
	row := rows[0]
	reason := userReason(row, nil, nil)
	item := webSummaryItem(row, row.State, nil, reason, nil)
	reason, _ = item["attention_reason"].(map[string]any)
	if item["state"] != "needs_user" || reason["type"] != "user_intervention" || reason["message"] != row.Fields["대기"] || reason["resume_condition"] != row.Fields["재개조건"] {
		t.Fatalf("canonical user hold lost with nil hold_review: %v", item)
	}
	// A stale runtime/review view cannot overwrite the current canonical policy.
	item = webSummaryItem(row, row.State, nil, userReason(row, map[string]any{"wait_kind": "external", "wait_note": "old wait"}, map[string]any{"health": "stale"}), map[string]any{"health": "stale"})
	reason = item["attention_reason"].(map[string]any)
	if reason["type"] != "user_intervention" || reason["message"] != row.Fields["대기"] {
		t.Fatalf("canonical policy overwritten: %v", reason)
	}
}

func TestUserAttentionDoesNotInferWaitKindOrIncludeHistory(t *testing.T) {
	row := Record{ID: "A-14", State: "hold", Location: "active", Fields: map[string]string{"대기": "사용자 정책 판단이 필요하다는 자유 서술"}}
	if reason, _ := canonicalOperationalState(row, nil, nil); len(reason) != 0 {
		t.Fatalf("inferred user wait from prose: %v", reason)
	}
	row.Fields["대기유형"] = "external"
	if reason := userReason(row, map[string]any{"wait_kind": "user"}, nil); len(reason) != 0 {
		t.Fatalf("stale review overrode external wait: %v", reason)
	}
	row.Fields["대기유형"] = "user"
	for _, state := range []string{"done", "todo"} {
		row.State = state
		if reason := userReason(row, nil, map[string]any{"health": "needs_user"}); len(reason) != 0 {
			t.Fatalf("resolved/inactive user wait retained: %v", reason)
		}
	}
	row.State = "hold"
	row.Location = "archive"
	if reason := userReason(row, nil, map[string]any{"health": "needs_user"}); len(reason) != 0 {
		t.Fatalf("archive retained attention: %v", reason)
	}
}

func TestPolicyUserHoldAcrossAPIsAndResolution(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	file := writeWebTask(t, folder, "000014.A-14.policy.hold.md", policyWaitFixture)
	snapshot, err := AttentionSnapshot(project, folder, true)
	if err != nil {
		t.Fatal(err)
	}
	attention := snapshot["attention"].([]map[string]any)
	if len(attention) != 1 || attention[0]["id"] != "A-14" || attention[0]["type"] != "user_intervention" || toString(attention[0]["resume_condition"]) == "" {
		t.Fatalf("policy user hold missing: %v", snapshot)
	}
	detail, err := TaskDetail(project, folder, "A-14")
	if err != nil {
		t.Fatal(err)
	}
	if detail["state"] != "needs_user" {
		t.Fatalf("detail=%v", detail)
	}
	list, err := BacklogPage(project, folder, 1, 20, []string{"todo"}, nil, "no match", "id_desc")
	if err != nil {
		t.Fatal(err)
	}
	if len(list["items"].([]map[string]any)) != 0 || len(list["attention"].([]map[string]any)) != 1 {
		t.Fatal("global user attention should survive task filters")
	}
	dashboard, err := DashboardSnapshot(project, folder, 5)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard["all_items"].(map[string]map[string]any)["A-14"]["state"] != "needs_user" {
		t.Fatal("dashboard did not use canonical user hold")
	}
	for _, target := range []string{"todo", "done"} {
		next := filepath.Join(folder, "000014.A-14.policy."+target+".md")
		if err := os.Rename(file, next); err != nil {
			t.Fatal(err)
		}
		file = next
		resolved, err := AttentionSnapshot(project, folder, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(resolved["attention"].([]map[string]any)) != 0 {
			t.Fatalf("resolved warning remained: %v", resolved)
		}
	}
}

func userReason(row Record, review, signal map[string]any) map[string]any {
	reason, _ := canonicalOperationalState(row, review, signal)
	return reason
}
