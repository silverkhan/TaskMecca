package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// No caller timestamp, actor impersonation, native state JSON or automatic
// role/execution approval is accepted at this boundary.
func runRuntimeBoundaryCLI(args []string, out, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "runtime boundary record|list --project DIR --json")
		return 2
	}
	action := args[0]
	f := flag.NewFlagSet("runtime boundary", flag.ContinueOnError)
	f.SetOutput(stderr)
	project := f.String("project", ".", "canonical project")
	operation := f.String("operation", "", "stable registration request/operation ID")
	key := f.String("key", "", "stable boundary key; retain on retry")
	kind := f.String("kind", "", "start|phase|wait|resume|end|task_link|handoff_request|handoff_delivery|handoff_claim|collection_failure")
	role := f.String("role", "", "observed controller|registrar|worker|reviewer")
	phase := f.String("phase", "", "actual current phase")
	thread := f.String("thread-id", "", "exact current T3 thread")
	native := f.String("session-id", "", "exact own native session")
	attempt := f.String("attempt", "", "exact observed runtime attempt")
	task := f.String("task", "", "list task filter")
	tasks := f.String("tasks", "", "comma-separated canonical task associations")
	evidence := f.String("evidence", "", "actual boundary evidence")
	related := f.String("related-id", "", "matching wait key or supported handoff ID")
	originalError := f.String("error", "", "original collection error; never replaces unknown with success")
	scope := f.String("wait-scope", "actor", "actor|task_next_stage")
	reason := f.String("reason", "", "actual wait reason; absent remains unknown")
	expected := f.String("expected-event", "", "expected event")
	owner := f.String("next-owner", "", "next owner")
	last := f.String("last-evidence", "", "last actual evidence")
	resume := f.String("resume-condition", "", "actual resume condition")
	f.Bool("json", false, "JSON response")
	if err := f.Parse(args[1:]); err != nil {
		return 2
	}
	if f.NArg() != 0 {
		return 2
	}
	now := time.Now().UTC()
	if action == "list" {
		detail := activity.ReadExecutionCoverage(*project, *task, activity.Workflow{})
		if err := json.NewEncoder(out).Encode(detail); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if len(detail.CollectionErrors) > 0 {
			return 1
		}
		return 0
	}
	if action != "record" && action != "preview" {
		fmt.Fprintln(stderr, "unsupported boundary action")
		return 2
	}
	if *native == "" || *native != os.Getenv("CODEX_THREAD_ID") || *thread == "" || (os.Getenv("T3CODE_THREAD_ID") != "" && *thread != os.Getenv("T3CODE_THREAD_ID")) {
		fmt.Fprintln(stderr, "exact own current Codex/T3 identity required; unsupported hosts remain unmeasured")
		return 1
	}
	receipt, err := runtimeobs.ReadT3Execution(runtimeobs.T3Selector{CanonicalProject: *project, ThreadID: *thread, SessionID: *native, RuntimeAgentID: *native}, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if receipt.State != runtimeobs.StateRunning {
		fmt.Fprintln(stderr, "current running native execution required; terminal/history is not a current operation")
		return 1
	}
	ledger, err := runtimeobs.BuildLedger(*project, 0, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	matched := false
	for _, a := range ledger.Attempts {
		if a.AttemptID == *attempt && a.SessionID == *native && !a.Terminal && a.NativeReceipt != nil && a.NativeReceipt.NativeAttemptID == receipt.NativeAttemptID && a.NativeReceipt.RunID == receipt.RunID {
			matched = true
			break
		}
	}
	if !matched {
		fmt.Fprintln(stderr, "exact current observed attempt required; this command never self-binds")
		return 1
	}
	// Native role metadata describes this actor, not implementation authority.
	// In particular an unbound pre-registration operation needs no fake task.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	metadata, err := runtimeobs.ResolveCodexThreadMetadata(ctx, *native)
	if err != nil || metadata.ID != *native || !boundaryRoleMatches(*role, metadata.Name) {
		fmt.Fprintf(stderr, "current native role metadata unavailable or mismatched: %v\n", err)
		return 1
	}
	b := runtimeobs.OperationBoundary{Key: *key, OperationID: *operation, Provider: receipt.Provider, NativeID: *native, AttemptID: *attempt, ThreadID: *thread, Role: *role, Kind: *kind, Phase: *phase, RelatedID: *related, Source: "current_native_boundary", Evidence: *evidence, Error: *originalError}
	if *tasks != "" {
		b.TaskIDs = strings.Split(*tasks, ",")
		for i, id := range b.TaskIDs {
			id = strings.TrimSpace(id)
			b.TaskIDs[i] = id
			v, e := backlog.Inspect(*project, "", id)
			if e != nil || v["exists"] != true || v["duplicate_count"] != 1 {
				if e == nil {
					e = errors.New("unique canonical task association required")
				}
				fmt.Fprintln(stderr, e)
				return 1
			}
		}
	}
	if *kind == "wait" {
		b.Wait = &runtimeobs.BoundaryWait{Scope: *scope, Reason: *reason, ExpectedEvent: *expected, NextOwner: *owner, LastEvidence: *last, ResumeCondition: *resume}
	}
	if strings.HasPrefix(*kind, "handoff_") {
		ledger, e := handoffjournal.BuildLedger(*project, now)
		if e != nil {
			fmt.Fprintln(stderr, e)
			return 1
		}
		h, ok := ledger.Handoffs[*related]
		valid := ok && ((*kind == "handoff_request" && h.PreparedAt != "" && h.SourceAttemptID == *attempt) || (*kind == "handoff_delivery" && h.DeliveryStatus == "dispatch_ok" && h.SourceAttemptID == *attempt) || (*kind == "handoff_claim" && h.ClaimedAt != "" && h.ClaimedAttemptID == *attempt))
		if !valid {
			fmt.Fprintln(stderr, "exact supported handoff event and actor attempt required")
			return 1
		}
		b.TaskIDs = append(b.TaskIDs, h.TaskID)
	}
	b, err = runtimeobs.PrepareOperationBoundary(b, now)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if action == "preview" {
		if err := json.NewEncoder(out).Encode(map[string]any{"ok": true, "preview": true, "operational_written": false, "boundary": b, "execution_authorized": false}); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	current, e := runtimeobs.ReadT3Execution(runtimeobs.T3Selector{CanonicalProject: *project, ThreadID: *thread, SessionID: *native, RuntimeAgentID: *native}, time.Now().UTC())
	if e != nil || current.State != runtimeobs.StateRunning || current.NativeAttemptID != receipt.NativeAttemptID || current.RunID != receipt.RunID {
		fmt.Fprintf(stderr, "native execution changed before boundary append: %v\n", e)
		return 1
	}
	b, recorded, err := runtimeobs.AppendOperationBoundary(*project, b, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(map[string]any{"ok": true, "recorded": recorded, "boundary": b, "execution_authorized": false}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func boundaryRoleMatches(role, name string) bool {
	patterns := map[string]string{"controller": `Controller`, "registrar": `Registrar`, "worker": `Worker_[A-Za-z0-9]+_G[1-9][0-9]*`, "reviewer": `Reviewer_[A-Za-z0-9]+_G[1-9][0-9]*`}
	pattern, ok := patterns[role]
	return ok && regexp.MustCompile(`^D[0-9]{4}_[0-9]{2}_`+pattern+`$`).MatchString(name)
}

// Registration apply owns this operation lifecycle; no agent heartbeat or
// extra reporting call is necessary. Each real invocation has a distinct stable
// operation identity, while RequestID groups retries of the same registration.
type registrationBoundary struct {
	project    string
	base       runtimeobs.OperationBoundary
	stderr     io.Writer
	firstError error
	failedKey  string
	disabled   bool
	finished   bool
	started    bool
	ended      bool
}

type registrationCollectionStatus struct {
	Status        string `json:"status"`
	RequestID     string `json:"request_id,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
	NativeID      string `json:"native_id,omitempty"`
	Started       bool   `json:"started"`
	Ended         bool   `json:"ended"`
	OriginalError string `json:"original_error,omitempty"`
}

func beginRegistrationBoundary(project, request string, stderr io.Writer) *registrationBoundary {
	fail := func(err error) *registrationBoundary {
		fmt.Fprintf(stderr, "execution_boundary collection_failure/unmeasured: %v\n", err)
		return &registrationBoundary{project: project, stderr: stderr, disabled: true, firstError: err, base: runtimeobs.OperationBoundary{RequestID: request}}
	}
	native, thread := os.Getenv("CODEX_THREAD_ID"), os.Getenv("T3CODE_THREAD_ID")
	if native == "" || thread == "" {
		return fail(errors.New("supported current native/T3 host identity unavailable; no registration start inferred"))
	}
	now := time.Now().UTC()
	sel := runtimeobs.T3Selector{CanonicalProject: project, ThreadID: thread, SessionID: native, RuntimeAgentID: native}
	r, err := runtimeobs.ReadT3Execution(sel, now)
	if err != nil {
		return fail(err)
	}
	if r.State != runtimeobs.StateRunning {
		return fail(errors.New("registration START requires current running native execution"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	m, err := runtimeobs.ResolveCodexThreadMetadata(ctx, native)
	if err != nil {
		return fail(err)
	}
	role := ""
	for _, candidate := range []string{"controller", "registrar", "worker", "reviewer"} {
		if m.ID == native && boundaryRoleMatches(candidate, m.Name) {
			role = candidate
			break
		}
	}
	if role == "" {
		return fail(errors.New("exact current native role metadata unavailable; no role inferred"))
	}
	a, observed, err := runtimeobs.ObserveT3Execution(project, sel, time.Now().UTC())
	if err != nil {
		return fail(err)
	}
	if observed.NativeAttemptID != r.NativeAttemptID || observed.RunID != r.RunID {
		return fail(errors.New("native execution changed before registration START"))
	}
	start := time.Now().UTC()
	base := runtimeobs.OperationBoundary{RequestID: request, OperationID: "registration:" + request + ":" + r.NativeAttemptID + ":" + start.Format("20060102T150405.000000000Z"), Provider: r.Provider, NativeID: native, AttemptID: a.AttemptID, ThreadID: thread, Role: role, Source: "registration_apply", Evidence: "approved registration apply request " + request}
	c := &registrationBoundary{project: project, base: base, stderr: stderr}
	c.write("start", "start", "", nil, "")
	c.phase("input_validation")
	return c
}

func (c *registrationBoundary) write(key, kind, phase string, tasks []string, originalError string) {
	if c == nil {
		return
	}
	if c.disabled {
		return
	}
	b := c.base
	b.Key = key
	b.Kind = kind
	b.Phase = phase
	b.TaskIDs = tasks
	b.Error = originalError
	_, _, err := runtimeobs.AppendOperationBoundary(c.project, b, time.Now().UTC())
	if err != nil {
		if c.firstError == nil {
			c.firstError = err
			c.failedKey = key
		}
		fmt.Fprintf(c.stderr, "execution_boundary collection_failure key=%s: %v\n", key, err)
	} else {
		if kind == "start" {
			c.started = true
		}
		if kind == "end" {
			c.ended = true
		}
	}
}
func (c *registrationBoundary) phase(phase string) {
	if c != nil {
		c.write("phase:"+phase, "phase", phase, nil, "")
	}
}
func (c *registrationBoundary) link(tasks []string) {
	if c != nil && len(tasks) > 0 {
		c.write("canonical_tasks", "task_link", "", tasks, "")
	}
}
func (c *registrationBoundary) finish(code int, originalError string) {
	if c == nil {
		return
	}
	if c.finished || c.disabled {
		return
	}
	c.finished = true
	if c.firstError != nil {
		b := c.base
		b.Key = "collection_failure:" + c.failedKey
		b.Kind = "collection_failure"
		b.RelatedID = c.failedKey
		b.Error = c.firstError.Error()
		if _, _, err := runtimeobs.AppendOperationBoundary(c.project, b, time.Now().UTC()); err != nil {
			fmt.Fprintf(c.stderr, "execution_boundary failure persistence unavailable: %v\n", err)
		}
	}
	if code != 0 && originalError == "" {
		originalError = fmt.Sprintf("registration apply exit code %d; original diagnostic unavailable", code)
	}
	c.write("end", "end", "", nil, originalError)
}

func (c *registrationBoundary) status() *registrationCollectionStatus {
	if c == nil {
		return nil
	}
	s := &registrationCollectionStatus{Status: "observed", RequestID: c.base.RequestID, OperationID: c.base.OperationID, NativeID: c.base.NativeID, Started: c.started, Ended: c.ended}
	if c.disabled {
		s.Status = "unmeasured"
	} else if c.firstError != nil || !c.started || !c.ended {
		s.Status = "collection_failure"
	}
	if c.firstError != nil {
		s.OriginalError = c.firstError.Error()
	}
	return s
}
