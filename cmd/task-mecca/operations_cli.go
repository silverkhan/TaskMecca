package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"github.com/silverkhan/TaskMecca/internal/webui"
)

func runOperationsCLI(args []string, out, errors io.Writer) int {
	if len(args) == 0 || args[0] != "reconcile" {
		fmt.Fprintln(errors, "operations reconcile --project ABSOLUTE --incident EXACT_ID [--dry-run]; no notification delivery")
		return 2
	}
	flags := flag.NewFlagSet("operations reconcile", flag.ContinueOnError)
	flags.SetOutput(errors)
	project := flags.String("project", "", "exact absolute project root")
	incident := flags.String("incident", "", "exact observation incident ID")
	dryRun := flags.Bool("dry-run", false, "read-only completion mapping preview")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if !filepath.IsAbs(*project) || *incident == "" {
		fmt.Fprintln(errors, "absolute --project and exact --incident are required")
		return 2
	}
	resolved, err := runtimeobs.ResolveProject(*project)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(*project) {
		fmt.Fprintln(errors, "exact Task Mecca project root required")
		return 2
	}
	value, err := webui.ReconcileCompletedOperation(resolved, *incident, time.Now().UTC(), *dryRun)
	if err != nil {
		_ = json.NewEncoder(out).Encode(map[string]any{"error": err.Error(), "notifications_sent": false})
		return 1
	}
	if err := json.NewEncoder(out).Encode(value); err != nil {
		return 1
	}
	return 0
}
