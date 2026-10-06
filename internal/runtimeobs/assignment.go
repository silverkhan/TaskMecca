package runtimeobs

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Assignment is the durable Controller dispatch boundary. It is deliberately
// separate from an execution attempt: assigning a worker does not prove start.
type Assignment struct {
	AssignmentID string `json:"assignment_id"`
	TaskID       string `json:"task_id"`
	AgentPath    string `json:"agent_path"`
	AssignedAt   string `json:"assigned_at"`
}

func assignmentJournalPath(project string) string {
	return filepath.Join(project, "_task_mecca", "data", "assignments", "events.jsonl")
}

func ListAssignments(project string) ([]Assignment, error) {
	f, err := os.Open(assignmentJournalPath(project))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []Assignment{}
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		var item Assignment
		if err := json.Unmarshal(scan.Bytes(), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, scan.Err()
}

// RecordAssignment accepts a caller-supplied ID for retry-safe dispatch. When
// omitted, it generates one and returns it to the Controller before spawning.
func RecordAssignment(project, assignmentID, taskID, agentPath string, now time.Time) (Assignment, error) {
	taskID = strings.ToUpper(strings.TrimSpace(taskID))
	agentPath = strings.TrimSpace(agentPath)
	assignmentID = strings.TrimSpace(assignmentID)
	if taskID == "" || agentPath == "" {
		return Assignment{}, errors.New("task ID and agent path are required")
	}
	existing, err := ListAssignments(project)
	if err != nil {
		return Assignment{}, err
	}
	if assignmentID != "" {
		for _, item := range existing {
			if item.AssignmentID != assignmentID {
				continue
			}
			if item.TaskID != taskID || item.AgentPath != agentPath {
				return Assignment{}, fmt.Errorf("assignment %s conflicts with existing task or agent", assignmentID)
			}
			return item, nil
		}
	} else {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return Assignment{}, err
		}
		assignmentID = "assignment-" + hex.EncodeToString(random)
	}
	item := Assignment{AssignmentID: assignmentID, TaskID: taskID, AgentPath: agentPath, AssignedAt: now.UTC().Format(time.RFC3339Nano)}
	path := assignmentJournalPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Assignment{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return Assignment{}, err
	}
	defer f.Close()
	data, err := json.Marshal(item)
	if err != nil {
		return Assignment{}, err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return Assignment{}, err
	}
	return item, nil
}

func FindAssignment(project, assignmentID string) (Assignment, error) {
	items, err := ListAssignments(project)
	if err != nil {
		return Assignment{}, err
	}
	for _, item := range items {
		if item.AssignmentID == assignmentID {
			return item, nil
		}
	}
	return Assignment{}, fmt.Errorf("assignment not found: %s", assignmentID)
}
