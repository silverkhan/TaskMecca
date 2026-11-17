package runtimeobs

import (
	"errors"
	"testing"
	"time"
)

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
