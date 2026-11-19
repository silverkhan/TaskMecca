package rolepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestA90RoutineMetadataCannotWeakenConfirmedReviewContract(t *testing.T) {
	project := t.TempDir()
	p := workerPolicy("/source")
	p.Implementation.NativeID = "implementation-native"
	r := a90Requirement()
	r.Scope = p.Scope
	r.Risk.ExplicitRequired = true
	p.ReviewRequirement = &r
	before, _ := json.Marshal(p)
	raw := "# A-78 contract\n\n## 요건 정의서\n" + Fence + string(before) + "\n```\n\n## 실행 정보\n"
	sum := sha256.Sum256([]byte(DecisionScopeRaw(raw)))
	base := hex.EncodeToString(sum[:])
	next := *p
	nextRisk := r
	nextRisk.Risk.ExplicitRequired = false
	nextRisk.Omit = true
	nextRisk.OmissionReason = "routine speed"
	next.ReviewRequirement = &nextRisk
	metadata, _ := json.Marshal(next)
	dir := filepath.Join(project, "_task_mecca/data/contract_decisions/events")
	os.MkdirAll(dir, 0700)
	write := func(id string, event any) {
		b, _ := json.Marshal(event)
		os.WriteFile(filepath.Join(dir, id+".json"), b, 0600)
	}
	write("1", map[string]any{"event_id": "1", "task_id": "A-78", "kind": "worker_metadata", "actor": "/root/controller", "worker_metadata": json.RawMessage(metadata), "base_sha256": base, "evidence": "actual runtime binding", "recorded_at": "1"})
	if _, _, err := Load(project, "A-78", raw); err == nil || !strings.Contains(err.Error(), "routine_metadata") {
		t.Fatal("routine facts weakened explicit contract")
	}
	write("2", map[string]any{"event_id": "2", "task_id": "A-78", "kind": "supplement", "actor": "/root/controller", "text": Fence + string(metadata) + "\n```", "base_sha256": base, "user_confirmed": true, "recorded_at": "2"})
	loaded, _, err := Load(project, "A-78", raw)
	if err != nil || loaded.ReviewRequirement == nil || !loaded.ReviewRequirement.Omit {
		t.Fatal("actual confirmed current-contract change could not replace policy", loaded, err)
	}
	p.Method = "controller_exception"
	if ValidateRequirementTransition(p, &next) == nil {
		t.Fatal("routine metadata replaced Controller exception")
	}
}

func TestA90ReviewerReadOnlyIndependenceAndSource(t *testing.T) {
	i := workerIdentity()
	r := i
	r.NativeID = "review-native"
	r.RuntimeAgentID = "review-native"
	r.AttemptID = "review-run"
	r.AgentPath = "/root/controller/reviewer/raichyu"
	p := &Policy{SchemaVersion: 1, Method: "worker", Workspace: "/candidate", Scope: []string{"product.go"}, Implementation: Owner{Role: "worker", AgentPath: i.AgentPath, NativeID: i.NativeID}, Operational: Owner{Role: "controller", AgentPath: "/root/controller"}, Reviewer: Owner{Role: "reviewer", AgentPath: r.AgentPath, NativeID: r.NativeID}}
	source := Source{Files: map[string]string{"product.go": strings.Repeat("a", 64)}}
	if !EvaluateReviewer(p, r, p.Workspace, source, []Identity{i}).Allowed {
		t.Fatal("formal reviewer rejected")
	}
	if Evaluate(p, r, p.Workspace, false).Allowed {
		t.Fatal("reviewer granted implementation")
	}
	if EvaluateReviewer(p, r, p.Workspace, Source{}, []Identity{i}).Allowed {
		t.Fatal("unprepared source")
	}
	previous := r
	previous.AgentPath = i.AgentPath
	previous.AttemptID = "old-implementation-turn"
	if EvaluateReviewer(p, r, p.Workspace, source, []Identity{i, previous}).Allowed {
		t.Fatal("role rename erased implementation history")
	}
	statement := ReviewStatement{TaskID: i.TaskID, ReviewerAttemptID: r.AttemptID, ImplementationNativeID: i.NativeID, Source: source, ReportSHA256: "report", Result: "accepted"}
	if _, err := ValidateReview(i.TaskID, i, r, p, statement, "report", source); err != nil {
		t.Fatal(err)
	}
	changed := Source{Files: map[string]string{"product.go": strings.Repeat("b", 64)}}
	if _, err := ValidateReview(i.TaskID, i, r, p, statement, "report", changed); err == nil {
		t.Fatal("old accepted copied to changed source")
	}
	r.NativeID = i.NativeID
	r.AttemptID = "another-turn"
	if _, err := ValidateReview(i.TaskID, i, r, p, statement, "report", source); err == nil {
		t.Fatal("same native different turn")
	}
}

func TestA90TypedOptionalAndLegacyMetadata(t *testing.T) {
	p := Policy{SchemaVersion: 1, Method: "worker", Workspace: "/candidate", Scope: []string{"product.go"}, Implementation: Owner{Role: "worker", AgentPath: "/root/controller/isanghaessi", NativeID: "native"}, Operational: Owner{Role: "controller", AgentPath: "/root/controller"}}
	r := a90Requirement()
	r.Omit = true
	r.OmissionReason = "Controller verified existing tests"
	p.ReviewRequirement = &r
	check := func(want bool) {
		t.Helper()
		raw, _ := json.Marshal(p)
		_, err := ParseWorkerMetadata(raw)
		if (err == nil) != want {
			t.Fatal(string(raw), err)
		}
	}
	check(true)
	r.Risk.ExplicitRequired = true
	check(false)
	r.Risk.ExplicitRequired = false
	r.Scope = []string{"other.go"}
	check(false)
	p.ReviewRequirement = nil
	p.Reviewer = Owner{Role: "worker", AgentPath: "/root/controller/raichyu"}
	check(true)
	p.Reviewer = Owner{Role: "reviewer", AgentPath: "/root/controller/reviewer/raichyu"}
	check(true)
	for path, role := range map[string]string{"/root/controller/raichyu": "worker", "/root/controller/reviewer/raichyu": "reviewer", "/root/controller/reviewer": "unknown", "/root/controller/reviewer/": "unknown"} {
		if Role(path) != role {
			t.Fatal(path, Role(path))
		}
	}
}
