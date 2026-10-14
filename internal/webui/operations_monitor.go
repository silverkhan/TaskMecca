package webui

import (
	stdcontext "context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/projectguard"
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
	ID               string                         `json:"id"`
	Key              string                         `json:"key"`
	Project          string                         `json:"project"`
	AttemptID        string                         `json:"attempt_id,omitempty"`
	AssignmentID     string                         `json:"assignment_id,omitempty"`
	TaskID           string                         `json:"task_id,omitempty"`
	AgentPath        string                         `json:"agent_path,omitempty"`
	Kind             string                         `json:"kind"`
	Quality          string                         `json:"quality"`
	Severity         string                         `json:"severity,omitempty"`
	Evidence         string                         `json:"evidence"`
	LastObservedAt   string                         `json:"last_observed_at,omitempty"`
	DetectedAt       string                         `json:"detected_at"`
	EndedAt          string                         `json:"ended_at,omitempty"`
	RecoveredAt      string                         `json:"recovered_at,omitempty"`
	Action           string                         `json:"action"`
	Resolution       *operationCompletionResolution `json:"resolution,omitempty"`
	RecoveryEvidence string                         `json:"recovery_evidence,omitempty"`
	PolicyResolution *operationPolicyResolution     `json:"policy_resolution,omitempty"`
}

type operationJournal struct {
	Version      int                 `json:"version"`
	Project      string              `json:"project"`
	LastScanAt   string              `json:"last_scan_at,omitempty"`
	NextSequence int                 `json:"next_sequence"`
	Incidents    []operationIncident `json:"incidents"`
	Stages       []operationStage    `json:"stages,omitempty"`
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
	for i := range out.Incidents {
		if out.Incidents[i].Severity == "" {
			out.Incidents[i].Severity = operationIncidentSeverity(out.Incidents[i])
		}
	}
	return out, nil
}

func writeOperationJournal(project string, journal operationJournal) error {
	releaseGuard, guardErr := projectguard.AcquireWrite(project)
	if guardErr != nil {
		return guardErr
	}
	defer releaseGuard()

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
	base := operationIncident{Project: project, AttemptID: attempt.AttemptID, AssignmentID: attempt.BindingEvidence["assignment_id"], TaskID: attempt.TaskID,
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

func operationIncidentSeverity(item operationIncident) string {
	if item.Quality == "confirmed" {
		return "error"
	}
	if item.Kind == "monitor_gap" {
		return "info"
	}
	return "warning"
}

func operationNotificationKind(item operationIncident) string {
	if item.Quality == "confirmed" {
		return "interrupted"
	}
	if item.Kind == "no_signal" || item.Kind == "handoff_stalled" {
		return "stalled"
	}
	return "runtime_unknown"
}

func operationEvidenceRecovery(item operationIncident, attempts []runtimeobs.Attempt, now time.Time) (string, bool) {
	if item.Quality == "confirmed" || (item.Kind != "runtime_unknown" && item.Kind != "no_signal") || item.AssignmentID == "" {
		return "", false
	}
	anchor := item.LastObservedAt
	if anchor == "" {
		anchor = item.DetectedAt
	}
	since, err := time.Parse(time.RFC3339Nano, anchor)
	if err != nil || since.IsZero() {
		return "", false
	}
	for _, attempt := range attempts {
		if (item.AttemptID != "" && attempt.AttemptID != item.AttemptID) || !strings.EqualFold(attempt.TaskID, item.TaskID) || attempt.AgentPath != item.AgentPath || attempt.BindingState != runtimeobs.BindingBound || attempt.BindingEvidence["assignment_id"] != item.AssignmentID {
			continue
		}
		if attempt.CurrentState != runtimeobs.StateRunning || attempt.Terminal || attempt.StateEvidenceSource != runtimeobs.EvidenceHook || (attempt.StateObservationQuality != runtimeobs.QualityObserved && attempt.StateObservationQuality != runtimeobs.QualityAuthoritative) {
			continue
		}
		observed, parseErr := time.Parse(time.RFC3339Nano, attempt.LastActivityAt)
		if parseErr != nil || observed.IsZero() || !observed.After(since) || observed.After(now) || now.Sub(observed) >= runtimeobs.StaleWarnDuration() {
			continue
		}
		return observed.UTC().Format(time.RFC3339Nano), true
	}
	return "", false
}

// A verified, later Hook event for the *same attempt* can retire an
// unverified warning even when the original binding had no assignment ID.
// It cannot retire a confirmed error, interruption, or shutdown.
func operationExactAttemptRecovered(item operationIncident, attempts []runtimeobs.Attempt, now time.Time) (string, string, bool) {
 if item.Quality!="verification_required" || (item.Kind!="runtime_unknown" && item.Kind!="no_signal") || item.AttemptID=="" {return "","",false}
 detected,err:=time.Parse(time.RFC3339Nano,item.DetectedAt)
 if err!=nil || detected.IsZero(){return "","",false}
 for _,a:=range attempts {
  if a.AttemptID!=item.AttemptID || !strings.EqualFold(a.TaskID,item.TaskID) ||
   a.AgentPath!=item.AgentPath || (item.AssignmentID!="" && a.BindingEvidence["assignment_id"]!=item.AssignmentID) ||
   a.StateEvidenceSource!=runtimeobs.EvidenceHook ||
   (a.StateObservationQuality!=runtimeobs.QualityObserved && a.StateObservationQuality!=runtimeobs.QualityAuthoritative){continue}
  switch a.CurrentState {
  case runtimeobs.StateCompleted:
   if !a.Terminal{continue}
   at,err:=time.Parse(time.RFC3339Nano,a.EndedAt)
   if err==nil&&at.After(detected)&&!at.After(now){
    return at.UTC().Format(time.RFC3339Nano),"same attempt has confirmed Hook completion; previous warning retired without changing runtime evidence",true
   }
  case runtimeobs.StateRunning:
   if a.Terminal{continue}
   at,err:=time.Parse(time.RFC3339Nano,a.LastObservedAt)
   if err==nil&&at.After(detected)&&!at.After(now)&&now.Sub(at)<runtimeobs.StaleWarnDuration(){
    return at.UTC().Format(time.RFC3339Nano),"same attempt has later observed Hook running state; previous warning retired without changing runtime evidence",true
   }
  }
 }
 return "","",false
}

// A canonical done task makes its previous *verification-required* assignment
// or silence warning no longer actionable. This is not proof of Worker death
// or success and never applies to actual confirmed terminal failures.
// Read only after a successful, unambiguous canonical inventory.
func operationCanonicalDoneRows(rows map[string][]backlog.Record, taskID string) bool {
 matches:=rows[strings.ToUpper(strings.TrimSpace(taskID))]
 return len(matches)==1 && matches[0].State=="done"
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
	policies := operationPolicies(project, ledger, now)
	recoveryPolicies := operationRecoveryPolicy(project, ledger, now)
	stages := currentOperationStages(operationStages(project, ledger, now), policies)
	// Lazily load only when an unverified session warning actually needs
	// resolution. Unavailable or ambiguous catalogs cannot close warnings.
	var canonicalRows map[string][]backlog.Record
	var canonicalLoaded, canonicalAvailable bool
	canonicalDone := func(taskID string) bool {
		if taskID == "" { return false }
		if !canonicalLoaded {
			canonicalRows, canonicalAvailable = operationCanonicalRows(project)
			canonicalLoaded = true
		}
		return canonicalAvailable && operationCanonicalDoneRows(canonicalRows, taskID)
	}
	for i := range stages {
		if completion, ok := completions[stages[i].AttemptID]; ok && !stages[i].Failed {
			stages[i].Stage = "controller_verified_complete"
			stages[i].Since = completion.resolution.CompletedAt
			stages[i].Evidence = "Controller verified backlog completion"
			stages[i].GraceUntil = ""
		}
	}
	journal.Stages = stages
	signals := map[string]operationSignal{}
	verified := map[string]bool{}
	for _, attempt := range ledger.Attempts {
		if _, ok := verified[attempt.Provider]; !ok {
			verified[attempt.Provider] = operationHookVerifier(project, attempt.Provider)
		}
		if signal, ok := classifyOperation(project, attempt, now, verified[attempt.Provider]); ok {
			// Completed canonical tasks no longer need unverified active-session
			// warnings. Confirmed failures remain actionable independently.
			if signal.alert && signal.incident.Quality=="verification_required" && canonicalDone(signal.incident.TaskID) {continue}
			// Suppress the same obsolete source on every scan, not only while
			// its first incident is active. Otherwise resolution would recur.
			latestAssignmentStillCanonical := operationLatestAssignmentSourceAlreadyResolved(signal.incident, journal) && operationCanonicalTaskIsDoing(project, signal.incident.TaskID)
			if operationSourceAlreadyResolved(signal.incident, journal) || latestAssignmentStillCanonical || (recoveryPolicies[strings.ToUpper(signal.incident.TaskID)] && latestOperationRecovery(signal.incident, ledger.Attempts, stages, now)) {
				continue
			}
			if stageSuppresses(signal.incident, stages, now) {
				continue
			}
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
		// A later dispatch/start is not evidence that an actual failure was
		// resolved. Preserve confirmed failure episodes for explicit review.
		if prior.CurrentState == runtimeobs.StateErrored || prior.CurrentState == runtimeobs.StateInterrupted || prior.CurrentState == runtimeobs.StateShutdown {
			continue
		}
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
			if parseErr == nil && started.After(ended) && later.BindingState == runtimeobs.BindingBound && later.StateEvidenceSource == runtimeobs.EvidenceHook && (later.StateObservationQuality == runtimeobs.QualityObserved || later.StateObservationQuality == runtimeobs.QualityAuthoritative) && !later.Terminal {
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
						if canonicalDone(id){continue}
						key := "task:" + id
						signals[key] = operationSignal{key: key, alert: true, incident: operationIncident{
							Project: project, TaskID: id, AgentPath: agent, AssignmentID: strings.TrimSpace(fmt.Sprint(item["assignment_id"])), Kind: "runtime_unknown", Quality: "verification_required",
							Evidence: "doing assignment has no linked runtime attempt",
							Action:   "Controller: 배정 기록과 runtime identity를 대조하세요. Worker 종료를 단정하거나 백로그 상태를 자동 변경하지 마세요.",
						}}
					}
				}
			}
		}
	}
	for _, stage := range stages {
		if stage.Stage == "worker_running" || stage.Stage == "runtime_observation_pending" || stage.Stage == "handoff_applied" || stage.Stage == "controller_verified_complete" || stage.deferred(now) {
			continue
		}
		kind := "handoff_stalled"
		if stage.Failed {
			kind = "handoff_failed"
		}
		key := "stage:" + stage.AssignmentID + ":" + stage.Stage
		if stage.AttemptID != "" {
			key = "attempt:" + stage.AttemptID
			if existing, ok := signals[key]; ok && existing.incident.Kind != "runtime_unknown" && existing.incident.Kind != "no_signal" {
				continue
			}
			if existing, ok := signals[key]; ok && existing.incident.Evidence == "terminal state lacks observed hook evidence" {
				continue
			}
		} else {
			delete(signals, "task:"+stage.TaskID)
		}
		signals[key] = operationSignal{key: key, alert: true, incident: operationIncident{Project: project, TaskID: stage.TaskID, AttemptID: stage.AttemptID, AssignmentID: stage.AssignmentID, Kind: kind, Quality: "verification_required", Evidence: stage.Evidence, LastObservedAt: stage.Since, Action: "Controller: 인계 전달·완료검토 근거와 정체 원인을 확인하세요. 백로그 완료를 자동 추정하지 마세요."}}
	}
	for key, signal := range signals {
		if operationPolicyForIncident(signal.incident, policies) != nil {
			delete(signals, key)
			continue
		}
		if stageSuppresses(signal.incident, stages, now) {
			delete(signals, key)
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
		if stageSuppresses(*item, stages, now) {
			item.RecoveredAt = nowText
			item.RecoveryEvidence = "bounded normal handoff/review grace; runtime evidence unchanged"
			continue
		}
		if at,evidence,recovered:=operationExactAttemptRecovered(*item,ledger.Attempts,now);recovered{
			item.RecoveredAt=at
			item.RecoveryEvidence=evidence
			if signal,ok:=signals[item.Key];ok&&signal.incident.Quality=="verification_required"{delete(signals,item.Key)}
			continue
		}
		if item.Quality=="verification_required" && (item.Kind=="runtime_unknown"||item.Kind=="no_signal") && canonicalDone(item.TaskID) {
			item.RecoveredAt=nowText
			item.RecoveryEvidence="canonical task is done; unverified session warning no longer actionable, runtime terminal evidence unchanged"
			if signal,ok:=signals[item.Key];ok&&signal.incident.Quality=="verification_required"{delete(signals,item.Key)}
			continue
		}
		if at, recovered := operationEvidenceRecovery(*item, ledger.Attempts, now); recovered {
			item.RecoveredAt = at
			item.RecoveryEvidence = "same assignment has recent observed Hook execution; prior runtime evidence unchanged"
			delete(signals, item.Key)
			continue
		}
		if recoveryPolicies[strings.ToUpper(item.TaskID)] && latestOperationRecovery(*item, ledger.Attempts, stages, now) {
			item.RecoveredAt = nowText
			item.RecoveryEvidence = "latest assignment has observed execution evidence; prior runtime evidence unchanged"
			delete(signals, item.Key)
			continue
		}
		if resolution := operationCompletedUnknown(*item, completions); resolution != nil {
			item.RecoveredAt = nowText
			item.Resolution = resolution
			delete(signals, item.Key)
			continue
		}
		if proof := operationPolicyForIncident(*item, policies); proof != nil {
			item.RecoveredAt = nowText
			item.PolicyResolution = proof
			item.RecoveryEvidence = proof.Reason + "; runtime evidence unchanged"
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
		} else if _, present := signals[key]; present {
			// A changed kind at the same attempt key is a new episode.
			item.RecoveredAt = nowText
			delete(active, key)
		} else if item.Kind != "runtime_unknown" {
			item.RecoveredAt = nowText
			delete(active, key)
		}
		// For runtime_unknown, missing a signal is not itself proof of
		// resolution (a sensor may simply have stopped reporting). The exact
		// Hook/canonical checks above retire it when evidence is available.
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
		signal.incident.Severity = operationIncidentSeverity(signal.incident)
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
		controllerWork := false
		for _, stage := range journal.Stages {
			if stage.ControllerReview && strings.EqualFold(stage.TaskID, incident.TaskID) && (incident.AttemptID == "" || stage.AttemptID == incident.AttemptID) {
				controllerWork = true
				break
			}
		}
		if controllerWork {
			continue
		}
		if incident.Kind == "normal_exit" {
			continue
		}
		// Recovered scan gaps remain diagnostic history, never actionable alerts.
		if incident.RecoveredAt != "" {
			continue
		}
		kind := operationNotificationKind(incident)
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

// Each active project needs the canonical registration/start/completion feed,
// even when no browser tab is open. The operation scanner already discovers
// registered projects, but its incident delivery covers only runtime alerts.
func ensureOperationNotificationFeed(project string) {
	ctx, err := webContext(project, "")
	if err != nil {
		return
	}
	selected, _, err := resolveBacklog(project, ctx, url.Values{})
	if err != nil {
		return
	}
	ensureAttentionFeed(project, selected)
}

func scanOperationProjects(primary string, now time.Time) {
	scanOperationProjectsWithPriority(primary,now,false)
}

// The first scan after a Web restart is mandatory: it detects downtime and
// preserves operation-gap evidence. Only subsequent periodic scans are best
// effort, allowing an active backlog HTTP request to finish first.
func scanOperationProjectsWithPriority(primary string, now time.Time, background bool) {
	for _, project := range operationProjects(primary) {
		work:=func(){
			ensureOperationNotificationFeed(project)
			journal, err := scanOperationProject(project, now)
			if err == nil {
				deliverOperationIncidents(project, journal)
			}
		}
		if background{
			maintenance.TryWithProjectMonitoring(project,work)
		}else{
			maintenance.WithProjectMonitoring(project,work)
		}
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
			scanOperationProjectsWithPriority(primary, now.UTC(),true)
		}
	}
}

func operationSnapshot(primary string) map[string]any {
	projects := []map[string]any{}
	active := []operationIncident{}
	recent := []operationIncident{}
	resolved := []operationIncident{}
	stages := []operationStage{}
	for _, project := range operationProjects(primary) {
		journal, err := readOperationJournal(project)
		if err != nil {
			projects = append(projects, map[string]any{"path": project, "error": err.Error()})
			continue
		}
		projects = append(projects, map[string]any{"path": project, "last_scan_at": journal.LastScanAt})
		stages = append(stages, journal.Stages...)
		for _, incident := range journal.Incidents {
			if (incident.Resolution != nil || incident.RecoveryEvidence != "") && incident.RecoveredAt != "" {
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
	return map[string]any{"snapshot_at": time.Now().Format(time.RFC3339Nano), "projects": projects, "active": active, "recent": recent, "resolved_observations": resolved, "stages": stages, "telegram_transport_disabled": notify.TelegramTransportDisabled()}
}
