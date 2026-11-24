package activity

import (
	"reflect"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA99ReviewPurposePreservesRoleSourceAndControllerConcurrency(t *testing.T) {
	_, at := workflowFixture()
	source := rolepolicy.Source{Files: map[string]string{"source.go": "hash"}}
	v := ReviewerPresentationEntry{AttemptID: "review", NativeID: "review-native", TaskID: "T", StoredRole: "worker", Purpose: "review", PurposeVerified: true, LinkageVerified: true, AssignmentID: "assignment", ContractSHA256: "contract", Source: &source}
	a := a97Attempt("review", "review-native", "/root/controller/pikachyu")
	a.BindingEvidence = map[string]string{"assignment_id": "assignment"}
	e := Event{TaskID: "T", AgentID: a.RuntimeAgentID, RuntimeAttemptID: a.AttemptID, AssignmentID: "assignment", Role: RoleWorker, Type: Other, Action: Start, OccurredAt: at(2), Explanation: "raw other", Provider: a.Provider, SessionID: a.SessionID, ContractSHA256: "contract", TaskRevision: ReviewSourceRevision(source)}
	for _, tc := range []struct {
		name   string
		change func(*Event)
		state  string
	}{
		{"actualWorkerOther", func(*Event) {}, "reviewing"},
		{"explicitReviewTargetedTest", func(e *Event) { e.Type = TargetedTest }, "reviewing"},
		{"missingPurposeProof", func(e *Event) { e.TaskRevision = "" }, "requested"},
		{"oldSource", func(e *Event) { e.TaskRevision = "old" }, "source_unverified"},
		{"foreignTask", func(e *Event) { e.TaskID = "OTHER" }, "requested"},
		{"foreignAttempt", func(e *Event) { e.RuntimeAttemptID = "OTHER" }, "requested"},
		{"foreignNative", func(e *Event) { e.AgentID = "OTHER" }, "requested"},
		{"foreignSession", func(e *Event) { e.SessionID = "OTHER" }, "identity_unverified"},
		{"foreignProvider", func(e *Event) { e.Provider = "OTHER" }, "identity_unverified"},
		{"resultNotAccepted", func(e *Event) { e.Action = Stop }, "result_recorded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := e
			tc.change(&copy)
			before := copy
			p := NewReviewerPresentation(t.TempDir(), "T", []runtimeobs.Attempt{a}, []Event{copy}, nil, map[string]ReviewerPresentationEntry{a.AttemptID: v})
			got := p.ByAttempt[a.AttemptID]
			if got.ActualState != tc.state || copy != before || got.StoredRole != "worker" {
				t.Fatal("purpose inferred or raw event changed", got, copy)
			}
			w := Workflow{TaskID: "T", Current: WorkflowCurrent{Phase: "implementation", ProcessingRole: "worker", ProcessingAgent: a.RuntimeAgentID, NextAction: "implement_and_verify"}, Rows: []WorkflowRow{{Role: "controller", AgentID: "controller-native", Intervals: []WorkflowInterval{{Kind: "activity_open", Type: string(ControllerReview), Start: at(1)}}}}}
			if tc.name == "actualWorkerOther" {
				solo := w
				solo.Rows = nil
				got := ApplyReviewPurpose(solo, p)
				if got.Current.ProcessingRole != "worker" || got.Current.NextOwner != "worker" || got.Current.NextAction != "await_independent_review" {
					t.Fatal("review purpose changed actual role/next owner", got.Current)
				}
			}
			measurement := w.Measurement
			out := ApplyReviewPurpose(w, p)
			if out.Current.Phase != "controller_review" || out.Current.ProcessingAgent != "controller-native" || len(out.Current.IndependentReviews) != 1 || !reflect.DeepEqual(out.Measurement, measurement) {
				t.Fatal("concurrent Controller regressed or measurement changed", out)
			}
		})
	}
}

func TestA99UnknownPurposeNeverRegressesTaskOrPromotesSelfReview(t *testing.T) {
	w := Workflow{TaskID: "T", Current: WorkflowCurrent{Phase: "controller_review", ProcessingRole: "controller", NextAction: "review_report"}}
	p := ReviewerPresentation{ByAttempt: map[string]ReviewerPresentationEntry{"unknown": {TaskID: "T", NativeID: "worker", Purpose: "unknown", ActualState: "unknown"}}}
	got := ApplyReviewPurpose(w, p)
	if got.Current.Phase != w.Current.Phase || got.Current.NextAction != w.Current.NextAction || len(got.Current.IndependentReviews) != 0 {
		t.Fatal(got)
	}
	w.Current = WorkflowCurrent{Phase: "implementation", ProcessingRole: "worker", ProcessingAgent: "unclassified", NextAction: "implement_and_verify"}
	w.Rows = []WorkflowRow{{Role: "worker", AgentID: "unclassified", Intervals: []WorkflowInterval{{Kind: "activity_open", Type: string(Other)}}}, {Role: "controller", AgentID: "controller", Intervals: []WorkflowInterval{{Kind: "activity_open", Type: string(ControllerReview)}}}}
	got = ApplyReviewPurpose(w, p)
	if got.Current.Phase != "controller_review" || got.Current.ProcessingAgent != "controller" || len(got.Current.IndependentReviews) != 0 {
		t.Fatal("unknown Other displaced actual Controller review or became independent review", got.Current)
	}
}
