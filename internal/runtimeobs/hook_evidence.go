package runtimeobs

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// HookEvidenceSince reads only received hook events. Bindings, inferred runtime
// states and historical attempts cannot make an untrusted hook look verified.
func HookEvidenceSince(project, provider, since string) (map[string]bool, string, error) {
	events := map[string]bool{"activity": false, "start": false, "stop": false}
	cutoff, err := time.Parse(time.RFC3339Nano, since)
	if err != nil {
		return events, "", nil
	}
	files, err := executionEventFiles(project)
	if err != nil {
		return events, "", err
	}
	last := ""
	seen := map[string]bool{}
	for _, path := range files {
		file, openErr := os.Open(path)
		if openErr != nil {
			return events, last, openErr
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			var event ExecutionEvent
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				continue
			}
			if event.EvidenceSource != EvidenceHook || !strings.EqualFold(event.Provider, provider) {
				continue
			}
			at, parseErr := time.Parse(time.RFC3339Nano, event.ObservedAt)
			if parseErr != nil || !at.After(cutoff) {
				continue
			}
			if event.EventID != "" && seen[event.EventID] {
				continue
			}
			seen[event.EventID] = true
			switch strings.ToLower(event.HookEventName) {
			case "subagentstart":
				events["start"] = true
			case "subagentstop":
				events["stop"] = true
			default:
				events["activity"] = true
			}
			if event.ObservedAt > last {
				last = event.ObservedAt
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return events, last, scanErr
		}
		if closeErr != nil {
			return events, last, closeErr
		}
	}
	return events, last, nil
}
