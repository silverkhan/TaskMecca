package notify

import (
	"encoding/json"
	"reflect"
)

// Preserve older/newer opaque delivery facts on disk without projecting them
// through credential-free public DeliveryRecord/ResumeStatus diagnostics.
func (ledger *deliveryLedger) UnmarshalJSON(data []byte) error {
	type fields deliveryLedger
	var decoded fields
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var records map[string]map[string]json.RawMessage
	var facts map[string]json.RawMessage
	if value := raw["records"]; value != nil {
		if err := json.Unmarshal(value, &records); err != nil {
			return err
		}
		if err := json.Unmarshal(value, &facts); err != nil {
			return err
		}
	}
	for _, extra := range records {
		for _, key := range []string{"event_id", "task_id", "kind", "channel", "event_at", "state", "attempts", "created_at", "last_attempt_at", "last_response_at", "last_error", "next_retry_at", "duplicate_possible", "attempt_history", "suppression"} {
			delete(extra, key)
		}
	}
	for _, key := range []string{"version", "records", "resume_boundary", "resume_history"} {
		delete(raw, key)
	}
	decoded.Preserved, decoded.RecordExtras = raw, records
	decoded.RecordFacts = facts
	*ledger = deliveryLedger(decoded)
	return nil
}

func (ledger deliveryLedger) MarshalJSON() ([]byte, error) {
	type fields deliveryLedger
	data, err := json.Marshal(fields(ledger))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var records map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw["records"], &records); err != nil {
		return nil, err
	}
	for id, extras := range ledger.RecordExtras {
		if record, exists := records[id]; exists {
			for key, value := range extras {
				record[key] = value
			}
		}
	}
	for id, fact := range ledger.RecordFacts {
		var previous DeliveryRecord
		if err := json.Unmarshal(fact, &previous); err != nil {
			return nil, err
		}
		current, exists := ledger.Records[id]
		if !exists {
			continue
		}
		// Preserve complete settled records, including unknown nested history and
		// original omitted fields, rather than manufacturing empty evidence.
		if reflect.DeepEqual(previous, current) {
			var original map[string]json.RawMessage
			if err := json.Unmarshal(fact, &original); err != nil {
				return nil, err
			}
			records[id] = original
			continue
		}
		var original map[string]json.RawMessage
		if err := json.Unmarshal(fact, &original); err != nil {
			return nil, err
		}
		var oldAttempts, newAttempts []map[string]json.RawMessage
		if value := original["attempt_history"]; value != nil {
			if err := json.Unmarshal(value, &oldAttempts); err != nil {
				return nil, err
			}
		}
		if value := records[id]["attempt_history"]; value != nil {
			if err := json.Unmarshal(value, &newAttempts); err != nil {
				return nil, err
			}
		}
		for index, attempt := range newAttempts {
			if index >= len(oldAttempts) || index >= len(previous.AttemptHistory) || index >= len(current.AttemptHistory) {
				break
			}
			if previous.AttemptHistory[index].Number != current.AttemptHistory[index].Number || previous.AttemptHistory[index].StartedAt != current.AttemptHistory[index].StartedAt {
				continue
			}
			for key, value := range oldAttempts[index] {
				if key != "number" && key != "started_at" && key != "responded_at" && key != "outcome" {
					attempt[key] = value
				}
			}
		}
		if len(newAttempts) > 0 {
			var err error
			records[id]["attempt_history"], err = json.Marshal(newAttempts)
			if err != nil {
				return nil, err
			}
		}
	}
	raw["records"], err = json.Marshal(records)
	if err != nil {
		return nil, err
	}
	for key, value := range ledger.Preserved {
		raw[key] = value
	}
	return json.Marshal(raw)
}
