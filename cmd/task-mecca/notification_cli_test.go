package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNotificationResumeCLIRequiresBlockedTransportExactPathAndValidSnapshot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	path := filepath.Join(root, "_task_mecca", ".runtime", "notification_events.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	journal := []byte(`{"events":[{"id":"old-cli","task_id":"OLD","kind":"completed","at":"invalid"}]}`)
	if err := os.WriteFile(path, journal, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"suppress-before-resume", "--project", root, "--confirm-project", root, "--cutoff", cutoff, "--json"}
	var out, stderr bytes.Buffer
	if code := runNotificationCLI(args, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["suppressed"] != float64(1) {
		t.Fatal("invalid CLI projection", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, journal) {
		t.Fatal("CLI mutated event journal")
	}
	out.Reset()
	stderr.Reset()
	if code := runNotificationCLI([]string{"resume-status", "--project", root}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if bytes.Contains(out.Bytes(), []byte("token")) || bytes.Contains(out.Bytes(), []byte("chat_id")) {
		t.Fatal("status leaked recipient fields")
	}
	ledger := filepath.Join(filepath.Dir(path), "notifications", "delivery_ledger.json")
	before, _ := os.ReadFile(ledger)
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	if code := runNotificationCLI(args, &out, &stderr); code == 0 {
		t.Fatal("unblocked mutation accepted")
	}
	after, _ = os.ReadFile(ledger)
	if !bytes.Equal(before, after) {
		t.Fatal("unblocked command mutated evidence")
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	args[4] = "wrong"
	if code := runNotificationCLI(args, &out, &stderr); code == 0 {
		t.Fatal("confirmation mismatch accepted")
	}
}

func TestNotificationResumeCLICorruptJournalFailsBeforeBoundaryWrites(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	path := filepath.Join(root, "_task_mecca", ".runtime", "notification_events.json")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("corrupt"), 0600)
	var out, stderr bytes.Buffer
	if code := runNotificationCLI([]string{"suppress-before-resume", "--project", root, "--confirm-project", root, "--cutoff", time.Now().UTC().Format(time.RFC3339Nano)}, &out, &stderr); code == 0 {
		t.Fatal("corrupt snapshot ignored")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "notifications", "delivery_ledger.json")); !os.IsNotExist(err) {
		t.Fatal("corrupt snapshot changed ledger", err)
	}
}
