package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func runProjectCLI(args []string, out, errors io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errors, "usage: task-mecca projects list|history|pause|resume|remove|delete-history|trash [--path ABSOLUTE --history-id ID --confirm-path EXACT]")
		return 2
	}
	action := args[0]
	flags := flag.NewFlagSet("projects", flag.ContinueOnError)
	flags.SetOutput(errors)
	path := flags.String("path", "", "absolute project path")
	id := flags.String("history-id", "", "exact removal history ID")
	confirm := flags.String("confirm-path", "", "repeat exact full path for destructive actions")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	write := func(value any, err error) int {
		if err != nil {
			fmt.Fprintln(errors, err)
			if value != nil {
				_ = json.NewEncoder(out).Encode(value)
			}
			return 1
		}
		if err = json.NewEncoder(out).Encode(value); err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		return 0
	}
	if action == "list" {
		value, err := maintenance.ManagedProjects()
		return write(value, err)
	}
	if action == "history" {
		value, err := maintenance.RemovalHistory()
		return write(value, err)
	}
	if *path == "" || !filepath.IsAbs(*path) {
		fmt.Fprintln(errors, "an absolute --path is required")
		return 2
	}
	if action == "remove" || action == "trash" || action == "delete-history" {
		if *confirm != *path {
			fmt.Fprintln(errors, "--confirm-path must repeat the exact absolute --path; registration/history/folder operations are separate")
			return 2
		}
	}
	switch action {
	case "pause", "resume":
		return write(map[string]string{"action": action}, maintenance.SetProjectMonitoring(*path, action == "resume"))
	case "remove":
		value, err := maintenance.RemoveProject(*path)
		return write(value, err)
	case "delete-history":
		history, err := maintenance.RemovalHistory()
		if err != nil {
			return write(nil, err)
		}
		for _, record := range history {
			if record.ID == *id && filepath.Clean(record.Path) == filepath.Clean(*path) {
				return write(map[string]string{"result": "history_deleted"}, maintenance.DeleteRemovalHistory(*id))
			}
		}
		return write(nil, fmt.Errorf("exact history/path binding required"))
	case "trash":
		if *id == "" {
			fmt.Fprintln(errors, "--history-id is required for folder cleanup")
			return 2
		}
		location, err := maintenance.MoveRemovedProjectToTrash(*id, *path)
		result := "moved_to_trash"
		if err != nil {
			result = "cleanup_failed_or_incomplete"
		}
		return write(map[string]string{"result": result, "trash_path": location, "staging_path": maintenance.TrashStagingPath(location)}, err)
	default:
		fmt.Fprintln(errors, "unknown project action:", action)
		return 2
	}
}
