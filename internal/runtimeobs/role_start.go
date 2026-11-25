package runtimeobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func sourceControllerClaim(task, native string, p *rolepolicy.Policy, r T3Receipt, attempts []Attempt, claims map[string]handoffjournal.Handoff) bool {
	if p == nil || p.Operational.NativeID != native || p.Operational.AgentPath != "/root/controller" || r.State != StateRunning || r.Provider != "codex" || r.SessionID != native {
		return false
	}
	for _, a := range attempts {
		if a.RuntimeAgentID != native || a.SessionID != native || a.AgentPath != "/root/controller" || a.BindingState != BindingBound || a.Terminal || a.CurrentState != StateRunning || !HasObservedExecution(a) || a.NativeReceipt == nil || (a.TaskID != "" && a.TaskID != task) {
			continue
		}
		if a.NativeReceipt.RunID != r.RunID || a.NativeReceipt.NativeAttemptID != r.NativeAttemptID || a.NativeReceipt.ThreadID != r.ThreadID {
			continue
		}
		for _, h := range claims {
			if h.TaskID == task && h.ClaimedBy == "/root/controller" && h.ClaimedAttemptID == a.AttemptID && h.ClaimedAt != "" && !h.Applied && h.Target.AgentPath == "/root/controller" && (h.Target.RuntimeAgentID == "" || h.Target.RuntimeAgentID == native) {
				return true
			}
		}
	}
	return false
}

func sourceWorkerCurrent(task, path, native, workspace string, p *rolepolicy.Policy, exceptionApproved bool, r T3Receipt, attempts []Attempt, assignments []Assignment) bool {
	if r.State != StateRunning || r.SessionID != native || p == nil || rolepolicy.Role(path) != "worker" {
		return false
	}
	for _, a := range attempts {
		if a.TaskID != task || a.AgentPath != path || a.RuntimeAgentID != native || a.BindingState != BindingBound || a.Terminal || a.CurrentState != StateRunning || !HasObservedExecution(a) || a.NativeReceipt == nil {
			continue
		}
		if a.NativeReceipt.RunID != r.RunID || a.NativeReceipt.NativeAttemptID != r.NativeAttemptID || a.NativeReceipt.ThreadID != r.ThreadID {
			continue
		}
		id := a.BindingEvidence["assignment_id"]
		if id == "" {
			continue
		}
		matches := 0
		for _, assignment := range assignments {
			if assignment.AssignmentID == id && assignment.TaskID == task && assignment.AgentPath == path {
				matches++
			}
		}
		if matches == 1 && rolepolicy.Evaluate(p, PolicyIdentity(a), workspace, exceptionApproved).Allowed {
			return true
		}
	}
	return false
}

func collectionSourcePath(c CollectionProject, native string, r T3Receipt, ledger Ledger, now time.Time) (string, error) {
	path := ""
	for _, a := range ledger.Attempts {
		if a.TaskID == c.SourceTask && a.RuntimeAgentID == native && a.BindingState == BindingBound && HasObservedExecution(a) {
			if path != "" && path != a.AgentPath {
				return "", errors.New("ambiguous source actor role")
			}
			path = a.AgentPath
		}
	}
	if path == "" || path == "/root/controller" {
		raw, err := rolepolicy.TaskMarkdown(c.Canonical, c.SourceTask)
		if err != nil {
			return "", err
		}
		p, _, err := rolepolicy.Load(c.Canonical, c.SourceTask, raw)
		if err != nil {
			return "", err
		}
		claims, err := handoffjournal.BuildLedger(c.Canonical, now)
		if err != nil {
			return "", err
		}
		if sourceControllerClaim(c.SourceTask, native, p, r, ledger.Attempts, claims.Handoffs) {
			return "/root/controller", nil
		}
		if path == "/root/controller" {
			return "", errors.New("current source Controller handoff claim required")
		}
	}
	if path == "" {
		return "", errors.New("actual source-task role binding or exact current operational Controller claim required")
	}
	if rolepolicy.Role(path) == "worker" {
		raw, err := rolepolicy.TaskMarkdown(c.Canonical, c.SourceTask)
		if err != nil {
			return "", err
		}
		p, approved, err := rolepolicy.Load(c.Canonical, c.SourceTask, raw)
		if err != nil {
			return "", err
		}
		assignments, err := ListAssignments(c.Canonical)
		if err != nil {
			return "", err
		}
		if !sourceWorkerCurrent(c.SourceTask, path, native, c.Owner.SourceWorktree, p, approved, r, ledger.Attempts, assignments) {
			return "", errors.New("exact current source Worker episode/assignment/policy required; historical initializer is provenance only")
		}
	}
	return path, nil
}

// CollectionProject is an explicitly isolated evidence destination. It never
// changes the host project, native identity, role, or implementation consent.
// Its marker is only a reference: the actual source owner and host scope are
// revalidated against the canonical ledger and live T3 on every use.
type CollectionProject struct {
	Canonical        string    `json:"canonical_project"`
	Destination      string    `json:"destination"`
	SourceTask       string    `json:"source_task"`
	Owner            RoleStart `json:"owner"`
	DeliveryDisabled bool      `json:"delivery_disabled"`
	MonitorDisabled  bool      `json:"monitor_disabled"`
}

func (c CollectionProject) grantPath() string {
	sum := sha256.Sum256([]byte(c.Destination))
	return filepath.Join(c.Owner.SourceWorktree, ".artifacts", ".collection-grants", fmt.Sprintf("%x.json", sum))
}

func collectionPath(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("exact absolute collection path required")
	}
	for p := path; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && st.Mode()&os.ModeSymlink != 0 {
			return errors.New("collection path symlink refused")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func (c CollectionProject) verifyOwner(now time.Time) error {
	if !c.DeliveryDisabled || !c.MonitorDisabled || c.SourceTask == "" || c.Owner.TaskID != c.SourceTask || c.Canonical != c.Owner.CanonicalProject {
		return errors.New("isolated collection policy missing")
	}
	root := filepath.Join(c.Owner.SourceWorktree, ".artifacts")
	if filepath.Dir(c.Destination) != root || !strings.HasPrefix(filepath.Base(c.Destination), "collection-fixture-") || c.Destination == c.Canonical {
		return errors.New("collection destination must be a dedicated owned fixture under source .artifacts")
	}
	if err := collectionPath(c.Destination); err != nil {
		return err
	}
	store, err := localT3Store()
	if err != nil {
		return err
	}
	if err = store.verifyRoleScope(c.Owner); err != nil {
		return err
	}
	ledger, err := BuildLedger(c.Canonical, 0, now)
	if err != nil {
		return err
	}
	var owner *Attempt
	for i := range ledger.Attempts {
		a := &ledger.Attempts[i]
		if a.TaskID == c.SourceTask && a.RuntimeAgentID == c.Owner.Selector.SessionID && a.BindingState == BindingBound && a.AgentPath == c.Owner.AgentPath && a.BindingEvidence["assignment_id"] == c.Owner.AssignmentID {
			if owner == nil || a.LastObservedAt > owner.LastObservedAt {
				owner = a
			}
		}
	}
	if owner == nil || !HasObservedExecution(*owner) || owner.NativeReceipt == nil || owner.NativeReceipt.ThreadID != c.Owner.Selector.ThreadID {
		return errors.New("canonical observed fixture owner required")
	}
	raw, err := rolepolicy.TaskMarkdown(c.Canonical, c.SourceTask)
	if err != nil {
		return err
	}
	policy, approved, err := rolepolicy.Load(c.Canonical, c.SourceTask, raw)
	if err != nil {
		return err
	}
	if policy == nil || !rolepolicy.Evaluate(policy, PolicyIdentity(*owner), c.Owner.SourceWorktree, approved).Allowed {
		return errors.New("current source Worker authorization required for fixture owner")
	}
	return nil
}

func CreateCollectionProject(destination string, owner RoleStart, now time.Time) (CollectionProject, error) {
	c := CollectionProject{Canonical: owner.CanonicalProject, Destination: destination, SourceTask: owner.TaskID, Owner: owner, DeliveryDisabled: true, MonitorDisabled: true}
	if owner.Role != "worker" || owner.Selector.SessionID != os.Getenv("CODEX_THREAD_ID") || owner.Selector.ThreadID != os.Getenv("T3CODE_THREAD_ID") {
		return c, errors.New("only the current authorized source Worker can initialize its isolated fixture")
	}
	if err := c.verifyOwner(now); err != nil {
		return c, err
	}
	current, err := ReadT3Execution(T3Selector{CanonicalProject: c.Canonical, ThreadID: owner.Selector.ThreadID, SessionID: owner.Selector.SessionID, RuntimeAgentID: owner.Selector.SessionID}, now)
	if err != nil {
		return c, err
	}
	if current.State != StateRunning {
		return c, errors.New("fixture initialization requires current running owner")
	}
	c.Owner.Selector.RunID = current.RunID
	c.Owner.Selector.NativeAttemptID = current.NativeAttemptID
	grant := c.grantPath()
	if err := collectionPath(grant); err != nil {
		return c, err
	}
	if _, err := os.Lstat(grant); !os.IsNotExist(err) {
		return c, errors.New("collection initialization grant already exists or unavailable")
	}
	// Exclusive creation: an existing directory, copied marker, or operating
	// project is never initialized or adopted by this command.
	if err := os.Mkdir(destination, 0700); err != nil {
		return c, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return c, err
	}
	f, err := os.OpenFile(filepath.Join(destination, ".collection-project.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return c, err
	}
	_, err = f.Write(raw)
	closeErr := f.Close()
	if err != nil {
		return c, err
	}
	if closeErr != nil {
		return c, closeErr
	}
	// The source-owned initialization grant is outside the destination. A
	// copied/edited marker cannot create or relocate this provenance record.
	if err := os.MkdirAll(filepath.Dir(grant), 0700); err != nil {
		return c, err
	}
	g, err := os.OpenFile(grant, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return c, err
	}
	_, err = g.Write(raw)
	if err == nil {
		err = g.Sync()
	}
	closeErr = g.Close()
	if err != nil {
		return c, err
	}
	if closeErr != nil {
		return c, closeErr
	}
	err = os.MkdirAll(filepath.Join(destination, "_task_mecca", ".runtime"), 0700)
	return c, err
}

func ReadCollectionProject(canonical, destination string, now time.Time) (CollectionProject, error) {
	var c CollectionProject
	if err := collectionPath(destination); err != nil {
		return c, err
	}
	p := filepath.Join(destination, ".collection-project.json")
	st, err := os.Lstat(p)
	if err != nil {
		return c, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > 32768 {
		return c, errors.New("unsafe collection marker")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	if c.Canonical != canonical || c.Destination != destination {
		return c, errors.New("collection project identity mismatch")
	}
	if err := collectionPath(c.grantPath()); err != nil {
		return c, err
	}
	grant, err := os.ReadFile(c.grantPath())
	if err != nil {
		return c, fmt.Errorf("source-owned initialization grant unavailable: %w", err)
	}
	if !bytes.Equal(raw, grant) {
		return c, errors.New("collection marker differs from source-owned initialization grant")
	}
	return c, c.verifyOwner(now)
}

// VerifyCollectionActor performs no writes. A fixture marker cannot supply
// actor authority: own native, live host parent/project/source and formal native
// role metadata must all agree before destination ledgers are touched.
func VerifyCollectionActor(canonical, destination string, in RoleStart, now time.Time) (CollectionProject, error) {
	c, err := ReadCollectionProject(canonical, destination, now)
	if err != nil {
		return c, err
	}
	if in.CanonicalProject != canonical || in.Selector.SessionID != os.Getenv("CODEX_THREAD_ID") || in.Selector.ThreadID != os.Getenv("T3CODE_THREAD_ID") {
		return c, errors.New("exact own collection actor required")
	}
	store, err := localT3Store()
	if err != nil {
		return c, err
	}
	if err = store.verifyRoleScope(in); err != nil {
		return c, err
	}
	r, err := ReadT3Execution(T3Selector{CanonicalProject: canonical, ThreadID: in.Selector.ThreadID, SessionID: in.Selector.SessionID, RuntimeAgentID: in.Selector.SessionID}, now)
	if err != nil {
		return c, err
	}
	if r.State != StateRunning {
		return c, errors.New("current running collection actor required")
	}
	patterns := map[string]string{"worker": "Worker_[A-Za-z0-9]+_G[1-9][0-9]*", "controller": "Controller", "registrar": "Registrar"}
	pattern, ok := patterns[in.Role]
	if !ok || !(rolepolicy.Role(in.AgentPath) == in.Role || in.Role == "registrar" && in.AgentPath == "/root/registrar") {
		return c, errors.New("collection role/path mismatch")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	m, err := ResolveCodexThreadMetadata(ctx, in.Selector.SessionID)
	if err != nil {
		return c, err
	}
	if m.ID != in.Selector.SessionID || !regexp.MustCompile(`^D[0-9]{4}_[0-9]{2}_`+pattern+`$`).MatchString(m.Name) {
		return c, errors.New("native collection role mismatch")
	}
	if in.Role == "worker" && in.Selector.SessionID != c.Owner.Selector.SessionID {
		return c, errors.New("fixture Worker must be its authorized source owner")
	}
	ledger, err := BuildLedger(canonical, 0, now)
	if err != nil {
		return c, err
	}
	actualPath, err := collectionSourcePath(c, in.Selector.SessionID, r, ledger, now)
	if err != nil {
		return c, err
	}
	if actualPath != in.AgentPath {
		return c, errors.New("actual source-task role binding required; native rename cannot upgrade a Worker")
	}
	if in.Role == "controller" {
		raw, e := rolepolicy.TaskMarkdown(canonical, c.SourceTask)
		if e != nil {
			return c, e
		}
		p, _, e := rolepolicy.Load(canonical, c.SourceTask, raw)
		if e != nil {
			return c, e
		}
		if p == nil || p.Operational.NativeID != in.Selector.SessionID {
			return c, errors.New("fixture Controller must be the actual source operational owner")
		}
	}
	return c, nil
}

// CollectionActor derives its lineage from the official host store and its
// role from the existing canonical source-task binding, never from a marker
// actor list or a caller's claimed role.
func CollectionActor(canonical, destination string, now time.Time) (RoleStart, error) {
	in := RoleStart{CanonicalProject: canonical, Selector: T3Selector{CanonicalProject: canonical, ThreadID: os.Getenv("T3CODE_THREAD_ID"), SessionID: os.Getenv("CODEX_THREAD_ID"), RuntimeAgentID: os.Getenv("CODEX_THREAD_ID")}}
	c, err := ReadCollectionProject(canonical, destination, now)
	if err != nil {
		return in, err
	}
	in.SourceWorktree, err = os.Getwd()
	if err != nil {
		return in, err
	}
	store, err := localT3Store()
	if err != nil {
		return in, err
	}
	raw, err := store.query(`SELECT json_extract(payload_json,'$.lineage.parentThreadId') AS parent FROM orchestration_v2_projection_threads WHERE thread_id=` + sqlLiteral(in.Selector.ThreadID) + `;`)
	if err != nil {
		return in, err
	}
	var parents []struct {
		Parent string `json:"parent"`
	}
	if err = json.Unmarshal(raw, &parents); err != nil || len(parents) != 1 {
		return in, errors.New("actual collection parent unavailable")
	}
	in.ParentThreadID = parents[0].Parent
	ledger, err := BuildLedger(canonical, 0, now)
	if err != nil {
		return in, err
	}
	r, err := ReadT3Execution(in.Selector, now)
	if err != nil {
		return in, err
	}
	in.AgentPath, err = collectionSourcePath(c, in.Selector.SessionID, r, ledger, now)
	if err != nil {
		return in, err
	}
	in.Role = rolepolicy.Role(in.AgentPath)
	if in.AgentPath == "/root/registrar" {
		in.Role = "registrar"
	}
	_, err = VerifyCollectionActor(canonical, destination, in, now)
	return in, err
}

// RoleStart is an explicit formal collection request, not creation, delivery,
// claim, or implementation permission. Parent identity is independently checked
// against the live host store; caller metadata alone is never a receipt.
type RoleStart struct {
	CanonicalProject string     `json:"canonical_project"`
	SourceWorktree   string     `json:"source_worktree"`
	TaskID           string     `json:"task_id"`
	AssignmentID     string     `json:"assignment_id"`
	Role             string     `json:"role"`
	AgentPath        string     `json:"agent_path"`
	ParentThreadID   string     `json:"parent_thread_id"`
	Selector         T3Selector `json:"selector"`
}

func ObserveT3RoleStart(in RoleStart, now time.Time) (WorkerStartResult, error) {
	if in.Role == "registrar" && in.AssignmentID == "" {
		return observeRegistrarPreparation(in, now)
	}
	store, err := localT3Store()
	if err != nil {
		return WorkerStartResult{Status: "collection_gap", Stage: "scope", Diagnostic: err.Error()}, err
	}
	return observeRoleCollection(in, now, store.verifyRoleScope, ObserveT3Execution)
}

func observeRegistrarPreparation(in RoleStart, now time.Time) (WorkerStartResult, error) {
	out := WorkerStartResult{Status: "collection_gap", Stage: "registrar_preparation", ReceivedAt: now.UTC().Format(time.RFC3339Nano)}
	fail := func(err error) (WorkerStartResult, error) { out.Diagnostic = err.Error(); return out, err }
	if in.Role != "registrar" || in.AgentPath != "/root/registrar" || in.SourceWorktree == "" || in.Selector.SessionID != os.Getenv("CODEX_THREAD_ID") || in.Selector.ThreadID != os.Getenv("T3CODE_THREAD_ID") {
		return fail(errors.New("exact own Registrar preparation identity required"))
	}
	store, err := localT3Store()
	if err != nil {
		return fail(err)
	}
	if err = store.verifyRoleScope(in); err != nil {
		return fail(err)
	}
	// The dedicated Registrar is a Root child, not a renamed implementation
	// child of the Controller. Native display names alone are insufficient.
	raw, err := store.query(`SELECT json_extract(payload_json,'$.lineage.parentThreadId') AS parent FROM orchestration_v2_projection_threads WHERE thread_id=` + sqlLiteral(in.ParentThreadID) + `;`)
	if err != nil {
		return fail(err)
	}
	var parents []struct {
		Parent *string `json:"parent"`
	}
	if err = json.Unmarshal(raw, &parents); err != nil || len(parents) != 1 || (parents[0].Parent != nil && *parents[0].Parent != "") {
		return fail(errors.New("dedicated Registrar requires its actual Root parent"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	m, err := ResolveCodexThreadMetadata(ctx, in.Selector.SessionID)
	if err != nil {
		return fail(err)
	}
	if m.ID != in.Selector.SessionID || !regexp.MustCompile(`^D[0-9]{4}_[0-9]{2}_Registrar$`).MatchString(m.Name) {
		return fail(errors.New("actual Registrar native name required"))
	}
	if in.TaskID != "" {
		if _, err := rolepolicy.TaskMarkdown(in.CanonicalProject, in.TaskID); err != nil {
			return fail(err)
		}
	}
	in.Selector.CanonicalProject = in.CanonicalProject
	r, err := ReadT3Execution(in.Selector, now)
	if err != nil {
		return fail(err)
	}
	if r.State != StateRunning {
		return fail(errors.New("Registrar must be currently running"))
	}
	out.Attempt, out.Receipt, err = ObserveT3Execution(in.CanonicalProject, in.Selector, now)
	if err != nil {
		return fail(err)
	}
	if in.TaskID != "" {
		out.Attempt, err = BindAttempt(in.CanonicalProject, out.Attempt.AttemptID, in.TaskID, in.AgentPath, "explicit", "", map[string]string{"origin": "registrar_preparation", "runtime_agent_id": in.Selector.SessionID}, now)
		if err != nil {
			return fail(err)
		}
	}
	out.Status = "observed"
	if in.TaskID != "" {
		out.Status = "bound"
	}
	out.Stage = "current_phase_required"
	return out, nil
}

func observeRoleCollection(in RoleStart, now time.Time, scope func(RoleStart) error, observe func(string, T3Selector, time.Time) (Attempt, T3Receipt, error)) (WorkerStartResult, error) {
	legacy := WorkerStart{in.CanonicalProject, in.SourceWorktree, in.TaskID, in.AssignmentID, in.AgentPath, in.Selector}
	fail := func(stage, reason string) (WorkerStartResult, error) {
		return WorkerStartResult{Status: "collection_gap", Stage: stage, Context: legacy, ReceivedAt: now.UTC().Format(time.RFC3339Nano), Diagnostic: reason}, errors.New(reason)
	}
	validRolePath := rolepolicy.Role(in.AgentPath) == in.Role || (in.Role == "registrar" && in.AgentPath == "/root/registrar")
	if (in.Role != "controller" && in.Role != "worker" && in.Role != "reviewer" && in.Role != "registrar") || !validRolePath {
		return fail("role", "role_path_mismatch: controller requires /root/controller; registrar requires /root/registrar; worker requires a direct Worker child path")
	}
	if in.ParentThreadID == "" || in.ParentThreadID == in.Selector.ThreadID {
		return fail("scope", "parent_identity_required: use exact host parent thread, never native ID or display name")
	}
	if in.CanonicalProject == "" || in.SourceWorktree == "" || in.TaskID == "" || in.AssignmentID == "" || in.Selector.ThreadID == "" || in.Selector.SessionID == "" || in.Selector.RuntimeAgentID != in.Selector.SessionID || in.Selector.ThreadID == in.Selector.SessionID || strings.HasPrefix(in.Selector.SessionID, "thread:") || strings.HasPrefix(in.Selector.ThreadID, "thread:provider:") {
		return fail("identity", "identity_kind_mismatch_or_missing: exact managed T3 and distinct provider-native identities required; provider wrappers are not managed creation proof")
	}
	if err := scope(in); err != nil {
		return fail("scope", err.Error())
	}
	verifiedObserve := func(p string, s T3Selector, n time.Time) (Attempt, T3Receipt, error) {
		if err := scope(in); err != nil {
			return Attempt{}, T3Receipt{}, err
		}
		a, r, err := observe(p, s, n)
		if err == nil {
			err = scope(in) // changed host scope cannot authorize binding
		}
		return a, r, err
	}
	if in.Role == "registrar" {
		return observeRegistrarCollection(legacy, now, verifiedObserve)
	}
	return observeRoleStart(legacy, now, verifiedObserve, in.Role == "controller")
}

// Registration is operational attribution, not Worker implementation consent.
// Require an existing exact task/Registrar assignment, managed parent/project/
// repository scope, and the current structured receipt. Never bind by a title,
// registration timestamp, an old run, or a caller-supplied execution object.
func observeRegistrarCollection(in WorkerStart, now time.Time, observe func(string, T3Selector, time.Time) (Attempt, T3Receipt, error)) (WorkerStartResult, error) {
	out := WorkerStartResult{Status: "collection_gap", Stage: "assignment", Context: in, ReceivedAt: now.UTC().Format(time.RFC3339Nano)}
	fail := func(err error) (WorkerStartResult, error) { out.Diagnostic = err.Error(); return out, err }
	project, err := ResolveProject(in.CanonicalProject)
	if err != nil {
		return fail(err)
	}
	project, err = exactDirectory(project)
	if err != nil {
		return fail(err)
	}
	requested, err := exactDirectory(in.CanonicalProject)
	if err != nil || requested != project {
		return fail(errors.New("--project must name the canonical project root explicitly"))
	}
	a, err := FindAssignment(project, in.AssignmentID)
	if err != nil {
		return fail(err)
	}
	if a.TaskID != in.TaskID || a.AgentPath != "/root/registrar" || in.WorkerPath != a.AgentPath {
		return fail(errors.New("Registrar task/path does not match exact canonical assignment"))
	}
	out.Stage = "observation"
	out.Attempt, out.Receipt, err = observe(project, in.Selector, now)
	if err != nil {
		return fail(err)
	}
	r := out.Receipt
	if r.ThreadID != in.Selector.ThreadID || r.SessionID != in.Selector.SessionID || out.Attempt.RuntimeAgentID != in.Selector.RuntimeAgentID || out.Attempt.NativeReceipt == nil || *out.Attempt.NativeReceipt != r || r.Provider != "codex" || r.RunID == "" || r.NativeAttemptID == "" || (in.Selector.RunID != "" && r.RunID != in.Selector.RunID) || r.State != StateRunning || out.Attempt.Terminal {
		return fail(errors.New("observed native/T3 execution does not match current Registrar START"))
	}
	out.Stage = "binding"
	bound, err := BindRuntimeAgentForAssignment(project, in.AssignmentID, in.Selector.RuntimeAgentID, "", now)
	if err != nil {
		return fail(err)
	}
	if bound.AttemptID != out.Attempt.AttemptID || bound.BindingState != BindingBound || bound.TaskID != in.TaskID || bound.AgentPath != "/root/registrar" || bound.BindingEvidence["assignment_id"] != in.AssignmentID {
		return fail(errors.New("exact observed Registrar attempt was not bound"))
	}
	out.Attempt, out.Status, out.Stage = bound, "bound", "activity_reporting"
	return out, nil
}

// Scope comes from the same schema-bound, ownership-checked read-only adapter
// as execution. No caller JSON, DB path, title or inferred UUID is accepted.
func (s t3Store) verifyRoleScope(in RoleStart) error {
	query := `SELECT t.project_id,json_extract(t.payload_json,'$.lineage.parentThreadId') AS parent_id,
json_extract(t.payload_json,'$.lineage.relationshipToParent') AS relationship,
json_extract(t.payload_json,'$.worktreePath') AS worktree,p.workspace_root,
parent.project_id AS parent_project,parent.deleted_at AS parent_deleted,parent.archived_at AS parent_archived
FROM orchestration_v2_projection_threads t JOIN projection_projects p ON p.project_id=t.project_id
JOIN orchestration_v2_projection_threads parent ON parent.thread_id=json_extract(t.payload_json,'$.lineage.parentThreadId')
WHERE t.deleted_at IS NULL AND t.archived_at IS NULL AND p.deleted_at IS NULL AND p.archived_at IS NULL AND t.thread_id=` + sqlLiteral(in.Selector.ThreadID) + `;`
	raw, err := s.query(query)
	if err != nil {
		return fmt.Errorf("host_scope_unavailable: %w; update the supported host bridge, never inject receipt JSON", err)
	}
	var rows []struct {
		Project       string  `json:"project_id"`
		Parent        string  `json:"parent_id"`
		Relationship  string  `json:"relationship"`
		Worktree      string  `json:"worktree"`
		WorkspaceRoot string  `json:"workspace_root"`
		ParentProject string  `json:"parent_project"`
		ParentDeleted *string `json:"parent_deleted"`
		ParentArchive *string `json:"parent_archived"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return fmt.Errorf("host_scope_invalid: %w", err)
	}
	if len(rows) != 1 || rows[0].Project == "" || rows[0].Parent != in.ParentThreadID || rows[0].Relationship != "subagent" || rows[0].ParentProject != rows[0].Project || rows[0].ParentDeleted != nil || rows[0].ParentArchive != nil {
		return errors.New("host_parent_or_project_mismatch: re-read exact managed child and parent in the intended project")
	}
	r := rows[0]
	canonical, err := exactDirectory(in.CanonicalProject)
	if err != nil {
		return fmt.Errorf("canonical_project_invalid: %w", err)
	}
	hostRoot, err := exactDirectory(r.WorkspaceRoot)
	if err != nil || hostRoot != canonical {
		return errors.New("host_project_mismatch: --project must be the host project's canonical workspace root")
	}
	source, err := exactDirectory(in.SourceWorktree)
	if err != nil {
		return fmt.Errorf("source_worktree_invalid: %w", err)
	}
	worktree := r.Worktree
	if worktree == "" { // supported shared thread uses project workspace
		worktree = r.WorkspaceRoot
	}
	hostSource, err := exactDirectory(worktree)
	if err != nil || hostSource != source {
		return errors.New("host_worktree_mismatch: use actual source cwd, never the canonical ledger as a substitute")
	}
	// A path matching a host payload is insufficient if it points to another
	// repository. Read-only Git identity preserves both separate and shared mode.
	common := func(dir string) (string, error) {
		cmd := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
		b, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return exactDirectory(strings.TrimSpace(string(b)))
	}
	a, err := common(canonical)
	if err != nil {
		return fmt.Errorf("canonical_repository_unverified: %w", err)
	}
	b, err := common(source)
	if err != nil || a != b {
		return errors.New("source_repository_mismatch: source and canonical project must share Git identity")
	}
	return nil
}

func exactDirectory(p string) (string, error) {
	if !filepath.IsAbs(p) {
		return "", errors.New("absolute path required")
	}
	return filepath.EvalSymlinks(filepath.Clean(p))
}
