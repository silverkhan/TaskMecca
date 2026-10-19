package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

type cliOptions struct {
	project             string
	jsonOutput          bool
	event               string
	from                string
	to                  string
	sourceAttempt       string
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

func RunCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "handoff requires one of: prepare, claim, mark, inspect, reconcile, capability")
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

	switch action {
	case "prepare":
		if len(opts.positionals) != 1 {
			fmt.Fprintln(stderr, "handoff prepare requires a task ID")
			return 2
		}
		result, callErr := Prepare(PrepareRequest{
			Project: project, TaskID: opts.positionals[0], EventType: EventType(strings.ReplaceAll(opts.event, "-", "_")),
			SourceAgentPath: opts.from, TargetAgentPath: opts.to,
			SourceAttemptID: opts.sourceAttempt, TargetAttemptID: opts.targetAttempt,
			ReportFile: opts.reportFile, ExpectedContractSHA256: opts.contractSHA256, ExecutionAuthorized: opts.executionAuthorized,
		}, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
	case "claim":
		if len(opts.positionals) != 1 || opts.as == "" || opts.sourceAttempt == "" {
			fmt.Fprintln(stderr, "handoff claim requires <HANDOFF_ID> --as <agent-path> --source-attempt <attempt-id>")
			return 2
		}
		result, callErr := Claim(project, opts.positionals[0], opts.as, opts.sourceAttempt, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
		}
		printValue(stdout, result, opts.jsonOutput)
		if result.ClaimConflict || result.ContractChanged {
			return 1
		}
	case "mark":
		if len(opts.positionals) != 1 || opts.step == "" || opts.result == "" {
			fmt.Fprintln(stderr, "handoff mark requires <HANDOFF_ID> --step <step> --result ok|failed|unknown")
			return 2
		}
		result, callErr := Mark(project, opts.positionals[0], opts.step, opts.result, opts.evidence, now)
		if callErr != nil {
			fmt.Fprintln(stderr, callErr)
			return 1
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
			opts.fields = args[i]

		case "--execution-authorized":
			opts.executionAuthorized = true
		case "--project", "--event", "--from", "--to", "--source-attempt", "--target-attempt", "--report-file", "--contract-sha256", "--as", "--step", "--result", "--evidence":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			value := args[i+1]
			i++
			switch arg {
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
			"version": ledger.Version,
			"generated_at": ledger.GeneratedAt,
			"total_handoffs": len(ledger.Handoffs),
			"applied_count": applied,
			"claimed_pending_count": claimedPending,
			"unclaimed_pending_count": unclaimedPending,
			"hold_count": held,
			"findings_count": len(ledger.Findings),
			"by_event": byEvent,
			"findings": ledger.Findings,
			"observation_scope": "handoff journal only; no native transport or runtime liveness assertion",
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
