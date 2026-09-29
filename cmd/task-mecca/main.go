package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/silverkhan/TaskMecca/internal/backlog"
    "github.com/silverkhan/TaskMecca/internal/install"
)

const version = "0.2.1"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
    if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
        fmt.Printf("task-mecca %s\n", version)
        return 0
    }
    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "usage: task-mecca <init|update|agent|ensure-backlog|next-id> [options]")
        return 2
    }
    command := args[0]
    project := "."
    rootOption := ""
    jsonOutput := false
    newWorker := false
    positional := []string{}
    for i := 1; i < len(args); i++ {
        if args[i] == "--project" && i+1 < len(args) {
            project = args[i+1]
            i++
        } else if args[i] == "--root" && i+1 < len(args) {
            rootOption = args[i+1]
            i++
        } else if args[i] == "--json" || args[i] == "--allow-empty" || args[i] == "--new" {
            if args[i] == "--json" { jsonOutput = true }
            if args[i] == "--new" { newWorker = true }
        } else if !strings.HasPrefix(args[i], "-") {
            positional = append(positional, args[i])
        } else {
            fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
            return 2
        }
    }
    root, err := filepath.Abs(project)
    if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    if rootOption != "" {
        if !filepath.IsAbs(rootOption) { rootOption = filepath.Join(root, rootOption) }
        rootOption, err = filepath.Abs(rootOption)
        if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    }
    switch command {
    case "agent":
        if len(positional) != 1 { fmt.Fprintln(os.Stderr, "agent requires a path"); return 2 }
        report := backlog.AgentReport(positional[0], newWorker)
        if jsonOutput { emitJSON(report) } else {
            label := "유효하지 않음"
            if report["ok"] == true { label = "유효" }
            mode := "(기존 identity/재사용)"
            if newWorker { mode = "(신규 생성)" }
            fmt.Println(label, report["path"], mode)
        }
        if report["ok"] != true { return 1 }
        return 0
    case "init":
        if err = install.Init(root, version); err == nil {
            fmt.Printf("Task Mecca %s installed to %s\n", version, filepath.Join(root, "_task_mecca"))
        }
    case "update":
        err = install.Update(root, version)
        if err == nil { fmt.Printf("Task Mecca %s updated\n", version) }
    case "ensure-backlog":
        var report map[string]any
        report, err = backlog.Ensure(root, rootOption)
        if err == nil {
            if jsonOutput { emitJSON(report) } else {
                label := "existing: "
                if report["created"] == true { label = "created: " }
                fmt.Println(label + report["path"].(string))
            }
        }
    case "next-id":
        if len(positional) != 1 { fmt.Fprintln(os.Stderr, "next-id requires a prefix"); return 2 }
        var report map[string]any
        report, err = backlog.NextID(root, rootOption, positional[0])
        if err == nil {
            if jsonOutput { emitJSON(report) } else { fmt.Printf("%s.%s\n", report["sort_key"], report["id"]) }
        }
    default:
        // Go runtime commands must be implemented before this CLI can replace Python.
        fmt.Fprintf(os.Stderr, "%s is not yet implemented in the Go runtime\n", command)
        return 2
    }
    if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    return 0
}

func emitJSON(value any) {
    encoder := json.NewEncoder(os.Stdout)
    encoder.SetEscapeHTML(false)
    _ = encoder.Encode(value)
}
