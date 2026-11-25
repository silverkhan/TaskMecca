package runtimeobs

import (
	"errors"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"testing"
	"time"
)

func TestA102OperationalControllerRequiresExactCurrentSourceClaim(t *testing.T) {
	r := T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "fixture-controller", ThreadID: "fixture-t3-controller", RunID: "fixture-current-run", NativeAttemptID: "fixture-current-episode", State: StateRunning, StartedAt: "2026-11-25T00:00:00Z", ObservedAt: "2026-11-25T00:01:00Z"}
	p := &rolepolicy.Policy{Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller", NativeID: r.SessionID}}
	a := Attempt{AttemptID: "fixture-runtime", AgentPath: "/root/controller", RuntimeAgentID: r.SessionID, SessionID: r.SessionID, Provider: "codex", TurnID: r.NativeAttemptID, LastObservedAt: r.ObservedAt, BindingState: BindingBound, CurrentState: StateRunning, NativeReceipt: &r, StateEvidenceSource: EvidenceT3, StateObservationQuality: QualityObserved}
	h := handoffjournal.Handoff{TaskID: "FIXTURE-SOURCE", ClaimedBy: "/root/controller", ClaimedAttemptID: a.AttemptID, ClaimedAt: r.ObservedAt, Target: handoffjournal.Target{AgentPath: "/root/controller", RuntimeAgentID: r.SessionID}}
	if !sourceControllerClaim("FIXTURE-SOURCE", r.SessionID, p, r, []Attempt{a}, map[string]handoffjournal.Handoff{"fixture-handoff": h}) {
		t.Fatal("legitimate task-empty operational Controller claim rejected")
	}
	for _, kind := range []string{"wrong_policy_native", "wrong_task", "stale_claim", "stale_native_episode", "worker_role", "terminal", "applied", "missing_claim"} {
		t.Run(kind, func(t *testing.T) {
			aa, hh, pp := a, h, *p
			rr := r
			claims := map[string]handoffjournal.Handoff{}
			switch kind {
			case "wrong_policy_native":
				pp.Operational.NativeID = "fixture-foreign"
			case "wrong_task":
				hh.TaskID = "FIXTURE-OTHER"
			case "stale_claim":
				hh.ClaimedAttemptID = "fixture-old-runtime"
			case "stale_native_episode":
				rr.NativeAttemptID = "fixture-new-episode"
			case "worker_role":
				aa.AgentPath = "/root/controller/fixtureworker"
			case "terminal":
				aa.Terminal = true
			case "applied":
				hh.Applied = true
			}
			if kind != "missing_claim" {
				claims["fixture-handoff"] = hh
			}
			if sourceControllerClaim("FIXTURE-SOURCE", r.SessionID, &pp, rr, []Attempt{aa}, claims) {
				t.Fatal("Controller source authority upgraded", kind)
			}
		})
	}
}

func TestA102WorkerHistoricalInitializerCannotAuthorizeNewEpisode(t *testing.T) {
	r := T3Receipt{Origin: "t3_local_projection_v2", Provider: "codex", SessionID: "fixture-worker", ThreadID: "fixture-worker-t3", RunID: "fixture-host-run2", NativeAttemptID: "fixture-native-episode2", State: StateRunning, StartedAt: "2026-11-25T00:00:00Z", ObservedAt: "2026-11-25T00:01:00Z"}
	path := "/root/controller/fixtureworker"
	workspace := t.TempDir()
	p := &rolepolicy.Policy{Method: "worker", Workspace: workspace, Scope: []string{"fixture.go"}, Operational: rolepolicy.Owner{Role: "controller", AgentPath: "/root/controller"}, Implementation: rolepolicy.Owner{Role: "worker", AgentPath: path, NativeID: r.SessionID}}
	a := Attempt{AttemptID: "fixture-runtime2", TaskID: "FIXTURE-SOURCE", AgentPath: path, RuntimeAgentID: r.SessionID, SessionID: r.SessionID, Provider: "codex", TurnID: r.NativeAttemptID, LastObservedAt: r.ObservedAt, BindingState: BindingBound, BindingEvidence: map[string]string{"assignment_id": "fixture-assignment"}, CurrentState: StateRunning, NativeReceipt: &r, StateEvidenceSource: EvidenceT3, StateObservationQuality: QualityObserved}
	assignments := []Assignment{{AssignmentID: "fixture-assignment", TaskID: a.TaskID, AgentPath: path}}
	check := func(aa Attempt, pp *rolepolicy.Policy) bool {
		return sourceWorkerCurrent(a.TaskID, path, r.SessionID, workspace, pp, false, r, []Attempt{aa}, assignments)
	}
	if !check(a, p) {
		t.Fatal("actual current Worker binding rejected")
	}
	for _, kind := range []string{"old_episode", "missing_assignment", "wrong_task", "wrong_policy_owner", "terminal", "controller_role"} {
		t.Run(kind, func(t *testing.T) {
			aa, pp := a, *p
			old := r
			switch kind {
			case "old_episode":
				old.RunID = "fixture-host-run1"
				old.NativeAttemptID = "fixture-native-episode1"
				aa.NativeReceipt = &old
				aa.TurnID = old.NativeAttemptID
			case "missing_assignment":
				aa.BindingEvidence = map[string]string{}
			case "wrong_task":
				aa.TaskID = "FIXTURE-OTHER"
			case "wrong_policy_owner":
				pp.Implementation.NativeID = "fixture-foreign-worker"
			case "terminal":
				aa.Terminal = true
			case "controller_role":
				aa.AgentPath = "/root/controller"
			}
			if check(aa, &pp) {
				t.Fatal("historical/foreign source admitted", kind)
			}
		})
	}
}

func TestA85RegistrarFormalCollectionAndRetry(t *testing.T) {
	in, store, _, now := roleFixture(t, "controller")
	in.Role = "registrar"
	in.AgentPath = "/root/registrar"
	in.AssignmentID = "registrar-assignment"
	if _, err := RecordAssignment(in.CanonicalProject, in.AssignmentID, in.TaskID, in.AgentPath, now); err != nil {
		t.Fatal(err)
	}
	observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		return observeT3Execution(p, s, n, store.read)
	}
	got, err := observeRoleCollection(in, now, store.verifyRoleScope, observe)
	if err != nil || got.Status != "bound" || got.Attempt.AgentPath != "/root/registrar" {
		t.Fatalf("Registrar unsupported: %+v %v", got, err)
	}
	retry, err := observeRoleCollection(in, now, store.verifyRoleScope, observe)
	if err != nil || retry.Attempt.AttemptID != got.Attempt.AttemptID {
		t.Fatalf("retry changed actor: %+v %v", retry, err)
	}
}

func TestA85RegistrarRejectsUnsupportedIdentityAndFailedObservation(t *testing.T) {
	for _, kind := range []string{"wrong_assignment", "wrong_role", "wrong_parent", "wrong_receipt", "observer_failure"} {
		t.Run(kind, func(t *testing.T) {
			in, store, _, now := roleFixture(t, "controller")
			in.Role = "registrar"
			in.AgentPath = "/root/registrar"
			in.AssignmentID = "registrar-assignment"
			if _, err := RecordAssignment(in.CanonicalProject, in.AssignmentID, in.TaskID, in.AgentPath, now); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "wrong_assignment":
				in.AssignmentID = "assignment"
			case "wrong_role":
				in.AgentPath = "/root/controller/naong"
			case "wrong_parent":
				in.ParentThreadID = "foreign-parent"
			}
			observe := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
				if kind == "observer_failure" {
					return Attempt{}, T3Receipt{}, errors.New("fixture collection transport failed")
				}
				a, r, err := observeT3Execution(p, s, n, store.read)
				if kind == "wrong_receipt" {
					r.SessionID = "foreign-native"
				}
				return a, r, err
			}
			result, err := observeRoleCollection(in, now, store.verifyRoleScope, observe)
			if err == nil || result.Status != "collection_gap" || result.Diagnostic == "" {
				t.Fatalf("failure concealed %+v %v", result, err)
			}
			ledger, err := BuildLedger(in.CanonicalProject, 0, now)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range ledger.Attempts {
				if a.BindingState == BindingBound && a.AgentPath == "/root/registrar" {
					t.Fatal("invalid request bound Registrar", a)
				}
			}
		})
	}
}
