package runtimeobs

import (
	"encoding/json"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func TestA78NativeReviewRequiresActualFinalAssistantResult(t *testing.T) {
	statement := rolepolicy.ReviewStatement{TaskID: "A-78", ReviewerAttemptID: "review-run", ImplementationNativeID: "worker", ReportSHA256: "exact-report", Result: "accepted", Source: rolepolicy.Source{Files: map[string]string{"source.go": "hash"}}}
	encoded, _ := json.Marshal(statement)
	text := rolepolicy.ReviewMarker + string(encoded)
	fixture := func(id, status, kind, phase, body string) []byte {
		b, _ := json.Marshal(map[string]any{"thread": map[string]any{"id": id, "turns": []any{map[string]any{"status": status, "items": []any{map[string]any{"type": kind, "phase": phase, "text": body}}}}}})
		return b
	}
	good := fixture("reviewer", "completed", "agentMessage", "final_answer", text)
	if s, e := nativeReviewFromThread(good, "reviewer", "A-78", "exact-report", "review-run"); e != nil || s.Result != "accepted" {
		t.Fatal(s, e)
	}
	for _, tc := range []struct{ name, id, status, kind, phase, body, task, report string }{
		{"caller-json", "reviewer", "completed", "userMessage", "final_answer", text, "A-78", "exact-report"},
		{"commentary", "reviewer", "completed", "agentMessage", "commentary", text, "A-78", "exact-report"},
		{"unfinished", "reviewer", "inProgress", "agentMessage", "final_answer", text, "A-78", "exact-report"},
		{"other-native", "someone-else", "completed", "agentMessage", "final_answer", text, "A-78", "exact-report"},
		{"other-task", "reviewer", "completed", "agentMessage", "final_answer", text, "B-621", "exact-report"},
		{"other-report", "reviewer", "completed", "agentMessage", "final_answer", text, "A-78", "imported-json"},
		{"duplicate", "reviewer", "completed", "agentMessage", "final_answer", text + "\n" + text, "A-78", "exact-report"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := nativeReviewFromThread(fixture(tc.id, tc.status, tc.kind, tc.phase, tc.body), "reviewer", tc.task, tc.report, "review-run"); e == nil {
				t.Fatal("unproven review accepted")
			}
		})
	}
}
