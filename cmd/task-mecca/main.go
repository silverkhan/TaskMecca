package main

import (
    "fmt"
    "os"
    "path/filepath"

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
        fmt.Fprintln(os.Stderr, "usage: task-mecca <init|update|doctor|web> [--project DIR]")
        return 2
    }
    command := args[0]
    project := "."
    for i := 1; i < len(args); i++ {
        if args[i] == "--project" && i+1 < len(args) {
            project = args[i+1]
            i++
        } else {
            fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
            return 2
        }
    }
    root, err := filepath.Abs(project)
    if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    switch command {
    case "init":
        if err = install.Init(root, version); err == nil {
            fmt.Printf("Task Mecca %s installed to %s\n", version, filepath.Join(root, "_task_mecca"))
        }
    case "update":
        err = install.Update(root, version)
        if err == nil { fmt.Printf("Task Mecca %s updated\n", version) }
    default:
        // Go runtime commands must be implemented before this CLI can replace Python.
        fmt.Fprintf(os.Stderr, "%s is not yet implemented in the Go runtime\n", command)
        return 2
    }
    if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    return 0
}
