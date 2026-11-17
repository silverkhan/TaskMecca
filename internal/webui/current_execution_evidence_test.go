package webui

import (
	stdcontext "context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func currentLoaderFixture(now time.Time) (map[string][]backlog.Record, []runtimeobs.Assignment, runtimeobs.Ledger, *rolepolicy.Policy) {
	r := qualityAttempt(now)
	r.Terminal = false
	r.CurrentState = runtimeobs.StateRunning
	r.NativeReceipt.State = r.CurrentState
	r.EndedAt = ""
	r.NativeReceipt.CompletedAt = ""
	p := &rolepolicy.Policy{Implementation: rolepolicy.Owner{Role: "worker", AgentPath: r.AgentPath, NativeID: r.SessionID}, Reviewer: rolepolicy.Owner{AgentPath: "/root/controller/reviewer", NativeID: "review-native"}}
	rows := map[string][]backlog.Record{"F-1": {{ID: "F-1", Location: "active", RawMarkdown: "current canonical policy"}}}
	assignments := []runtimeobs.Assignment{{TaskID: "F-1", AgentPath: r.AgentPath, AssignmentID: "fixture-assignment", AssignedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}}
	return rows, assignments, runtimeobs.Ledger{Attempts: []runtimeobs.Attempt{r}}, p
}

func TestCurrentExecutionLoaderUsesTypedOwnerAndAssignmentOnce(t *testing.T) {
	now := time.Now().UTC()
	rows, assignments, ledger, p := currentLoaderFixture(now)
	loads, reads := 0, 0
	load := func(project, task, raw string) (*rolepolicy.Policy, bool, error) {
		loads++
		if project != "/fixture/project" || task != "F-1" || raw != "current canonical policy" {
			t.Fatal("wrong canonical context")
		}
		return p, false, nil
	}
	read := func(ctx stdcontext.Context, s runtimeobs.CurrentExecutionSnapshot, at time.Time) runtimeobs.CurrentExecutionSnapshot {
		reads++
		s.OfficialCurrent = true
		return s
	}
	got := loadCurrentExecutionEvidence(stdcontext.Background(), "/fixture/project", ledger, rows, true, assignments, nil, now, true, load, read)["F-1"]
	if loads != 1 || reads != 1 || !got.Matched || !got.OfficialCurrent || got.ReviewNativeID != "review-native" || got.Identity.AssignmentID != "fixture-assignment" {
		t.Fatalf("shared projection=%+v calls %d %d", got, loads, reads)
	}
	// More recent reviewer activity/assignment must not become implementation.
	reviewer := ledger.Attempts[0]
	reviewer.AgentPath = p.Reviewer.AgentPath
	reviewer.SessionID = p.Reviewer.NativeID
	reviewer.RuntimeAgentID = p.Reviewer.NativeID
	ledger.Attempts = append(ledger.Attempts, reviewer)
	assignments = append(assignments, runtimeobs.Assignment{TaskID: "F-1", AgentPath: p.Reviewer.AgentPath, AssignmentID: "review-assignment", AssignedAt: now.Format(time.RFC3339Nano)})
	got = loadCurrentExecutionEvidence(stdcontext.Background(), "/fixture/project", ledger, rows, true, assignments, nil, now, false, load, read)["F-1"]
	if !got.Matched || got.Identity.NativeID == p.Reviewer.NativeID {
		t.Fatalf("reviewer substituted: %+v", got)
	}
	p.Implementation.NativeID = ""
	if got := loadCurrentExecutionEvidence(stdcontext.Background(), "/fixture/project", ledger, rows, true, assignments, nil, now, true, load, read)["F-1"]; got.Matched || got.OfficialCurrent {
		t.Fatal("legacy Agent supplied missing typed native")
	}
}

func TestCurrentExecutionLoaderBudgetAmbiguityAndFailures(t *testing.T) {
	now := time.Now().UTC()
	rows, assignments, ledger, p := currentLoaderFixture(now)
	load := func(string, string, string) (*rolepolicy.Policy, bool, error) { return p, false, nil }
	reads := 0
	read := func(ctx stdcontext.Context, s runtimeobs.CurrentExecutionSnapshot, at time.Time) runtimeobs.CurrentExecutionSnapshot {
		reads++
		<-ctx.Done()
		s.OfficialCurrent = true
		return s
	}
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	got := loadCurrentExecutionEvidence(ctx, "/fixture/project", ledger, rows, true, assignments, nil, now, true, load, read)["F-1"]
	if reads != 1 || got.Fresh || got.OfficialCurrent || got.FailedStage != "official_observation_budget" || time.Since(start) > time.Second {
		t.Fatalf("deadline failed: %+v", got)
	}
	for _, failure := range []string{"ambiguous row", "no assignment", "assignment error", "provider conflict", "policy error", "wrong role"} {
		t.Run(failure, func(t *testing.T) {
			r, a, l, policy := currentLoaderFixture(now)
			loader := load
			var e error
			switch failure {
			case "ambiguous row":
				r["F-1"] = append(r["F-1"], r["F-1"][0])
			case "no assignment":
				a = nil
			case "assignment error":
				e = errors.New("read failed")
			case "provider conflict":
				other := l.Attempts[0]
				other.Provider = "claude"
				l.Attempts = append(l.Attempts, other)
			case "policy error":
				loader = func(string, string, string) (*rolepolicy.Policy, bool, error) {
					return nil, false, errors.New("stale policy")
				}
			case "wrong role":
				policy.Implementation.Role = "reviewer"
				loader = func(string, string, string) (*rolepolicy.Policy, bool, error) { return policy, false, nil }
			}
			s := loadCurrentExecutionEvidence(stdcontext.Background(), "/fixture/project", l, r, true, a, e, now, false, loader, read)["F-1"]
			if s.Matched || s.OfficialCurrent || s.FailedStage == "" {
				t.Fatalf("%s accepted: %+v", failure, s)
			}
		})
	}
}

func TestCurrentExecutionLoaderCapsOfficialReads(t *testing.T) {
	now := time.Now().UTC()
	rows, assignments, ledger, p := currentLoaderFixture(now)
	r, a, attempt := rows["F-1"][0], assignments[0], ledger.Attempts[0]
	for i := 2; i <= 20; i++ {
		task := fmt.Sprintf("F-%d", i)
		row := r
		row.ID = task
		rows[task] = []backlog.Record{row}
		assigned := a
		assigned.TaskID = task
		assignments = append(assignments, assigned)
		run := attempt
		run.TaskID = task
		ledger.Attempts = append(ledger.Attempts, run)
	}
	reads := 0
	out := loadCurrentExecutionEvidence(stdcontext.Background(), "/fixture/project", ledger, rows, true, assignments, nil, now, true, func(string, string, string) (*rolepolicy.Policy, bool, error) { return p, false, nil }, func(_ stdcontext.Context, s runtimeobs.CurrentExecutionSnapshot, _ time.Time) runtimeobs.CurrentExecutionSnapshot {
		reads++
		s.OfficialCurrent = true
		return s
	})
	deferred := 0
	for _, s := range out {
		if s.FailedStage == "official_observation_budget" {
			deferred++
			if s.Fresh || s.OfficialCurrent {
				t.Fatal("deferred stored state certified")
			}
		}
	}
	if reads != 16 || deferred != 4 {
		t.Fatalf("reads=%d deferred=%d", reads, deferred)
	}
}
