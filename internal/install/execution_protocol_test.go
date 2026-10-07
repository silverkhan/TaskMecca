package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtocolOnlyMigrationRequestsInstructionRefresh(t *testing.T) {
	for _, language := range []string{"", ".en"} {
		t.Run(language, func(t *testing.T) {
			project := t.TempDir()
			if err := Init(project, "0.2.0"); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(project, targetName)
			name := "framework/EXECUTION_PROTOCOL" + language + ".md"
			previous := []byte("# Previous protocol\n")
			if err := os.WriteFile(filepath.Join(target, name), previous, 0644); err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(target, "manifest.json")
			raw, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var m manifest
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			entry := m.Managed[name]
			entry.BaselineSHA256 = hash(previous)
			m.Managed[name] = entry
			if err := saveManifest(target, m); err != nil {
				t.Fatal(err)
			}
			result, err := MigrateWithResult(project, "0.2.1")
			if err != nil {
				t.Fatal(err)
			}
			if !result.InstructionRefreshRequired || len(result.ChangedInstructions) != 1 || result.ChangedInstructions[0] != name {
				t.Fatalf("protocol-only update omitted refresh: %+v", result)
			}
		})
	}
}

// Check the delivered instructions, including the Python package, rather than
// just the source document: an omitted asset or stale role loses the procedure.
func TestDeliveredExecutionProtocol(t *testing.T) {
	project := t.TempDir()
	if err := Init(project, "0.2.1"); err != nil {
		t.Fatal(err)
	}
	files := []string{"EXECUTION_PROTOCOL.md", "EXECUTION_PROTOCOL.en.md", "SESSION_GUIDE.md", "SESSION_GUIDE.en.md", "collab.md", "roles/controller.md", "roles/worker.md", "roles/root.md", "roles/registrar.md"}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			installed, err := os.ReadFile(filepath.Join(project, targetName, "framework", name))
			if err != nil {
				t.Fatal(err)
			}
			python, err := os.ReadFile(filepath.Join("..", "..", "src", "task_mecca", "template", targetName, "framework", name))
			if err != nil {
				t.Fatal(err)
			}
			if string(installed) != string(python) {
				t.Fatalf("Go install and Python template differ: %s", name)
			}
			if !strings.HasPrefix(name, "EXECUTION_PROTOCOL") && !strings.Contains(string(installed), "EXECUTION_PROTOCOL") {
				t.Fatalf("role/guide lacks shared procedure: %s", name)
			}
			if strings.Contains(string(installed), "canonical backlog·원래 workspace·stale runtime을 쓰거나") {
				t.Fatal("ambiguous prohibition also forbids the canonical operational writer")
			}
			if strings.HasPrefix(name, "EXECUTION_PROTOCOL") {
				if policy("framework/"+name) != "customizable" || !sessionInstructionPath("framework/"+name) {
					t.Fatal("protocol must preserve customizations and trigger session refresh")
				}
			}
		})
	}
}

func TestProtocolCoversObservedRecoveryFailures(t *testing.T) {
	for _, language := range []string{"", ".en"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "goassets", "template", targetName, "framework", "EXECUTION_PROTOCOL"+language+".md"))
		if err != nil {
			t.Fatal(err)
		}
		doc := string(body)
		for _, boundary := range []string{"ephemeral probe/cache", "handoff-evidence/", "durable report"} {
			if !strings.Contains(doc, boundary) {
				t.Errorf("protocol%s omits preflight/failure retention boundary %q", language, boundary)
			}
		}
		// Both failure classes need an actionable recovery path. Queue projection
		// alone missed completed agents; Worker final alone did not deliver a turn.
		for _, evidence := range []string{"A-19 / A-20", "todo/unassigned", "recovery_queue", "collaboration.send_message", "collaboration.followup_task", "runtime bind-assignment", "lifecycle record assigned", "Root ACK/wake", "external-synced=unknown", "--step applied --result ok", "TM-WORKER-RESUME-20261007-1110", "run-c644efc406c0ceba", "01a11420-7e64-7b81-85ec-197e563a16a0"} {
			if !strings.Contains(doc, evidence) {
				t.Errorf("protocol%s omits recovery requirement %q", language, evidence)
			}
		}
		ordered := []string{"1. ", "preflight --require-full-access", "2. ", "inspect <ID>", "3. ", "worker-name --used", "4. ", "5. ", "runtime assign", "lifecycle record assigned", "6. ", "7. ", "runtime bind-assignment", "8. "}
		remainder := doc[strings.Index(doc, "## Assignment checklist"):]
		for _, step := range ordered {
			index := strings.Index(remainder, step)
			if index < 0 {
				t.Fatalf("protocol%s assignment order missing %q", language, step)
			}
			remainder = remainder[index+len(step):]
		}
	}
}
