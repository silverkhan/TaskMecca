package orchestration

// Environment evidence is deliberately separate from a saved mode and from
// execution receipts. JSON profiles are useful policy inputs, never authority.
import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ToolEvidence struct {
	Support   string    `json:"support"` // supported, unsupported, unknown
	Source    string    `json:"source"`
	CheckedAt time.Time `json:"checked_at"`
	Shared    string    `json:"shared,omitempty"`   // actual isolation:shared schema support
	Worktree  string    `json:"worktree,omitempty"` // actual isolation:worktree schema support
}

type Environment struct {
	Host                string                  `json:"host"` // t3, native, other, unknown
	Provider            string                  `json:"provider"`
	RootSessionID       string                  `json:"root_session_id"`
	T3ThreadID          string                  `json:"t3_thread_id,omitempty"`
	CollaborationHandle string                  `json:"collaboration_handle,omitempty"`
	Source              string                  `json:"source"`
	CheckedAt           time.Time               `json:"checked_at"`
	Tools               map[string]ToolEvidence `json:"tools"`
	TrackingVisibility  string                  `json:"tracking_visibility"`
	Creator             *CreatorIdentity        `json:"creator,omitempty"`
	IdentityLinkage     string                  `json:"identity_linkage"` // host official read: verified|unknown|unsupported
}

// Creator is the actual tool caller, which can be a Controller below the Root.
// Its independently observed identity must never be replaced with the Root ID.
type CreatorIdentity struct {
	NativeSessionID string    `json:"native_session_id"`
	T3ThreadID      string    `json:"t3_thread_id"`
	ProjectID       string    `json:"project_id"`
	Source          string    `json:"source"`
	CheckedAt       time.Time `json:"checked_at"`
	Linkage         string    `json:"linkage"`
}

type DispatchRequest struct {
	CreatorNativeID   string `json:"creator_native_id,omitempty"`
	CreatorT3ThreadID string `json:"creator_t3_thread_id,omitempty"`
	T3ProjectID       string `json:"t3_project_id,omitempty"`
	GroupID           string `json:"group_id,omitempty"`
	Role              string `json:"role,omitempty"` // actual naming role, e.g. worker for review purpose
	Worker            string `json:"worker,omitempty"`
	Generation        int    `json:"generation,omitempty"`
	Purpose           string `json:"purpose,omitempty"` // implementation, reviewer, etc.; not identity

	CanonicalProject string `json:"canonical_project"`
	Provider         string `json:"provider"`
	RootSessionID    string `json:"root_session_id"`
	T3ThreadID       string `json:"t3_thread_id,omitempty"`
	SeparateWorktree bool   `json:"separate_worktree"`
	SourceCWD        string `json:"source_cwd"`
	Base             string `json:"base"`
}

type DispatchPlan struct {
	Allowed             bool     `json:"allowed"`
	ExecutionAuthorized bool     `json:"execution_authorized"` // always false for a plan
	Tool                string   `json:"tool,omitempty"`
	Isolation           string   `json:"isolation,omitempty"`
	Reasons             []string `json:"reasons"`
	Owner               string   `json:"owner,omitempty"`
	ResumeWhen          string   `json:"resume_when,omitempty"`
	RequiredEvidence    []string `json:"required_evidence"`
	Instructions        []string `json:"instructions"`
	Visibility          string   `json:"visibility"`
	VerificationMethod  string   `json:"verification_method"`
	EvidenceKind        string   `json:"evidence_kind"`
}

const EnvironmentMaxAge = 5 * time.Minute

func fresh(at, now time.Time) bool {
	return !at.IsZero() && !at.After(now) && now.Sub(at) <= EnvironmentMaxAge
}

// PlanDispatch selects a tool, never another provider, mode or isolation. Even a
// fully populated caller profile cannot authorize execution or prove linkage.
func PlanDispatch(e Environment, req DispatchRequest, mode Mode, now time.Time) DispatchPlan {
	p := DispatchPlan{EvidenceKind: "diagnostic_profile_not_verified_tool_support_or_creation", Reasons: []string{}, RequiredEvidence: []string{}, Instructions: []string{
		"Immediately before the actual tool call re-observe this exact Root environment and current callable tool schema, and run fresh canonical Full Access preflight; cached profiles are not authorization.",
		"Keep canonical contract, runtime, assignment and handoff authorization gates. Use the existing naming directive and exact selected provider/model/settings; do not substitute a mode or provider.",
		"The host agent calls the selected tool. CLI does not invoke MCP. Record request, assignment preparation, creation, START and tracking verification separately.",
		"After creation independently read exact native/T3 identities, source cwd, worktree and committed base; then use existing runtime START/binding and current-phase Activity procedures. Receipt acknowledgement is not acceptance.",
	}}
	block := func(reason string) DispatchPlan {
		p.Reasons = append(p.Reasons, reason)
		p.Owner = "dispatching_host"
		p.ResumeWhen = "Obtain fresh exact creator/project/tool-schema/role-name evidence for the selected route; preserve saved mode and retry without fallback."
		p.RequiredEvidence = append(p.RequiredEvidence, "Re-observe exact current Root, selected tool schema and actual host receipt; retry the same requested route after verification.")
		return p
	}
	info, ok := ModeByCode(mode)
	if !ok {
		return block("invalid_mode")
	}
	if req.Provider == "" || req.RootSessionID == "" || e.Provider != req.Provider || e.RootSessionID != req.RootSessionID || e.T3ThreadID != req.T3ThreadID {
		return block("environment_identity_mismatch")
	}
	if e.Host != "t3" && e.Host != "native" && e.Host != "other" {
		return block("environment_host_unknown")
	}
	if (e.Host == "t3") != (e.T3ThreadID != "") {
		return block("environment_host_identity_conflict")
	}
	if e.Source == "" || !fresh(e.CheckedAt, now) {
		return block("environment_stale_or_unobserved")
	}
	if !filepath.IsAbs(req.CanonicalProject) || !filepath.IsAbs(req.SourceCWD) || !regexp.MustCompile(`^[a-fA-F0-9]{40}([a-fA-F0-9]{24})?$`).MatchString(req.Base) {
		return block("source_cwd_and_committed_base_required")
	}
	if strings.EqualFold(strings.TrimSpace(req.Role), "reviewer") && req.Purpose == "implementation" {
		return block("reviewer_cannot_dispatch_product_implementation")
	}
	if e.Host == "t3" {
		if req.CreatorNativeID == "" || req.CreatorT3ThreadID == "" || req.T3ProjectID == "" {
			return block("explicit_actual_creator_and_t3_project_required")
		}
		if e.Creator == nil || e.Creator.NativeSessionID != req.CreatorNativeID || e.Creator.T3ThreadID != req.CreatorT3ThreadID || e.Creator.ProjectID != req.T3ProjectID || e.Creator.Source == "" || !fresh(e.Creator.CheckedAt, now) || e.Creator.Linkage != "verified" {
			return block("actual_creator_identity_unverified")
		}
		if _, err := dispatchName(req); err != nil {
			return block("supported_role_naming_directive_required")
		}
		if req.Purpose == "implementation" && strings.EqualFold(strings.TrimSpace(req.Role), "worker") && !req.SeparateWorktree {
			return block("implementation_worker_requires_worktree")
		}

		if info.Execution != "subagent" {
			return block("selected_session_mode_has_no_verified_t3_managed_route")
		}
		p.Tool, p.Isolation = "delegate_task", "shared"
		if req.SeparateWorktree {
			p.Isolation = "worktree"
		}
	} else if info.Execution == "subagent" {
		p.Tool, p.Isolation = "spawn_agent", "shared"
		if req.SeparateWorktree {
			return block("native_separate_worktree_route_unverified")
		}
	} else {
		p.Tool, p.Isolation = "create_session", "shared"
		if req.SeparateWorktree {
			return block("session_separate_worktree_route_unverified")
		}
	}
	p.Visibility = "Native children may be absent from T3 Thread Details/Lineage; display name, native session ID, T3 thread ID and collaboration handle are different namespaces."
	p.VerificationMethod = "Use the actual native creation receipt and supported native status/read tool on its returned exact handle; report unavailable observations as unknown."
	if p.Tool == "delegate_task" {
		p.Visibility = "T3 Thread Details/Lineage linkage is required and remains unverified until independently read after creation."
		p.VerificationMethod = "Read the exact returned T3 thread via official t3_thread_read AND match that exact ID in t3_thread_list. Verify actual creator parent (not automatically Root), project, subagent relationship, generated role-name, native identity link, actual workspace cwd and committed base. Missing fields remain unknown; never respawn a created-but-unlinked child."
	}
	t, exists := e.Tools[p.Tool]
	if exists && t.Support == "unsupported" {
		return block("selected_tool_unsupported")
	}
	if !exists || t.Support != "supported" {
		return block("selected_tool_unknown")
	}
	if t.Source == "" {
		return block("selected_tool_source_missing")
	}
	if !fresh(t.CheckedAt, now) {
		return block("selected_tool_stale")
	}
	if p.Isolation == "worktree" && t.Worktree != "supported" {
		return block("selected_tool_worktree_schema_unverified")
	}
	if p.Tool == "delegate_task" && p.Isolation == "shared" && t.Shared != "supported" {
		return block("selected_tool_shared_schema_unverified")
	}
	p.Allowed = true
	return p
}

// HostAdapter is an in-process boundary implemented by the actual tool-owning
// host, never deserialized from JSON. It must use current tool inventory and
// independent official observations, not echo caller assertions. No manager,
// transport, database editing or MCP invocation is hidden in this package.
type HostAdapter interface {
	ObserveEnvironment(context.Context) (Environment, error)
	FullAccess(context.Context) (AccessEvidence, error)
	ReadCreation(context.Context, CreationReceipt) (CreationObservation, error)
}

type AccessEvidence struct {
	CanonicalProject string
	Passed           bool
	Source           string
	CheckedAt        time.Time
}
type CreationReceipt struct {
	NativeSessionID     string `json:"native_session_id"`
	T3ThreadID          string `json:"t3_thread_id,omitempty"`
	CollaborationHandle string `json:"collaboration_handle,omitempty"`
	Worktree            string `json:"worktree"`
	Base                string `json:"base"`
}

// Only the actual host adapter constructs these from official reads. CLI JSON
// is diagnostic input and cannot instantiate an authoritative host adapter.
type ThreadObservation struct {
	ThreadID, ParentThreadID, ProjectID, Relationship, Title, Workspace string
	Source                                                              string
	CheckedAt                                                           time.Time
}
type NativeLinkObservation struct {
	NativeSessionID, T3ThreadID, Provider, Kind, Source string
	CheckedAt                                           time.Time
}
type CreationObservation struct {
	T3Read     *ThreadObservation
	T3List     *ThreadObservation
	NativeLink *NativeLinkObservation

	CreationReceipt
	ParentT3ThreadID string // Legacy field retained; T3 proof must use independent T3Read/T3List.
	Provider         string
	Source           string
	CheckedAt        time.Time
	SourceCWD        string
}
type DispatchVerification struct {
	Created          bool     `json:"created"`
	TrackingVerified bool     `json:"tracking_verified"`
	Started          bool     `json:"started"` // exclusively existing runtime START evidence
	Status           string   `json:"status"`
	Reasons          []string `json:"reasons"`
	Owner            string   `json:"owner,omitempty"`
	ResumeWhen       string   `json:"resume_when,omitempty"`
	NextAction       string   `json:"next_action"`
}

// CheckDispatchBoundary performs no dispatch. The host must call it immediately
// before its actual tool call, with its clock (never a caller-supplied JSON time).
func CheckDispatchBoundary(ctx context.Context, host HostAdapter, req DispatchRequest, mode Mode, clock func() time.Time) (DispatchPlan, error) {
	if host == nil {
		return DispatchPlan{}, errors.New("actual host adapter required")
	}
	if clock == nil {
		clock = time.Now
	}
	started := clock()
	e, err := host.ObserveEnvironment(ctx)
	if err != nil {
		return DispatchPlan{}, err
	}
	initial := PlanDispatch(e, req, mode, clock())
	if !initial.Allowed {
		return initial, errors.New("dispatch environment blocked; inspect reasons and resume conditions")
	}
	a, err := host.FullAccess(ctx)
	now := clock()
	p := PlanDispatch(e, req, mode, now)
	if !p.Allowed {
		return p, errors.New("dispatch environment blocked; inspect reasons and resume conditions")
	}
	if e.IdentityLinkage != "verified" {
		return p, errors.New("independent exact Root identity/linkage observation required")
	}
	// A boundary proof must belong to this check, rather than the profile TTL.
	if err != nil || !a.Passed || a.CanonicalProject != req.CanonicalProject || a.Source == "" || a.CheckedAt.Before(started) || a.CheckedAt.After(now) {
		return p, errors.New("fresh Full Access proof required at dispatch boundary")
	}
	if e.CheckedAt.Before(started) || e.Tools[p.Tool].CheckedAt.Before(started) {
		return p, errors.New("fresh environment and current tool support required at dispatch boundary")
	}
	if e.Host == "t3" && e.Creator.CheckedAt.Before(started) {
		return p, errors.New("fresh actual creator identity proof required at dispatch boundary")
	}
	return p, nil
}

func dispatchName(req DispatchRequest) (string, error) {
	g := Group{GroupID: req.GroupID, DisplayPrefix: req.GroupID + "_"}
	return SessionName(g, req.Role, req.Worker, req.Generation)
}

// VerifyCreation never accepts caller JSON as observation. Created-but-unlinked
// remains a partial result, never a technical interruption or START claim.
func VerifyCreation(ctx context.Context, host HostAdapter, req DispatchRequest, plan DispatchPlan, receipt CreationReceipt, now time.Time) (DispatchVerification, error) {
	v := DispatchVerification{Status: "blocked", Reasons: []string{}, NextAction: "obtain_actual_creation_receipt", Owner: "dispatching_host"}
	// A successful creation receipt is retained even if later native/linkage proof
	// is unavailable. No failure below authorizes creating a replacement session.
	v.Created = receipt.T3ThreadID != "" || receipt.NativeSessionID != ""
	if v.Created {
		v.Status, v.NextAction = "created_tracking_unverified", "reconcile_same_receipt"
	}
	fail := func(reason string) (DispatchVerification, error) {
		v.Reasons = append(v.Reasons, reason)
		v.ResumeWhen = "Dispatching host obtains fresh independent official evidence for the same exact receipt; preserve assignment/START/claim separately. Do not spawn again or fall back."
		return v, errors.New(reason)
	}
	if host == nil || !plan.Allowed {
		return fail("actual_host_and_allowed_plan_required")
	}
	if req.T3ThreadID != "" && plan.Tool != "delegate_task" {
		return fail("t3_managed_route_required")
	}
	expectedIsolation := "shared"
	if req.SeparateWorktree {
		expectedIsolation = "worktree"
	}
	if plan.Isolation != expectedIsolation {
		return fail("requested_isolation_mismatch")
	}
	if receipt.NativeSessionID == "" || receipt.Worktree == "" || receipt.Base != req.Base {
		return fail("exact_creation_native_cwd_base_required")
	}
	o, err := host.ReadCreation(ctx, receipt)
	if err != nil {
		return fail("official_creation_read_failed: " + err.Error())
	}
	if o.Source == "" || !fresh(o.CheckedAt, now) || o.Provider != req.Provider || o.SourceCWD != req.SourceCWD || o.CreationReceipt != receipt {
		return fail("creation_observation_mismatch_or_stale")
	}
	if !filepath.IsAbs(receipt.Worktree) {
		return fail("absolute_created_workspace_required")
	}
	if plan.Isolation == "worktree" && receipt.Worktree == req.SourceCWD {
		return fail("separate_worktree_was_not_created")
	}
	if plan.Isolation == "shared" && receipt.Worktree != req.SourceCWD {
		return fail("shared_workspace_mismatch")
	}
	if plan.Tool == "delegate_task" {
		expected, err := dispatchName(req)
		if err != nil {
			return fail("supported_role_naming_directive_required")
		}
		if receipt.T3ThreadID == "" || receipt.T3ThreadID == req.CreatorT3ThreadID || receipt.T3ThreadID == req.T3ThreadID || req.CreatorT3ThreadID == "" || req.T3ProjectID == "" {
			return fail("distinct_child_and_explicit_creator_required")
		}
		for _, proof := range []*ThreadObservation{o.T3Read, o.T3List} {
			if proof == nil || proof.Source == "" || !fresh(proof.CheckedAt, now) {
				return fail("official_exact_thread_read_and_list_required")
			}
			if proof.ThreadID != receipt.T3ThreadID || proof.ProjectID != req.T3ProjectID || proof.Relationship != "subagent" || proof.Title != expected {
				return fail("official_child_project_relationship_or_name_mismatch")
			}
			// Either official surface may omit parent. Absence is not a match; any
			// explicit contradiction blocks, and at least one explicit match is needed.
			if proof.ParentThreadID != "" && proof.ParentThreadID != req.CreatorT3ThreadID {
				return fail("actual_creator_parent_mismatch")
			}
			if proof.Workspace != "" && proof.Workspace != receipt.Worktree {
				return fail("official_workspace_mismatch")
			}
		}
		if o.T3Read.ParentThreadID == "" && o.T3List.ParentThreadID == "" {
			return fail("official_parent_unknown")
		}
		if o.T3Read.Workspace == "" && o.T3List.Workspace == "" {
			return fail("official_workspace_unknown")
		}
		n := o.NativeLink
		if n == nil || n.Source == "" || !fresh(n.CheckedAt, now) || n.Kind != "provider_native_session" || n.NativeSessionID != receipt.NativeSessionID || n.T3ThreadID != receipt.T3ThreadID || n.Provider != req.Provider || n.NativeSessionID == req.RootSessionID || n.NativeSessionID == req.CreatorNativeID || n.NativeSessionID == receipt.T3ThreadID {
			return fail("provider_native_link_unverified_or_wrong_identity_kind")
		}
		v.TrackingVerified, v.Status = true, "created_tracking_verified"
	} else {
		v.Status = "created_native_verified_t3_visibility_limited"
	}
	v.NextAction, v.Owner = "use_existing_start_binding_and_handoff", ""
	return v, nil
}
