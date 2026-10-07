package webui

import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/notify"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

const operationScanInterval = 15 * time.Second
const operationGapAfter = 2 * operationScanInterval

// An operation incident is an observation, never a backlog lifecycle decision.
type operationIncident struct {
	ID             string                         `json:"id"`
	Key            string                         `json:"key"`
	Project        string                         `json:"project"`
	AttemptID      string                         `json:"attempt_id,omitempty"`
	TaskID         string                         `json:"task_id,omitempty"`
	AgentPath      string                         `json:"agent_path,omitempty"`
	Kind           string                         `json:"kind"`
	Quality        string                         `json:"quality"`
	Evidence       string                         `json:"evidence"`
	LastObservedAt string                         `json:"last_observed_at,omitempty"`
	DetectedAt     string                         `json:"detected_at"`
	EndedAt        string                         `json:"ended_at,omitempty"`
	RecoveredAt    string                         `json:"recovered_at,omitempty"`
	Action         string                         `json:"action"`
	Resolution     *operationCompletionResolution `json:"resolution,omitempty"`
}

type operationJournal struct {
	Version      int                 `json:"version"`
	Project      string              `json:"project"`
	LastScanAt   string              `json:"last_scan_at,omitempty"`
	NextSequence int                 `json:"next_sequence"`
	Incidents    []operationIncident `json:"incidents"`
}

var operationMu sync.Mutex
var operationDeliver = notify.Deliver
var operationHookVerifier = hookVerified

func operationJournalPath(project string) string {
	return filepath.Join(project, "_task_mecca", ".runtime", "operations", "session-monitor.json")
}

func readOperationJournal(project string) (operationJournal, error) {
	out := operationJournal{Version: 1, Project: project, Incidents: []operationIncident{}}
	data, err := os.ReadFile(operationJournalPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Incidents == nil {
		out.Incidents = []operationIncident{}
	}
	return out, nil
}

func writeOperationJournal(project string, journal operationJournal) error {
	path := operationJournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-monitor-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

type operationSignal struct {
	key      string
	incident operationIncident
	alert    bool
}

func hookVerified(project, provider string) bool {
	if provider != "codex" && provider != "claude" {
		return false
	}
	setups := []runtimeobs.HookSetup{}
	if setup, err := runtimeobs.HookStatus(project, provider); err == nil && setup.Installed {
		setups = append(setups, setup)
	}
	if setup, err := runtimeobs.GlobalHookStatus(provider); err == nil && setup.Installed {
		setups = append(setups, setup)
	}
	for _, setup := range setups {
		events, _, err := runtimeobs.HookEvidenceSince(project, provider, setup.ConfiguredAt)
		if err == nil && (events["start"] || events["stop"] || events["activity"]) {
			return true
		}
	}
	return false
}

func classifyOperation(project string, attempt runtimeobs.Attempt, now time.Time, verified bool) (operationSignal, bool) {
	if attempt.AttemptID == "" {
		return operationSignal{}, false
	}
	base := operationIncident{Project: project, AttemptID: attempt.AttemptID, TaskID: attempt.TaskID,
		AgentPath: attempt.AgentPath, LastObservedAt: attempt.LastObservedAt, EndedAt: attempt.EndedAt}
	key := "attempt:" + attempt.AttemptID
	confirmed := attempt.StateEvidenceSource == runtimeobs.EvidenceHook &&
		(attempt.StateObservationQuality == runtimeobs.QualityObserved || attempt.StateObservationQuality == runtimeobs.QualityAuthoritative)
	switch attempt.CurrentState {
	case runtimeobs.StateErrored, runtimeobs.StateInterrupted, runtimeobs.StateShutdown:
		if confirmed {
			base.Kind, base.Quality, base.Evidence = string(attempt.CurrentState), "confirmed", "hook terminal state"
			base.Action = "Controller: 종료 근거와 작업 상태를 확인하고 재배정 또는 복구를 판단하세요."
			return operationSignal{key: key, incident: base, alert: true}, true
		}
		base.Kind, base.Quality, base.Evidence = "runtime_unknown", "verification_required", "terminal state lacks observed hook evidence"
	case runtimeobs.StateCompleted:
		if confirmed {
			base.Kind, base.Quality, base.Evidence = "normal_exit", "confirmed", "hook completed state"
			base.Action = "종료가 확인되었습니다. 백로그 완료 여부는 Controller가 별도로 판단하세요."
			return operationSignal{key: key, incident: base}, true
		}
		return operationSignal{}, false
	default:
		last, err := time.Parse(time.RFC3339Nano, attempt.LastObservedAt)
		if verified && err == nil && now.Sub(last) >= runtimeobs.StaleWarnDuration() && !attempt.Terminal {
			base.Kind, base.Quality, base.Evidence = "no_signal", "verification_required", "verified hook has no recent signal"
			base.Action = "Controller: 세션과 Hook 수신 경로를 확인하세요. 무신호만으로 Worker 종료를 확정하지 마세요."
			return operationSignal{key: key, incident: base, alert: true}, true
		}
		if attempt.CurrentState != runtimeobs.StateRuntimeUnknown && verified {
			return operationSignal{}, false
		}
		base.Kind, base.Quality, base.Evidence = "runtime_unknown", "verification_required", "runtime identity or hook execution unverified"
	}
	if attempt.TaskID == "" && attempt.AgentPath == "" {
		return operationSignal{}, false
	}
	base.Action = "Controller: runtime identity와 Hook 승인·연결 근거를 확인하세요. Worker 종료로 단정하지 마세요."
	return operationSignal{key: key, incident: base, alert: true}, true
}

func operationID(project, key string, sequence int) string {
	sum := sha256.Sum256([]byte(project + "\x00" + key + "\x00" + fmt.Sprint(sequence)))
	return "ops-" + hex.EncodeToString(sum[:8])
}

func scanOperationProject(project string, now time.Time) (operationJournal, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	release, err := lockOperationJournal(project)
	if err != nil {
		return operationJournal{}, err
	}
	defer release()
	journal, err := readOperationJournal(project)
	if err != nil {
		return journal, err
	}
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		return journal, err
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	completions := operationCompletionEvidence(project, ledger, now)
	signals := map[string]operationSignal{}
	verified := map[string]bool{}
	for _, attempt := range ledger.Attempts {
		if _, ok := verified[attempt.Provider]; !ok {
			verified[attempt.Provider] = operationHookVerifier(project, attempt.Provider)
		}
		if signal, ok := classifyOperation(project, attempt, now, verified[attempt.Provider]); ok {
			if operationCompletedUnknown(signal.incident, completions) != nil {
				continue
			}
			signals[signal.key] = signal
		}
	}
	// A later observed attempt for the same task is recovery evidence for an
	// earlier terminal episode. Keep both attempts' history, but close the old
	// warning instead of leaving it active forever.
	for _, prior := range ledger.Attempts {
		if prior.TaskID == "" || !prior.Terminal || prior.EndedAt == "" {
			continue
		}
		ended, err := time.Parse(time.RFC3339Nano, prior.EndedAt)
		if err != nil {
			continue
		}
		for _, later := range ledger.Attempts {
			if later.AttemptID == prior.AttemptID || !strings.EqualFold(later.TaskID, prior.TaskID) {
				continue
			}
			started, parseErr := time.Parse(time.RFC3339Nano, later.StartedAt)
			if parseErr == nil && started.After(ended) {
				delete(signals, "attempt:"+prior.AttemptID)
				break
			}
		}
	}
	// A doing assignment without a linked attempt is an identity gap, not
	// evidence that the assigned Worker died.
	if ctx, ctxErr := webContext(project, ""); ctxErr == nil {
		if selected, _, selectErr := resolveBacklog(project, ctx, url.Values{}); selectErr == nil && selected != "" {
			if workload, workErr := backlog.WorkloadSnapshot(project, selected); workErr == nil {
				if items, ok := workload["all_items"].(map[string]map[string]any); ok {
					for id, item := range items {
						agent := strings.TrimSpace(fmt.Sprint(item["agent"]))
						if agent == "" || agent == "-" || agent == "<nil>" {
							continue
						}
						matched := false
						for _, attempt := range ledger.Attempts {
							if strings.EqualFold(attempt.TaskID, id) {
								matched = true
								break
							}
						}
						if matched {
							continue
						}
						key := "task:" + id
						signals[key] = operationSignal{key: key, alert: true, incident: operationIncident{
							Project: project, TaskID: id, AgentPath: agent, Kind: "runtime_unknown", Quality: "verification_required",
							Evidence: "doing assignment has no linked runtime attempt",
							Action:   "Controller: 배정 기록과 runtime identity를 대조하세요. Worker 종료를 단정하거나 백로그 상태를 자동 변경하지 마세요.",
						}}
					}
				}
			}
		}
	}
	if prior, err := time.Parse(time.RFC3339Nano, journal.LastScanAt); err == nil && now.Sub(prior) > operationGapAfter {
		gap := operationIncident{Project: project, Kind: "monitor_gap", Quality: "monitoring_gap",
			Evidence: "web server scan gap; exact session stop time is unknown", LastObservedAt: journal.LastScanAt,
			Action: "Controller: 감시 공백 동안의 세션 상태를 별도 근거로 확인하세요.", RecoveredAt: nowText}
		signals["gap:"+journal.LastScanAt] = operationSignal{key: "gap:" + journal.LastScanAt, incident: gap, alert: true}
	}
	active := map[string]int{}
	for i := range journal.Incidents {
		item := &journal.Incidents[i]
		if item.RecoveredAt != "" {
			continue
		}
		if resolution := operationCompletedUnknown(*item, completions); resolution != nil {
			item.RecoveredAt = nowText
			item.Resolution = resolution
			delete(signals, item.Key)
			continue
		}
		key := item.Key
		if key == "" && item.AttemptID != "" {
			key = "attempt:" + item.AttemptID
		}
		if key == "" {
			continue
		}
		active[key] = i
		if signal, ok := signals[key]; ok && signal.incident.Kind == item.Kind {
			item.LastObservedAt = signal.incident.LastObservedAt
			delete(signals, key)
		} else if _, present := signals[key]; present || item.Kind != "runtime_unknown" {
			item.RecoveredAt = nowText
			delete(active, key)
		}
	}
	keys := make([]string, 0, len(signals))
	for key := range signals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		signal := signals[key]
		journal.NextSequence++
		signal.incident.ID = operationID(project, key, journal.NextSequence)
		signal.incident.Key = key
		signal.incident.DetectedAt = nowText
		journal.Incidents = append(journal.Incidents, signal.incident)
	}
	journal.LastScanAt = nowText
	if err := writeOperationJournal(project, journal); err != nil {
		return journal, err
	}
	return journal, nil
}

func deliverOperationIncidents(project string, journal operationJournal) {
	events := []notify.Event{}
	for _, incident := range journal.Incidents {
		if incident.Kind == "normal_exit" {
			continue
		}
		if incident.RecoveredAt != "" && incident.Kind != "monitor_gap" {
			continue
		}
		kind := "runtime_unknown"
		if incident.Quality == "confirmed" {
			kind = "interrupted"
		}
		if incident.Kind == "no_signal" {
			kind = "stalled"
		}
		events = append(events, notify.Event{ID: incident.ID, TaskID: incident.TaskID, Kind: kind,
			Title: strings.ReplaceAll(incident.Kind, "_", " "), Message: incident.Evidence,
			ResumeCondition: incident.Action, At: incident.DetectedAt})
	}
	_ = operationDeliver(project, events)
}

func operationProjects(primary string) []string {
	seen := map[string]bool{}
	projects := []string{}
	primaryKnown := false
	managed, err := maintenance.ManagedProjects()
	if err == nil {
		for _, item := range managed {
			if filepath.Clean(item.Path) == filepath.Clean(primary) {
				primaryKnown = true
			}
			// A registry entry may be retained for safe removal history or be
			// explicitly paused.  Neither is an active monitoring target, and
			// scanning it would make a removed/stopped project look live again.
			if !item.Monitoring || item.Presence != "present" || item.Path == "" || seen[item.Path] {
				continue
			}
			seen[item.Path] = true
			projects = append(projects, item.Path)
		}
		// A removed project is no longer managed, but its durable removal
		// history still makes it a known non-target.  Do not resurrect it just
		// because an old Web process still has it as its primary path.
		if !primaryKnown {
			if history, historyErr := maintenance.RemovalHistory(); historyErr == nil {
				for _, item := range history {
					if filepath.Clean(item.Path) == filepath.Clean(primary) {
						primaryKnown = true
						break
					}
				}
			}
		}
	}
	// Preserve the old fallback only for an unregistered primary project.
	// Registered-but-paused/missing and removed projects intentionally stay
	// outside the monitor's collection.
	if primary != "" && !primaryKnown && !seen[primary] && maintenance.ProjectMonitoringAllowed(primary) {
		projects = append(projects, primary)
	}
	sort.Strings(projects)
	return projects
}

func scanOperationProjects(primary string, now time.Time) {
	for _, project := range operationProjects(primary) {
		maintenance.WithProjectMonitoring(project, func() {
			journal, err := scanOperationProject(project, now)
			if err == nil {
				deliverOperationIncidents(project, journal)
			}
		})
	}
}

func runOperationMonitor(ctx stdcontext.Context, primary string) {
	scanOperationProjects(primary, time.Now().UTC())
	ticker := time.NewTicker(operationScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			scanOperationProjects(primary, now.UTC())
		}
	}
}

func operationSnapshot(primary string) map[string]any {
	projects := []map[string]any{}
	active := []operationIncident{}
	recent := []operationIncident{}
	resolved := []operationIncident{}
	for _, project := range operationProjects(primary) {
		journal, err := readOperationJournal(project)
		if err != nil {
			projects = append(projects, map[string]any{"path": project, "error": err.Error()})
			continue
		}
		projects = append(projects, map[string]any{"path": project, "last_scan_at": journal.LastScanAt})
		for _, incident := range journal.Incidents {
			if incident.Resolution != nil && incident.RecoveredAt != "" {
				resolved = append(resolved, incident)
			}
			if incident.RecoveredAt == "" && incident.Kind != "normal_exit" {
				active = append(active, incident)
			}
			recent = append(recent, incident)
		}
	}
	sort.Slice(recent, func(i, j int) bool { return recent[i].DetectedAt > recent[j].DetectedAt })
	if len(recent) > 50 {
		recent = recent[:50]
	}
	sort.Slice(resolved, func(i, j int) bool { return resolved[i].RecoveredAt > resolved[j].RecoveredAt })
	if len(resolved) > 20 {
		resolved = resolved[:20]
	}
	return map[string]any{"projects": projects, "active": active, "recent": recent, "resolved_observations": resolved, "telegram_transport_disabled": notify.TelegramTransportDisabled()}
}
