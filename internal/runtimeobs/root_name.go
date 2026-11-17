package runtimeobs

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"
)

// RootNameObservation is display metadata only, never execution evidence.
// A snapshot (including empty/unavailable) replaces older display metadata.
type RootNameObservation struct {
	Version          int    `json:"version"`
	Name             string `json:"name,omitempty"`
	Source           string `json:"source"`
	Status           string `json:"status"`
	CheckedAt        string `json:"checked_at"`
	T3ThreadID       string `json:"t3_thread_id,omitempty"`
	NativeName       string `json:"native_name,omitempty"`
	HostStatus       string `json:"host_status,omitempty"`
	Failure          string `json:"failure,omitempty"`
	HostProjectID    string `json:"host_project_id,omitempty"`
	ParentThreadID   string `json:"parent_thread_id,omitempty"`
	Relationship     string `json:"relationship,omitempty"`
	IdentityVerified bool   `json:"identity_verified,omitempty"`
}

func (o RootNameObservation) displaySnapshot() RootNameObservation {
	if o.Version != 1 || (o.Status != "verified" && o.Status != "empty" && o.Status != "unavailable") ||
		(o.Source != "provider_name" && o.Source != "t3_title" && o.Source != "task_mecca_fallback") ||
		(o.Source == "t3_title" && o.T3ThreadID == "") ||
		(o.Status == "verified" && (strings.TrimSpace(o.Name) == "" || o.Source == "task_mecca_fallback")) {
		o.Name = ""
		o.Source = "task_mecca_fallback"
		o.Status = "unavailable"
		o.IdentityVerified = false
	}
	if o.IdentityVerified && (o.HostProjectID == "" || o.T3ThreadID == "" || (o.HostStatus != "verified" && o.HostStatus != "empty") || (o.ParentThreadID != "" && o.Relationship != "subagent") || (o.ParentThreadID == "" && o.Relationship != "")) {
		o.IdentityVerified = false
	}
	if o.Status != "verified" {
		o.Name = ""
		o.Source = "task_mecca_fallback"
	}
	return o
}

type t3RootTitle struct {
	ThreadID         string  `json:"thread_id"`
	Title            string  `json:"title"`
	ProjectID        string  `json:"project_id"`
	ParentThreadID   string  `json:"parent_thread_id"`
	Relationship     string  `json:"relationship"`
	ParentProjectID  string  `json:"parent_project_id"`
	ParentDeleted    *string `json:"parent_deleted"`
	ParentArchived   *string `json:"parent_archived"`
	IdentityVerified bool    `json:"-"`
}

// Discover an owned thread from structured identity, not from a supplied title.
// Reuse the exact native/thread receipt validator; no lifecycle is written.
func (s t3Store) rootTitle(project, sessionID string, now time.Time) (t3RootTitle, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return t3RootTitle{}, err
	}
	query := `SELECT DISTINCT t.thread_id,t.title,t.project_id,
 COALESCE(json_extract(t.payload_json,'$.lineage.parentThreadId'),'') AS parent_thread_id,
 COALESCE(json_extract(t.payload_json,'$.lineage.relationshipToParent'),'') AS relationship,
 parent.project_id AS parent_project_id,parent.deleted_at AS parent_deleted,parent.archived_at AS parent_archived FROM orchestration_v2_projection_threads t
JOIN projection_projects p ON p.project_id=t.project_id
 LEFT JOIN orchestration_v2_projection_threads parent ON parent.thread_id=json_extract(t.payload_json,'$.lineage.parentThreadId')
JOIN orchestration_v2_projection_run_attempts a ON a.thread_id=t.thread_id
WHERE p.workspace_root=` + sqlLiteral(filepath.Clean(project)) + ` AND p.deleted_at IS NULL
AND t.deleted_at IS NULL AND t.archived_at IS NULL
AND a.provider='codex' AND json_extract(a.payload_json,'$.nativeThreadId')=` + sqlLiteral(sessionID) + `;`
	raw, err := s.query(query)
	if err != nil {
		return t3RootTitle{}, err
	}
	var rows []t3RootTitle
	if err = json.Unmarshal(raw, &rows); err != nil {
		return t3RootTitle{}, err
	}
	if len(rows) != 1 || strings.TrimSpace(rows[0].ThreadID) == "" {
		return t3RootTitle{}, errors.New("T3 native/project mapping unavailable or ambiguous")
	}
	if rows[0].ProjectID == "" || (rows[0].ParentThreadID == "" && rows[0].Relationship != "") || (rows[0].ParentThreadID != "" && (rows[0].Relationship != "subagent" || rows[0].ParentProjectID != rows[0].ProjectID || rows[0].ParentDeleted != nil || rows[0].ParentArchived != nil)) {
		return t3RootTitle{}, errors.New("T3 parent/project relation unverified")
	}
	_, err = s.read(T3Selector{ThreadID: rows[0].ThreadID, SessionID: sessionID, RuntimeAgentID: sessionID, CanonicalProject: project}, false, now)
	if err != nil {
		return t3RootTitle{}, err
	}
	// Reject a mapping/title change across the identity validation window.
	again, err := s.query(query)
	if err != nil {
		return t3RootTitle{}, err
	}
	var confirmed []t3RootTitle
	if json.Unmarshal(again, &confirmed) != nil || len(confirmed) != 1 || confirmed[0] != rows[0] {
		return t3RootTitle{}, errors.New("T3 display metadata changed during verification")
	}
	rows[0].IdentityVerified = true
	return rows[0], nil
}

var resolveT3RootTitle = func(ctx context.Context, project, sessionID string, now time.Time) (t3RootTitle, error) {
	s, err := localT3Store()
	if err != nil {
		return t3RootTitle{}, err
	}
	return s.withContext(ctx).rootTitle(project, sessionID, now)
}

func observeRootName(project, sessionID string, now time.Time) RootNameObservation {
	return observeRootNameContext(context.Background(), project, sessionID, now)
}

func observeRootNameContext(ctx context.Context, project, sessionID string, now time.Time) RootNameObservation {
	result := RootNameObservation{Version: 1, Source: "task_mecca_fallback", Status: "unavailable", CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	// Host identity/title and native name are independent display reads. Neither
	// read records execution, usage, Activity, consent, or a role binding.
	title, hostErr := resolveT3RootTitle(ctx, project, sessionID, now)
	if hostErr == nil {
		result.T3ThreadID, result.HostProjectID = title.ThreadID, title.ProjectID
		result.ParentThreadID, result.Relationship = title.ParentThreadID, title.Relationship
		result.IdentityVerified = title.IdentityVerified
		result.HostStatus = "verified"
		if strings.TrimSpace(title.Title) != "" {
			result.Name, result.Source, result.Status = title.Title, "t3_title", "verified"
		} else {
			result.HostStatus = "empty"
			result.Status = "empty"
			result.Failure = "host_title_empty"
		}
	} else {
		result.HostStatus = "unavailable"
		result.Failure = "host_identity_or_title_unavailable"
	}
	if ctx.Err() != nil {
		result.Failure = "name_refresh_deferred"
		return result
	}
	meta, nativeErr := resolveCodexThreadMetadata(ctx, sessionID)
	if nativeErr == nil && meta.ID == sessionID {
		result.NativeName = strings.TrimSpace(meta.Name)
		if result.Name == "" && result.NativeName != "" {
			result.Name, result.Source, result.Status = result.NativeName, "provider_name", "verified"
		}
	}
	return result
}

// RootSessionExecution is a bounded observation of this exact Root's own host
// run. It is never a descendant/task rollup and never replaces stored status.
type RootSessionExecution struct {
	State           CanonicalState `json:"state"`
	Source          EvidenceSource `json:"source,omitempty"`
	ObservedAt      string         `json:"observed_at,omitempty"`
	OfficialCurrent bool           `json:"official_current"`
	FailedStage     string         `json:"failed_stage,omitempty"`
	Limitation      string         `json:"limitation,omitempty"`
}

func ObserveRootSessionState(ctx context.Context, root RootSession, now time.Time) RootSessionExecution {
	result := RootSessionExecution{State: StateRuntimeUnknown, FailedStage: "root_identity", Limitation: "exact verified Root and host linkage required"}
	if root.Role != "root" || root.RelationStatus != "verified" || root.T3ThreadID == "" || root.Provider != "codex" || root.ProviderSessionID == "" || !filepath.IsAbs(root.CanonicalProject) {
		return result
	}
	store, err := localT3Store()
	if err != nil {
		return result
	}
	receipt, err := store.withContext(ctx).read(T3Selector{ThreadID: root.T3ThreadID, SessionID: root.ProviderSessionID, RuntimeAgentID: root.ProviderSessionID, CanonicalProject: root.CanonicalProject}, false, now)
	if err != nil || receipt.CanonicalProject != filepath.Clean(root.CanonicalProject) {
		result.FailedStage = "official_root_observation"
		result.Limitation = "current exact Root execution unavailable; stored execution remains separate"
		return result
	}
	return RootSessionExecution{State: receipt.State, Source: EvidenceT3, ObservedAt: receipt.ObservedAt, OfficialCurrent: true}
}
