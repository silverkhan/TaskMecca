package notify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func resumeFixture(t *testing.T) (string, time.Time) {
	t.Helper()
	project := t.TempDir()
	t.Setenv("TASK_MECCA_HOME", t.TempDir())
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	active := true
	cfg := TelegramConfig{Token: "fixture-not-real", ChatID: 1, Enabled: true, Kinds: defaultKinds(), ActivatedAt: time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339Nano), Delivered: []string{"historic-sent"}, TaskPhases: map[string]int{"OLD": 40}, ProjectEnabled: &active, RecipientMode: "shared"}
	if err := saveTelegram(project, cfg); err != nil {
		t.Fatal(err)
	}
	return project, time.Now().Add(-time.Minute).UTC()
}

func resumeFakeSender(t *testing.T, failures int) *atomic.Int32 {
	t.Helper()
	var sends atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := sends.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": int(count) > failures})
	}))
	old := telegramAPIBase
	telegramAPIBase = server.URL
	t.Cleanup(func() { telegramAPIBase = old; server.Close() })
	return &sends
}

func TestResumeSuppressesExistingAttemptsPreservesSentHistoryAndConfiguration(t *testing.T) {
	project, cutoff := resumeFixture(t)
	configBefore, err := os.ReadFile(telegramPath(project))
	if err != nil {
		t.Fatal(err)
	}
	ledger := deliveryLedger{Version: 1, Records: map[string]DeliveryRecord{}}
	for _, state := range []string{"pending", "failed", "sending", "uncertain", "unknown"} {
		ledger.Records[state] = DeliveryRecord{EventID: state, TaskID: state, Kind: "completed", EventAt: cutoff.Add(time.Hour).Format(time.RFC3339Nano), CreatedAt: "invalid", State: state, Attempts: 2, DuplicatePossible: true, LastError: "fixture-error", NextRetryAt: "fixture-retry", AttemptHistory: []DeliveryAttempt{{Number: 2, Outcome: "sending", StartedAt: "fixture-start"}}}
	}
	sent := DeliveryRecord{EventID: "historic-sent", TaskID: "OLD", Kind: "completed", State: "sent", Attempts: 1, EventAt: "old", CreatedAt: "original", AttemptHistory: []DeliveryAttempt{{Number: 1, Outcome: "sent", RespondedAt: "recorded"}}}
	ledger.Records[sent.EventID] = sent
	ledger.Records["consumed"] = DeliveryRecord{EventID: "consumed", State: "consumed_legacy"}
	if err := saveLedger(project, ledger); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(ledgerPath(project))
	result, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil)
	if err != nil || result.Suppressed != 5 || result.SentPreserved != 1 {
		t.Fatal(result, err)
	}
	after, err := readLedger(project)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Records[sent.EventID], sent) {
		t.Fatal("sent history changed")
	}
	for _, state := range []string{"pending", "failed", "sending", "uncertain", "unknown"} {
		r := after.Records[state]
		original := ledger.Records[state]
		if r.State != "suppressed_before_resume" || r.Suppression == nil || r.Suppression.PreviousState != state || r.Suppression.EvidenceID != result.Boundary.ID {
			t.Fatal("untruthful suppression", state)
		}
		if r.Attempts != original.Attempts || r.LastError != original.LastError || r.NextRetryAt != original.NextRetryAt || !reflect.DeepEqual(r.AttemptHistory, original.AttemptHistory) || !r.DuplicatePossible {
			t.Fatal("attempt evidence erased", state)
		}
	}
	backup, err := os.ReadFile(result.Boundary.BackupPath)
	if err != nil || !bytes.Equal(backup, before) {
		t.Fatal("backup is not exact", err)
	}
	digest := sha256.Sum256(before)
	if result.Boundary.BackupSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("backup evidence digest missing")
	}
	configAfter, _ := os.ReadFile(telegramPath(project))
	if !bytes.Equal(configBefore, configAfter) {
		t.Fatal("configuration changed")
	}
	if !TelegramTransportDisabled() {
		t.Fatal("command enabled gate")
	}
	committed, _ := os.ReadFile(ledgerPath(project))
	again, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil)
	if err != nil || !again.Unchanged || again.Suppressed != 0 {
		t.Fatal(again, err)
	}
	unchanged, _ := os.ReadFile(ledgerPath(project))
	if !bytes.Equal(committed, unchanged) {
		t.Fatal("idempotent cutoff changed evidence")
	}
}

func TestResumeLateOldInvalidAndNewEventsAcrossRestartAndRetry(t *testing.T) {
	project, cutoff := resumeFixture(t)
	sends := resumeFakeSender(t, 1)
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	old := Event{ID: "late-old", TaskID: "OLD-1", Kind: "completed", At: cutoff.Add(-time.Minute).Format(time.RFC3339Nano)}
	equal := Event{ID: "equal", TaskID: "OLD-2", Kind: "completed", At: cutoff.Format(time.RFC3339Nano)}
	invalid := Event{ID: "invalid", TaskID: "OLD-3", Kind: "completed", At: "invalid"}
	missing := Event{ID: "missing", TaskID: "OLD-4", Kind: "completed"}
	future := Event{ID: "future", TaskID: "OLD-5", Kind: "completed", At: time.Now().Add(time.Hour).Format(time.RFC3339Nano)}
	newEvent := Event{ID: "new", TaskID: "NEW-1", Kind: "completed", At: cutoff.Add(time.Second).Format(time.RFC3339Nano)}
	events := []Event{old, equal, invalid, missing, future, newEvent}
	if errs := Deliver(project, events); len(errs) != 1 {
		t.Fatal(errs)
	}
	if sends.Load() != 1 {
		t.Fatal("past notifications sent", sends.Load())
	}
	ledger, err := readLedger(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []Event{old, equal, invalid, missing, future} {
		if ledger.Records[e.ID].State != "suppressed_before_resume" {
			t.Fatal("late old/invalid event replayed", e.ID)
		}
	}
	if ledger.Records[newEvent.ID].State != "failed" {
		t.Fatal("new failure incorrectly consumed")
	}
	// Rerunning the exact cutover must not consume a legitimate new retry.
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "disabled")
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), events); err != nil {
		t.Fatal(err)
	}
	ledger, err = readLedger(project)
	if err != nil || ledger.Records[newEvent.ID].State != "failed" {
		t.Fatal("rerun suppressed post-cutoff retry", err)
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	forceRetryDue(t, project, newEvent.ID)
	if errs := Deliver(project, events); len(errs) != 0 {
		t.Fatal(errs)
	}
	if sends.Load() != 2 {
		t.Fatal("new retry missing or old replay", sends.Load())
	}
	if errs := Deliver(project, events); len(errs) != 0 {
		t.Fatal(errs)
	}
	if sends.Load() != 2 {
		t.Fatal("restart/dedup replay", sends.Load())
	}
	// A changed timestamp cannot resurrect the already-known old ID.
	old.At = newEvent.At
	if errs := Deliver(project, []Event{old}); len(errs) != 0 || sends.Load() != 2 {
		t.Fatal("old identity resurrected", errs)
	}
}

func TestResumeFailsClosedOnInvalidCutoffCorruptLedgerBackupAndWriteFailure(t *testing.T) {
	for _, scenario := range []string{"invalid", "future", "gate", "corrupt", "backup", "write", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			project, cutoff := resumeFixture(t)
			ledger := deliveryLedger{Version: 1, Records: map[string]DeliveryRecord{"old": {EventID: "old", State: "failed"}}}
			if err := saveLedger(project, ledger); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(ledgerPath(project))
			cfgBefore, _ := os.ReadFile(telegramPath(project))
			at := cutoff.Format(time.RFC3339Nano)
			switch scenario {
			case "invalid":
				at = "invalid"
			case "future":
				at = time.Now().Add(time.Hour).Format(time.RFC3339Nano)
			case "gate":
				t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
			case "corrupt":
				os.WriteFile(ledgerPath(project), []byte("{bad"), 0600)
				before = []byte("{bad")
			case "backup":
				os.WriteFile(filepath.Join(filepath.Dir(ledgerPath(project)), "resume-backups"), []byte("blocked"), 0600)
			case "write":
				if os.Geteuid() == 0 || runtime.GOOS == "windows" {
					t.Skip("root bypasses permissions")
				}
				if err := os.Chmod(filepath.Dir(ledgerPath(project)), 0500); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(filepath.Dir(ledgerPath(project)), 0700)
			case "symlink":
				outside := t.TempDir()
				path := filepath.Join(filepath.Dir(ledgerPath(project)), "resume-backups")
				if err := os.Symlink(outside, path); err != nil {
					t.Skip(err)
				}
			}
			if _, err := SuppressBeforeResume(project, at, nil); err == nil {
				t.Fatal("unsafe suppression succeeded")
			}
			after, _ := os.ReadFile(ledgerPath(project))
			cfgAfter, _ := os.ReadFile(telegramPath(project))
			if !bytes.Equal(before, after) || !bytes.Equal(cfgBefore, cfgAfter) {
				t.Fatal("failure changed ledger or configuration")
			}
		})
	}
	project, cutoff := resumeFixture(t)
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SuppressBeforeResume(project, cutoff.Add(-time.Second).Format(time.RFC3339Nano), nil); err == nil {
		t.Fatal("cutoff rewind accepted")
	}
}

func TestResumeLateMalformedAttemptEvidenceCannotBecomeNewRetry(t *testing.T) {
	project, cutoff := resumeFixture(t)
	sends := resumeFakeSender(t, 0)
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	ledger, err := readLedger(project)
	if err != nil {
		t.Fatal(err)
	}
	event := Event{ID: "unknown-late-attempt", TaskID: "UNKNOWN-NEW", Kind: "completed", At: cutoff.Add(time.Second).Format(time.RFC3339Nano)}
	ledger.Records[event.ID] = DeliveryRecord{EventID: event.ID, TaskID: event.TaskID, Kind: event.Kind, EventAt: event.At, State: "failed", Attempts: 1, AttemptHistory: []DeliveryAttempt{{Number: 1, Outcome: "failed_or_uncertain"}}}
	if err := saveLedger(project, ledger); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	if errs := Deliver(project, []Event{event}); len(errs) != 0 || sends.Load() != 0 {
		t.Fatal("malformed evidence replayed", errs)
	}
	ledger, err = readLedger(project)
	if err != nil || ledger.Records[event.ID].State != "suppressed_before_resume" || ledger.Records[event.ID].Suppression.Reason != "missing_delivery_evidence_time" {
		t.Fatal("missing evidence was not classified", err)
	}
}

func TestResumePreservesOpaqueDeliveryFactsWithoutDiagnosticLeak(t *testing.T) {
	project, cutoff := resumeFixture(t)
	raw := []byte(`{"version":1,"operator_extension":{"private_fact":"opaque-private"},"records":{"sent":{"event_id":"sent","state":"sent","attempts":1,"legacy_provider_fact":{"retained":true}},"pending":{"event_id":"pending","state":"pending","custom_evidence":["retained"]}}}`)
	if err := os.MkdirAll(filepath.Dir(ledgerPath(project)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath(project), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(ledgerPath(project))
	var before, after map[string]any
	if err := json.Unmarshal(raw, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before["operator_extension"], after["operator_extension"]) {
		t.Fatal("opaque root evidence lost")
	}
	priorRecords := before["records"].(map[string]any)
	newRecords := after["records"].(map[string]any)
	if !reflect.DeepEqual(priorRecords["sent"], newRecords["sent"]) {
		t.Fatal("opaque sent evidence lost")
	}
	if !reflect.DeepEqual(priorRecords["pending"].(map[string]any)["custom_evidence"], newRecords["pending"].(map[string]any)["custom_evidence"]) {
		t.Fatal("opaque pending evidence lost")
	}
	status, err := ResumeStatus(project)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, _ := json.Marshal(status)
	records, err := DeliveryRecords(project, "", "")
	if err != nil {
		t.Fatal(err)
	}
	projection, _ := json.Marshal(records)
	if bytes.Contains(diagnostic, []byte("opaque-private")) || bytes.Contains(projection, []byte("custom_evidence")) || bytes.Contains(projection, []byte("legacy_provider_fact")) {
		t.Fatal("opaque facts leaked through public projection")
	}
}

func TestResumeCommitFailureAfterVerifiedBackupLeavesOriginalBoundaryAndConfig(t *testing.T) {
	project, cutoff := resumeFixture(t)
	if err := saveLedger(project, deliveryLedger{Version: 1, Records: map[string]DeliveryRecord{"pending": {EventID: "pending", State: "pending"}}}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(ledgerPath(project))
	config, _ := os.ReadFile(telegramPath(project))
	original := persistResumeLedger
	persistResumeLedger = func(string, deliveryLedger) error { return errors.New("injected atomic commit failure") }
	defer func() { persistResumeLedger = original }()
	if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), nil); err == nil {
		t.Fatal("commit failure hidden")
	}
	after, _ := os.ReadFile(ledgerPath(project))
	configAfter, _ := os.ReadFile(telegramPath(project))
	if !bytes.Equal(before, after) || !bytes.Equal(config, configAfter) || !TelegramTransportDisabled() {
		t.Fatal("failed commit changed boundary/config/gate")
	}
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(ledgerPath(project)), "resume-backups", "delivery-before-resume-*.json"))
	if err != nil || len(backups) != 1 {
		t.Fatal("verified backup not retained", err)
	}
	saved, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(before, saved) {
		t.Fatal("backup changed on failed commit", err)
	}
}

func TestResumeProcessHelper(t *testing.T) {
	mode := os.Getenv("TASK_MECCA_A29_CHILD")
	if mode == "" {
		return
	}
	project, cutoff := os.Getenv("TASK_MECCA_A29_PROJECT"), os.Getenv("TASK_MECCA_A29_CUTOFF")
	if mode == "resume" {
		if _, err := SuppressBeforeResume(project, cutoff, nil); err != nil {
			t.Fatal(err)
		}
		return
	}
	telegramAPIBase = os.Getenv("TASK_MECCA_A29_SERVER")
	at, err := time.Parse(time.RFC3339Nano, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if errs := Deliver(project, []Event{{ID: "late-old-process", TaskID: "OLD-PROCESS", Kind: "completed", At: cutoff}, {ID: "new-process", TaskID: "NEW-PROCESS", Kind: "completed", At: at.Add(time.Second).Format(time.RFC3339Nano)}}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestResumeCrossProcessSnapshotRestartAndExactlyOnceNewDelivery(t *testing.T) {
	project, cutoff := resumeFixture(t)
	sends := resumeFakeSender(t, 0)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	startChildren := func(mode string, count int) {
		t.Helper()
		var commands []*exec.Cmd
		for i := 0; i < count; i++ {
			command := exec.Command(binary, "-test.run=^TestResumeProcessHelper$")
			gate := "disabled"
			if mode == "deliver" {
				gate = ""
			}
			command.Env = append(os.Environ(), "TASK_MECCA_A29_CHILD="+mode, "TASK_MECCA_A29_PROJECT="+project, "TASK_MECCA_A29_CUTOFF="+cutoff.Format(time.RFC3339Nano), "TASK_MECCA_A29_SERVER="+telegramAPIBase, "TASK_MECCA_TELEGRAM_TRANSPORT="+gate)
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			commands = append(commands, command)
		}
		for _, command := range commands {
			if err := command.Wait(); err != nil {
				t.Fatal("isolated child failed", err)
			}
		}
	}
	startChildren("resume", 4)
	startChildren("deliver", 8)
	startChildren("deliver", 2)
	if sends.Load() != 1 {
		t.Fatal("old replay or new duplicate", sends.Load())
	}
	ledger, err := readLedger(project)
	if err != nil || ledger.Records["late-old-process"].State != "suppressed_before_resume" || ledger.Records["new-process"].State != "sent" || ledger.Records["new-process"].Attempts != 1 {
		t.Fatal("cross-process outcomes lost", err)
	}
}

func TestResumeConcurrentCutoversMonotonicAndMalformedBoundaryBlocksSend(t *testing.T) {
	project, cutoff := resumeFixture(t)
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := SuppressBeforeResume(project, cutoff.Format(time.RFC3339Nano), []Event{{ID: "old", At: cutoff.Format(time.RFC3339Nano)}}); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	ledger, err := readLedger(project)
	if err != nil || len(ledger.Records) != 1 || ledger.Records["old"].State != "suppressed_before_resume" {
		t.Fatal("concurrent cutovers lost boundary", err)
	}
	if _, err := SuppressBeforeResume(project, cutoff.Add(time.Second).Format(time.RFC3339Nano), nil); err != nil {
		t.Fatal(err)
	}
	ledger, err = readLedger(project)
	if err != nil || len(ledger.ResumeHistory) != 1 {
		t.Fatal("cutover audit history lost", err)
	}
	sends := resumeFakeSender(t, 0)
	ledger.ResumeBoundary.CutoffAt = "invalid"
	if err := saveLedger(project, ledger); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TASK_MECCA_TELEGRAM_TRANSPORT", "")
	if errs := Deliver(project, []Event{{ID: "new", TaskID: "NEW", Kind: "completed", At: time.Now().UTC().Format(time.RFC3339Nano)}}); len(errs) == 0 || sends.Load() != 0 {
		t.Fatal("malformed boundary allowed network", errs)
	}
}
