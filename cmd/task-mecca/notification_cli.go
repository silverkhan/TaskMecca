package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/notify"
)

func runNotificationCLI(args []string, out, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: task-mecca notifications resume-status|suppress-before-resume --project ABSOLUTE [--cutoff RFC3339 --confirm-project EXACT]")
		return 2
	}
	flags := flag.NewFlagSet("notifications", flag.ContinueOnError)
	flags.SetOutput(stderr)
	project := flags.String("project", "", "absolute project path")
	cutoff := flags.String("cutoff", "", "approved resume cutoff in RFC3339")
	confirm := flags.String("confirm-project", "", "repeat exact project path; no configuration or network changes")
	_ = flags.Bool("json", false, "output is always credential-free JSON")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if !filepath.IsAbs(*project) {
		fmt.Fprintln(stderr, "an absolute --project is required")
		return 2
	}
	if info, err := os.Stat(filepath.Join(*project, "_task_mecca")); err != nil || !info.IsDir() {
		fmt.Fprintln(stderr, "an installed project is required")
		return 2
	}
	var result notify.ResumeResult
	var err error
	switch args[0] {
	case "resume-status":
		result, err = notify.ResumeStatus(*project)
	case "suppress-before-resume":
		if *confirm != *project || *cutoff == "" {
			fmt.Fprintln(stderr, "--confirm-project must repeat --project and --cutoff is required")
			return 2
		}
		if !notify.TelegramTransportDisabled() {
			fmt.Fprintln(stderr, "Telegram transport must remain disabled while committing resume suppression")
			return 2
		}
		var rows []map[string]any
		rows, err = backlog.ReadNotificationEvents(*project)
		if err == nil {
			var events []notify.Event
			field := func(row map[string]any, key string) string { value, _ := row[key].(string); return value }
			for _, row := range rows {
				events = append(events, notify.Event{ID: field(row, "id"), TaskID: field(row, "task_id"), Kind: field(row, "kind"), At: field(row, "at")})
			}
			result, err = notify.SuppressBeforeResume(*project, *cutoff, events)
		}
	default:
		fmt.Fprintln(stderr, "unknown notification action")
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "resume operation failed; keep Telegram transport disabled:", err)
		return 1
	}
	if err := json.NewEncoder(out).Encode(result); err != nil {
		fmt.Fprintln(stderr, "resume result output failed; verify resume-status before enabling transport")
		return 1
	}
	return 0
}
