package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/silverkhan/TaskMecca/internal/backlog"
)

// runRecoveryEventCLI is an explicit event hook for Root/Controller. It is
// observational; provider-native resume, assignment, and Root messaging remain
// owned by the live agents, not this command.
func runRecoveryEventCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "event" {
		fmt.Fprintln(stderr, "usage: task-mecca recovery event --event registration|controller_start|controller_resume|worker_report [--task ID] [--project DIR] [--root DIR] [--json]")
		return 2
	}
	flags := flag.NewFlagSet("recovery event", flag.ContinueOnError)
	flags.SetOutput(stderr)
	event := flags.String("event", "", "orchestration event")
	task := flags.String("task", "", "exact triggering task ID")
	project := flags.String("project", ".", "project directory")
	root := flags.String("root", "", "optional explicit backlog root")
	jsonOutput := flags.Bool("json", false, "emit JSON")
	if err := flags.Parse(args[1:]); err != nil { return 2 }
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "recovery event takes no positional arguments")
		return 2
	}
	absProject, err := filepath.Abs(*project)
	if err != nil { fmt.Fprintln(stderr, err); return 2 }
	absRoot := ""
	if *root != "" {
		absRoot = *root
		if !filepath.IsAbs(absRoot) { absRoot = filepath.Join(absProject, absRoot) }
		absRoot, err = filepath.Abs(absRoot)
		if err != nil { fmt.Fprintln(stderr, err); return 2 }
	}
	report, err := backlog.RecoveryEventSnapshot(absProject, absRoot, *event, *task)
	if err != nil { fmt.Fprintln(stderr, err); return 2 }
	if *jsonOutput {
		if err := json.NewEncoder(stdout).Encode(report); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	fmt.Fprintf(stdout, "Recovery event: %v (trigger: %v)\n", report["event"], report["trigger_task"])
	fmt.Fprintf(stdout, "Previously stranded candidates: %v; user decisions: %v\n", report["candidate_count"], report["requires_user_count"])
	for _, item := range report["candidates"].([]map[string]any) {
		fmt.Fprintf(stdout, "- %v: %v (%v; %v)\n", item["id"], item["title"], item["health"], item["code"])
	}
	fmt.Fprintln(stdout, report["notice"])
	return 0
}
