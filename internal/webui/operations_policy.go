package webui

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/backlog"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

// Policy evidence retires observation warnings, not runtime attempts. Legacy
// Controller reports without identity IDs are task-wide boundaries only when
// the current canonical task is unique and all durable activity predates them.
type operationPolicyResolution struct {
	Reason         string `json:"reason"`
	EventID        string `json:"event_id"`
	TaskPath       string `json:"task_path"`
	BoundaryAt     string `json:"boundary_at"`
	EvidenceSource string `json:"evidence_source"`
}

type operationPolicy struct {
	project            string
	proof              operationPolicyResolution
	boundary           time.Time
	assignments        map[string]bool
	attempts           map[string]bool
	agents             map[string]bool
	attemptAgents      map[string]string
	assignmentAgents   map[string]string
	attemptAssignments map[string]string
}

func operationReleasedClaim(row backlog.Record) bool {
	// The canonical parser normalizes '-' to empty. Require an explicit
	// released claim, not a missing or accidentally blank ownership field.
	fields := map[string]bool{}
	for _, line := range strings.Split(row.RawMarkdown, "\n") {
		line = strings.TrimSpace(line)
		for _, name := range []string{"Agent", "변경범위"} {
			prefix := "- " + name + ":"
			if strings.HasPrefix(line, prefix) {
				fields[name] = strings.TrimSpace(strings.TrimPrefix(line, prefix)) == "-"
			}
		}
	}
	return fields["Agent"] && fields["변경범위"]
}

func operationCanonicalRows(project string) (map[string][]backlog.Record, bool) {
	rows := map[string][]backlog.Record{}
	candidates, err := backlog.Discover(project, "")
	if err != nil {
		return rows, false
	}
	for _, candidate := range candidates {
		if filepath.Dir(candidate.Path) != filepath.Join(project, "_task_mecca", "data") {
			continue
		}
		catalog, err := backlog.Catalog(project, candidate.Path)
		if err != nil {
			return rows, false
		}
		for _, row := range catalog {
			id := strings.ToUpper(row.ID)
			rows[id] = append(rows[id], row)
		}
	}
	return rows, true
}

// A prior exact source can remain retired by a newer observed assignment, but
// only while the unique current canonical ownership still agrees. A later
// failure of the successor stays a distinct actionable signal.
func operationRecoveryPolicy(project string, ledger runtimeobs.Ledger, now time.Time) map[string]bool {
	out := map[string]bool{}
	rows, ok := operationCanonicalRows(project)
	if !ok {
		return out
	}
	for _, f := range ledger.Findings {
		if f.Severity != "info" {
			return out
		}
	}
	scan, err := backlog.ReadLifecycleTransitions(project)
	if err != nil || len(scan.Findings) != 0 {
		return out
	}
	assignments, err := runtimeobs.ListAssignments(project)
	if err != nil {
		return out
	}
	for id, matches := range rows {
		if len(matches) != 1 || matches[0].State != "doing" || matches[0].Location != "active" {
			continue
		}
		var latest runtimeobs.Assignment
		var at time.Time
		valid := true
		for _, a := range assignments {
			if !strings.EqualFold(a.TaskID, id) {
				continue
			}
			t, e := time.Parse(time.RFC3339Nano, a.AssignedAt)
			if e != nil || t.IsZero() || t.After(now) || t.Equal(at) {
				valid = false
				break
			}
			if t.After(at) {
				latest, at = a, t
			}
		}
		if !valid || latest.AgentPath != strings.TrimSpace(matches[0].Fields["Agent"]) {
			continue
		}
		latestKind := ""
		var lifecycleAt time.Time
		for _, e := range scan.Events {
			if !strings.EqualFold(e.TaskID, id) {
				continue
			}
			t, te := time.Parse(time.RFC3339Nano, e.OccurredAt)
			r, re := time.Parse(time.RFC3339Nano, e.RecordedAt)
			if te != nil || re != nil || t.IsZero() || r.IsZero() || t.After(now) || r.Before(t) || r.After(now) || t.Equal(lifecycleAt) {
				valid = false
				break
			}
			if t.After(lifecycleAt) {
				lifecycleAt, latestKind = t, e.Kind
			}
		}
		if !valid || latestKind == "completed" || latestKind == "waiting" {
			continue
		}
		count, observed := 0, false
		for _, a := range ledger.Attempts {
			if a.BindingState != runtimeobs.BindingBound || a.BindingEvidence["assignment_id"] != latest.AssignmentID || a.AgentPath != latest.AgentPath || !strings.EqualFold(a.TaskID, id) {
				continue
			}
			count++
			t, e := time.Parse(time.RFC3339Nano, a.LastObservedAt)
			observed = e == nil && !t.IsZero() && !t.Before(at) && !t.After(now) && a.StateEvidenceSource == runtimeobs.EvidenceHook && (a.StateObservationQuality == runtimeobs.QualityObserved || a.StateObservationQuality == runtimeobs.QualityAuthoritative)
		}
		out[id] = count == 1 && observed
	}
	return out
}

func operationPolicies(project string, ledger runtimeobs.Ledger, now time.Time) map[string]operationPolicy {
	out := map[string]operationPolicy{}
	for _, finding := range ledger.Findings {
		if finding.Severity != "info" {
			return out
		}
	}
	scan, err := backlog.ReadLifecycleTransitions(project)
	if err != nil || len(scan.Findings) != 0 {
		return out
	}
	assignments, err := runtimeobs.ListAssignments(project)
	if err != nil {
		return out
	}
	rows, available := operationCanonicalRows(project)
	if !available {
		return out
	}
	for id, matches := range rows {
		if len(matches) != 1 {
			continue
		}
		row := matches[0]
		kind, reason := "", ""
		if row.State == "done" && operationMeaningfulEvidence(row.Fields["결과"]) && operationMeaningfulEvidence(row.Fields["검증"]) {
			kind, reason = "completed", "canonical_task_completed"
		} else if row.State == "hold" && row.Location == "active" && operationReleasedClaim(row) {
			wait := row.Fields["대기유형"]
			if (wait == "user" || wait == "external" || wait == "dependency") && operationMeaningfulEvidence(row.Fields["대기"]) && operationMeaningfulEvidence(row.Fields["재개조건"]) && operationMeaningfulEvidence(row.Fields["대기근거"]) {
				kind, reason = "waiting", "canonical_released_"+wait+"_hold"
			}
		}
		if kind == "" {
			continue
		}
		var latest backlog.LifecycleTransition
		var boundary time.Time
		valid := true
		for _, event := range scan.Events {
			if !strings.EqualFold(event.TaskID, id) {
				continue
			}
			at, e := time.Parse(time.RFC3339Nano, event.OccurredAt)
			recorded, re := time.Parse(time.RFC3339Nano, event.RecordedAt)
			if e != nil || re != nil || at.IsZero() || recorded.IsZero() || at.After(now) || recorded.Before(at) || recorded.After(now) {
				valid = false
				break
			}
			if at.Equal(boundary) {
				valid = false
				break
			}
			if at.After(boundary) {
				latest, boundary = event, at
			}
		}
		if !valid || latest.Kind != kind || latest.Actor != "/root/controller" || !operationMeaningfulEvidence(latest.EvidenceRef) {
			continue
		}
		if latest.EvidenceSource != "controller_verified" && latest.EvidenceSource != "controller_report" && !(kind == "waiting" && latest.EvidenceSource == "worker_report") {
			continue
		}
		p := operationPolicy{
			project: project, boundary: boundary, assignments: map[string]bool{}, attempts: map[string]bool{},
			agents: map[string]bool{}, attemptAgents: map[string]string{}, assignmentAgents: map[string]string{}, attemptAssignments: map[string]string{},
			proof: operationPolicyResolution{Reason: reason, EventID: latest.EventID, TaskPath: row.Path, BoundaryAt: latest.OccurredAt, EvidenceSource: latest.EvidenceSource},
		}
		for _, a := range assignments {
			if !strings.EqualFold(a.TaskID, id) {
				continue
			}
			at, e := time.Parse(time.RFC3339Nano, a.AssignedAt)
			if e != nil || at.IsZero() || at.After(boundary) || p.assignments[a.AssignmentID] {
				valid = false
				break
			}
			p.assignments[a.AssignmentID] = true
			p.agents[a.AgentPath] = true
			p.assignmentAgents[a.AssignmentID] = a.AgentPath
		}
		if len(p.assignments) == 0 || (latest.AssignmentID != "" && !p.assignments[latest.AssignmentID]) {
			continue
		}
		for _, a := range ledger.Attempts {
			if !strings.EqualFold(a.TaskID, id) {
				continue
			}
			// Pre-assignment/manual legacy attempts are separate evidence.
			// They cannot retire an attempt warning or veto an unrelated
			// assignment-only stage. Their actual errors remain classified.
			if a.BindingEvidence["assignment_id"] == "" {
				continue
			}
			if a.CurrentState == runtimeobs.StateErrored || a.CurrentState == runtimeobs.StateInterrupted || a.CurrentState == runtimeobs.StateShutdown || (a.Terminal && a.CurrentState == runtimeobs.StateRuntimeUnknown) {
				valid = false
				break
			}
			if a.BindingState != runtimeobs.BindingBound || !p.assignments[a.BindingEvidence["assignment_id"]] || p.assignmentAgents[a.BindingEvidence["assignment_id"]] != a.AgentPath {
				valid = false
				break
			}
			for _, value := range []string{a.FirstObservedAt, a.BindingAt, a.StartedAt} {
				if value == "" {
					continue
				}
				at, e := time.Parse(time.RFC3339Nano, value)
				if e != nil || at.IsZero() || at.After(boundary) {
					valid = false
				}
			}
			endBoundary := boundary
			// A released Worker may finish reporting within the same exact
			// attempt after Controller records the hold. Trust a real terminal
			// Hook, not polling time or a guessed grace window. Fresh running
			// activity/new bindings and post-terminal activity still fail closed.
			if latest.AttemptID == a.AttemptID && latest.AssignmentID == a.BindingEvidence["assignment_id"] && a.CurrentState == runtimeobs.StateCompleted && a.Terminal && a.StateEvidenceSource == runtimeobs.EvidenceHook && (a.StateObservationQuality == runtimeobs.QualityObserved || a.StateObservationQuality == runtimeobs.QualityAuthoritative) {
				ended, e := time.Parse(time.RFC3339Nano, a.EndedAt)
				if e != nil || ended.IsZero() || ended.After(now) {
					valid = false
				} else if ended.After(boundary) {
					endBoundary = ended
				}
			}
			for _, value := range []string{a.LastObservedAt, a.LastActivityAt, a.EndedAt} {
				if value == "" {
					continue
				}
				at, e := time.Parse(time.RFC3339Nano, value)
				if e != nil || at.IsZero() || at.After(endBoundary) {
					valid = false
				}
			}
			p.attempts[a.AttemptID] = true
			p.attemptAgents[a.AttemptID] = a.AgentPath
			p.attemptAssignments[a.AttemptID] = a.BindingEvidence["assignment_id"]
		}
		if latest.AttemptID != "" && !p.attempts[latest.AttemptID] {
			valid = false
		}
		if latest.AttemptID != "" && latest.AssignmentID != "" && p.attemptAssignments[latest.AttemptID] != latest.AssignmentID {
			valid = false
		}
		if valid {
			out[id] = p
		}
	}
	return out
}

func operationPolicyForIncident(item operationIncident, policies map[string]operationPolicy) *operationPolicyResolution {
	eligible := item.Kind == "handoff_stalled" || item.Kind == "no_signal" || (item.Kind == "runtime_unknown" && (item.Evidence == "runtime identity or hook execution unverified" || item.Evidence == "doing assignment has no linked runtime attempt"))
	if !eligible || item.Quality != "verification_required" {
		return nil
	}
	p, ok := policies[strings.ToUpper(item.TaskID)]
	if !ok || filepath.Clean(item.Project) != filepath.Clean(p.project) || (item.AttemptID != "" && !p.attempts[item.AttemptID]) {
		return nil
	}
	if item.AgentPath != "" && ((!p.agents[item.AgentPath]) || (item.AttemptID != "" && p.attemptAgents[item.AttemptID] != item.AgentPath)) {
		return nil
	}
	if item.LastObservedAt != "" {
		at, err := time.Parse(time.RFC3339Nano, item.LastObservedAt)
		if err != nil || at.IsZero() || at.After(p.boundary) {
			return nil
		}
	}
	proof := p.proof
	return &proof
}
