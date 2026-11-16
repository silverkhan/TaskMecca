package runtimeobs

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/rolepolicy"
)

func PolicyIdentity(a Attempt) rolepolicy.Identity {
	return rolepolicy.Identity{AttemptID: a.AttemptID, NativeID: a.SessionID, RuntimeAgentID: a.RuntimeAgentID, Provider: a.Provider, AgentPath: a.AgentPath, TaskID: a.TaskID, Bound: a.BindingState == BindingBound, Observed: a.SessionID != "" && a.RuntimeAgentID != "" && (a.StateEvidenceSource == EvidenceHook || a.StateEvidenceSource == EvidenceT3) && (a.StateObservationQuality == QualityObserved || a.StateObservationQuality == QualityAuthoritative)}
}

// ReadNativeReview reads the reviewer's actual final assistant result through
// the official provider interface. JSON files, role labels, Activity outcomes,
// and caller-supplied independent:true are not native review evidence.
// Other providers remain unknown until an equivalent adapter exists.
func ReadNativeReview(ctx context.Context, a Attempt, task, reportSHA string, now time.Time) (rolepolicy.ReviewStatement, error) {
	var zero rolepolicy.ReviewStatement
	identity := PolicyIdentity(a)
	if !identity.Bound || !identity.Observed || a.Provider != "codex" || a.SessionID == "" || a.TaskID != task {
		return zero, errors.New("review_native_identity_unverified_or_unsupported")
	}
	if a.StateEvidenceSource == EvidenceT3 {
		if e := VerifyT3Attempt(a, now); e != nil {
			return zero, e
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, e := openCodexMetadata(ctx)
	if e != nil {
		return zero, e
	}
	defer c.close()
	raw, e := c.call("thread/read", map[string]any{"threadId": a.SessionID, "includeTurns": true})
	if e != nil {
		return zero, e
	}
	return nativeReviewFromThread(raw, a.SessionID, task, reportSHA, a.AttemptID)
}

func nativeReviewFromThread(raw []byte, native, task, reportSHA, attempt string) (rolepolicy.ReviewStatement, error) {
	var payload struct {
		Thread struct {
			ID    string `json:"id"`
			Turns []struct {
				Status string `json:"status"`
				Items  []struct {
					Type  string `json:"type"`
					Text  string `json:"text"`
					Phase string `json:"phase"`
				} `json:"items"`
			} `json:"turns"`
		} `json:"thread"`
	}
	var result rolepolicy.ReviewStatement
	if e := json.Unmarshal(raw, &payload); e != nil {
		return result, e
	}
	if payload.Thread.ID != native {
		return result, errors.New("review_native_readback_mismatch")
	}
	count := 0
	for _, turn := range payload.Thread.Turns {
		if turn.Status != "completed" {
			continue
		}
		for _, item := range turn.Items {
			if item.Type != "agentMessage" || item.Phase != "final_answer" {
				continue
			}
			for _, line := range strings.Split(item.Text, "\n") {
				if !strings.HasPrefix(line, rolepolicy.ReviewMarker) {
					continue
				}
				var s rolepolicy.ReviewStatement
				if e := json.Unmarshal([]byte(strings.TrimPrefix(line, rolepolicy.ReviewMarker)), &s); e != nil {
					continue
				}
				if s.TaskID == task && s.ReportSHA256 == reportSHA && s.ReviewerAttemptID == attempt {
					result = s
					count++
				}
			}
		}
	}
	if count != 1 {
		return result, errors.New("exact_native_review_result_missing_or_ambiguous")
	}
	return result, nil
}
