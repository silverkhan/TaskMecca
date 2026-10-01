package handoff

import (
	"encoding/json"
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
}

func RunCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "handoff requires one of: prepare, claim, mark, inspect, reconcile")
		return 2
	}
	action := strings.ToLower(args[0])
	opts, err := parseCLI(args[1:])
	if err != nil {
		fmt.Fprintln(stderr, err)
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
		printValue(stdout, ledger, opts.jsonOutput)
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
