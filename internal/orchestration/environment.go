package orchestration

// Environment evidence is deliberately separate from a saved mode and from
// execution receipts. JSON profiles are useful policy inputs, never authority.
import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"time"
)

type ToolEvidence struct {
	Support   string    `json:"support"` // supported, unsupported, unknown
	Source    string    `json:"source"`
	CheckedAt time.Time `json:"checked_at"`
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
	IdentityLinkage     string                  `json:"identity_linkage"` // host official read: verified|unknown|unsupported
}

type DispatchRequest struct {
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
	if req.SeparateWorktree && e.Host == "t3" {
		if info.Execution != "subagent" {
			return block("selected_session_mode_has_no_verified_t3_worktree_route")
		}
		p.Tool, p.Isolation = "delegate_task", "worktree"
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
		p.VerificationMethod = "Read the exact returned T3 thread via official t3_thread_read, verify parent Root/Controller linkage plus native identity, actual worktree cwd and committed base with independent host observations."
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
type CreationObservation struct {
	CreationReceipt
	ParentT3ThreadID string
	Provider         string
	Source           string
	CheckedAt        time.Time
	SourceCWD        string
}
type DispatchVerification struct {
	Created          bool   `json:"created"`
	TrackingVerified bool   `json:"tracking_verified"`
	Started          bool   `json:"started"` // exclusively existing runtime START evidence
	Status           string `json:"status"`
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
	return p, nil
}

// VerifyCreation never accepts caller JSON as observation. Created-but-unlinked
// remains a partial result, never a technical interruption or START claim.
func VerifyCreation(ctx context.Context, host HostAdapter, req DispatchRequest, plan DispatchPlan, receipt CreationReceipt, now time.Time) (DispatchVerification, error) {
	v := DispatchVerification{Status: "unverified"}
	if host == nil || !plan.Allowed || receipt.NativeSessionID == "" || receipt.Worktree == "" || receipt.Base != req.Base {
		return v, errors.New("exact creation identities/cwd/base and host required")
	}
	o, err := host.ReadCreation(ctx, receipt)
	if err != nil {
		return v, err
	}
	if o.Source == "" || !fresh(o.CheckedAt, now) || o.Provider != req.Provider || o.SourceCWD != req.SourceCWD || o.CreationReceipt != receipt {
		return v, errors.New("creation observation mismatch or stale")
	}
	if plan.Isolation == "worktree" && receipt.Worktree == req.SourceCWD {
		return v, errors.New("separate worktree was not created")
	}
	v.Created, v.Status = true, "created_tracking_unobserved"
	if plan.Tool == "delegate_task" {
		if receipt.T3ThreadID == "" || o.ParentT3ThreadID != req.T3ThreadID {
			return v, errors.New("created but T3 linkage missing or mismatched")
		}
		v.TrackingVerified, v.Status = true, "created_tracking_verified"
	} else {
		v.Status = "created_native_verified_t3_visibility_limited"
	}
	return v, nil
}
