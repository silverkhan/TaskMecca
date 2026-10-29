package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoveryEventCLIRegistrationReportsPreviouslyStrandedWork(t *testing.T) {
	project := t.TempDir()
	folder := filepath.Join(project, "_task_mecca", "data", "backlog")
	if err := os.MkdirAll(folder, 0755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca", ".runtime", "agents"), 0755); err != nil { t.Fatal(err) }
	for name, body := range map[string]string{
		"000527.B-527.abandoned.doing.md": "# B-527 Abandoned\n- Agent: /root/controller/kkobugi\n- 변경범위: src\n",
		"000550.B-550.new.todo.md": "# B-550 New\n- Agent: -\n- 변경범위: -\n",
	} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0644); err != nil { t.Fatal(err) }
	}
	var out, errs bytes.Buffer
	code := runRecoveryEventCLI([]string{"event", "--event", "registration", "--task", "B-550", "--project", project, "--json"}, &out, &errs)
	if code != 0 { t.Fatalf("exit %d: %s", code, errs.String()) }
	var report struct {
		Count int `json:"candidate_count"`
		ReportToRoot bool `json:"report_to_root"`
		Candidates []struct{ ID string `json:"id"` } `json:"candidates"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil { t.Fatal(err) }
	if report.Count != 1 || !report.ReportToRoot || len(report.Candidates) != 1 || report.Candidates[0].ID != "B-527" {
		t.Fatalf("unexpected registration report: %s", out.String())
	}
}

func TestRecoveryEventCLIRejectsInvalidEventWithoutOutput(t *testing.T) {
	var out, errs bytes.Buffer
	code := runRecoveryEventCLI([]string{"event", "--event", "registration", "--json"}, &out, &errs)
	if code != 2 || out.Len() != 0 { t.Fatalf("expected validation error, exit=%d out=%s err=%s", code, out.String(), errs.String()) }
}
