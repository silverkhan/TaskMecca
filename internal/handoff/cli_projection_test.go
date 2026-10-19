package handoff

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHandoffReconcileSummaryAndFields(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Date(2026, 10, 19, 7, 0, 0, 0, time.UTC)
	prepared, err := Prepare(PrepareRequest{
		Project: project, TaskID: "A-1", EventType: EventRegistrationReady,
		SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller",
		ExecutionAuthorized: false,
	}, now)
	if err != nil { t.Fatal(err) }

	var out, errout bytes.Buffer
	invoke := func(args ...string) (int, map[string]any, string) {
		t.Helper()
		out.Reset()
		errout.Reset()
		argv := append([]string{"reconcile", "--project", project}, args...)
		code := RunCLI(argv, &out, &errout)
		var value map[string]any
		if code == 0 {
			if err := json.Unmarshal(out.Bytes(), &value); err != nil { t.Fatalf("invalid JSON: %v: %s", err, out.String()) }
		}
		return code, value, errout.String()
	}
	code, legacy, detail := invoke("--json")
	if code != 0 { t.Fatalf("legacy reconcile: %d %s", code, detail) }
	if _, ok := legacy["handoffs"]; !ok { t.Fatal("legacy handoffs missing") }
	if _, ok := legacy["journal_path"]; !ok { t.Fatal("legacy journal path missing") }
	if _, ok := legacy["total_handoffs"]; ok { t.Fatal("legacy payload changed") }

	code, summary, detail := invoke("--summary", "--json")
	if code != 0 { t.Fatalf("summary: %d %s", code, detail) }
	if got := summary["total_handoffs"]; got != float64(1) { t.Fatalf("total_handoffs=%v", got) }
	if got := summary["unclaimed_pending_count"]; got != float64(1) { t.Fatalf("unclaimed_pending_count=%v", got) }
	if _, ok := summary["handoffs"]; ok { t.Fatal("summary exposed handoff details") }
	if _, ok := summary["journal_path"]; ok { t.Fatal("summary exposed journal path") }
	if _, ok := summary["findings"]; !ok { t.Fatal("summary must retain diagnostic findings") }

	code, filtered, detail := invoke("--summary", "--fields", "total_handoffs,findings_count", "--json")
	if code != 0 { t.Fatalf("filtered summary: %d %s", code, detail) }
	if len(filtered) != 2 || filtered["total_handoffs"] != float64(1) || filtered["findings_count"] != float64(1) {
		t.Fatalf("filtered=%v", filtered)
	}
	code, fullFields, detail := invoke("--fields", "version,generated_at", "--json")
	if code != 0 { t.Fatalf("full filtered: %d %s", code, detail) }
	if len(fullFields) != 2 || fullFields["generated_at"] == nil { t.Fatalf("fields=%v", fullFields) }

	claim, err := Claim(project, prepared.HandoffID, "/root/controller", "current-controller-attempt", now.Add(time.Second))
	if err != nil || !claim.Claimed { t.Fatalf("claim=%+v err=%v", claim, err) }
	code, claimed, detail := invoke("--summary", "--fields", "claimed_pending_count,unclaimed_pending_count", "--json")
	if code != 0 { t.Fatalf("claimed summary: %d %s", code, detail) }
	if claimed["claimed_pending_count"] != float64(1) || claimed["unclaimed_pending_count"] != float64(0) { t.Fatalf("claimed=%v", claimed) }
	if _, err := Mark(project, prepared.HandoffID, "applied", "ok", "explicit verification", now.Add(2*time.Second)); err != nil { t.Fatal(err) }
	code, applied, detail := invoke("--summary", "--fields", "applied_count,claimed_pending_count", "--json")
	if code != 0 { t.Fatalf("applied summary: %d %s", code, detail) }
	if applied["applied_count"] != float64(1) || applied["claimed_pending_count"] != float64(0) { t.Fatalf("applied=%v", applied) }
}

func TestHandoffReconcileProjectionRefusesInvalidFlags(t *testing.T) {
	project := t.TempDir()
	for _, tc := range []struct {args []string; needle string}{
		{[]string{"reconcile", "--project", project, "--summary", "--fields", "unknown", "--json"}, "unsupported"},
		{[]string{"reconcile", "--project", project, "--summary", "--fields", "total_handoffs,total_handoffs"}, "duplicate"},
		{[]string{"reconcile", "--project", project, "--summary", "--fields", "total_handoffs,"}, "empty"},
		{[]string{"reconcile", "--fields"}, "requires"},
		{[]string{"reconcile", "--fields", ""}, "non-empty"},
		{[]string{"inspect", "--summary", "handoff-any"}, "only"},
		{[]string{"claim", "handoff-any", "--fields", "version"}, "only"},
	} {
		var out, errout bytes.Buffer
		code := RunCLI(tc.args, &out, &errout)
		if code != 2 || !strings.Contains(errout.String(), tc.needle) || out.Len() != 0 {
			t.Errorf("args=%v code=%d stdout=%q stderr=%q", tc.args, code, out.String(), errout.String())
		}
	}
}

func TestHandoffSummaryEmptyLedger(t *testing.T) {
	var out, errout bytes.Buffer
	project := t.TempDir()
	code := RunCLI([]string{"reconcile", "--project", project, "--summary", "--json"}, &out, &errout)
	if code != 0 { t.Fatalf("code=%d stderr=%s", code, errout.String()) }
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil { t.Fatal(err) }
	if got["total_handoffs"] != float64(0) || got["applied_count"] != float64(0) { t.Fatalf("unexpected empty ledger: %v", got) }
}
