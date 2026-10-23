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
		fmt.Fprintln(out,"  task-mecca flow              Inspect read-only next-action decisions; Controller cleanup/exit gate")
		fmt.Fprintln(out,"  task-mecca worktree          Inspect and explicitly preserve/verify/remove safe Git worktrees")
		fmt.Fprintln(out,"  task-mecca protect           Snapshot, verify project data and inspect component versions")
		fmt.Fprintln(out,"  task-mecca runtime           Observe/query/bind execution evidence")
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
	case "flow":
		flowNextUsage(out)
	case "contract":
		contractUsage(out)
	case "runtime":
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
		fmt.Fprintln(out, "  task-mecca handoff capability show PROVIDER --project DIR --json")
		fmt.Fprintln(out, "  task-mecca handoff capability record PROVIDER NAME supported|unsupported|unknown --project DIR --evidence REF --json")
		fmt.Fprintln(out, "Options: --project DIR, --json; prepare: --event, --from, --to, --source-attempt, --target-attempt, --report-file, --contract-sha256, --execution-authorized")
		fmt.Fprintln(out, "         claim: --as, --source-attempt; mark: --step, --result, --evidence; capability record: --evidence")
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
