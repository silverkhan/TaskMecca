package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/silverkhan/TaskMecca/internal/handoffjournal"
	"time"

	"github.com/silverkhan/TaskMecca/internal/activity"
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
)

func activityCollectionReceipt(e activity.Event, status, problem string, now time.Time) runtimeobs.CollectionReceipt {
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return runtimeobs.CollectionReceipt{Key: "activity:" + e.TaskID + ":" + e.EventID, Status: status, At: now.UTC(), Error: problem, Owner: "activity_reporter:" + string(e.Role), ResumeCondition: "Read the exact immutable Activity event and verify its content hash; retry the same event ID and original explanation after restoring attribution/collection access. Never backfill a missing phase.", Activity: &runtimeobs.CollectionActivity{Role: string(e.Role), EventID: e.EventID, TaskID: e.TaskID, AgentID: e.AgentID, AttemptID: e.RuntimeAttemptID, PayloadSHA256: hex.EncodeToString(sum[:])}}
}

// An identical retry can reconcile an ACK lost after immutable publication.
// An old event with no delivery receipt is left untouched, never backfilled.
func ackActivityCollection(project string, e activity.Event) error {
	rows, err := runtimeobs.ReadCollectionReceipts(project)
	if err != nil {
		return err
	}
	want := activityCollectionReceipt(e, "ack", "", time.Now().UTC())
	latest := ""
	found := false
	for _, r := range rows {
		if r.Key != want.Key || r.Activity == nil {
			continue
		}
		if r.Activity.PayloadSHA256 != want.Activity.PayloadSHA256 {
			// A rejected/unpublished earlier attempt has no immutable event
			// time to replay. The ledger's actual later publication is current
			// evidence; preserve those older failed/pending payloads without
			// making them a conflict against the new immutable publication.
			if !e.RecordedAt.IsZero() && r.At.Before(e.RecordedAt) {
				continue
			}
			return fmt.Errorf("collection receipt content conflicts with immutable Activity event")
		}
		found = true
		latest = r.Status
	}
	if !found || latest == "ack" {
		return nil
	}
	_, err = runtimeobs.RecordCollectionReceipt(project, want)
	return err
}

type activityReportResolution struct {
	Action        activity.Action
	Type          activity.Type
	Round         int
	ReviewOutcome string
}

// Resolve a reported phase event using the same standard taxonomy as the
// Decision Engine's activity advice. This observes an event that the agent
// explicitly reports; it does not claim that a suggestion started any work.
func resolveActivityReport(store activity.Ledger, task string, role activity.Role, agent, attempt, eventName string) (activityReportResolution, error) {
	rows, err := store.List(task)
	if err != nil {
		return activityReportResolution{}, err
	}
	handoffs, err := handoffjournal.BuildLedger(store.Project, time.Now().UTC())
	if err != nil {
		return activityReportResolution{}, err
	}
	suggested := activity.SuggestedRoundFromEvidence(task, rows, handoffs.Handoffs)
	var latest *activity.Event
	for i := len(rows) - 1; i >= 0; i-- {
		row := &rows[i]
		if row.Role == role && row.AgentID == agent && row.RuntimeAttemptID == attempt && row.Action != activity.Correct {
			latest = row
			break
		}
	}
	active := latest != nil && latest.Action != activity.Stop && latest.Action != activity.Abandon
	round := 0
	if active {
		round = latest.Round
	} else {
		round = suggested
	}
	outcome := ""
	terminal := false
	switch eventName {
	case "worker_completed", "worker_blocked", "registration_completed", "review_completed", "phase_completed":
		terminal = true
	case "review_rejected":
		terminal = true
		outcome = "rejected"
	case "review_accepted":
		terminal = true
		outcome = "accepted"
	case "review_needs_changes":
		terminal = true
		outcome = "needs_changes"
	}
	if terminal {
		if !active {
			return activityReportResolution{}, fmt.Errorf("phase event %q requires an open activity", eventName)
		}
		if outcome != "" {
			if role != activity.RoleController || (latest.Type != activity.ControllerReview && latest.Type != activity.ControllerReReview) {
				return activityReportResolution{}, fmt.Errorf("review outcome requires the active Main Controller review")
			}
		}
		return activityReportResolution{Action: activity.Stop, Type: latest.Type, Round: round, ReviewOutcome: outcome}, nil
	}
	if nextRound := suggested; nextRound > round {
		round = nextRound
	}
	suggestion := activity.Suggest(role, eventName, round)
	next := activity.Start
	if active {
		next = activity.Switch
	}
	return activityReportResolution{Action: next, Type: suggestion.Type, Round: round}, nil
}
