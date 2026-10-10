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
		fmt.Fprintln(errors, "usage: task-mecca projects list|archived|add|unregister|register|archive|restore|forget [--path ABSOLUTE --archive-id ID --confirm-path EXACT]")
		return 2
	}
	action := args[0]
	flags := flag.NewFlagSet("projects", flag.ContinueOnError)
	flags.SetOutput(errors)
	path := flags.String("path", "", "absolute project path")
	id := flags.String("archive-id", "", "exact archive ID")
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
	if action == "archived" {
		value, err := maintenance.ArchivedProjects()
		return write(value, err)
	}
	if *path == "" || !filepath.IsAbs(*path) {
		fmt.Fprintln(errors, "an absolute --path is required")
		return 2
	}
	if action == "archive" || action == "forget" {
		if *confirm != *path {
			fmt.Fprintln(errors, "--confirm-path must repeat the exact absolute --path; files are always preserved")
			return 2
		}
	}
	switch action {
	case "add":
		return write(map[string]string{"action": action}, maintenance.RegisterProject(*path))
	case "unregister", "register":
		return write(map[string]string{"action": action}, maintenance.SetProjectMonitoring(*path, action == "register"))
	case "archive":
		value, err := maintenance.ArchiveProject(*path)
		return write(value, err)
	case "forget":
		return write(map[string]string{"result": "forgotten"}, maintenance.ForgetArchivedProject(*id, *path))
	case "restore":
		return write(map[string]string{"result": "restored"}, maintenance.RestoreArchivedProject(*id, *path))
	default:
		fmt.Fprintln(errors, "unknown project action:", action)
		return 2
	}
}
