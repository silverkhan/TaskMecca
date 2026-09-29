package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
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
        fmt.Fprintln(os.Stderr, "usage: task-mecca <init|update|agent|worker-name|ensure-backlog|next-id|search|ready|inspect|workload> [options]")
        return 2
    }
    command := args[0]
    project := "."
    rootOption := ""
    jsonOutput := false
    newWorker := false
    used := []string{}
    limit := 10
    positional := []string{}
    for i := 1; i < len(args); i++ {
        if args[i] == "--project" && i+1 < len(args) {
            project = args[i+1]
            i++
        } else if args[i] == "--root" && i+1 < len(args) {
            rootOption = args[i+1]
            i++
        } else if args[i] == "--used" && i+1 < len(args) {
            used = append(used, args[i+1])
            i++
        } else if args[i] == "--limit" && i+1 < len(args) {
            parsed, parseErr := strconv.Atoi(args[i+1])
            if parseErr != nil { fmt.Fprintln(os.Stderr, "--limit requires an integer"); return 2 }
            limit = parsed
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
    case "worker-name":
        report, workerErr := backlog.WorkerName(root, rootOption, used)
        if workerErr != nil { err = workerErr; break }
        if jsonOutput { emitJSON(report) } else if report["ok"] == true {
            fmt.Println(report["path"])
            fmt.Println("신규 검증: agent " + report["path"].(string) + " --new --json")
        } else { fmt.Fprintln(os.Stderr, "사용 가능한 포켓몬 worker 이름이 없습니다.") }
        if report["ok"] != true { return 2 }
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
    case "search":
        if len(positional) != 1 { fmt.Fprintln(os.Stderr, "search requires a query"); return 2 }
        var report backlog.SearchReport
        report, err = backlog.Search(root, rootOption, positional[0], limit)
        if err == nil {
            if jsonOutput { emitJSON(report) } else if len(report.Results)==0 {
                fmt.Println("(후보 없음)")
            } else {
                for _,row:=range report.Results {
                    fmt.Printf("%-8s %-5s %s score=%d\n", row.ID, row.State, row.Title, row.Score)
                }
            }
        }
    case "ready":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "ready takes no positional arguments"); return 2 }
        var report map[string]any
        report, err = backlog.Ready(root, rootOption)
        if err == nil {
            if jsonOutput { emitJSON(report) } else {
                readyRows,_:=report["ready"].([]map[string]any)
                blockedRows,_:=report["blocked"].([]map[string]any)
                readyIDs:=[]string{}
                blockedIDs:=[]string{}
                for _,row:=range readyRows { readyIDs=append(readyIDs,row["id"].(string)) }
                for _,row:=range blockedRows { blockedIDs=append(blockedIDs,row["id"].(string)) }
                readyText:="-"; if len(readyIDs)>0 { readyText=strings.Join(readyIDs,", ") }
                blockedText:="-"; if len(blockedIDs)>0 { blockedText=strings.Join(blockedIDs,", ") }
                fmt.Println("Ready: "+readyText)
                fmt.Println("Blocked: "+blockedText)
                for _,row:=range blockedRows {
                    if note,ok:=row["waiting_note"].(string); ok && note!="" { fmt.Printf("  %s 대기: %s\n",row["id"],note) }
                }
            }
        }
        if err==nil {
            problems,_:=report["problems"].(map[string]any)
            if problems!=nil {
                missing,_:=problems["missing"].([]map[string]string)
                cycles,_:=problems["cycles"].([][]string)
                if len(missing)>0 || len(cycles)>0 { return 1 }
            }
        }
    case "inspect":
        if len(positional) != 1 { fmt.Fprintln(os.Stderr, "inspect requires an id"); return 2 }
        var report map[string]any
        report, err = backlog.Inspect(root, rootOption, positional[0])
        if err==nil {
            if report["exists"]!=true {
                fmt.Fprintf(os.Stderr,"%s 항목을 찾지 못했다.\n",strings.ToUpper(positional[0]))
                return 2
            }
            if jsonOutput { emitJSON(report) } else {
                fmt.Printf("%s %s %s\n",report["id"],report["state"],report["title"])
                fmt.Printf("Agent: %v  범위: %v\n",emptyDash(report["agent"]),emptyDash(report["change_scope"]))
                fmt.Printf("ready: %v  waiting: %s\n",report["ready"],joinAnyStrings(report["waiting_for"]))
                if note,ok:=report["waiting_note"].(string); ok && note!="" { fmt.Println("대기: "+note) }
            }
            if count,ok:=report["duplicate_count"].(int); ok && count>1 { return 1 }
        }
    case "workload":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "workload takes no positional arguments"); return 2 }
        var report map[string]any
        report, err = backlog.Workload(root, rootOption)
        if err==nil {
            if jsonOutput { emitJSON(report) } else {
                agents,_:=report["agents"].([]map[string]any)
                for _,row:=range agents {
                    fmt.Printf("%s: doing=%s blocking=%s ready_history=%s\n",
                        row["agent"],
                        joinAnyStrings(row["doing"]),
                        joinAnyStrings(row["blocking"]),
                        readyCandidateIDs(row["ready_candidates"]),
                    )
                }
                fmt.Println("execution: RuntimeProvider/Dispatch상태 only; model/effort tracking removed")
                if len(agents)==0 { fmt.Println("(Agent workload 없음)") }
            }
            if unassigned,ok:=report["unassigned_doing"].([]string); ok && len(unassigned)>0 { return 1 }
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

func emptyDash(value any) any {
    if text,ok:=value.(string); ok && text=="" { return "-" }
    return value
}

func joinAnyStrings(value any) string {
    values:=[]string{}
    switch rows:=value.(type) {
    case []string:
        values=rows
    case []any:
        for _,row:=range rows { values=append(values,fmt.Sprint(row)) }
    }
    if len(values)==0 { return "-" }
    return strings.Join(values,",")
}

func readyCandidateIDs(value any) string {
    rows,ok:=value.([]map[string]any)
    if !ok || len(rows)==0 { return "-" }
    ids:=[]string{}
    for _,row:=range rows { ids=append(ids,fmt.Sprint(row["id"])) }
    return strings.Join(ids,",")
}
