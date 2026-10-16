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
		fmt.Fprintln(out,"  task-mecca migrate           Migrate project framework (preserve project-owned data)")
		fmt.Fprintln(out,"  task-mecca contract          Validate or preview structured task contracts (read-only)")
		fmt.Fprintln(out,"  task-mecca runtime           Observe/query/bind execution evidence")
		fmt.Fprintln(out,"  task-mecca lifecycle         Read or write canonical lifecycle evidence")
		fmt.Fprintln(out,"  task-mecca handoff           Inspect and record explicit handoff evidence")
		fmt.Fprintln(out,"  task-mecca inspect ID        Inspect an existing backlog task")
		fmt.Fprintln(out,"  task-mecca preflight         Check project/runtime readiness")
		fmt.Fprintln(out,"  task-mecca next-id PREFIX    Suggest next backlog identifier (no reservation)")
		fmt.Fprintln(out,"  task-mecca tags              Inspect canonical task taxonomy")
		fmt.Fprintln(out,"  task-mecca --help            Details: task-mecca help contract|runtime|upgrade|migrate|web|handoff")
	case "contract":
		contractUsage(out)
	case "runtime":
		fmt.Fprintln(out,"Runtime actions: list, reconcile, observe, hooks, assign, bind, bind-agent, bind-assignment, enrich-name")
		fmt.Fprintln(out,"Runtime diagnosis (read-only): task-mecca runtime diagnose --provider codex --agent-id UUID --json")
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
	case "handoff":
		fmt.Fprintln(out,"task-mecca handoff prepare|claim|mark|inspect|reconcile|capability [OPTIONS]")
		fmt.Fprintln(out,"The CLI records explicit handoff evidence; it cannot assume unsupported native Agent transport succeeded.")
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
