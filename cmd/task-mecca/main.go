package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
    "github.com/silverkhan/TaskMecca/internal/install"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/webui"
)

const version = "0.2.8"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
    if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
        fmt.Printf("task-mecca %s\n", version)
        return 0
    }
    if len(args) == 0 {
        root,err:=filepath.Abs(".")
        if err!=nil { fmt.Fprintln(os.Stderr,err); return 2 }
        _ = maintenance.RegisterProject(root)
        if err:=webui.Run(webui.Config{Project:root,Port:8765,OpenBrowser:true,Version:version}); err!=nil {
            fmt.Fprintln(os.Stderr,err); return 2
        }
        return 0
    }
    command := args[0]
    project := "."
    rootOption := ""
    jsonOutput := false
    newWorker := false
    used := []string{}
    limit := 10
    workerCap := 3
    protocol := ""
    backlogOnly := false
    includeDone := false
    requireFullAccess := false
    watch := false
    once := false
    noOpen := false
    port := 8765
    interval := 1.0
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
        } else if args[i] == "--worker-cap" && i+1 < len(args) {
            parsed, parseErr := strconv.Atoi(args[i+1])
            if parseErr != nil { fmt.Fprintln(os.Stderr, "--worker-cap requires an integer"); return 2 }
            workerCap = parsed
            i++
        } else if args[i] == "--protocol" && i+1 < len(args) {
            protocol = args[i+1]
            i++
        } else if args[i] == "--port" && i+1 < len(args) {
            parsed,parseErr:=strconv.Atoi(args[i+1])
            if parseErr!=nil { fmt.Fprintln(os.Stderr,"--port requires an integer"); return 2 }
            port=parsed; i++
        } else if args[i] == "--interval" && i+1 < len(args) {
            parsed,parseErr:=strconv.ParseFloat(args[i+1],64)
            if parseErr!=nil { fmt.Fprintln(os.Stderr,"--interval requires a number"); return 2 }
            interval=parsed; i++
        } else if args[i] == "--json" || args[i] == "--allow-empty" || args[i] == "--new" || args[i] == "--backlog-only" || args[i] == "--done" || args[i] == "--require-full-access" || args[i] == "--watch" || args[i] == "--once" || args[i] == "--no-open" {
            if args[i] == "--json" { jsonOutput = true }
            if args[i] == "--new" { newWorker = true }
            if args[i] == "--backlog-only" { backlogOnly = true }
            if args[i] == "--done" { includeDone = true }
            if args[i] == "--require-full-access" { requireFullAccess = true }
            if args[i] == "--watch" { watch = true }
            if args[i] == "--once" { once = true }
            if args[i] == "--no-open" { noOpen = true }
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
            _ = maintenance.RegisterProject(root)
            fmt.Printf("Task Mecca %s installed to %s\n", version, filepath.Join(root, "_task_mecca"))
        }
    case "migrate":
        err = install.Migrate(root, version)
        if err == nil { _ = maintenance.RegisterProject(root); fmt.Printf("Task Mecca %s migrated\n", version) }
    case "upgrade":
        var result maintenance.UpgradeResult
        result, err = maintenance.Upgrade(version)
        if err == nil {
            if result.To == "" || result.To == version { fmt.Printf("Task Mecca %s is already current\n", version) } else { fmt.Printf("Task Mecca upgraded %s -> %s\n", result.From, result.To); if result.RestartRequired { fmt.Println("Restart Task Mecca to use the new version.") } }
        }
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
    case "coordinate":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "coordinate takes no positional arguments"); return 2 }
        var report map[string]any
        report, err = backlog.Coordinate(root, rootOption, workerCap)
        if err==nil {
            if jsonOutput { emitJSON(report) } else {
                fmt.Printf("snapshot: %v\n",report["snapshot_at"])
                fmt.Printf("scheduling_needed: %v\n",report["scheduling_needed"])
                fmt.Printf("controller_review_needed: %v (advisory)\n",report["controller_review_needed"])
                fmt.Println("ready: "+mapIDs(report["ready"]))
                fmt.Println("doing: "+doingIDs(report["doing"]))
                fmt.Println("execution: RuntimeProvider/Dispatch상태 only; model/effort tracking removed")
            }
            if conflicts,ok:=report["scope_conflicts"].([]map[string]any); ok && len(conflicts)>0 { return 1 }
        }
    case "audit":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "audit takes no positional arguments"); return 2 }
        var findings []map[string]string
        findings, err = backlog.Audit(root, rootOption)
        if err==nil {
            if len(findings)==0 {
                fmt.Println("통과: 상태별 필수 칸이 모두 채워져 있다.")
            } else {
                emitJSON(findings)
                return 1
            }
        }
    case "check":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "check takes no positional arguments"); return 2 }
        if protocol!="" && !filepath.IsAbs(protocol) { protocol=filepath.Join(root,protocol) }
        var problems []string
        problems, err = backlog.Check(root, rootOption, protocol)
        if err==nil {
            if len(problems)>0 && strings.HasPrefix(problems[0],"__missing_protocol__:") {
                fmt.Fprintln(os.Stderr,strings.TrimPrefix(problems[0],"__missing_protocol__:")+" 이 없다.")
                return 2
            }
            if len(problems)>0 {
                fmt.Fprintln(os.Stderr,strings.Join(problems,"\n"))
                return 1
            }
            fmt.Println("통과: 문서 링크와 백로그 선행관계가 유효하다.")
        }
    case "doctor":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "doctor takes no positional arguments"); return 2 }
        var report map[string]any
        report, err = backlog.Doctor(root, rootOption, !backlogOnly)
        if err==nil {
            if jsonOutput {
                emitJSON(report)
            } else if report["ok"]==true {
                fmt.Println("통과: 기계 검사가 모두 정상이다.")
            } else {
                fmt.Println("문제 발견:")
                if checks,ok:=report["checks"].(map[string]any); ok {
                    for _,name:=range []string{"backlog_presence","audit","duplicate_ids","missing_dependencies","dependency_cycles","filenames","agent_paths","scope_conflicts","contracts","dangling_links"} {
                        if value,exists:=checks[name]; exists && hasItems(value) {
                            data,_:=json.Marshal(value)
                            fmt.Printf("  %s: %s\n",name,string(data))
                        }
                    }
                }
            }
            if report["ok"]!=true { return 1 }
        }
    case "status":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "status takes no positional arguments"); return 2 }
        if watch && !jsonOutput {
            _ = maintenance.RegisterProject(root)
            err=webui.Run(webui.Config{Project:root,Root:rootOption,Port:8765,OpenBrowser:true,Version:version})
            break
        }
        if watch && jsonOutput {
            if interval<0.5 { interval=0.5 }
            for {
                report,statusErr:=backlog.Status(root,rootOption,includeDone)
                if statusErr!=nil { err=statusErr; break }
                emitJSON(report)
                time.Sleep(time.Duration(interval*float64(time.Second)))
            }
            break
        }
        var report map[string]any
        report, err = backlog.Status(root, rootOption, includeDone)
        if err==nil {
            if jsonOutput { emitJSON(report) } else { printStatus(report) }
        }
    case "web":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "web takes no positional arguments"); return 2 }
        _ = maintenance.RegisterProject(root)
        err=webui.Run(webui.Config{Project:root,Root:rootOption,Port:port,OpenBrowser:!noOpen,Version:version})
    case "monitor":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "monitor takes no positional arguments"); return 2 }
        if !once && !jsonOutput {
            _ = maintenance.RegisterProject(root)
            err=webui.Run(webui.Config{Project:root,Root:rootOption,Port:port,OpenBrowser:!noOpen,Version:version})
            break
        }
        if interval<0.5 { interval=0.5 }
        for {
            report,statusErr:=backlog.Status(root,rootOption,includeDone)
            if statusErr!=nil { err=statusErr; break }
            if jsonOutput { emitJSON(report) } else { printStatus(report) }
            if once { break }
            time.Sleep(time.Duration(interval*float64(time.Second)))
        }
    case "preflight":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "preflight takes no positional arguments"); return 2 }
        var report map[string]any
        report, err = backlog.Preflight(root, rootOption, requireFullAccess)
        if err==nil {
            if jsonOutput {
                emitJSON(report)
            } else {
                if backlogReport,ok:=report["backlog"].(map[string]any); ok {
                    fmt.Printf("BACKLOG %s: %v\n",strings.ToUpper(fmt.Sprint(backlogReport["status"])),backlogReport["message"])
                }
                if access,ok:=report["access"].(map[string]any); ok {
                    fmt.Printf("ACCESS %s: %v\n",strings.ToUpper(fmt.Sprint(access["status"])),access["message"])
                    if access["network"]=="disabled" { fmt.Println("NETWORK DISABLED: 네트워크가 필요한 작업은 별도 권한 확인이 필요합니다.") }
                }
            }
            if backlogReport,ok:=report["backlog"].(map[string]any); ok && backlogReport["ok"]!=true { return 2 }
            if requireFullAccess && report["orchestration_ready"]!=true { return 3 }
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

func singleLine(value any) any {
    text,ok:=value.(string)
    if !ok { return value }
    text=strings.TrimSpace(text)
    if text=="" { return "-" }
    if i:=strings.IndexByte(text,'\n'); i>=0 { text=text[:i] }
    return strings.TrimSpace(text)
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

func mapIDs(value any) string {
    rows,ok:=value.([]map[string]any)
    if !ok || len(rows)==0 { return "-" }
    ids:=[]string{}
    for _,row:=range rows { ids=append(ids,fmt.Sprint(row["id"])) }
    return strings.Join(ids,", ")
}

func doingIDs(value any) string {
    rows,ok:=value.([]map[string]any)
    if !ok || len(rows)==0 { return "-" }
    ids:=[]string{}
    for _,row:=range rows { ids=append(ids,fmt.Sprintf("%v@%v",row["id"],emptyDash(row["agent"]))) }
    return strings.Join(ids,", ")
}

func hasItems(value any) bool {
    switch v:=value.(type) {
    case []any: return len(v)>0
    case []string: return len(v)>0
    case [][]string: return len(v)>0
    case []map[string]any: return len(v)>0
    case []map[string]string: return len(v)>0
    default: return value!=nil
    }
}

func printStatus(report map[string]any) {
    counts,_:=report["counts"].(map[string]any)
    fmt.Println("Task Mecca · root/registrar/controller/worker backlog")
    fmt.Printf("updated=%v\n",report["snapshot_at"])
    fmt.Printf("root=%v\n",report["root"])
    fmt.Printf("doing=%v ready=%v blocked=%v hold=%v done=%v warnings=%v\n\n",
        counts["doing"],counts["ready"],counts["blocked"],counts["hold"],counts["done_total"],counts["warnings"])
    fmt.Println("Active")
    active,_:=report["active"].([]map[string]any)
    if len(active)==0 { fmt.Println("(active 항목 없음)") }
    for _,item:=range active {
        fmt.Printf("%-8v %-8v %-24v %-12v %v\n",item["id"],item["state"],singleLine(emptyDash(item["agent"])),item["time"],item["title"])
    }
    done,_:=report["done"].([]map[string]any)
    if len(done)>0 {
        fmt.Println("\nDone")
        for _,item:=range done {
            fmt.Printf("%-8v done     %-24v %-12v %v\n",item["id"],singleLine(emptyDash(item["agent"])),item["time"],item["title"])
        }
    }
    fmt.Println("\nLive agents: use the current model runtime agent list; this local monitor does not infer liveness from Git.")
}
