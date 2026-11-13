// Read-only one-shot projection probe. Never starts a server or reconciles data.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func hashes(project string) map[string]string {
	result := map[string]string{}
	// Audit all canonical files read by this probe, not mutable preflight caches.
	for _, rel := range []string{"_task_mecca/data/backlog", "_task_mecca/.runtime/executions", "_task_mecca/data/lifecycle", "_task_mecca/.runtime/handoffs", "_task_mecca/data/activity"} {
		root := filepath.Join(project, rel)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			key, _ := filepath.Rel(project, p)
			result[key] = hex.EncodeToString(sum[:])
			return nil
		})
		must(err)
	}
	return result
}
func main() {
	project := flag.String("project", "", "read-only canonical project")
	task := flag.String("task", "A-68", "exact task")
	flag.Parse()
	if *project == "" {
		panic("--project required")
	}
	before := hashes(*project)
	events, err := (activity.Ledger{Project: *project}).List(*task)
	must(err)
	scan, err := backlog.ReadLifecycleTransitions(*project)
	must(err)
	facts := []activity.LifecycleFact{}
	for _, e := range scan.Events {
		if e.TaskID != *task {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, e.OccurredAt)
		must(err)
		facts = append(facts, activity.LifecycleFact{EventID: e.EventID, TaskID: e.TaskID, Kind: e.Kind, OccurredAt: at, EvidenceSource: e.EvidenceSource, Actor: e.Actor, AssignmentID: e.AssignmentID, AttemptID: e.AttemptID})
	}
	handoffs, err := handoffjournal.BuildLedger(*project, time.Now())
	must(err)
	observations, err := activity.ReadWorkflowExecutions(*project, *task, time.Now())
	must(err)
	summary := activity.Summarize(*task, events)
	// Use the exact shipped domain projection; no control-plane refresh or writes.
	workflow := activity.SummarizeWorkflow(activity.WorkflowInput{TaskID: *task, Events: events, Summary: summary, Facts: facts, Handoffs: handoffs.Handoffs, ExecutionObservations: observations, Now: time.Now().UTC()})
	after := hashes(*project)
	same := reflect.DeepEqual(before, after)
	keys := []string{}
	for key := range before {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	digest := sha256.New()
	for _, key := range keys {
		fmt.Fprintf(digest, "%s\x00%s\n", key, before[key])
	}
	// Minimal exact-task metadata only; no raw operational ledger or transcripts.
	receipt := []map[string]any{}
	for _, a := range observations {
		if a.AttemptID == "run-ad478c65e15a5d8a" {
			receipt = append(receipt, map[string]any{"attempt_id": a.AttemptID, "runtime_agent_id": a.RuntimeAgentID, "task_id": a.TaskID, "agent_path": a.AgentPath, "binding_state": a.BindingState, "assignment_id": a.BindingEvidence["assignment_id"], "state": a.CurrentState, "terminal": a.Terminal, "ended_at": a.EndedAt, "source": a.StateEvidenceSource, "receipt_present": a.NativeReceipt != nil, "observed_execution_verified": runtimeobs.HasObservedExecution(a), "receipt": a.NativeReceipt, "binding_evidence": a.BindingEvidence, "turn_id": a.TurnID, "last_observed_at": a.LastObservedAt})
		}
	}
	must(json.NewEncoder(os.Stdout).Encode(map[string]any{"read_only": true, "read_no_write_equal": same, "files_hashed": len(before), "source_digest": hex.EncodeToString(digest.Sum(nil)), "runtime": receipt, "summary": summary, "workflow": workflow}))
	if !same {
		os.Exit(1)
	}
}
