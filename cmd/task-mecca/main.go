package main

import (
	"bufio"
	"errors"
	"golang.org/x/term"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/silverkhan/TaskMecca/internal/backlog"
    "github.com/silverkhan/TaskMecca/internal/handoff"
    "github.com/silverkhan/TaskMecca/internal/install"
    "github.com/silverkhan/TaskMecca/internal/maintenance"
    "github.com/silverkhan/TaskMecca/internal/notify"
    "github.com/silverkhan/TaskMecca/internal/runtimeobs"
    "github.com/silverkhan/TaskMecca/internal/webui"
)

var version = "0.3.0"

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
    if len(args)>0 && args[0]=="notifications" { return runNotificationCLI(args[1:],os.Stdout,os.Stderr) }
    if len(args)>0 && args[0]=="operations" { return runOperationsCLI(args[1:],os.Stdout,os.Stderr) }
 if len(args)>0 && args[0]=="projects" { return runProjectCLI(args[1:],os.Stdout,os.Stderr) }
    if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
        fmt.Printf("task-mecca %s\n", version)
        fmt.Printf("channel %s\n", maintenance.CurrentChannel())
        info:=maintenance.RefreshVersionInfo(version)
        if info.UpdateAvailable {
            fmt.Printf("latest %s · update available\n",info.Latest)
            fmt.Println("run: task-mecca upgrade")
        } else if info.Latest!="" {
            fmt.Printf("latest %s\n",info.Latest)
        } else if info.Error!="" {
            fmt.Fprintf(os.Stderr,"latest check failed: %s\n",info.Error)
        }
        return 0
    }
    if len(args) > 0 && args[0] == "handoff" {
        return handoff.RunCLI(args[1:], os.Stdout, os.Stderr)
    }
    if len(args) == 0 {
        root,err:=filepath.Abs(".")
        if err!=nil { fmt.Fprintln(os.Stderr,err); return 2 }
        _ = maintenance.RegisterWebProject(root)
        state,startErr:=webui.StartService(webui.Config{Project:root,Host:"auto",Port:webui.DefaultPort,OpenBrowser:true,Version:version})
        if startErr!=nil { fmt.Fprintln(os.Stderr,startErr); return 2 }
        printWebState("Task Mecca Web is running",state)
        return 0
    }
    command := args[0]
    project := "."
    rootOption := ""
    jsonOutput := false
    migrationChoice := ""
    migrationExpectedPlan := ""
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
    foreground := false
    follow := false
    webInstanceID := ""
    webControlToken := ""
    host := "auto"
    port := webui.DefaultPort
    interval := 1.0
    positional := []string{}
    for i := 1; i < len(args); i++ {
        if args[i] == "--expect-plan" && i+1 < len(args) {
            migrationExpectedPlan=args[i+1]; i++
        } else if args[i] == "--choice" && i+1 < len(args) {
            migrationChoice=args[i+1]; i++
        } else if args[i] == "--project" && i+1 < len(args) {
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
        } else if args[i] == "--host" && i+1 < len(args) {
            host = args[i+1]
            i++
        } else if args[i] == "--web-instance-id" && i+1 < len(args) {
            webInstanceID = args[i+1]
            i++
        } else if args[i] == "--web-control-token" && i+1 < len(args) {
            webControlToken = args[i+1]
            i++
        } else if args[i] == "--port" && i+1 < len(args) {
            parsed,parseErr:=strconv.Atoi(args[i+1])
            if parseErr!=nil { fmt.Fprintln(os.Stderr,"--port requires an integer"); return 2 }
            port=parsed; i++
        } else if args[i] == "--interval" && i+1 < len(args) {
            parsed,parseErr:=strconv.ParseFloat(args[i+1],64)
            if parseErr!=nil { fmt.Fprintln(os.Stderr,"--interval requires a number"); return 2 }
            interval=parsed; i++
        } else if args[i] == "--json" || args[i] == "--allow-empty" || args[i] == "--new" || args[i] == "--backlog-only" || args[i] == "--done" || args[i] == "--require-full-access" || args[i] == "--watch" || args[i] == "--once" || args[i] == "--no-open" || args[i] == "--foreground" || args[i] == "--follow" {
            if args[i] == "--json" { jsonOutput = true }
            if args[i] == "--new" { newWorker = true }
            if args[i] == "--backlog-only" { backlogOnly = true }
            if args[i] == "--done" { includeDone = true }
            if args[i] == "--require-full-access" { requireFullAccess = true }
            if args[i] == "--watch" { watch = true }
            if args[i] == "--once" { once = true }
            if args[i] == "--no-open" { noOpen = true }
            if args[i] == "--foreground" { foreground = true }
            if args[i] == "--follow" { follow = true }
        } else if !strings.HasPrefix(args[i], "-") {
            positional = append(positional, args[i])
        } else {
            fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
            return 2
        }
    }
    if webInstanceID!="" && webControlToken!="" {
        host=webui.NormalizeManagedHost(host)
    }
    root, err := filepath.Abs(project)
    if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    if rootOption != "" {
        if !filepath.IsAbs(rootOption) { rootOption = filepath.Join(root, rootOption) }
        rootOption, err = filepath.Abs(rootOption)
        if err != nil { fmt.Fprintln(os.Stderr, err); return 2 }
    }
    switch command {
    case "deliveries":
        if len(positional)>2 {fmt.Fprintln(os.Stderr,"deliveries accepts [task-id] [event-id]");return 2}
        taskID,eventID:="",""
        if len(positional)>0 {taskID=positional[0]}
        if len(positional)>1 {eventID=positional[1]}
        records,deliveryErr:=notify.DeliveryRecords(root,taskID,eventID)
        if deliveryErr!=nil {err=deliveryErr;break}
        if jsonOutput {emitJSON(map[string]any{"deliveries":records})} else {
            for _,record:=range records {fmt.Printf("%s %s %s %s attempts=%d duplicate_possible=%t\n",record.TaskID,record.EventID,record.Kind,record.State,record.Attempts,record.DuplicatePossible)}
        }
    case "channel":
        if len(positional)==0 {
            fmt.Println(maintenance.CurrentChannel())
            return 0
        }
        if len(positional)!=1 {
            fmt.Fprintln(os.Stderr,"channel takes zero arguments or one of: stable, dev")
            return 2
        }
        before:=maintenance.CurrentChannel()
        if err=maintenance.SetChannel(positional[0]); err==nil {
            after:=maintenance.CurrentChannel()
            if before==after {
                fmt.Printf("Update channel: %s\n",after)
            } else {
                fmt.Printf("Update channel changed: %s -> %s\n",before,after)
            }
            info:=maintenance.RefreshVersionInfo(version)
            if info.Error!="" {
                fmt.Fprintf(os.Stderr,"latest check failed: %s\n",info.Error)
            } else if info.UpdateAvailable {
                fmt.Printf("latest %s · update available\n",info.Latest)
                fmt.Println("run: task-mecca upgrade")
            } else if info.Latest!="" {
                fmt.Printf("latest %s\n",info.Latest)
            }
        }
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
            _,_ = backlog.EnsureTagRegistry(root)
            if registerErr := maintenance.RegisterProject(root); registerErr != nil { err = fmt.Errorf("framework installed; Web monitoring registration failed (framework preserved): %w", registerErr); break }
            fmt.Printf("Task Mecca %s installed to %s\n", version, filepath.Join(root, "_task_mecca"))
        }
	case "migrate":
		var migration install.MigrationResult
		migration, err = install.MigrateWithPlan(root, version, migrationChoice,migrationExpectedPlan)
		var required *install.ChoiceRequiredError
		if errors.As(err, &required) {
			if jsonOutput || !term.IsTerminal(int(os.Stdin.Fd())) {
				emitJSON(migration)
				return 3
			}
			fmt.Println("수정된 프레임워크 파일:")
			for _, path := range migration.ModifiedFiles {
				fmt.Println("  " + path)
			}
			fmt.Print("1 새 버전으로 덮어쓰기 / 2 기존 수정사항을 백업하고 진행 / 3 취소: ")
			answer, readErr := bufio.NewReader(os.Stdin).ReadString('\n')
			choice := "cancel"
			if readErr == nil {
				switch strings.TrimSpace(answer) {
				case "1":
					choice = "overwrite"
				case "2":
					choice = "backup"
				}
			}
			migration, err = install.MigrateWithPlan(root, version, choice,migration.PlanDigest)
		}
		if errors.As(err, &required) {
			emitJSON(migration)
			return 3
		}
		if err == nil {
			if jsonOutput {
				emitJSON(migration)
				return 0
			}
			if migration.Status == "cancelled" {
				fmt.Println("Migration cancelled; no files changed.")
				return 0
			}
			fmt.Printf("Task Mecca %s migrated\n", version)
			if migration.InstructionRefreshRequired {
				fmt.Println("Root session refresh required: Task Mecca operating instructions changed.")
				fmt.Println("Open the Web Global Hub migration result or ask the active Root session to reread the current Task Mecca instructions before continuing.")
			}
		}
    case "upgrade":
        var result maintenance.UpgradeResult
        result, err = maintenance.Upgrade(version)
        if err == nil {
            if result.To == "" || result.To == version {
                fmt.Printf("Task Mecca %s is already current\n", version)
            } else {
                fmt.Printf("Task Mecca upgraded %s -> %s\n", result.From, result.To)
                fmt.Printf("CLI update complete. New task-mecca commands will use %s; no CLI restart is required.\n", result.To)
                webState:=webui.ServiceStatus()
                if webState.Running {
                    if result.Scheduled {
                        fmt.Printf("Task Mecca Web is running on %s. Stopping it so Windows can replace the executable after this command exits...\n", webState.Version)
                        if _,stopErr:=webui.StopService(); stopErr!=nil {
                            fmt.Fprintf(os.Stderr,"CLI update is staged, but Task Mecca Web could not be stopped: %v\n",stopErr)
                            fmt.Fprintln(os.Stderr,"Close the running Task Mecca Web process, then run: task-mecca web restart")
                        } else {
                            fmt.Println("Task Mecca Web stopped. After the executable replacement completes, start it again with: task-mecca web")
                        }
                    } else {
                        fmt.Printf("Task Mecca Web is running on %s; restarting it automatically...\n", webState.Version)
                        restarted,restartErr:=webui.RestartService(webui.Config{
                            Project:webState.Project,
                            Host:webui.NormalizeManagedHost(webState.Host),
                            Port:webState.Port,
                            OpenBrowser:false,
                            Version:result.To,
                        })
                        if restartErr!=nil {
                            fmt.Fprintf(os.Stderr,"CLI update succeeded, but Task Mecca Web restart failed: %v\n",restartErr)
                            fmt.Fprintln(os.Stderr,"Run: task-mecca web restart")
                        } else {
                            fmt.Printf("Task Mecca Web restarted · version %s\n",restarted.Version)
                        }
                    }
                }
            }
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
                if source,ok:=report["source"].(map[string]string); ok && source["label"]!="" { if source["url"]!="" { fmt.Printf("출처: %s  %s\n",source["label"],source["url"]) } else { fmt.Printf("출처: %s\n",source["label"]) } }
                if tags,tagErr:=backlog.TaskTags(root,rootOption,positional[0]); tagErr==nil { fmt.Printf("Tags: %s\n",joinAnyStrings(tags["tags"])) }
                fmt.Printf("ready: %v  waiting: %s\n",report["ready"],joinAnyStrings(report["waiting_for"]))
                if note,ok:=report["waiting_note"].(string); ok && note!="" { fmt.Println("대기: "+note) }
            }
            if count,ok:=report["duplicate_count"].(int); ok && count>1 { return 1 }
        }
    case "tags":
        action:="list"
        if len(positional)>0 { action=strings.ToLower(positional[0]) }
        switch action {
        case "list":
            if len(positional)>1 { fmt.Fprintln(os.Stderr,"tags list takes no arguments"); return 2 }
            var rows []backlog.TagDefinition
            rows,err=backlog.TagList(root)
            if err==nil {
                if jsonOutput { emitJSON(rows) } else { printTagDefinitions(rows) }
            }
        case "search","discover":
            if len(positional)<2 { fmt.Fprintln(os.Stderr,"tags search/discover requires a query"); return 2 }
            var rows []backlog.TagDefinition
            rows,err=backlog.TagSearch(root,strings.Join(positional[1:]," "))
            if err==nil {
                if jsonOutput { emitJSON(rows) } else { printTagDefinitions(rows) }
            }
        case "show":
            if len(positional)!=2 { fmt.Fprintln(os.Stderr,"tags show requires a tag"); return 2 }
            var report map[string]any
            report,err=backlog.TagShow(root,positional[1])
            if err==nil {
                if jsonOutput { emitJSON(report) } else if report["found"]!=true {
                    fmt.Println("(태그 없음)")
                } else if row,ok:=report["tag"].(backlog.TagDefinition); ok {
                    printTagDefinitions([]backlog.TagDefinition{row})
                } else { emitJSON(report) }
            }
        case "resolve":
            if len(positional)<2 { fmt.Fprintln(os.Stderr,"tags resolve requires a query"); return 2 }
            var report map[string]any
            report,err=backlog.TagResolve(root,strings.Join(positional[1:]," "))
            if err==nil {
                if jsonOutput { emitJSON(report) } else if report["found"]!=true {
                    fmt.Println("(해결 가능한 태그 없음)")
                } else if row,ok:=report["tag"].(backlog.TagDefinition); ok {
                    fmt.Println(row.Canonical)
                    if matched:=fmt.Sprint(report["matched"]); matched!="" && matched!=row.Canonical { fmt.Println("matched: "+matched) }
                } else { emitJSON(report) }
            }
        case "tasks","query":
            if len(positional)!=2 { fmt.Fprintln(os.Stderr,"tags tasks/query requires an expression (AND=comma, OR=pipe)"); return 2 }
            var rows []map[string]any
            rows,err=backlog.TagTasks(root,rootOption,positional[1])
            if err==nil {
                if jsonOutput { emitJSON(rows) } else if len(rows)==0 { fmt.Println("(태스크 없음)") } else {
                    for _,row:=range rows { fmt.Printf("%-8v %-7v %-28v %s\n",row["id"],row["state"],singleLine(row["title"]),joinAnyStrings(row["tags"])) }
                }
            }
        case "stats":
            if len(positional)!=1 { fmt.Fprintln(os.Stderr,"tags stats takes no arguments"); return 2 }
            var rows []backlog.TagStat
            rows,err=backlog.TagStatsReport(root,rootOption)
            if err==nil {
                if jsonOutput { emitJSON(rows) } else { printTagStats(rows) }
            }
        case "catalog":
            if len(positional)!=1 { fmt.Fprintln(os.Stderr,"tags catalog takes no arguments"); return 2 }
            var report map[string]any
            report,err=backlog.TagCatalogReport(root,rootOption)
            if err==nil {
                if jsonOutput { emitJSON(report) } else {
                    if rows,ok:=report["registry"].([]backlog.TagDefinition); ok { printTagDefinitions(rows) }
                    if stats,ok:=report["stats"].([]backlog.TagStat); ok { fmt.Println(); printTagStats(stats) }
                }
            }
        case "assign":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"tags assign <task-id> <tag>"); return 2 }
            var report map[string]any
            report,err=backlog.TaskTagAdd(root,rootOption,positional[1],positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",positional[1],joinAnyStrings(report["tags"])) } }
        case "remove":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"tags remove <task-id> <tag>"); return 2 }
            var report map[string]any
            report,err=backlog.TaskTagRemove(root,rootOption,positional[1],positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",positional[1],joinAnyStrings(report["tags"])) } }
        case "set":
            if len(positional)<2 { fmt.Fprintln(os.Stderr,"tags set <task-id> [tag...]"); return 2 }
            values:=expandTagArgs(positional[2:])
            var report map[string]any
            report,err=backlog.TaskTagSet(root,rootOption,positional[1],values)
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",positional[1],joinAnyStrings(report["tags"])) } }
        case "define":
            if len(positional)<2 || len(positional)>4 { fmt.Fprintln(os.Stderr,"tags define <namespace:name> [description] [alias1,alias2]"); return 2 }
            description:=""; aliases:=""
            if len(positional)>=3 { description=positional[2] }
            if len(positional)>=4 { aliases=positional[3] }
            var row backlog.TagDefinition
            row,err=backlog.TagDefine(root,positional[1],description,aliases)
            if err==nil {
                if jsonOutput { emitJSON(row) } else { printTagDefinitions([]backlog.TagDefinition{row}) }
            }
        case "rename":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"tags rename <old> <new>"); return 2 }
            var report map[string]any
            report,err=backlog.TagRename(root,rootOption,positional[1],positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%v -> %v · tasks updated: %v\n",report["from"],report["to"],report["tasks_updated"]) } }
        case "merge":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"tags merge <old> <target>"); return 2 }
            var report map[string]any
            report,err=backlog.TagMerge(root,rootOption,positional[1],positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%v -> %v · tasks updated: %v\n",report["from"],report["to"],report["tasks_updated"]) } }
        case "retire":
            if len(positional)<2 || len(positional)>3 { fmt.Fprintln(os.Stderr,"tags retire <tag> [replacement]"); return 2 }
            replacement:=""; if len(positional)==3 { replacement=positional[2] }
            var report map[string]any
            report,err=backlog.TagRetire(root,rootOption,positional[1],replacement)
            if err==nil {
                if jsonOutput { emitJSON(report) } else {
                    if from,ok:=report["from"]; ok { fmt.Printf("%v -> %v · tasks updated: %v\n",from,report["to"],report["tasks_updated"]) } else { fmt.Printf("%v · %v\n",report["tag"],report["status"]) }
                }
            }
        case "rebuild":
            if len(positional)!=1 { fmt.Fprintln(os.Stderr,"tags rebuild takes no arguments"); return 2 }
            var index backlog.TagIndex
            index,err=backlog.RebuildTagIndex(root,rootOption)
            if err==nil {
                if jsonOutput { emitJSON(index) } else { fmt.Printf("tag index rebuilt: %d tasks · %d tags\n",len(index.Tasks),len(index.Stats)) }
            }
        default:
            fmt.Fprintln(os.Stderr,"unknown tags action: "+action+" (use list, search, discover, show, resolve, tasks, query, stats, catalog, define, assign, remove, set, rename, merge, retire, rebuild)")
            return 2
        }
    case "task":
        if len(positional)<2 { fmt.Fprintln(os.Stderr,"task requires an action and task id"); return 2 }
        action:=strings.ToLower(positional[0])
        id:=positional[1]
        switch action {
        case "tags":
            if len(positional)!=2 { fmt.Fprintln(os.Stderr,"task tags <id>"); return 2 }
            var report map[string]any
            report,err=backlog.TaskTags(root,rootOption,id)
            if err==nil {
                if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",id,joinAnyStrings(report["tags"])) }
            }
        case "tag-add":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"task tag-add <id> <tag>"); return 2 }
            var report map[string]any
            report,err=backlog.TaskTagAdd(root,rootOption,id,positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",id,joinAnyStrings(report["tags"])) } }
        case "tag-remove":
            if len(positional)!=3 { fmt.Fprintln(os.Stderr,"task tag-remove <id> <tag>"); return 2 }
            var report map[string]any
            report,err=backlog.TaskTagRemove(root,rootOption,id,positional[2])
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",id,joinAnyStrings(report["tags"])) } }
        case "tag-set":
            values:=expandTagArgs(positional[2:])
            var report map[string]any
            report,err=backlog.TaskTagSet(root,rootOption,id,values)
            if err==nil { if jsonOutput { emitJSON(report) } else { fmt.Printf("%s: %s\n",id,joinAnyStrings(report["tags"])) } }
        default:
            fmt.Fprintln(os.Stderr,"unknown task action: "+action+" (use tags, tag-add, tag-remove, tag-set)")
            return 2
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
            _ = maintenance.RegisterWebProject(root)
            _,err=webui.StartService(webui.Config{Project:root,Root:rootOption,Host:"auto",Port:webui.DefaultPort,OpenBrowser:true,Version:version})
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
            if jsonOutput { emitJSON(report) } else {
                printStatus(report)
                printUpdateHint(maintenance.ReadCachedVersionInfo(version))
            }
        }
    case "web":
        if len(positional)>1 { fmt.Fprintln(os.Stderr, "web accepts at most one action: status, restart, stop, or logs"); return 2 }
        action:=""
        if len(positional)==1 { action=strings.ToLower(positional[0]) }
        _ = maintenance.RegisterWebProject(root)
        config:=webui.Config{Project:root,Root:rootOption,Host:host,Port:port,OpenBrowser:!noOpen,Version:version,InstanceID:webInstanceID,ControlToken:webControlToken}
        switch action {
        case "", "start":
            if foreground {
                if webInstanceID=="" {
                    if current:=webui.ServiceStatus(); current.Running {
                        printWebState("Task Mecca Web is already running",current)
                        break
                    }
                    var id,token string
                    id,token,err=webui.NewServiceIdentity()
                    if err!=nil { break }
                    config.InstanceID=id
                    config.ControlToken=token
                }
                err=webui.Run(config)
            } else {
                var state webui.ServiceState
                state,err=webui.StartService(config)
                if err==nil {
                    printWebState("Task Mecca Web is running",state)
                }
            }
        case "status":
            state:=webui.ServiceStatus()
            if !state.Running {
                fmt.Println("Task Mecca Web is not running")
                break
            }
            uptime:="-"
            if started,parseErr:=time.Parse(time.RFC3339,state.StartedAt); parseErr==nil { uptime=time.Since(started).Round(time.Second).String() }
            fmt.Printf("Task Mecca Web\nStatus    running\nPID       %d\nVersion   %s\nPort      %d\n",state.PID,state.Version,state.Port)
            if state.LocalURL!="" { fmt.Println("Local     "+state.LocalURL) }
            if state.TailscaleURL!="" {
                fmt.Println("Tailscale "+state.TailscaleURL)
            } else if state.TailscaleMode=="initializing" {
                fmt.Println("TLS       initializing · Local Web is already available")
            } else if state.TLSError!="" {
                fmt.Println("TLS       unavailable · "+state.TLSError)
            }
            fmt.Printf("Started   %s\nUptime    %s\n",state.StartedAt,uptime)
        case "restart":
            var state webui.ServiceState
            state,err=webui.RestartService(config)
            if err==nil { printWebState("Task Mecca Web restarted",state) }
        case "stop":
            var state webui.ServiceState
            state,err=webui.StopService()
            if err==nil {
                if state.PID==0 { fmt.Println("Task Mecca Web is not running") } else { fmt.Println("Task Mecca Web stopped") }
            }
        case "logs":
            err=webui.StreamLogs(os.Stdout,follow)
        default:
            fmt.Fprintln(os.Stderr,"unknown web action: "+action+" (use status, restart, stop, or logs)")
            return 2
        }
	case "lifecycle":
		if len(positional)==0 { fmt.Fprintln(os.Stderr,"lifecycle requires record or list"); return 2 }
		switch positional[0] {
		case "record":
			if len(positional)<6 || len(positional)>9 {
				fmt.Fprintln(os.Stderr,"lifecycle record <kind> <task-id> <event-id> <actor> <evidence-source> [evidence-ref] [assignment-id] [attempt-id]")
				return 2
			}
			event:=backlog.LifecycleTransition{Kind:positional[1],TaskID:positional[2],EventID:positional[3],Actor:positional[4],EvidenceSource:positional[5]}
			if len(positional)>6 { event.EvidenceRef=positional[6] }
			if len(positional)>7 { event.AssignmentID=positional[7] }
			if len(positional)>8 { event.AttemptID=positional[8] }
			var recorded backlog.LifecycleTransition
			recorded,err=backlog.RecordLifecycleTransition(root,event,time.Now())
			if err==nil { if jsonOutput { emitJSON(recorded) } else { fmt.Printf("%s %s %s %s\n",recorded.EventID,recorded.TaskID,recorded.Kind,recorded.OccurredAt) } }
		case "list":
			if len(positional)!=1 { fmt.Fprintln(os.Stderr,"lifecycle list takes no arguments"); return 2 }
			var scan backlog.LifecycleEventScan
			scan,err=backlog.ReadLifecycleTransitions(root)
			if err==nil { if jsonOutput { emitJSON(scan) } else { for _,event:=range scan.Events { fmt.Printf("%s %s %s %s\n",event.EventID,event.TaskID,event.Kind,event.OccurredAt) }; for _,finding:=range scan.Findings { fmt.Fprintf(os.Stderr,"%s: %s\n",finding.EventID,finding.Message) } } }
		default: fmt.Fprintln(os.Stderr,"unknown lifecycle action: "+positional[0]); return 2
		}
	case "runtime":
        if len(positional)==0 {
		fmt.Fprintln(os.Stderr,"runtime requires an action: observe, list, reconcile, assign, bind, bind-agent, or bind-assignment")
            return 2
        }
        action:=strings.ToLower(positional[0])
        var runtimeProject string
        runtimeProject,err=runtimeobs.ResolveProject(root)
        if err!=nil { break }
        switch action {
        case "observe":
            if len(positional)!=2 {
                fmt.Fprintln(os.Stderr,"runtime observe requires a provider: codex or claude")
                return 2
            }
            var event runtimeobs.ExecutionEvent
            event,err=runtimeobs.ObserveHook(runtimeProject,positional[1],os.Stdin,time.Now())
            if err==nil && jsonOutput { emitJSON(event) }
        case "list","reconcile":
            if len(positional)>2 {
                fmt.Fprintf(os.Stderr,"runtime %s accepts at most one provider: codex or claude\n",action)
                return 2
            }
            provider:=""
            if len(positional)==2 {
                provider=strings.ToLower(positional[1])
                if provider!="codex" && provider!="claude" {
                    fmt.Fprintln(os.Stderr,"runtime provider must be codex or claude")
                    return 2
                }
            }
            var ledger runtimeobs.Ledger
            if action=="reconcile" {
                ledger,err=runtimeobs.ReconcileLedger(runtimeProject,limit,time.Now())
            } else {
                ledger,err=runtimeobs.BuildLedger(runtimeProject,limit,time.Now())
            }
            if err==nil {
                ledger=runtimeobs.FilterLedger(ledger,provider)
                if jsonOutput { emitJSON(ledger) } else { printRuntimeLedger(ledger) }
            }
        case "enrich-name":
            if len(positional)!=2 { fmt.Fprintln(os.Stderr,"runtime enrich-name requires a Codex session_id"); return 2 }
            var event runtimeobs.ExecutionEvent
            var changed bool
            event,changed,err=runtimeobs.EnrichCodexSessionName(runtimeProject,positional[1],time.Now())
            if err==nil {
                if jsonOutput { emitJSON(map[string]any{"changed":changed,"event":event}) } else if changed { fmt.Println(event.SessionName) } else { fmt.Println("(Codex thread has no name/title)") }
            }
        case "bind":
            if len(positional)<4 || len(positional)>5 {
                fmt.Fprintln(os.Stderr,"runtime bind requires: <attempt-id> <task-id|-> <agent-path> [parent-attempt-id]")
                return 2
            }
            parent:=""
            if len(positional)==5 { parent=positional[4] }
            taskID:=positional[2]
            if taskID=="-" { taskID="" }
            var attempt runtimeobs.Attempt
            attempt,err=runtimeobs.BindAttempt(runtimeProject,positional[1],taskID,positional[3],"explicit",parent,nil,time.Now())
            if err==nil {
                if jsonOutput { emitJSON(attempt) } else {
                    fmt.Printf("%s %s %s -> %s %s\n",attempt.AttemptID,attempt.BindingState,attempt.Provider,attempt.TaskID,attempt.AgentPath)
                }
            }
		case "assign":
			if len(positional)<3 || len(positional)>4 { fmt.Fprintln(os.Stderr,"runtime assign requires: <task-id> <agent-path> [assignment-id]"); return 2 }
			assignmentID:=""
			if len(positional)==4 { assignmentID=positional[3] }
			var assignment runtimeobs.Assignment
			assignment,err=runtimeobs.RecordAssignment(runtimeProject,assignmentID,positional[1],positional[2],time.Now())
			if err==nil { if jsonOutput { emitJSON(assignment) } else { fmt.Printf("%s %s %s %s\n",assignment.AssignmentID,assignment.TaskID,assignment.AgentPath,assignment.AssignedAt) } }
		case "bind-assignment":
			if len(positional)<3 || len(positional)>4 { fmt.Fprintln(os.Stderr,"runtime bind-assignment requires: <assignment-id> <runtime-agent-id> [parent-attempt-id]"); return 2 }
			parent:=""
			if len(positional)==4 { parent=positional[3] }
			var attempt runtimeobs.Attempt
			attempt,err=runtimeobs.BindRuntimeAgentForAssignment(runtimeProject,positional[1],positional[2],parent,time.Now())
			if err==nil { if jsonOutput { emitJSON(attempt) } else { fmt.Printf("%s %s %s -> %s %s\n",attempt.AttemptID,attempt.BindingState,attempt.RuntimeAgentID,attempt.TaskID,attempt.AgentPath) } }
		case "bind-agent":
            if len(positional)<4 || len(positional)>5 {
                fmt.Fprintln(os.Stderr,"runtime bind-agent requires: <runtime-agent-id> <task-id|-> <agent-path> [parent-attempt-id]")
                return 2
            }
            parent:=""
            if len(positional)==5 { parent=positional[4] }
            taskID:=positional[2]
            if taskID=="-" { taskID="" }
            var attempt runtimeobs.Attempt
            attempt,err=runtimeobs.BindRuntimeAgent(runtimeProject,positional[1],taskID,positional[3],parent,time.Now())
            if err==nil {
                if jsonOutput { emitJSON(attempt) } else {
                    fmt.Printf("%s %s %s -> %s %s\n",attempt.AttemptID,attempt.BindingState,attempt.RuntimeAgentID,attempt.TaskID,attempt.AgentPath)
                }
            }
        case "hooks":
            if len(positional)<2 || len(positional)>3 {
                fmt.Fprintln(os.Stderr,"runtime hooks requires: status|enable|disable <codex|claude|all>")
                return 2
            }
            hooksAction:=strings.ToLower(positional[1])
            provider:="all"
            if len(positional)==3 { provider=strings.ToLower(positional[2]) }
            if provider!="codex" && provider!="claude" && provider!="all" {
                fmt.Fprintln(os.Stderr,"runtime hooks provider must be codex, claude, or all")
                return 2
            }
            providers:=[]string{"codex","claude"}
            if provider!="all" { providers=[]string{provider} }
            setups:=[]runtimeobs.HookSetup{}
            for _,name:=range providers {
                var setup runtimeobs.HookSetup
                if hooksAction=="enable" {
                    setup,err=runtimeobs.EnsureHooks(runtimeProject,name)
                } else if hooksAction=="disable" {
                    setup,err=runtimeobs.DisableHooks(runtimeProject,name)
                } else if hooksAction=="status" {
                    setup,err=runtimeobs.HookStatus(runtimeProject,name)
                } else {
                    fmt.Fprintln(os.Stderr,"runtime hooks action must be status, enable, or disable")
                    return 2
                }
                if err!=nil { break }
                setups=append(setups,setup)
            }
            if err==nil {
                if jsonOutput { emitJSON(setups) } else {
                    for _,setup:=range setups {
                        label:="not configured"
                        if setup.Installed { label="configured" }
                        changed:=""
                        if setup.Changed { changed=" · updated" }
                        fmt.Printf("%s  %s%s  %s\n",setup.Provider,label,changed,setup.Path)
                    }
                }
            }
        default:
            fmt.Fprintln(os.Stderr,"unknown runtime action: "+action+" (use observe, list, reconcile, bind, bind-agent, or hooks)")
            return 2
        }
    case "runtime-spike":
        if len(positional)==0 {
            fmt.Fprintln(os.Stderr,"runtime-spike requires an action: observe or report")
            return 2
        }
        action:=strings.ToLower(positional[0])
        switch action {
        case "observe":
            if len(positional)!=2 {
                fmt.Fprintln(os.Stderr,"runtime-spike observe requires a provider: codex or claude")
                return 2
            }
            var runtimeProject string
            runtimeProject,err=runtimeobs.ResolveProject(root)
            if err!=nil { break }
            var event runtimeobs.SpikeEvent
            event,err=runtimeobs.Observe(runtimeProject,positional[1],os.Stdin,time.Now())
            if err==nil && jsonOutput { emitJSON(event) }
        case "report":
            if len(positional)>2 {
                fmt.Fprintln(os.Stderr,"runtime-spike report accepts at most one provider: codex or claude")
                return 2
            }
            var runtimeProject string
            runtimeProject,err=runtimeobs.ResolveProject(root)
            if err!=nil { break }
            var report runtimeobs.SpikeReport
            report,err=runtimeobs.Report(runtimeProject,limit)
            if err==nil && len(positional)==2 {
                provider:=strings.ToLower(positional[1])
                if provider!="codex" && provider!="claude" {
                    fmt.Fprintln(os.Stderr,"runtime-spike report provider must be codex or claude")
                    return 2
                }
                report=runtimeobs.FilterReport(report,provider)
            }
            if err==nil {
                if jsonOutput { emitJSON(report) } else { printRuntimeSpikeReport(report) }
            }
        default:
            fmt.Fprintln(os.Stderr,"unknown runtime-spike action: "+action+" (use observe or report)")
            return 2
        }
    case "monitor":
        if len(positional) != 0 { fmt.Fprintln(os.Stderr, "monitor takes no positional arguments"); return 2 }
        if !once && !jsonOutput {
            _ = maintenance.RegisterWebProject(root)
            _,err=webui.StartService(webui.Config{Project:root,Root:rootOption,Host:host,Port:port,OpenBrowser:!noOpen,Version:version})
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

func printTagDefinitions(rows []backlog.TagDefinition) {
    if len(rows)==0 { fmt.Println("(태그 없음)"); return }
    fmt.Printf("%-28s %-10s %-9s %s\n","TAG","NAMESPACE","STATUS","DESCRIPTION")
    for _,row:=range rows {
        description:=row.Description
        if row.ReplacedBy!="" { description=strings.TrimSpace(description+" -> "+row.ReplacedBy) }
        fmt.Printf("%-28s %-10s %-9s %s\n",row.Canonical,row.Namespace,row.Status,description)
        if len(row.Aliases)>0 { fmt.Printf("  aliases: %s\n",strings.Join(row.Aliases,", ")) }
    }
}

func printTagStats(rows []backlog.TagStat) {
    if len(rows)==0 { fmt.Println("(태그 사용 없음)"); return }
    fmt.Printf("%-28s %6s %7s %6s %6s\n","TAG","TOTAL","ACTIVE","HOLD","DONE")
    for _,row:=range rows {
        fmt.Printf("%-28s %6d %7d %6d %6d\n",row.Tag,row.Total,row.Active,row.Hold,row.Done)
    }
}

func expandTagArgs(values []string) []string {
    out:=[]string{}
    for _,value:=range values {
        for _,part:=range strings.Split(value,",") {
            if trimmed:=strings.TrimSpace(part); trimmed!="" { out=append(out,trimmed) }
        }
    }
    return out
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

func printWebState(prefix string,state webui.ServiceState) {
    if prefix!="" { fmt.Println(prefix) }
    if state.LocalURL!="" { fmt.Println("Local     "+state.LocalURL) }
    if state.TailscaleURL!="" {
        fmt.Println("Tailscale "+state.TailscaleURL)
    } else if state.TailscaleMode=="initializing" {
        fmt.Println("Tailscale HTTPS initializing in background")
    } else if state.TLSError!="" {
        fmt.Println("Tailscale HTTPS unavailable: "+state.TLSError)
    }
    fmt.Printf("PID       %d\n",state.PID)
}

func printRuntimeLedger(ledger runtimeobs.Ledger) {
    fmt.Printf("Task Mecca · Execution Lifecycle\nattempts=%d findings=%d\n",len(ledger.Attempts),len(ledger.Findings))
    for _,attempt:=range ledger.Attempts {
        binding:=string(attempt.BindingState)
        target:="-"
        if attempt.TaskID!="" || attempt.AgentPath!="" { target=strings.TrimSpace(attempt.TaskID+" "+attempt.AgentPath) }
        elapsed:=time.Duration(attempt.ElapsedMillis)*time.Millisecond
        last:=attempt.LastActivityAt
        if last=="" { last=attempt.LastObservedAt }
        fmt.Printf("%s %-7s %-16s %-12s binding=%-9s target=%s elapsed=%s last=%s\n",
            attempt.AttemptID,attempt.Provider,attempt.RuntimeAgentID,attempt.CurrentState,binding,target,elapsed.Round(time.Second),last)
        if len(attempt.RecentTransitions)>0 {
            for _,transition:=range attempt.RecentTransitions {
                label:=transition.Kind
                if transition.State!="" { label=string(transition.State) }
                fmt.Printf("  %s  %s  %s/%s\n",transition.At,label,transition.EvidenceSource,transition.ObservationQuality)
            }
        }
    }
    if len(ledger.Findings)>0 {
        fmt.Println("\nFindings")
        for _,finding:=range ledger.Findings {
            target:=""
            if finding.AttemptID!="" { target=" ["+finding.AttemptID+"]" }
            fmt.Printf("- %s %s%s: %s\n",strings.ToUpper(finding.Severity),finding.Code,target,finding.Message)
        }
    }
}

func printRuntimeSpikeReport(report runtimeobs.SpikeReport) {
    fmt.Println("Task Mecca · Runtime Observability Spike")
    fmt.Println("journal="+report.JournalPath)
    if !report.Exists {
        fmt.Println("events=0 (아직 수집된 hook 이벤트 없음)")
        return
    }
    fmt.Printf("events=%d agents=%d findings=%d\n",report.Events,len(report.Agents),len(report.Findings))
    for _,provider:=range report.Providers {
        fmt.Printf("%s: events=%d agents=%d start=%d stop=%d tool=%d attributed=%d unattributed=%d\n",
            provider.Provider,provider.Events,provider.Agents,provider.Starts,provider.Stops,
            provider.ToolActivityEvents,provider.AttributedToolActivity,provider.UnattributedToolActivity)
    }
    if len(report.Agents)>0 {
        fmt.Println("\nAgents")
        for _,agent:=range report.Agents {
            status:="observed"
            if agent.SawStart && agent.SawStop { status="start→stop" } else if agent.SawStart { status="start→?" }
            fmt.Printf("%s %-24s %-11s events=%d activity=%d first=%s last=%s\n",
                agent.Provider,agent.AgentID,status,agent.EventCount,agent.ActivityCount,agent.FirstObserved,agent.LastObserved)
        }
    }
    if len(report.Findings)>0 {
        fmt.Println("\nFindings")
        for _,finding:=range report.Findings {
            target:=finding.Provider
            if finding.AgentID!="" { target+=":"+finding.AgentID }
            if target!="" { target=" ["+target+"]" }
            fmt.Printf("- %s %s%s: %s\n",strings.ToUpper(finding.Severity),finding.Code,target,finding.Message)
        }
    }
}

func printUpdateHint(info maintenance.VersionInfo) {
    if !info.UpdateAvailable || info.Latest=="" { return }
    fmt.Printf("\nUpdate available (%s): %s → %s\n",info.Channel,info.Current,info.Latest)
    fmt.Println("Run: task-mecca upgrade")
}
