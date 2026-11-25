package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type cliOptions struct {
	collectionProject   string
	contextFile         string
	sourceWorkspace     string
	project             string
	jsonOutput          bool
	event               string
	from                string
	to                  string
	sourceAttempt       string
	fromAttempt         string
	toAttempt           string
	targetAttempt       string
	reportFile          string
	contractSHA256      string
	executionAuthorized bool
	as                  string
	step                string
	result              string
	evidence            string
	positionals         []string
	summary             bool
	fields              string
}

// Diagnostics are read-only and cannot authorize or refresh an actor. Unknown
// fields remain explicit; command arguments alone never become verified identity.
func managedHandoffDiagnostic(canonical string, opts cliOptions, action, stage string, actor *runtimeobs.RoleStart, actorVerified, beforeBusiness bool, cause error, now time.Time) map[string]any {
	result := map[string]any{"ok": false, "status": "blocked", "collection_status": "blocked", "stage": stage, "action": action, "error": cause.Error(), "original_error": cause.Error(), "executed": nil, "business_execution": "unknown", "task_id": nil, "task_status": "unknown", "handoff_id": nil, "handoff_status": "unknown", "actual_actor": nil, "actor_identity_status": "unverified", "requested_source_attempt_id": opts.sourceAttempt, "actual_attempt_id": nil, "operation_id": nil, "request_id": nil, "last_good_ack": nil, "open_operations": []any{}, "recovery_owner": "unknown", "resume_condition": "Verify the actual actor and owned collection scope. Refresh only your own supported source and fixture observation, then retry this same request/HID; no approval, identity, phase or timestamp backfill."}
	if beforeBusiness {
		result["executed"] = false
		result["business_execution"] = "not_started"
	} else {
		result["resume_condition"] = "Business write/ACK outcome is unknown: inspect the actual handoff journal and collection receipt before retrying. Preserve this error; refresh only your own source/fixture when required, never replay a successful business write."
	}
	if actor != nil && actor.Role != "" && actor.AgentPath != "" {
		result["recovery_owner"] = actor.AgentPath
		result["actual_actor"] = map[string]any{"native_id": actor.Selector.SessionID, "agent_path": actor.AgentPath, "role": actor.Role, "source_worktree": actor.SourceWorktree, "source_identity_verified": actorVerified}
	}
	if actorVerified {
		result["actor_identity_status"] = "verified_at_entry; not execution authorization"
	}
	if _, err := runtimeobs.ReadCollectionProject(canonical, opts.collectionProject, now); err != nil {
		result["scope_diagnostic_error"] = err.Error()
		return result
	}
	task := ""
	if len(opts.positionals) == 1 {
		if action == "prepare" {
			task = opts.positionals[0]
			result["task_id"] = task
			result["task_status"] = "requested; not yet verified"
		} else if h, ok, err := Inspect(opts.collectionProject, opts.positionals[0], now); err == nil && ok {
			task = h.TaskID
			result["task_id"] = task
			result["task_status"] = "recorded_handoff"
			result["handoff_id"] = h.HandoffID
			result["handoff_status"] = "recorded"
		}
	}
	if actor == nil || actor.Selector.SessionID == "" || actor.Role == "" {
		return result
	}
	current, err := runtimeobs.ReadT3Execution(actor.Selector, now)
	if err != nil {
		result["native_diagnostic_error"] = err.Error()
		return result
	}
	ledger, err := runtimeobs.BuildLedger(opts.collectionProject, 0, now)
	if err != nil {
		result["ledger_diagnostic_error"] = err.Error()
		return result
	}
	var own *runtimeobs.Attempt
	for _, a := range ledger.Attempts {
		if a.RuntimeAgentID == actor.Selector.SessionID && a.AgentPath == actor.AgentPath && a.NativeReceipt != nil && a.NativeReceipt.ThreadID == current.ThreadID && a.NativeReceipt.NativeAttemptID == current.NativeAttemptID && a.NativeReceipt.RunID == current.RunID && runtimeobs.HasObservedExecution(a) {
			if own != nil {
				return result
			}
			copy := a
			own = &copy
		}
	}
	if own == nil {
		return result
	}
	result["actual_attempt_id"] = own.AttemptID
	result["destination_observed_at"] = own.LastObservedAt
	result["requested_attempt_matches_actual"] = opts.sourceAttempt == own.AttemptID
	if own.TaskID == task && task != "" {
		result["task_status"] = "observed_exact_task_binding"
	}
	rows, err := runtimeobs.ReadOperationBoundaries(opts.collectionProject)
	if err != nil {
		result["boundary_diagnostic_error"] = err.Error()
		return result
	}
	type operation struct {
		start, last  runtimeobs.OperationBoundary
		open, linked bool
	}
	ops := map[string]operation{}
	for _, b := range rows {
		if b.NativeID != own.RuntimeAgentID || b.AttemptID != own.AttemptID || b.Role != actor.Role {
			continue
		}
		op := ops[b.OperationID]
		if b.Kind == "start" {
			op.start = b
			op.open = true
		}
		if b.Kind == "end" {
			op.open = false
		}
		op.last = b
		for _, id := range b.TaskIDs {
			if id == task && task != "" {
				op.linked = true
			}
		}
		ops[b.OperationID] = op
	}
	keys := []string{}
	for key, op := range ops {
		if op.open && op.linked {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	open := []any{}
	for _, key := range keys {
		op := ops[key]
		open = append(open, map[string]any{"operation_id": key, "request_id": op.start.RequestID, "start_boundary_id": op.start.ID, "last_boundary_id": op.last.ID, "last_boundary_at": op.last.ObservedAt})
	}
	result["open_operations"] = open
	if len(keys) == 1 {
		result["operation_id"] = keys[0]
		result["request_id"] = ops[keys[0]].start.RequestID
	}
	receipts, err := runtimeobs.ReadCollectionReceipts(opts.collectionProject)
	if err != nil {
		result["receipt_diagnostic_error"] = err.Error()
		return result
	}
	var last *runtimeobs.CollectionReceipt
	for _, r := range receipts {
		if r.Status != "ack" || r.At.After(now) {
			continue
		}
		match := r.Boundary != nil && r.Boundary.NativeID == own.RuntimeAgentID && r.Boundary.AttemptID == own.AttemptID && ops[r.Boundary.OperationID].linked
		if r.Activity != nil && r.Activity.AgentID == own.RuntimeAgentID && r.Activity.AttemptID == own.AttemptID && r.Activity.TaskID == task {
			match = true
		}
		if match && (last == nil || r.At.After(last.At)) {
			copy := r
			last = &copy
		}
	}
	if last != nil {
		result["last_good_ack"] = last
	}
	return result
}

func RunCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "handoff requires one of: context, context-check, prepare, transport-plan, claim, transfer, recover-plan, redispatch, mark, inspect, reconcile, capability")
		return 2
	}
	action := strings.ToLower(args[0])
	opts, err := parseCLI(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if action != "reconcile" && (opts.summary || opts.fields != "") {
		fmt.Fprintln(stderr, "--summary and --fields are supported only by handoff reconcile")
		return 2
	}
	project := opts.project
	if project == "" {
		project = "."
	}
	project, err = filepath.Abs(project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	now := time.Now()
	canonical := project
	var managedActor *runtimeobs.RoleStart
	blocked := func(stage string, cause error, actor *runtimeobs.RoleStart, verified, before bool) {
		if opts.collectionProject != "" && opts.jsonOutput {
			printValue(stdout, managedHandoffDiagnostic(canonical, opts, action, stage, actor, verified, before, cause, now), true)
		}
	}
	if opts.collectionProject != "" {
		actor, e := runtimeobs.CollectionActor(project, opts.collectionProject, now)
		if e != nil {
			blocked("actor_identity", e, &actor, false, true)
			fmt.Fprintln(stderr, e)
			return 1
		}
		project = opts.collectionProject
		managedActor = &actor
		if action == "prepare" || action == "claim" || action == "mark" {
			task := ""
			if action == "prepare" && len(opts.positionals) == 1 {
				task = opts.positionals[0]
			}
			if action != "prepare" && len(opts.positionals) == 1 {
				if h, ok, err := Inspect(project, opts.positionals[0], now); err == nil && ok {
					task = h.TaskID
				}
			}
			if task == "" || opts.sourceAttempt == "" {
				blocked("managed_context", errors.New("managed handoff requires exact task and own source-attempt"), managedActor, true, true)
				fmt.Fprintln(stderr, "managed handoff requires exact task and own source-attempt")
				return 1
			}
			if action == "prepare" && opts.from != actor.AgentPath || action == "claim" && opts.as != actor.AgentPath {
				blocked("actor_path", errors.New("managed handoff actor path mismatch"), managedActor, true, true)
				fmt.Fprintln(stderr, "managed handoff actor path mismatch")
				return 1
			}
			if err := runtimeobs.RequireCollectionACK(project, task, actor.Selector.SessionID, opts.sourceAttempt, actor.Role, now); err != nil {
				blocked("collection_ack", err, managedActor, true, true)
				fmt.Fprintln(stderr, err)
				return 1
			}
			if action == "prepare" && (opts.event == "worker_done" || opts.event == "worker-done") {
				r := activity.CheckReporting(project, task, activity.RoleWorker, opts.sourceAttempt, "", "", "worker_completed", now)
				if r.Status != "recorded" {
					if opts.jsonOutput {
						v := managedHandoffDiagnostic(canonical, opts, action, "completion_phase_ack", managedActor, true, true, errors.New("managed completion held: actual Worker completion phase ACK required"), now)
						v["phase_reporting"] = r
						printValue(stdout, v, true)
					} else {
						printValue(stdout, r, false)
					}
					fmt.Fprintln(stderr, "managed completion held: actual Worker completion phase ACK required")
					return 1
				}
			}
		}
	}

	switch action {
	case "context":
		if len(opts.positionals) != 1 || opts.targetAttempt == "" {
			fmt.Fprintln(stderr, "handoff context requires exact original handoff ID --target-attempt CURRENT")
			return 2
		}
		result, callErr := StartContext(project, opts.positionals[0], opts.targetAttempt, now)
		printValue(stdout, result, opts.jsonOutput)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
	case "context-check":
		if len(opts.positionals) != 1 || opts.targetAttempt == "" || opts.contextFile == "" {
			fmt.Fprintln(stderr, "handoff context-check requires TASK --target-attempt CURRENT --context-file FILE")
			return 2
		}
		c, callErr := readExecutionContext(opts.contextFile)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		result, callErr := CheckContext(project, opts.positionals[0], opts.targetAttempt, c, false, now)
		printValue(stdout, result, opts.jsonOutput)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}

	case "transport-plan":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "handoff transport-plan requires exact handoff ID")
			return 2
		}
		result, callErr := PlanTransport(project, opts.positionals[0], now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
	case "prepare":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "handoff prepare requires a task ID")
			return 2
		}
		result, callErr := Prepare(PrepareRequest{
			Project: project, TaskID: opts.positionals[0], EventType: EventType(strings.ReplaceAll(opts.event, "-", "_")),
			SourceAgentPath: opts.from, TargetAgentPath: opts.to,
			SourceAttemptID: opts.sourceAttempt, SourceWorkspace: opts.sourceWorkspace, TargetAttemptID: opts.targetAttempt,
			ContextFile: opts.contextFile, ReportFile: opts.reportFile, ExpectedContractSHA256: opts.contractSHA256, ExecutionAuthorized: opts.executionAuthorized,
		}, now)
		if callErr != nil {
			blocked("prepare_outcome_unknown", callErr, managedActor, managedActor != nil, false)
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		if opts.event == "worker-done" || opts.event == "worker_done" {
			r := activity.CheckReporting(project, opts.positionals[0], activity.RoleWorker, opts.sourceAttempt, "", "", "worker_completed", now)
			result.ActivityReporting = &r
		}
		printValue(stdout, result, opts.jsonOutput)
	case "claim":
		if len(opts.positionals) != 1 || opts.as == "" || opts.sourceAttempt == "" {
			fmt.Fprintln(stderr, "handoff claim requires <HANDOFF_ID> --as <agent-path> --source-attempt <attempt-id>")
			return 2
		}
		result, callErr := Claim(project, opts.positionals[0], opts.as, opts.sourceAttempt, now)
		if callErr != nil {
			blocked("claim_outcome_unknown", callErr, managedActor, managedActor != nil, false)
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		if (result.Claimed || result.AlreadyClaimed) && !result.AlreadyApplied {
			if h, ok, _ := Inspect(project, result.HandoffID, now); ok && (h.EventType == EventWorkerDone || h.EventType == EventWorkerBlocked || h.EventType == EventCompletionReport) {
				r := activity.CheckReporting(project, h.TaskID, activity.RoleController, result.ClaimedAttemptID, "", h.HandoffID, "review_started", now)
				result.ActivityReporting = &r
			}
		}
		printValue(stdout, result, opts.jsonOutput)
		if result.ClaimConflict || result.ContractChanged {
			if !opts.jsonOutput && result.RecoveryHint != "" {
				fmt.Fprintln(stderr, "recovery: "+result.RecoveryHint)
			}
			return 1
		}
	case "transfer":
		if len(opts.positionals) != 1 || opts.fromAttempt == "" || opts.toAttempt == "" || opts.evidence == "" {
			fmt.Fprintln(stderr, "handoff transfer requires <HANDOFF_ID> --from-attempt OLD --to-attempt NEW --evidence TEXT")
			return 2
		}
		result, callErr := TransferClaim(project, opts.positionals[0], opts.fromAttempt, opts.toAttempt, opts.evidence, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
	case "recover-plan":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "handoff recover-plan requires <HANDOFF_ID>")
			return 2
		}
		result, callErr := PlanRecovery(project, opts.positionals[0], now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
	case "redispatch":
		if len(opts.positionals) != 1 || opts.targetAttempt == "" || opts.evidence == "" {
			fmt.Fprintln(stderr, "handoff redispatch requires <HANDOFF_ID> --target-attempt CURRENT --evidence NATIVE_TRANSPORT_PROOF")
			return 2
		}
		result, callErr := Redispatch(project, opts.positionals[0], opts.targetAttempt, opts.evidence, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
	case "mark":
		if len(opts.positionals) != 1 || opts.step == "" || opts.result == "" {
			fmt.Fprintln(stderr, "handoff mark requires <HANDOFF_ID> --step <step> --result ok|failed|unknown")
			return 2
		}
		result, callErr := MarkAs(project, opts.positionals[0], opts.step, opts.result, opts.evidence, opts.sourceAttempt, now)
		if callErr != nil {
			blocked("mark_outcome_unknown", callErr, managedActor, managedActor != nil, false)
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		if opts.step == "acceptance" {
			if h, ok, _ := Inspect(project, result.HandoffID, now); ok {
				r := activity.CheckReporting(project, h.TaskID, activity.RoleController, h.ClaimedAttemptID, "", h.HandoffID, "review_outcome", now)
				result.ActivityReporting = &r
			}
		}
		printValue(stdout, result, opts.jsonOutput)
	case "inspect":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "handoff inspect requires a handoff ID")
			return 2
		}
		row, ok, callErr := Inspect(project, opts.positionals[0], now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		if !ok {
			fmt.Fprintln(stderr, "handoff not found: "+opts.positionals[0])
			return 1
		}
		printValue(stdout, row, opts.jsonOutput)
	case "reconcile":
		if len(opts.positionals) != 0 {
			fmt.Fprintln(stderr, "handoff reconcile takes no positional arguments")
			return 2
		}
		ledger, callErr := Reconcile(project, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		result, projectionErr := projectReconcileLedger(ledger, opts.summary, opts.fields)
		if projectionErr != nil {
			fmt.Fprintln(stderr, projectionErr)
			return 2
		}
		printValue(stdout, result, opts.jsonOutput)
	case "capability":
		if len(opts.positionals) < 2 {
			fmt.Fprintln(stderr, "handoff capability requires: show <provider> | record <provider> <name> <supported|unsupported|unknown>")
			return 2
		}
		subaction := strings.ToLower(opts.positionals[0])
		provider := strings.ToLower(opts.positionals[1])
		switch subaction {
		case "show":
			if len(opts.positionals) != 2 {
				fmt.Fprintln(stderr, "handoff capability show requires exactly one provider")
				return 2
			}
			caps, callErr := LoadCapabilities(project, provider)
			if callErr != nil {
				fmt.Fprintln(stderr, callErr)
				return 1
			}
			printValue(stdout, caps, opts.jsonOutput)
		case "record":
			if len(opts.positionals) != 4 {
				fmt.Fprintln(stderr, "handoff capability record requires <provider> <name> <supported|unsupported|unknown>")
				return 2
			}
			caps, callErr := RecordCapability(project, provider, opts.positionals[2], CapabilityState(strings.ToLower(opts.positionals[3])), opts.evidence, now)
			if callErr != nil {
				fmt.Fprintln(stderr, callErr)
				return 1
			}
			printValue(stdout, caps, opts.jsonOutput)
		default:
			fmt.Fprintln(stderr, "handoff capability action must be show or record")
			return 2
		}
	default:
		fmt.Fprintln(stderr, "unknown handoff action: "+action)
		return 2
	}
	return 0
}

func parseCLI(args []string) (cliOptions, error) {
	opts := cliOptions{to: "/root/controller"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.jsonOutput = true
		case "--summary":
			opts.summary = true
		case "--fields":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return opts, errors.New("--fields requires a comma-separated list of top-level field names")
			}
			if opts.fields != "" {
				return opts, errors.New("--fields may be provided only once")
			}
			i++
			if strings.TrimSpace(args[i]) == "" {
				return opts, errors.New("--fields requires a non-empty comma-separated list")
			}
			opts.fields = args[i]
		case "--execution-authorized":
			opts.executionAuthorized = true
		case "--collection-project", "--context-file", "--source-worktree", "--project", "--event", "--from", "--to", "--source-attempt", "--target-attempt", "--from-attempt", "--to-attempt", "--report-file", "--contract-sha256", "--as", "--step", "--result", "--evidence":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			value := args[i+1]
			i++
			switch arg {
			case "--collection-project":
				opts.collectionProject = value
			case "--context-file":
				opts.contextFile = value
			case "--source-worktree":
				opts.sourceWorkspace = value
			case "--project":
				opts.project = value
			case "--event":
				opts.event = value
			case "--from":
				opts.from = value
			case "--to":
				opts.to = value
			case "--source-attempt":
				opts.sourceAttempt = value
			case "--from-attempt":
				opts.fromAttempt = value
			case "--to-attempt":
				opts.toAttempt = value
			case "--target-attempt":
				opts.targetAttempt = value
			case "--report-file":
				opts.reportFile = value
			case "--contract-sha256":
				opts.contractSHA256 = value
			case "--as":
				opts.as = value
			case "--step":
				opts.step = value
			case "--result":
				opts.result = value
			case "--evidence":
				opts.evidence = value
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown argument: %s", arg)
			}
			opts.positionals = append(opts.positionals, arg)
		}
	}
	return opts, nil
}

func projectReconcileLedger(ledger Ledger, summary bool, fields string) (any, error) {
	// Preserve the legacy JSON contract exactly when no projection is requested.
	if !summary && fields == "" {
		return ledger, nil
	}

	var data map[string]any
	if summary {
		byEvent := map[string]int{}
		applied, claimedPending, unclaimedPending, held := 0, 0, 0, 0
		for _, row := range ledger.Handoffs {
			byEvent[string(row.EventType)]++
			if row.Applied {
				applied++
				continue
			}
			if row.Action == ActionHold {
				held++
			}
			if row.ClaimedBy != "" {
				claimedPending++
			} else {
				unclaimedPending++
			}
		}
		data = map[string]any{
			"version":                 ledger.Version,
			"generated_at":            ledger.GeneratedAt,
			"total_handoffs":          len(ledger.Handoffs),
			"applied_count":           applied,
			"claimed_pending_count":   claimedPending,
			"unclaimed_pending_count": unclaimedPending,
			"hold_count":              held,
			"findings_count":          len(ledger.Findings),
			"by_event":                byEvent,
			"findings":                ledger.Findings,
			"observation_scope":       "handoff journal only; no native transport or runtime liveness assertion",
		}
	} else {
		// Marshal canonical JSON tags rather than reflecting Go field names.
		payload, err := json.Marshal(ledger)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, err
		}
	}
	if fields == "" {
		return data, nil
	}
	selected := map[string]any{}
	for _, raw := range strings.Split(fields, ",") {
		field := strings.TrimSpace(raw)
		if field == "" {
			return nil, errors.New("--fields contains an empty field name")
		}
		value, ok := data[field]
		if !ok {
			return nil, fmt.Errorf("unsupported handoff reconcile field %q (use task-mecca help handoff)", field)
		}
		if _, exists := selected[field]; exists {
			return nil, fmt.Errorf("duplicate handoff reconcile field %q", field)
		}
		selected[field] = value
	}
	return selected, nil
}

func printValue(w io.Writer, value any, jsonOutput bool) {
	if jsonOutput {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(value)
		return
	}
	switch row := value.(type) {
	case PrepareResult:
		fmt.Fprintf(w, "%s action=%s target=%s state=%s duplicate=%v\n", row.HandoffID, row.Action, row.Target.AgentPath, row.Target.State, row.Duplicate)
	case ClaimResult:
		fmt.Fprintf(w, "%s claimed=%v already_claimed=%v already_applied=%v conflict=%v contract_changed=%v\n", row.HandoffID, row.Claimed, row.AlreadyClaimed, row.AlreadyApplied, row.ClaimConflict, row.ContractChanged)
	case MarkResult:
		fmt.Fprintf(w, "%s step=%s result=%s duplicate=%v applied=%v\n", row.HandoffID, row.Step, row.Result, row.Duplicate, row.Applied)
	default:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(value)
	}
}
