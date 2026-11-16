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
		"delegate_task":  {Support: "supported", Worktree: "supported", Shared: "supported", Source: "fixture:current-schema", CheckedAt: now},
		"spawn_agent":    {Support: "supported", Source: "fixture:native-schema", CheckedAt: now},
		"create_session": {Support: "supported", Source: "fixture:session-schema", CheckedAt: now},
	}}
	e.IdentityLinkage = "verified"
	e.Creator = &CreatorIdentity{NativeSessionID: "native-controller", T3ThreadID: "t3-controller", ProjectID: "project", Source: "fixture:official-current-creator", CheckedAt: now, Linkage: "verified"}
	r := DispatchRequest{CreatorNativeID: "native-controller", CreatorT3ThreadID: "t3-controller", T3ProjectID: "project", GroupID: "D1112_01", Role: "worker", Worker: "isanghaessi", Generation: 4, CanonicalProject: "/fixture/canonical", Provider: "codex", RootSessionID: "native-root", T3ThreadID: "t3-root", SeparateWorktree: true, SourceCWD: "/fixture/parent", Base: strings.Repeat("a", 40)}
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
		{"no mode fallback", func(*Environment, *DispatchRequest) {}, ModeC, false, "", "selected_session_mode_has_no_verified_t3_managed_route"},
		{"t3 shared managed", func(_ *Environment, r *DispatchRequest) { r.SeparateWorktree = false }, ModeB, true, "delegate_task", ""},
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
	if err == nil || !v.Created || v.TrackingVerified || v.Started || v.Status != "created_tracking_unverified" {
		t.Fatalf("%+v %v", v, err)
	}
	h.observation = linkedObservation(r, receipt, now)
	v, err = VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err != nil || !v.TrackingVerified || v.Started {
		t.Fatalf("%+v %v", v, err)
	}
	h.observation.NativeSessionID = "mismatched"
	v, err = VerifyCreation(context.Background(), h, r, p, receipt, now)
	if err == nil || !v.Created || v.TrackingVerified {
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
	e.Host, e.T3ThreadID, r.T3ThreadID = "native", "", ""
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
	h.observation = linkedObservation(r, receipt, now)
	if _, err = VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil {
		t.Fatal("shared cwd accepted as separate worktree")
	}
}

// All IDs are sanitized fixture values, not actual runtime evidence.
func linkedObservation(r DispatchRequest, receipt CreationReceipt, now time.Time) CreationObservation {
	name, _ := dispatchName(r)
	read := &ThreadObservation{ThreadID: receipt.T3ThreadID, ParentThreadID: r.CreatorT3ThreadID, ProjectID: r.T3ProjectID, Relationship: "subagent", Title: name, Workspace: receipt.Worktree, Source: "fixture:official-read", CheckedAt: now}
	list := *read
	list.Source = "fixture:official-list"
	return CreationObservation{CreationReceipt: receipt, Provider: r.Provider, SourceCWD: r.SourceCWD, Source: "fixture:official-host", CheckedAt: now, T3Read: read, T3List: &list, NativeLink: &NativeLinkObservation{NativeSessionID: receipt.NativeSessionID, T3ThreadID: receipt.T3ThreadID, Provider: r.Provider, Kind: "provider_native_session", Source: "fixture:trusted-supported-native-projection", CheckedAt: now}}
}
func TestAllManagedRolesAndIsolation(t *testing.T) {
	for _, role := range []string{"controller", "registrar", "worker"} {
		for _, worktree := range []bool{false, true} {
			for _, mode := range []Mode{ModeA, ModeB, ModeC, ModeD} {
				e, r, now := dispatchFixture()
				r.Role, r.SeparateWorktree = role, worktree
				if role != "worker" {
					r.Worker = ""
				}
				p := PlanDispatch(e, r, mode, now)
				if mode == ModeC || mode == ModeD {
					if p.Allowed {
						t.Fatal("independent mode silently became subagent")
					}
					continue
				}
				isolation := "shared"
				if worktree {
					isolation = "worktree"
				}
				if !p.Allowed || p.Tool != "delegate_task" || p.Isolation != isolation || p.ExecutionAuthorized {
					t.Fatalf("%s %s %+v", role, mode, p)
				}
			}
		}
	}
	e, r, now := dispatchFixture()
	r.Purpose = "reviewer" // Actual identity is still officially named Worker.
	if !PlanDispatch(e, r, ModeB, now).Allowed {
		t.Fatal("review purpose must not invent a reviewer naming role")
	}
	r.Role = "reviewer"
	if PlanDispatch(e, r, ModeB, now).Allowed {
		t.Fatal("unsupported naming role")
	}
	r.Role = "worker"
	r.Purpose = "implementation"
	r.SeparateWorktree = false
	if PlanDispatch(e, r, ModeB, now).Allowed {
		t.Fatal("implementation Worker shared checkout")
	}
}
func TestManagedUnknownContextAndSchema(t *testing.T) {
	for _, mutate := range []func(*Environment, *DispatchRequest){
		func(e *Environment, r *DispatchRequest) { r.CreatorT3ThreadID = "" },
		func(e *Environment, r *DispatchRequest) { e.Creator = nil },
		func(e *Environment, r *DispatchRequest) { e.Creator.T3ThreadID = r.T3ThreadID },
		func(e *Environment, r *DispatchRequest) { e.Creator.NativeSessionID = "foreign" },
		func(e *Environment, r *DispatchRequest) { e.Creator.ProjectID = "foreign" },
		func(e *Environment, r *DispatchRequest) { e.Creator.CheckedAt = e.CheckedAt.Add(-time.Hour) },
		func(e *Environment, r *DispatchRequest) { e.Creator.Linkage = "unknown" },
		func(e *Environment, r *DispatchRequest) { r.T3ProjectID = "" },
		func(e *Environment, r *DispatchRequest) { r.GroupID = "handcrafted" },
		func(e *Environment, r *DispatchRequest) {
			r.SeparateWorktree = false
			v := e.Tools["delegate_task"]
			v.Shared = "unknown"
			e.Tools["delegate_task"] = v
		},
	} {
		e, r, now := dispatchFixture()
		mutate(&e, &r)
		if PlanDispatch(e, r, ModeB, now).Allowed {
			t.Fatal("unverified context/schema accepted")
		}
	}
}
func TestManagedIndependentCreationProofMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*CreationObservation)
	}{
		{"wrong parent Root is not Controller", func(o *CreationObservation) { o.T3Read.ParentThreadID = "t3-root" }},
		{"both parents absent", func(o *CreationObservation) { o.T3Read.ParentThreadID = ""; o.T3List.ParentThreadID = "" }},
		{"wrong list child", func(o *CreationObservation) { o.T3List.ThreadID = "other" }},
		{"wrong project", func(o *CreationObservation) { o.T3List.ProjectID = "other" }},
		{"wrong relation", func(o *CreationObservation) { o.T3Read.Relationship = "fork" }},
		{"wrong name", func(o *CreationObservation) { o.T3List.Title = "Reviewer_handcrafted" }},
		{"workspace unknown", func(o *CreationObservation) { o.T3Read.Workspace = ""; o.T3List.Workspace = "" }},
		{"wrong workspace", func(o *CreationObservation) { o.T3Read.Workspace = "/foreign" }},
		{"stale list", func(o *CreationObservation) { o.T3List.CheckedAt = o.CheckedAt.Add(-time.Hour) }},
		{"missing read", func(o *CreationObservation) { o.T3Read = nil }},
		{"missing list", func(o *CreationObservation) { o.T3List = nil }},
		{"unverified native", func(o *CreationObservation) { o.NativeLink = nil }},
		{"wrong native kind", func(o *CreationObservation) { o.NativeLink.Kind = "collaboration_handle" }},
		{"wrong native value", func(o *CreationObservation) { o.NativeLink.NativeSessionID = "fake-uuid" }},
		{"wrong native thread", func(o *CreationObservation) { o.NativeLink.T3ThreadID = "other" }},
		{"wrong provider", func(o *CreationObservation) { o.NativeLink.Provider = "claude" }},
		{"stale native", func(o *CreationObservation) { o.NativeLink.CheckedAt = o.CheckedAt.Add(-time.Hour) }},
		{"wrong source cwd", func(o *CreationObservation) { o.SourceCWD = "/foreign" }},
		{"wrong base", func(o *CreationObservation) { o.Base = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, r, now := dispatchFixture()
			p := PlanDispatch(e, r, ModeB, now)
			receipt := CreationReceipt{NativeSessionID: "native-child", T3ThreadID: "t3-child", Worktree: "/fixture/child", Base: r.Base}
			h := &fixtureHost{observation: linkedObservation(r, receipt, now)}
			tc.mutate(&h.observation)
			for i := 0; i < 2; i++ {
				v, err := VerifyCreation(context.Background(), h, r, p, receipt, now)
				if err == nil || !v.Created || v.TrackingVerified || v.Started || v.NextAction != "reconcile_same_receipt" || v.ResumeWhen == "" || v.Owner == "" {
					t.Fatalf("%+v %v", v, err)
				}
			}
			// Rechecking the SAME receipt only calls the existing read primitive. There
			// is no creation operation or new manager at this boundary.
			if strings.Join(h.calls, ",") != "independent_read,independent_read" {
				t.Fatal(h.calls)
			}
			h.observation = linkedObservation(r, receipt, now)
			h.observation.T3List.ParentThreadID = "" // actual supported omission
			v, err := VerifyCreation(context.Background(), h, r, p, receipt, now)
			if err != nil || !v.TrackingVerified || v.Started {
				t.Fatalf("%+v %v", v, err)
			}
		})
	}
}
func TestManagedCreationReceiptWithoutNativeStillCreated(t *testing.T) {
	e, r, now := dispatchFixture()
	v, err := VerifyCreation(context.Background(), nil, r, PlanDispatch(e, r, ModeB, now), CreationReceipt{T3ThreadID: "t3-created"}, now)
	if err == nil || !v.Created || v.TrackingVerified || v.NextAction != "reconcile_same_receipt" {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestManagedCannotVerifyWithSubstitutedPlan(t *testing.T) {
	e, r, now := dispatchFixture()
	receipt := CreationReceipt{NativeSessionID: "native-child", T3ThreadID: "t3-child", Worktree: "/fixture/child", Base: r.Base}
	h := &fixtureHost{observation: linkedObservation(r, receipt, now)}
	p := PlanDispatch(e, r, ModeB, now)
	p.Tool = "spawn_agent"
	if v, err := VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil || v.TrackingVerified {
		t.Fatal("substituted native plan accepted")
	}
	p = PlanDispatch(e, r, ModeB, now)
	p.Isolation = "shared"
	if _, err := VerifyCreation(context.Background(), h, r, p, receipt, now); err == nil {
		t.Fatal("substituted isolation")
	}
	for _, id := range []string{r.RootSessionID, r.CreatorNativeID, receipt.T3ThreadID} {
		h.observation = linkedObservation(r, receipt, now)
		h.observation.NativeLink.NativeSessionID = id
		if _, err := VerifyCreation(context.Background(), h, r, PlanDispatch(e, r, ModeB, now), receipt, now); err == nil {
			t.Fatal("wrong namespace accepted")
		}
	}
}
func TestManagedDispatchDoesNotNeedNativeSpawnCapability(t *testing.T) {
	e, r, _ := dispatchFixture()
	now := time.Now()
	e.CheckedAt = now
	e.Creator.CheckedAt = now
	for name, v := range e.Tools {
		v.CheckedAt = now
		e.Tools[name] = v
	}
	d, err := Decide(Request{Mode: ModeB, Event: "dispatch_requested", Environment: &e, Dispatch: r, Capabilities: Capabilities{SpawnSubagent: "unsupported", MessageRoot: "supported"}})
	if err != nil || !d.Allowed || d.Mode != ModeB || d.Dispatch.Tool != "delegate_task" {
		t.Fatalf("%+v %v", d, err)
	}
}
