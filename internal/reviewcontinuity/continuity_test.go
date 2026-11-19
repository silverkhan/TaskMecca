package reviewcontinuity

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/contextlife"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func save(t *testing.T, root, path string, value any) Reference {
	t.Helper()
	b, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, path), b, 0600); e != nil {
		t.Fatal(e)
	}
	return Reference{path, hash(b)}
}
func fixture(t *testing.T) (string, Input) {
	t.Helper()
	root := t.TempDir()
	if b, e := exec.Command("git", "init", "-q", root).CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	body := []byte("package candidate\n")
	if e := os.WriteFile(filepath.Join(root, "product.go"), body, 0600); e != nil {
		t.Fatal(e)
	}
	impl := rolepolicy.Identity{AttemptID: "run-worker", NativeID: "native-worker", RuntimeAgentID: "native-worker", Provider: "codex", AgentPath: "/root/controller/worker", TaskID: "A-1", Bound: true, Observed: true}
	rev := rolepolicy.Identity{AttemptID: "run-reviewer", NativeID: "native-reviewer", RuntimeAgentID: "native-reviewer", Provider: "codex", AgentPath: "/root/controller/reviewer/raichyu", TaskID: "A-1", Bound: true, Observed: true}
	round := 3
	r := rolepolicy.ReviewRequest{TaskID: "A-1", AssignmentID: "assignment-current", ContractSHA256: strings.Repeat("a", 64), Implementation: impl, Reviewer: rev, Source: rolepolicy.Source{Files: map[string]string{"product.go": hash(body)}}, Requirement: rolepolicy.ReviewRequirement{Risk: rolepolicy.ReviewRisk{Impact: "high", Complexity: "complex", Reversibility: "easy", Verifiability: "established", ExplicitRequired: true}, Scope: []string{"product.go"}, Rationale: "explicit contract"}}
	in := Input{Current: Snapshot{Request: r, ChainID: "chain-task-source", ReviewRound: &round, SessionGeneration: 11, State: "needs_changes", Findings: []Finding{{ID: "f1", Text: "prior unresolved defect"}}, Errors: []string{"original native error"}, Validation: []string{"selected fixture only"}, Remaining: []string{"independent review, acceptance, dev/CI"}}, ImplementationHistory: []rolepolicy.Identity{impl}, Host: Host{Kind: "t3"}}
	report := save(t, root, "report.json", map[string]string{"result": "needs_changes", "error": "original native error"})
	in.Current.Report = report
	in.Current.Request.ReportFile = report.Path
	cp, e := BuildCheckpoint(root, []Snapshot{in.Current}, contextlife.Checkpoint{Role: "reviewer:raichyu", OwnerAttempt: rev.AttemptID, Fingerprint: "actual-clm-fingerprint"})
	if e != nil {
		t.Fatal(e)
	}
	in.Continuity = save(t, root, "checkpoint.json", cp)
	in.Current.Request.ContinuityRef = in.Continuity.Path
	in.ObservationRef = save(t, root, "observation.json", Observation{AttemptID: rev.AttemptID, NativeID: rev.NativeID, State: "reviewing", NativeState: "running", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	pct := 30.0
	s := contextlife.StatusReport{Eligible: true, State: contextlife.State{Role: "reviewer:raichyu", Generation: 11, OwnerAttempt: rev.AttemptID, Observation: &contextlife.Observation{Provider: "codex", AttemptID: rev.AttemptID, UsagePercent: &pct, WindowTokens: 100, Trust: "provider_verified", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}}, Decision: contextlife.Decision{Action: "continue", ContextStatus: "measured"}}
	in.ContextRef = save(t, root, "context.json", s)
	in.Current.Request.ContextRef = in.ContextRef.Path
	return root, in
}
func TestT3ContinuityPreservesHistoryWithoutReusingAcceptance(t *testing.T) {
	root, in := fixture(t)
	old := in.Current
	in.Current.Request.Reviewer.AttemptID = "run-successor"
	in.Current.Request.Reviewer.NativeID = "native-successor"
	in.Current.Request.Reviewer.RuntimeAgentID = "native-successor"
	in.Current.SessionGeneration = 12
	in.ObservationRef = Reference{}
	in.ContextRef = Reference{}
	in.Current.Request.ContextRef = ""
	got, e := Assess(root, in)
	if e != nil || got.Mode != "new_delegate_task" || got.ActualState != "unknown" || got.ContextStatus != "unknown" || !got.PurposeVerified {
		t.Fatal(got, e)
	}
	if got.ReviewRound == nil || *got.ReviewRound != 3 || got.SessionGeneration != nil || len(got.Previous) != 1 || !reflect.DeepEqual(got.Previous[0].Errors, old.Errors) || got.Previous[0].Request.Reviewer.NativeID == got.NativeID {
		t.Fatal("history/round/native distinction lost", got)
	}
	b := []byte("package candidate\nfunc Changed(){}\n")
	os.WriteFile(filepath.Join(root, "product.go"), b, 0600)
	in.Current.Request.Source.Files = map[string]string{"product.go": hash(b)}
	got, e = Assess(root, in)
	if e != nil || len(got.SourceDelta) != 1 || got.SourceDelta[0].PreviousSHA256 == got.SourceDelta[0].CurrentSHA256 {
		t.Fatal(got, e)
	}
}
func TestHostReuseFailsClosed(t *testing.T) {
	for _, kind := range []string{"t3", "unknown", "codex", "claude"} {
		t.Run(kind, func(t *testing.T) {
			root, in := fixture(t)
			in.Host = Host{Kind: kind, ReuseSupport: "supported", PolicyAllowsReuse: true}
			got, e := Assess(root, in)
			if e != nil {
				t.Fatal(e)
			}
			want := "reuse_session"
			if kind == "t3" {
				want = "new_delegate_task"
			}
			if kind == "unknown" {
				want = "new_session"
			}
			if got.Mode != want {
				t.Fatal(got)
			}
		})
	}
	for _, change := range []func(*Input){func(i *Input) { i.Host.ReuseSupport = "unknown" }, func(i *Input) { i.Host.PolicyAllowsReuse = false }, func(i *Input) { i.ContextRef = Reference{}; i.Current.Request.ContextRef = "" }, func(i *Input) {
		i.Current.Request.Reviewer.AttemptID = "different-attempt"
		i.ObservationRef = Reference{}
		i.ContextRef = Reference{}
		i.Current.Request.ContextRef = ""
	}} {
		root, in := fixture(t)
		in.Host = Host{Kind: "codex", ReuseSupport: "supported", PolicyAllowsReuse: true}
		change(&in)
		got, e := Assess(root, in)
		if e != nil {
			t.Fatal(e)
		}
		if got.Mode == "reuse_session" {
			t.Fatal("unverified reuse", got)
		}
	}
}
func TestIdentitySourceAndReferenceConflicts(t *testing.T) {
	for name, mutate := range map[string]func(*Input){"self history": func(i *Input) { i.ImplementationHistory = append(i.ImplementationHistory, i.Current.Request.Reviewer) }, "busy": func(i *Input) { i.ConflictingAttempts = []string{"another-task"} }, "contract": func(i *Input) { i.Current.Request.ContractSHA256 = "unknown" }, "wrong chain": func(i *Input) { i.Current.ChainID = "alias-only" }, "source mismatch": func(i *Input) { i.Current.Request.Source.Files["product.go"] = strings.Repeat("b", 64) }, "ref mismatch": func(i *Input) { i.Continuity.SHA256 = strings.Repeat("b", 64) }, "observation mismatch": func(i *Input) { i.Current.Request.Reviewer.NativeID = "other" }} {
		t.Run(name, func(t *testing.T) {
			root, in := fixture(t)
			mutate(&in)
			got, e := Assess(root, in)
			if e == nil || got.Mode != "blocked" {
				t.Fatal(got, e)
			}
		})
	}
}
func TestCheckpointTamperRetryAndUnsafeReference(t *testing.T) {
	root, in := fixture(t)
	original, e := ReadCheckpoint(root, in.Continuity)
	if e != nil {
		t.Fatal(e)
	}
	original.Snapshots[0].Errors = nil
	changed := save(t, root, "tamper.json", original)
	if _, e = ReadCheckpoint(root, changed); e == nil {
		t.Fatal("changed original errors accepted")
	}
	if _, e = ReadCheckpoint(root, in.Continuity); e != nil {
		t.Fatal("failed retry destroyed original", e)
	}
	if _, e = ReadReference(root, Reference{Path: "../report.json", SHA256: in.Continuity.SHA256}); e == nil {
		t.Fatal("unsafe reference accepted")
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "file"), []byte("outside"), 0600)
	os.Symlink(filepath.Join(outside, "file"), filepath.Join(root, "escape"))
	if _, e = ReadReference(root, Reference{Path: "escape", SHA256: hash([]byte("outside"))}); e == nil {
		t.Fatal("symlink escape accepted")
	}
}
func TestUnknownAndThresholdContextCannotReuse(t *testing.T) {
	for _, pct := range []float64{65, 80, 90} {
		root, in := fixture(t)
		in.Host = Host{Kind: "codex", ReuseSupport: "supported", PolicyAllowsReuse: true}
		c := contextlife.StatusReport{Eligible: true, State: contextlife.State{Role: "reviewer:raichyu", Generation: 11, OwnerAttempt: in.Current.Request.Reviewer.AttemptID, Observation: &contextlife.Observation{Provider: "codex", AttemptID: in.Current.Request.Reviewer.AttemptID, Trust: "provider_verified", WindowTokens: 100, UsagePercent: &pct, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}}, Decision: contextlife.Decision{Action: "continue", ContextStatus: "measured"}}
		in.ContextRef = save(t, root, "context.json", c)
		got, e := Assess(root, in)
		if got.Mode == "reuse_session" {
			t.Fatal(got, e)
		}
		if pct < 90 && e != nil {
			t.Fatal(e)
		}
	}
}
