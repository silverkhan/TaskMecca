package webui

import (
	"encoding/json"
	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/handoff"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReviewUsesRealAttentionTelegramConsumer(t *testing.T) {
	for _, mode := range []string{"pending", "review", "finalizing", "recovery", "user", "done"} {
		t.Run(mode, func(t *testing.T) {
			project := t.TempDir()
			registerWebFixture(t, project)
			now := time.Now().UTC().Add(-time.Second)
			folder := filepath.Join(project, "_task_mecca", "data", "backlog")
			os.MkdirAll(folder, 0755)
			body := "# A-33 검토 테스트\n- Agent: /root/controller/worker\n## 작업 정의\n- 목표: 실제 소비자 검증\n"
			file := filepath.Join(folder, "000033.A-33.delivery.doing.md")
			os.WriteFile(file, []byte(body), 0644)
			events := []runtimeobs.ExecutionEvent{{EventKind: "state", AttemptID: "w1", Provider: "codex", RuntimeAgentID: "wn", SessionID: "s1", ObservedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), State: runtimeobs.StateRunning, EvidenceSource: runtimeobs.EvidenceHook, ObservationQuality: runtimeobs.QualityObserved}, {EventKind: "binding", AttemptID: "w1", TaskID: "A-33", AgentPath: "/root/controller/worker", ObservedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), EvidenceSource: runtimeobs.EvidenceManualBinding, ObservationQuality: runtimeobs.QualityAuthoritative}}
			for _, e := range events {
				if err := runtimeobs.AppendExecutionEvent(project, e); err != nil {
					t.Fatal(err)
				}
			}
			configDir := filepath.Join(project, "_task_mecca", ".runtime", "notifications")
			os.MkdirAll(configDir, 0755)
			cfg, _ := json.Marshal(map[string]any{"token": "local-test", "chat_id": 7, "enabled": true, "activated_at": now.Add(-time.Hour).Format(time.RFC3339Nano), "kinds": map[string]bool{"intervention": true, "completed": true, "finalize": true}})
			os.WriteFile(filepath.Join(configDir, "telegram.json"), cfg, 0600)
			var sent atomic.Int32
			previous := http.DefaultTransport
			http.DefaultTransport = lifecycleFakeTelegramTransport{fallback: previous, telegram: func(r *http.Request) (*http.Response, error) {
				sent.Add(1)
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)), Request: r}, nil
			}}
			defer func() { http.DefaultTransport = previous }()
			feed := &attentionFeed{project: project, root: folder, subscribers: map[chan []byte]struct{}{}}
			feed.refreshActive()
			baseline := sent.Load()
			end := events[0]
			end.ObservedAt = now.Format(time.RFC3339Nano)
			end.State = runtimeobs.StateCompleted
			end.Terminal = true
			runtimeobs.AppendExecutionEvent(project, end)
			if mode != "pending" && mode != "done" {
				controller := events[0]
				controller.AttemptID = "c1"
				controller.RuntimeAgentID = "cn"
				runtimeobs.AppendExecutionEvent(project, controller)
				bind := events[1]
				bind.AttemptID = "c1"
				bind.TaskID = ""
				bind.AgentPath = "/root/controller"
				runtimeobs.AppendExecutionEvent(project, bind)
				if mode == "recovery" || mode == "user" {
					controller.ObservedAt = now.Format(time.RFC3339Nano)
					if mode == "user" {
						controller.State = runtimeobs.StateWaitingUser
					} else {
						controller.State = runtimeobs.StateCompleted
						controller.Terminal = true
					}
					runtimeobs.AppendExecutionEvent(project, controller)
				}
				contract, _ := handoff.ContractFingerprint(body)
				records := []handoff.Record{{HandoffID: "h1", RecordKind: "prepared", EventType: handoff.EventWorkerDone, TaskID: "A-33", ContractSHA256: contract, SourceAgentPath: "/root/controller/worker", SourceAttemptID: "w1", TargetAgentPath: "/root/controller", TargetAttemptID: "c1", TargetRuntimeAgentID: "cn", TargetSessionID: "s1", Provider: "codex", ObservedAt: now.Add(-500 * time.Millisecond).Format(time.RFC3339Nano)}, {HandoffID: "h1", RecordKind: "claimed", ClaimedBy: "/root/controller", ClaimedAttemptID: "c1", ObservedAt: now.Format(time.RFC3339Nano)}}
				if mode == "finalizing" {
					records = append(records, handoff.Record{HandoffID: "h1", RecordKind: "step_succeeded", Step: "acceptance", Result: "ok", ObservedAt: now.Format(time.RFC3339Nano)})
				}
				journal := handoff.JournalPath(project)
				os.MkdirAll(filepath.Dir(journal), 0755)
				f, _ := os.Create(journal)
				for _, r := range records {
					json.NewEncoder(f).Encode(r)
				}
				f.Close()
			}
			if mode == "done" {
				os.Rename(file, strings.Replace(file, ".doing.md", ".done.md", 1))
			}
			feed.refreshActive()
			if mode != "user" && mode != "done" {
				deliverOperationIncidents(project, operationJournal{Stages: []operationStage{{TaskID: "A-33", AttemptID: "w1", ControllerReview: true}}, Incidents: []operationIncident{{ID: "controller-work", TaskID: "A-33", AttemptID: "w1", Kind: "runtime_unknown", DetectedAt: now.Format(time.RFC3339Nano)}}})
			}
			want := int32(0)
			if mode == "user" || mode == "done" {
				want = 1
			}
			if delta := sent.Load() - baseline; delta != want {
				t.Fatalf("%s actual Telegram sends=%d want=%d", mode, delta, want)
			}
			snapshot, err := backlog.AttentionSnapshot(project, folder, true)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "user" && len(snapshot["attention"].([]map[string]any)) != 0 {
				t.Fatalf("Controller work leaked: %v", snapshot)
			}
			feed.refreshActive()
			if sent.Load()-baseline != want {
				t.Fatal("repeated refresh duplicated delivery")
			}
		})
	}
}
