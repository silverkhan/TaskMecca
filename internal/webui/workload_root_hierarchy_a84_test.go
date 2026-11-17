package webui

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/orchestration"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func TestA84WorkloadListAndDetailRefreshWithoutRootGET(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	project := t.TempDir()
	now := time.Now().UTC()
	if err := os.MkdirAll(filepath.Join(project, "_task_mecca"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := maintenance.RegisterProject(project); err != nil {
		t.Fatal(err)
	}
	groups := filepath.Join(project, "_task_mecca", "data", "orchestration", "groups")
	if err := os.MkdirAll(groups, 0700); err != nil {
		t.Fatal(err)
	}
	group := orchestration.Group{SchemaVersion: 1, GroupID: "D1112_01", DisplayPrefix: "D1112_01_", Mode: orchestration.ModeB, Provider: "codex", RootSessionID: "root-native", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)}
	raw, _ := json.Marshal(group)
	if err := os.WriteFile(filepath.Join(groups, "D1112_01.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root-native", "new-worker"} {
		err := runtimeobs.AppendExecutionEvent(project, runtimeobs.ExecutionEvent{EventKind: "state", Provider: "codex", SessionID: id, RuntimeAgentID: id, AttemptID: "attempt-" + id, ObservedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), State: runtimeobs.StateCompleted, Terminal: true, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved})
		if err != nil {
			t.Fatal(err)
		}
	}
	oldRefresh, oldObserve := refreshWorkloadNames, observeWorkloadRoot
	t.Cleanup(func() { refreshWorkloadNames = oldRefresh; observeWorkloadRoot = oldObserve })
	calls := 0
	refreshWorkloadNames = func(p string, _ runtimeobs.Ledger, at time.Time) (bool, error) {
		calls++
		snapshot := runtimeobs.RootNameObservation{Version: 1, Name: "🌱 1112 Task Mecca", NativeName: "native-secondary", Source: "t3_title", Status: "verified", CheckedAt: at.Format(time.RFC3339Nano), T3ThreadID: "host-root", HostProjectID: "host-project", IdentityVerified: true, HostStatus: "verified"}
		err := runtimeobs.AppendExecutionEvent(p, runtimeobs.ExecutionEvent{EventKind: "session_metadata", ObservedAt: snapshot.CheckedAt, AttemptID: "meta", Provider: "codex", SessionID: "root-native", RootName: &snapshot})
		return true, err
	}
	observeWorkloadRoot = func(ctx stdcontext.Context, r runtimeobs.RootSession, at time.Time) runtimeobs.RootSessionExecution {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded observation")
		}
		return runtimeobs.RootSessionExecution{State: runtimeobs.StateRunning, Source: runtimeobs.EvidenceT3, ObservedAt: at.Format(time.RFC3339Nano), OfficialCurrent: true}
	}
	get := func(id string) []byte {
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/workload-sessions?project="+url.QueryEscape(project)+"&root_session_id="+url.QueryEscape(id), nil)
		rec := httptest.NewRecorder()
		workloadSessionsHandler(rec, req)
		if rec.Code != 200 {
			t.Fatalf("HTTP%d %s", rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}
	var list runtimeobs.SessionWorkload
	if err := json.Unmarshal(get(""), &list); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || list.Representative == nil || list.Representative.DisplayName != "🌱 1112 Task Mecca" || list.Representative.NativeName != "native-secondary" || list.Representative.Status != runtimeobs.RootSessionTerminal || list.Representative.Execution == nil || !list.Representative.Execution.OfficialCurrent {
		t.Fatalf("list projection %+v calls%d", list, calls)
	}
	var detail runtimeobs.SessionWorkloadDetail
	if err := json.Unmarshal(get(list.LatestSessionID), &detail); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || detail.RootSession.DisplayName != list.Representative.DisplayName || detail.RootSession.LastUsedAt != list.Representative.LastUsedAt || detail.RootSession.LastActivityAt != list.Representative.LastActivityAt || detail.RootSession.Status != runtimeobs.RootSessionTerminal {
		t.Fatalf("detail mutated use/state %+v", detail)
	}
	if len(detail.Attempts) != 1 || !detail.Attempts[0].Terminal {
		t.Fatal("observation rewrote stored attempt")
	}
}

func TestA84ReviewerRequiresExactTypedNativeTaskAndProvider(t *testing.T) {
	snapshot := runtimeobs.CurrentExecutionSnapshot{Identity: runtimeobs.CurrentExecutionIdentity{TaskID: "A-84", Provider: "codex"}, ReviewAgentPath: "/root/controller/reviewer", ReviewNativeID: "review-native"}
	attempt := runtimeobs.Attempt{SessionID: "review-native", RuntimeAgentID: "review-native", AgentPath: "/root/controller/reviewer", TaskID: "A-84", Provider: "codex", BindingState: runtimeobs.BindingBound}
	for _, tc := range []struct {
		name   string
		change func(*runtimeobs.Attempt, *runtimeobs.RootSession, *runtimeobs.CurrentExecutionSnapshot)
		want   string
	}{
		{"exact", func(*runtimeobs.Attempt, *runtimeobs.RootSession, *runtimeobs.CurrentExecutionSnapshot) {}, "reviewer"},
		{"unknown-native", func(_ *runtimeobs.Attempt, _ *runtimeobs.RootSession, s *runtimeobs.CurrentExecutionSnapshot) {
			s.ReviewNativeID = ""
		}, "worker"},
		{"other-task", func(a *runtimeobs.Attempt, _ *runtimeobs.RootSession, _ *runtimeobs.CurrentExecutionSnapshot) {
			a.TaskID = "A-83"
		}, "worker"},
		{"other-provider", func(a *runtimeobs.Attempt, _ *runtimeobs.RootSession, _ *runtimeobs.CurrentExecutionSnapshot) {
			a.Provider = "claude"
		}, "worker"},
		{"root-is-never-reviewer", func(_ *runtimeobs.Attempt, r *runtimeobs.RootSession, _ *runtimeobs.CurrentExecutionSnapshot) {
			r.Role = "root"
		}, "root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, s := attempt, snapshot
			r := runtimeobs.RootSession{ProviderSessionID: "review-native", Role: "worker", RelationStatus: "verified"}
			tc.change(&a, &r, &s)
			annotateSessionRole(&r, runtimeobs.Ledger{Attempts: []runtimeobs.Attempt{a}}, map[string]runtimeobs.CurrentExecutionSnapshot{"A-84": s})
			if r.Role != tc.want {
				t.Fatalf("role=%s want%s", r.Role, tc.want)
			}
		})
	}
}
