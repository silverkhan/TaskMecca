package orchestration

import (
	"context"
	"strings"
	"testing"
	"time"
)

func dispatchFixture() (Environment, DispatchRequest, time.Time) {
	now := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	e := Environment{Host: "t3", Provider: "codex", RootSessionID: "native-root", T3ThreadID: "t3-root", Source: "fixture:host-tool-inventory", CheckedAt: now, Tools: map[string]ToolEvidence{
		"delegate_task":  {Support: "supported", Worktree: "supported", Source: "fixture:current-schema", CheckedAt: now},
		"spawn_agent":    {Support: "supported", Source: "fixture:native-schema", CheckedAt: now},
		"create_session": {Support: "supported", Source: "fixture:session-schema", CheckedAt: now},
	}}
	e.IdentityLinkage = "verified"
	r := DispatchRequest{CanonicalProject: "/fixture/canonical", Provider: "codex", RootSessionID: "native-root", T3ThreadID: "t3-root", SeparateWorktree: true, SourceCWD: "/fixture/parent", Base: strings.Repeat("a", 40)}
	return e, r, now
}

func TestEnvironmentRouteMatrix(t *testing.T) {
	for _, tc := range []struct {
		name         string
		change       func(*Environment, *DispatchRequest)
		mode         Mode
		allowed      bool
		tool, reason string
	}{
		{"t3 worktree", func(*Environment, *DispatchRequest) {}, ModeB, true, "delegate_task", ""},
		{"unknown tool", func(e *Environment, _ *DispatchRequest) { delete(e.Tools, "delegate_task") }, ModeB, false, "delegate_task", "selected_tool_unknown"},
		{"blocked tool", func(e *Environment, _ *DispatchRequest) {
			v := e.Tools["delegate_task"]
			v.Support = "unsupported"
			e.Tools["delegate_task"] = v
		}, ModeB, false, "delegate_task", "selected_tool_unsupported"},
		{"unknown isolation", func(e *Environment, _ *DispatchRequest) {
			v := e.Tools["delegate_task"]
			v.Worktree = "unknown"
			e.Tools["delegate_task"] = v
		}, ModeB, false, "delegate_task", "selected_tool_worktree_schema_unverified"},
		{"no mode fallback", func(*Environment, *DispatchRequest) {}, ModeC, false, "", "selected_session_mode_has_no_verified_t3_worktree_route"},
		{"t3 shared native", func(_ *Environment, r *DispatchRequest) { r.SeparateWorktree = false }, ModeB, true, "spawn_agent", ""},
		{"non t3 native", func(e *Environment, r *DispatchRequest) {
			e.Host = "native"
			e.T3ThreadID = ""
			r.T3ThreadID = ""
			r.SeparateWorktree = false
		}, ModeA, true, "spawn_agent", ""},
		{"non t3 session", func(e *Environment, r *DispatchRequest) {
			e.Host = "other"
			e.T3ThreadID = ""
			r.T3ThreadID = ""
			r.SeparateWorktree = false
		}, ModeD, true, "create_session", ""},
		{"non t3 worktree unknown", func(e *Environment, r *DispatchRequest) { e.Host = "native"; e.T3ThreadID = ""; r.T3ThreadID = "" }, ModeB, false, "spawn_agent", "native_separate_worktree_route_unverified"},
		{"unknown host", func(e *Environment, _ *DispatchRequest) { e.Host = "unknown" }, ModeB, false, "", "environment_host_unknown"},
		{"stale", func(e *Environment, _ *DispatchRequest) { e.CheckedAt = e.CheckedAt.Add(-time.Hour) }, ModeB, false, "", "environment_stale_or_unobserved"},
		{"future", func(e *Environment, _ *DispatchRequest) { e.CheckedAt = e.CheckedAt.Add(time.Minute) }, ModeB, false, "", "environment_stale_or_unobserved"},
		{"environment changed", func(e *Environment, _ *DispatchRequest) { e.Host = "native" }, ModeB, false, "", "environment_host_identity_conflict"},
		{"native mismatch", func(e *Environment, _ *DispatchRequest) { e.RootSessionID = "other" }, ModeB, false, "", "environment_identity_mismatch"},
		{"t3 mismatch", func(_ *Environment, r *DispatchRequest) { r.T3ThreadID = "other" }, ModeB, false, "", "environment_identity_mismatch"},
		{"provider mismatch", func(e *Environment, _ *DispatchRequest) { e.Provider = "other" }, ModeB, false, "", "environment_identity_mismatch"},
		{"stale tool", func(e *Environment, _ *DispatchRequest) {
			v := e.Tools["delegate_task"]
			v.CheckedAt = v.CheckedAt.Add(-time.Hour)
			e.Tools["delegate_task"] = v
		}, ModeB, false, "delegate_task", "selected_tool_stale"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, r, now := dispatchFixture()
			tc.change(&e, &r)
			p := PlanDispatch(e, r, tc.mode, now)
			if p.Allowed != tc.allowed || p.Tool != tc.tool || p.ExecutionAuthorized {
				t.Fatalf("%+v", p)
			}
			if tc.reason != "" && !strings.Contains(strings.Join(p.Reasons, ","), tc.reason) {
				t.Fatalf("%+v", p)
			}
			if p.Allowed && (p.Visibility == "" || p.VerificationMethod == "") {
				t.Fatal("missing visibility/verification guidance")
			}
		})
	}
}

type fixtureHost struct {
	e           Environment
	access      AccessEvidence
	observation CreationObservation
	calls       []string
}

func (h *fixtureHost) ObserveEnvironment(context.Context) (Environment, error) {
	h.calls = append(h.calls, "environment")
	return h.e, nil
}
func (h *fixtureHost) FullAccess(context.Context) (AccessEvidence, error) {
	h.calls = append(h.calls, "access")
	return h.access, nil
}
func (h *fixtureHost) ReadCreation(context.Context, CreationReceipt) (CreationObservation, error) {
	h.calls = append(h.calls, "independent_read")
	return h.observation, nil
}

func TestDispatchBoundaryRequiresCurrentHostProof(t *testing.T) {
	e, r, now := dispatchFixture()
	h := &fixtureHost{e: e, access: AccessEvidence{CanonicalProject: r.CanonicalProject, Passed: true, Source: "fixture:effective-probe", CheckedAt: now}}
	clock := func() time.Time { return now }
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err != nil {
		t.Fatal(err)
	}
	h.access.CanonicalProject = "/fixture/other"
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err == nil {
		t.Fatal("Full Access proof for another project")
	}
	h.access.CanonicalProject = r.CanonicalProject
	h.access.CheckedAt = now.Add(-time.Second)
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err == nil {
		t.Fatal("reused Full Access proof")
	}
	h.access.CheckedAt = now
	h.e.CheckedAt = now.Add(-time.Second)
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err == nil {
		t.Fatal("reused environment proof")
	}
	h.e = e
	v := h.e.Tools["delegate_task"]
	v.CheckedAt = now.Add(-time.Second)
	h.e.Tools["delegate_task"] = v
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err == nil {
		t.Fatal("reused tool proof")
	}
	if _, err := CheckDispatchBoundary(context.Background(), nil, r, ModeB, clock); err == nil {
		t.Fatal("JSON-only host accepted")
	}
	e, _, _ = dispatchFixture()
	h.e = e
	h.e.IdentityLinkage = "unknown"
	if _, err := CheckDispatchBoundary(context.Background(), h, r, ModeB, clock); err == nil {
		t.Fatal("Root linkage not independently verified")
	}
}

func TestCreatedUnlinkedIsPartialNeverStarted(t *testing.T) {
	e, r, now := dispatchFixture()
	p := PlanDispatch(e, r, ModeB, now)
	receipt := CreationReceipt{NativeSessionID: "native-child", T3ThreadID: "t3-child", Worktree: "/fixture/child", Base: r.Base}
	h := &fixtureHost{observation: CreationObservation{CreationReceipt: receipt, Provider: r.Provider, SourceCWD: r.SourceCWD, Source: "fixture:official-read", CheckedAt: now}}
	v, err := VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err == nil || !v.Created || v.TrackingVerified || v.Started || v.Status != "created_tracking_unobserved" {
		t.Fatalf("%+v %v", v, err)
	}
	h.observation.ParentT3ThreadID = r.T3ThreadID
	v, err = VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err != nil || !v.TrackingVerified || v.Started {
		t.Fatalf("%+v %v", v, err)
	}
	h.observation.NativeSessionID = "mismatched"
	v, err = VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err == nil || v.Created {
		t.Fatal("caller receipt promoted over independent read")
	}
	if _, err = VerifyCreation(context.Background(), nil, r, p, receipt, now); err == nil {
		t.Fatal("caller JSON promoted")
	}
}

func TestDispatchInputRequiredWithoutInventingHistoricalCause(t *testing.T) {
	d, err := Decide(Request{Mode: ModeB, Event: "dispatch_requested", Capabilities: Capabilities{SpawnSubagent: "supported", MessageRoot: "supported"}})
	if err != nil || d.Allowed || d.Action != "observe_environment_first" {
		t.Fatalf("%+v %v", d, err)
	}
	// Existing A-D selection and completion recovery are not redefined by a missing profile.
	d, err = Decide(Request{Mode: ModeB, Event: "worker_done"})
	if err != nil || !d.Allowed {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestCreationChecksNativeCWDBaseAndVisibility(t *testing.T) {
	e, r, now := dispatchFixture()
	r.SeparateWorktree = false
	p := PlanDispatch(e, r, ModeB, now)
	receipt := CreationReceipt{NativeSessionID: "native-child", CollaborationHandle: "native-handle", Worktree: r.SourceCWD, Base: r.Base}
	h := &fixtureHost{observation: CreationObservation{CreationReceipt: receipt, Provider: r.Provider, SourceCWD: r.SourceCWD, Source: "fixture:native-official-read", CheckedAt: now}}
	v, err := VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err != nil || !v.Created || v.TrackingVerified || v.Started || v.Status != "created_native_verified_t3_visibility_limited" {
		t.Fatalf("%+v %v", v, err)
	}
	h.observation.SourceCWD = "/fixture/wrong"
	if _, err = VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil {
		t.Fatal("source cwd mismatch")
	}
	h.observation.SourceCWD = r.SourceCWD
	receipt.Base = strings.Repeat("b", 40)
	if _, err = VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil {
		t.Fatal("base mismatch")
	}
	e, r, now = dispatchFixture()
	p = PlanDispatch(e, r, ModeB, now)
	receipt.Base = r.Base
	receipt.T3ThreadID = "t3-child"
	h.observation.CreationReceipt = receipt
	h.observation.ParentT3ThreadID = r.T3ThreadID
	if _, err = VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil {
		t.Fatal("shared cwd accepted as separate worktree")
	}
}
