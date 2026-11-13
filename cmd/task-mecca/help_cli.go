package main

import (
	"fmt"
	"io"
	"strings"
)

// Help is intentionally generated from supported user-facing CLI entry points,
// not from legacy Python commands. It is safe to run before project setup.
func runHelpCLI(args []string,out,stderr io.Writer) int {
	if len(args)>1 {
		fmt.Fprintln(stderr,"help takes zero arguments or one command name")
		return 2
	}
	topic:=""
	if len(args)==1{topic=strings.ToLower(strings.TrimSpace(args[0]))}
	switch topic {
	case "":
		fmt.Fprintln(out,"Task Mecca CLI — command reference")
		fmt.Fprintln(out,"  task-mecca --help            Show this help (no project required)")
		fmt.Fprintln(out,"  task-mecca help COMMAND      Show command-specific usage")
		fmt.Fprintln(out,"  task-mecca --version         Installed version/channel and best-effort latest check")
		fmt.Fprintln(out,"  task-mecca web               Start local Web dashboard")
		fmt.Fprintln(out,"  task-mecca upgrade           Upgrade the Task Mecca binary")
		fmt.Fprintln(out,"  task-mecca uninstall --dry-run  Inspect program, service and owned Hook removal")
		fmt.Fprintln(out,"  task-mecca migrate           Migrate project framework (preserve project-owned data)")
		fmt.Fprintln(out,"  task-mecca contract          Validate, plan, register or record contract decisions")
		fmt.Fprintln(out,"  task-mecca context           Context lifecycle status, policy, event and safe rotation")
		fmt.Fprintln(out,"  task-mecca orchestration     A/B/C/D modes, confirmed Root selection and notification handoff plan")
		fmt.Fprintln(out,"  task-mecca recovery event    Inspect older stranded doing tasks on orchestration events (read-only)")
		fmt.Fprintln(out,"  task-mecca flow              Inspect read-only next-action decisions; Controller cleanup/exit gate")
		fmt.Fprintln(out,"  task-mecca worktree          Inspect and explicitly preserve/verify/remove safe Git worktrees")
		fmt.Fprintln(out,"  task-mecca protect           Snapshot, verify project data and inspect component versions")
		fmt.Fprintln(out,"  task-mecca runtime           Observe/query/bind execution evidence")
        fmt.Fprintln(out,"  task-mecca activity          Record verified-bound agent reports and inspect Activity timings")
		fmt.Fprintln(out,"  task-mecca lifecycle         Read or write canonical lifecycle evidence")
		fmt.Fprintln(out,"  task-mecca handoff           Inspect and record explicit handoff evidence")
		fmt.Fprintln(out,"  task-mecca inspect ID        Inspect an existing backlog task")
		fmt.Fprintln(out,"  task-mecca preflight         Check project/runtime readiness")
		fmt.Fprintln(out,"  task-mecca next-id PREFIX    Suggest next backlog identifier (no reservation)")
		fmt.Fprintln(out,"  task-mecca tags              Inspect canonical task taxonomy")
		fmt.Fprintln(out,"  task-mecca --help            Details: task-mecca help contract|runtime|worktree|upgrade|migrate|web|handoff")
	case "uninstall":
        fmt.Fprintln(out,"Safely remove the Task Mecca program, Web service registration and global Task Mecca-owned agent Hook commands.")
        fmt.Fprintln(out,"task-mecca uninstall --dry-run [--json] [--purge-settings]")
        fmt.Fprintln(out,"task-mecca uninstall [--approve] [--purge-settings]")
        fmt.Fprintln(out,"Default: preserve all project directories, _task_mecca data, global registry/settings and project-local Hooks.")
        fmt.Fprintln(out,"--purge-settings: also remove recognized global projects.json and channel metadata; never delete projects.")
        fmt.Fprintln(out,"Modified global agent Hook JSON files are backed up before exact Task Mecca commands are removed.")
        fmt.Fprintln(out,"Interactive uninstall requires typing 'uninstall'; --approve is explicit noninteractive confirmation.")
	case "orchestration":
		fmt.Fprintln(out,"task-mecca orchestration modes --project DIR --provider codex --json")
		fmt.Fprintln(out,"task-mecca handoff capability record --project DIR codex can_spawn_subagent supported --evidence OBSERVED_PROOF --json")
		fmt.Fprintln(out,"task-mecca orchestration configure --project DIR --provider codex --root-session-id EXACT --mode B --confirmed --timezone Asia/Seoul --json")
		fmt.Fprintln(out,"task-mecca orchestration status --project DIR --provider codex --root-session-id EXACT --json")
		fmt.Fprintln(out,"task-mecca orchestration name --project DIR --group D1109_01 --role controller --json")
		fmt.Fprintln(out,"task-mecca orchestration session-name plan|apply|verify --project DIR --group DMMDD_NN --role ROLE --provider codex --execution newSession|existingSession --session-id EXACT --json")
		fmt.Fprintln(out,"Names are display metadata. Apply/verify require the exact current native session; T3 title requires separate official tool readback.")
		fmt.Fprintln(out,"task-mecca orchestration notify --project DIR --provider codex --root-session-id EXACT --task B-620 --controller-attempt EXACT --root-attempt EXACT --json")
		fmt.Fprintln(out,"A/B/C/D are fixed user-facing labels; unknown capabilities are unavailable. Native spawn/send/resume remain provider-owned.")
	case "context":
        fmt.Fprintln(out,"task-mecca context status --role controller --project DIR --json")
        fmt.Fprintln(out,"task-mecca context event --role controller --event controller_resume --attempt EXACT --apply --project DIR --json")
        fmt.Fprintln(out,"task-mecca context observe --role controller --attempt EXACT --used N --window N --evidence TEXT --json")
        fmt.Fprintln(out,"task-mecca context rotation prepare --role controller --attempt EXACT --json")
        fmt.Fprintln(out,"task-mecca context rotation acknowledge --role controller --id ROTATION --successor EXACT --checkpoint SHA --json")
        fmt.Fprintln(out,"task-mecca context rotation commit --role controller --id ROTATION --json")
        fmt.Fprintln(out,"Context usage reports are unverified. Native successor creation is external; commit requires old terminal/new running exact attempts.")
	case "recovery":
		fmt.Fprintln(out,"task-mecca recovery event --event registration --task B-550 --project DIR --json")
		fmt.Fprintln(out,"task-mecca recovery event --event controller_start|controller_resume --project DIR --json")
		fmt.Fprintln(out,"task-mecca recovery event --event worker_report --task B-527 --project DIR --json")
		fmt.Fprintln(out,"Read-only; returns all confirmed recovery candidates, stable incident keys and a Root report hint.")
		fmt.Fprintln(out,"Not a watchdog, delivery acknowledgement, permission to resume, or proof an agent is dead.")
	case "activity":
        fmt.Fprintln(out,"task-mecca activity start|switch|stop TASK-ID --project DIR --role worker --agent EXACT-RUNTIME-AGENT --attempt EXACT-ATTEMPT --assignment ID --event-id STABLE-ID --type TYPE --evidence TEXT --json")
        fmt.Fprintln(out,"Controller review: use --role controller --handoff CLAIMED-HANDOFF instead of --assignment.")
        fmt.Fprintln(out,"Registrar: use --role registrar with an exact bound /root/registrar Runtime attempt, without --assignment/--handoff.")
        fmt.Fprintln(out,"Sub-Controller: use --role sub_controller only with a bound /root/controller/sub-* attempt. This reports activity, never grants delegation or canonical write authority.")
        fmt.Fprintln(out,"task-mecca activity history|status TASK-ID --project DIR --json")
        fmt.Fprintln(out,"task-mecca activity diagnose --project DIR [--task B-589] [--samples 1..10] [--cpu-profile FILE] --json")
        fmt.Fprintln(out,"Measures explicit read-only ledger I/O wall time, storage and diagnostic process heap; not CPU, energy or battery drain.")
        fmt.Fprintln(out,"task-mecca activity diagnose --project DIR --json # read-only ledger file count/bytes and metadata scan wall time")
        fmt.Fprintln(out,"task-mecca activity correct TASK --supersedes-id ORIGINAL_EVENT --event-id CORRECTION_ID --type CORRECTED_TYPE --evidence REASON [--occurred-at RFC3339] --project DIR --role ROLE --agent EXACT --attempt EXACT [--assignment ID|--handoff ID] --json")
        fmt.Fprintln(out,"Corrections append an immutable audit record; original event bytes remain untouched. Exact active runtime attribution is still required.")
        fmt.Fprintln(out,"task-mecca activity suggest --role worker|controller|registrar|sub_controller --event PHASE --round N --json (advisory; no writes)")
        fmt.Fprintln(out,"task-mecca activity report TASK --project DIR --role ROLE --agent EXACT --attempt EXACT --assignment ASSIGNMENT_OR_HANDOFF --event implementation_started --event-id STABLE-ID --evidence TEXT --json")
        fmt.Fprintln(out,"report infers start/switch/stop and type from an explicitly observed phase event; it never assumes a decision was actually executed.")
        fmt.Fprintln(out,"task-mecca activity run B-589 --project DIR --role worker --agent EXACT --attempt EXACT --assignment ID --event-id RUNID --type targeted_test|full_regression|build_check --git-sha SHA --scope SCOPE -- PROGRAM [ARG...]")
        fmt.Fprintln(out,"Runs only the supplied command (without a shell). Records the outcome and restores the previous phase; rerunning a completed RUNID is refused.")
        fmt.Fprintln(out,"Use --ci-url https://... to link a separately verified CI run; CI itself is not attributed to an agent or backlog automatically.")
        fmt.Fprintln(out,"Writes are attributed against observed Runtime/assignment or Controller claim evidence. Activity phase remains agent-reported (inferred), not independently authenticated by a native provider.")
        fmt.Fprintln(out,"Unverified/stale/ambiguous claims fail closed. Other roles currently cannot write.")
	case "flow":
		flowNextUsage(out)
	case "contract":
		contractUsage(out)
	case "runtime":
		fmt.Fprintln(out,"Controller START: runtime observe-t3 --project CANONICAL --thread-id EXACT_T3 --session-id EXACT_NATIVE --agent-id EXACT_NATIVE --assignment ID --task-id TASK --worker-path /root/controller/WORKER --source-worktree ACTUAL_CWD [--phase CURRENT_EVENT] --json")
    fmt.Fprintln(out,"Runtime actions: list, reconcile, observe, hooks, assign, bind, bind-agent, bind-assignment, enrich-name")
		fmt.Fprintln(out,"Runtime diagnosis (read-only): task-mecca runtime diagnose --provider codex --session-id SESSION_ID --json")
		fmt.Fprintln(out,"Exact Codex session ID enables private native JSONL event-type count comparison; transcript contents are not returned.")
		fmt.Fprintln(out,"Runtime query (read-only):")
		runtimeQueryUsage(out)
		fmt.Fprintln(out,"Examples: task-mecca runtime list --json")
		fmt.Fprintln(out,"          task-mecca runtime hooks status codex --json")
	case "upgrade":
		fmt.Fprintln(out,"task-mecca upgrade")
		fmt.Fprintln(out,"Updates the installed CLI binary on the selected stable/dev channel.")
		fmt.Fprintln(out,"Does NOT migrate project framework or prove a running Web process uses the new binary.")
		fmt.Fprintln(out,"Run task-mecca migrate from each approved project root when its framework needs updating.")
	case "migrate":
		fmt.Fprintln(out,"task-mecca migrate [--project DIR] [--json]")
		fmt.Fprintln(out,"Applies the installed CLI's project framework to one selected project.")
		fmt.Fprintln(out,"User-modified managed files can require --choice overwrite|backup|cancel with --expect-plan DIGEST.")
		fmt.Fprintln(out,"Backlog, lifecycle, runtime evidence and project-owned data must be preserved.")
	case "web":
		fmt.Fprintln(out,"task-mecca web [status|restart|stop|logs]")
		fmt.Fprintln(out,"Controls the currently running Task Mecca Web service; CLI binary update is a separate operation.")
	case "protect":
		fmt.Fprintln(out,"task-mecca protect snapshot --project DIR --output EXTERNAL_MANIFEST.json --json")
		fmt.Fprintln(out,"task-mecca protect verify --project DIR --snapshot EXTERNAL_MANIFEST.json --json")
		fmt.Fprintln(out,"task-mecca protect versions --project DIR --json")
	case "worktree":
		fmt.Fprintln(out,"task-mecca worktree plan --project DIR --into origin/dev --json")
		fmt.Fprintln(out,"Read-only; unknown use, ignored/untracked data, unmerged or detached HEAD stay protected.")
		fmt.Fprintln(out,"task-mecca worktree preserve --project DIR --into REF --target EXACT_PATH --archive EXTERNAL_DIR --expect-plan DIGEST --approve --json")
		fmt.Fprintln(out,"task-mecca worktree verify --archive EXTERNAL_DIR --json")
		fmt.Fprintln(out,"task-mecca worktree remove --project DIR --into REF --target EXACT_PATH --archive EXTERNAL_DIR --expect-plan DIGEST --approve --confirm-unused --json")
	case "handoff":
		fmt.Fprintln(out, "Handoff CLI (explicit journal evidence; read-only reconcile/inspect):")
		fmt.Fprintln(out, "  task-mecca handoff reconcile --project DIR --summary --json")
		fmt.Fprintln(out, "  task-mecca handoff reconcile --project DIR --summary --fields total_handoffs,claimed_pending_count,unclaimed_pending_count,findings_count --json")
		fmt.Fprintln(out, "  task-mecca handoff reconcile --project DIR --fields version,generated_at,findings --json")
		fmt.Fprintln(out, "  task-mecca handoff reconcile --project DIR --json   # legacy complete ledger, unchanged")
		fmt.Fprintln(out, "  task-mecca handoff inspect HANDOFF_ID --project DIR --json")
		fmt.Fprintln(out, "  task-mecca handoff prepare TASK_ID --project DIR --event registration-ready --from /root/registrar --to /root/controller --source-attempt ID --target-attempt ID --report-file PATH --contract-sha256 SHA256 [--execution-authorized] --json")
		fmt.Fprintln(out, "  task-mecca handoff claim HANDOFF_ID --project DIR --as /root/controller --source-attempt CURRENT_CONTROLLER_ATTEMPT --json")
		fmt.Fprintln(out, "  task-mecca handoff mark HANDOFF_ID --project DIR --step dispatch --result ok|failed|unknown --evidence NATIVE_TRANSPORT_PROOF --json")
		fmt.Fprintln(out, "  task-mecca handoff redispatch HANDOFF_ID --project DIR --target-attempt CURRENT_ATTEMPT --evidence RECOVERY_NATIVE_TRANSPORT_PROOF --json")
		fmt.Fprintln(out, "  task-mecca handoff recover-plan HANDOFF_ID --project DIR --json  # read-only exact-runtime recovery decision")
		fmt.Fprintln(out,"  task-mecca handoff transport-plan HANDOFF_ID --project DIR --json  # read-only exact tool/address; native UUID is not a collaboration handle")
		fmt.Fprintln(out, "  On claimant_attempt_mismatch: unclaimed -> verify current native Controller and rerun original prepare with new --target-attempt; claimed -> exact-owner handoff transfer.")
		fmt.Fprintln(out, "  dispatch=ok, redispatch receipt, actual claim and implementation start are different evidence stages.")
		fmt.Fprintln(out, "  task-mecca handoff capability show PROVIDER --project DIR --json")
		fmt.Fprintln(out, "  task-mecca handoff transfer HANDOFF_ID --from-attempt OLD --to-attempt NEW --evidence REASON --project DIR --json")
		fmt.Fprintln(out, "  task-mecca handoff capability record PROVIDER NAME supported|unsupported|unknown --project DIR --evidence REF --json")
		fmt.Fprintln(out, "Options: --project DIR, --json; prepare: --event, --from, --to, --source-attempt, --target-attempt, --report-file, --contract-sha256, --execution-authorized")
		fmt.Fprintln(out, "         claim: --as, --source-attempt; transfer: --from-attempt, --to-attempt, --evidence")
        fmt.Fprintln(out, "         mark: --step, --result, --evidence, --source-attempt (mandatory after transfer); capability record: --evidence")
		fmt.Fprintln(out, "         reconcile only: --summary (omits per-handoff entries), --fields CSV (top-level JSON field projection)")
		fmt.Fprintln(out, "Summary fields: version,generated_at,total_handoffs,applied_count,claimed_pending_count,unclaimed_pending_count,hold_count,findings_count,by_event,findings,observation_scope")
		fmt.Fprintln(out, "Full reconcile fields: version,generated_at,journal_path,handoffs,findings")
		fmt.Fprintln(out, "Never infer native message delivery, execution approval, or live agent state from prepare/claim/reconcile alone.")
		fmt.Fprintln(out, "Use actual transport evidence before mark dispatch=ok; claim with the current exact Controller attempt.")
	case "version","--version":
		fmt.Fprintln(out,"task-mecca --version")
		fmt.Fprintln(out,"Prints installed version and channel; latest-check failure does not invalidate local version.")
	case "update":
		fmt.Fprintln(out,"No 'update' command. Use 'task-mecca upgrade' (CLI binary) or 'task-mecca migrate' (project framework).")
	case "lifecycle":
		fmt.Fprintln(out,"task-mecca lifecycle list --json")
		fmt.Fprintln(out,"task-mecca lifecycle record <kind> <task-id> <event-id> <actor> <evidence-source> [evidence-ref] [assignment-id] [attempt-id]")
	case "inspect","preflight","next-id","tags":
		fmt.Fprintf(out,"task-mecca %s --json\n",topic)
		fmt.Fprintln(out,"Use the project framework role instructions for full command-specific options.")
	default:
		fmt.Fprintf(stderr,"Unknown help topic %q. Use: task-mecca --help\n",topic)
		return 2
	}
	return 0
}
