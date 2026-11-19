package webui

import (
	stdcontext "context"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

// This explicitly scoped API must never fall back to the Backlog/service
// project when a saved session selection is removed or invalid.
func workloadSessionProject(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if !filepath.IsAbs(requested) || !maintenance.IsRegisteredProject(requested) {
		return "", fmt.Errorf("session project is not registered")
	}
	return filepath.Clean(requested), nil
}

// Both routes use this display-only boundary. It never substitutes name
// freshness for usage or execution freshness.
var refreshWorkloadNames = runtimeobs.RefreshCodexRootNames
var observeWorkloadRoot = runtimeobs.ObserveRootSessionState

func enrichSessionWorkload(project string, ledger runtimeobs.Ledger, data *runtimeobs.SessionWorkload, now time.Time) {
	data.CurrentExecution = currentExecutionEvidence(project, ledger, now, true)

	for i := range data.Active.Items {
		annotateSessionRole(&data.Active.Items[i], ledger, data.CurrentExecution)
	}
	for i := range data.Historical.Items {
		annotateSessionRole(&data.Historical.Items[i], ledger, data.CurrentExecution)
	}
	if root := data.Representative; root != nil {
		ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 250*time.Millisecond)
		defer cancel()
		observation := observeWorkloadRoot(ctx, *root, now)
		root.Execution = &observation
		for _, rows := range [][]runtimeobs.RootSession{data.Active.Items, data.Historical.Items} {
			for i := range rows {
				if rows[i].RootSessionID == root.RootSessionID {
					rows[i].Execution = &observation
				}
			}
		}
	}
}

func workloadSessionsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON := func(w http.ResponseWriter, payload any, status int) {
		body, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
	if r.Method != http.MethodGet {
		writeJSON(w, map[string]any{"error": "GET required"}, 405)
		return
	}
	requested := strings.TrimSpace(r.URL.Query().Get("project"))
	if requested == "" {
		writeJSON(w, map[string]any{"error": "explicit project required"}, 400)
		return
	}
	activeProject, err := workloadSessionProject(requested)
	if err != nil {
		writeJSON(w, map[string]any{"error": err.Error()}, 404)
		return
	}
	now := time.Now()
	ledger, err := runtimeobs.ReconcileLedger(activeProject, 10, now)
	if err != nil {
		writeJSON(w, map[string]any{"error": err.Error()}, 500)
		return
	}
	// The list and detail share the same bounded display-only refresh. Name
	// observations do not reconcile execution or refresh usage/Activity clocks.
	_, _ = refreshWorkloadNames(activeProject, ledger, now)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if id := r.URL.Query().Get("root_session_id"); id != "" {
		detail, err := runtimeobs.QuerySessionWorkloadDetail(activeProject, ledger, id, page, now)
		if err != nil {
			writeJSON(w, map[string]any{"error": err.Error()}, 404)
			return
		}
		projection := runtimeobs.SessionWorkload{Active: runtimeobs.RootSessionPage{Items: []runtimeobs.RootSession{detail.RootSession}}}
		if detail.RootSession.Role == "root" {
			selected := detail.RootSession
			projection.Representative = &selected
		}
		enrichSessionWorkload(activeProject, ledger, &projection, now)
		detail.RootSession = projection.Active.Items[0]
		detail.CurrentExecution = projection.CurrentExecution
		writeJSON(w, reviewerPresentationEnvelope(detail,readReviewerPresentation(activeProject,"",&ledger)), 200)
		return
	}
	historicalPage, _ := strconv.Atoi(r.URL.Query().Get("historical_page"))
	data, err := runtimeobs.QuerySessionWorkload(activeProject, ledger, page, historicalPage, 10, now)
	if err != nil {
		writeJSON(w, map[string]any{"error": err.Error()}, 500)
		return
	}
	enrichSessionWorkload(activeProject, ledger, &data, now)
	writeJSON(w, reviewerPresentationEnvelope(data,readReviewerPresentation(activeProject,"",&ledger)), 200)

}

func annotateSessionRole(root *runtimeobs.RootSession, ledger runtimeobs.Ledger, snapshots map[string]runtimeobs.CurrentExecutionSnapshot) {
	if root.RelationStatus != "verified" || (root.Role != "worker" && root.Role != "unknown") {
		return
	}
	for _, s := range snapshots {
		if s.ReviewNativeID == "" || s.ReviewNativeID != root.ProviderSessionID || s.ReviewAgentPath == "" {
			continue
		}
		for _, a := range ledger.Attempts {
			if a.Provider == s.Identity.Provider && strings.EqualFold(a.TaskID, s.Identity.TaskID) && a.SessionID == s.ReviewNativeID && a.RuntimeAgentID == s.ReviewNativeID && a.AgentPath == s.ReviewAgentPath && a.BindingState == runtimeobs.BindingBound {
				root.Role = "reviewer"
			}
		}
	}
}
