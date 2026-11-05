package handoff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandoffClaimKeepsLegacyCheckedFingerprintWithoutRewritingIt(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "session", "native", runtimeobs.StateRunning, now)
	path := filepath.Join(project, "_task_mecca", "data", "backlog", "000001.A-1.handoff-test.todo.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checked := strings.Replace(string(raw), "- [ ]", "- [x]", 1)
	if err = os.WriteFile(path, []byte(checked), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(contractscope.RawSection(checked)))
	legacy := hex.EncodeToString(sum[:])
	p, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventRegistrationReady, SourceAgentPath: "/root/registrar", TargetAgentPath: "/root/controller", TargetAttemptID: "controller", ExecutionAuthorized: true, ExpectedContractSHA256: legacy}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	claim, err := Claim(project, p.HandoffID, "/root/controller", "controller", now.Add(2*time.Second))
	if err != nil || !claim.Claimed || claim.ContractChanged || claim.ContractCompatibility != "acceptance_check_state_only" {
		t.Fatalf("legacy handoff falsely changed: %+v %v", claim, err)
	}
	h, ok, err := Inspect(project, p.HandoffID, now.Add(3*time.Second))
	if err != nil || !ok || h.ContractSHA256 != legacy {
		t.Fatal("legacy fingerprint overwritten")
	}
}

func TestHandoffClaimCLIDetectsReviewReportingOmission(t *testing.T) {
	project := t.TempDir()
	writeSimpleTask(t, project, "A-1")
	now := time.Now().UTC()
	appendControllerAttempt(t, project, "controller", "session", "native", runtimeobs.StateRunning, now)
	prepared, err := Prepare(PrepareRequest{Project: project, TaskID: "A-1", EventType: EventCompletionReport, SourceAgentPath: "/root/controller/pairi", SourceAttemptID: "worker", TargetAgentPath: "/root/controller", TargetAttemptID: "controller"}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	code := RunCLI([]string{"claim", prepared.HandoffID, "--project", project, "--as", "/root/controller", "--source-attempt", "controller", "--json"}, &out, &errs)
	if code != 0 {
		t.Fatalf("claim failed: %s", errs.String())
	}
	var result ClaimResult
	if err = json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Claimed || result.ActivityReporting == nil || result.ActivityReporting.Status != "not_recorded" || result.ActivityReporting.RequiredPhase != "review_started" {
		t.Fatalf("CLI omission guard missing: %s", out.String())
	}
}
