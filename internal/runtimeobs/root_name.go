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
	Version    int    `json:"version"`
	Name       string `json:"name,omitempty"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	CheckedAt  string `json:"checked_at"`
	T3ThreadID string `json:"t3_thread_id,omitempty"`
}

func (o RootNameObservation) displaySnapshot() RootNameObservation {
	if o.Version != 1 || (o.Status != "verified" && o.Status != "empty" && o.Status != "unavailable") ||
		(o.Source != "provider_name" && o.Source != "t3_title" && o.Source != "task_mecca_fallback") ||
		(o.Source == "t3_title" && o.T3ThreadID == "") ||
		(o.Status == "verified" && (strings.TrimSpace(o.Name) == "" || o.Source == "task_mecca_fallback")) {
		o.Name = ""
		o.Source = "task_mecca_fallback"
		o.Status = "unavailable"
	}
	if o.Status != "verified" {
		o.Name = ""
		o.Source = "task_mecca_fallback"
	}
	return o
}

type t3RootTitle struct {
	ThreadID string `json:"thread_id"`
	Title    string `json:"title"`
}

// Discover an owned thread from structured identity, not from a supplied title.
// Reuse the exact native/thread receipt validator; no lifecycle is written.
func (s t3Store) rootTitle(project, sessionID string, now time.Time) (t3RootTitle, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return t3RootTitle{}, err
	}
	query := `SELECT DISTINCT t.thread_id,t.title FROM orchestration_v2_projection_threads t
JOIN projection_projects p ON p.project_id=t.project_id
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
	_, err = s.read(T3Selector{ThreadID: rows[0].ThreadID, SessionID: sessionID, RuntimeAgentID: sessionID}, false, now)
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
	return rows[0], nil
}

var resolveT3RootTitle = func(project, sessionID string, now time.Time) (t3RootTitle, error) {
	s, err := localT3Store()
	if err != nil {
		return t3RootTitle{}, err
	}
	return s.rootTitle(project, sessionID, now)
}

func observeRootName(project, sessionID string, now time.Time) RootNameObservation {
	result := RootNameObservation{Version: 1, Source: "task_mecca_fallback", Status: "unavailable", CheckedAt: now.UTC().Format(time.RFC3339Nano)}
	meta, err := resolveCodexThreadMetadata(context.Background(), sessionID)
	// An unsupported/failed native read cannot establish the native-name priority.
	if err != nil || meta.ID != sessionID {
		return result
	}
	if strings.TrimSpace(meta.Name) != "" {
		result.Name = meta.Name
		result.Source = "provider_name"
		result.Status = "verified"
		return result
	}
	title, err := resolveT3RootTitle(project, sessionID, now)
	if err != nil {
		return result
	}
	result.T3ThreadID = title.ThreadID
	result.Status = "empty"
	if strings.TrimSpace(title.Title) != "" {
		result.Name = title.Title
		result.Source = "t3_title"
		result.Status = "verified"
	}
	return result
}
