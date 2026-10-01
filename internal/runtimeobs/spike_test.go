package runtimeobs

import (
    "bytes"
    "os"
    "strings"
    "testing"
    "time"
)

func TestObserveNormalizesCodexSubagentStartWithoutPersistingToolInput(t *testing.T) {
    project := t.TempDir()
    raw := []byte(`{
      "session_id":"sess-1",
      "turn_id":"turn-1",
      "hook_event_name":"SubagentStart",
      "agent_id":"agent-1",
      "agent_type":"worker",
      "cwd":"/repo",
      "tool_input":{"secret":"do-not-persist"}
    }`)
    observed := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
    event, err := Observe(project, "codex", bytes.NewReader(raw), observed)
    if err != nil {
        t.Fatal(err)
    }
    if event.Provider != "codex" || event.HookEventName != "SubagentStart" || event.AgentID != "agent-1" {
        t.Fatalf("unexpected event: %+v", event)
    }
    data, err := os.ReadFile(JournalPath(project))
    if err != nil {
        t.Fatal(err)
    }
    if strings.Contains(string(data), "do-not-persist") {
        t.Fatalf("sensitive nested hook payload should not be persisted: %s", data)
    }
    if !strings.Contains(string(data), "tool_input") {
        t.Fatalf("raw field names should remain available for schema inspection: %s", data)
    }
}

func TestReportTracksLifecycleAndAttributedActivity(t *testing.T) {
    project := t.TempDir()
    base := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
    samples := []string{
        `{"session_id":"s","hook_event_name":"SubagentStart","agent_id":"a","agent_type":"general-purpose"}`,
        `{"session_id":"s","hook_event_name":"PreToolUse","agent_id":"a","agent_type":"general-purpose","tool_name":"Bash","tool_use_id":"t1"}`,
        `{"session_id":"s","hook_event_name":"PostToolUse","agent_id":"a","agent_type":"general-purpose","tool_name":"Bash","tool_use_id":"t1"}`,
        `{"session_id":"s","hook_event_name":"SubagentStop","agent_id":"a","agent_type":"general-purpose"}`,
    }
    for i, sample := range samples {
        if _, err := Observe(project, "claude", strings.NewReader(sample), base.Add(time.Duration(i)*time.Second)); err != nil {
            t.Fatal(err)
        }
    }
    report, err := Report(project, 10)
    if err != nil {
        t.Fatal(err)
    }
    if report.Events != 4 || len(report.Agents) != 1 {
        t.Fatalf("unexpected report: %+v", report)
    }
    agent := report.Agents[0]
    if !agent.SawStart || !agent.SawStop || agent.ActivityCount != 2 {
        t.Fatalf("unexpected agent summary: %+v", agent)
    }
    if len(report.Providers) != 1 || report.Providers[0].AttributedToolActivity != 2 {
        t.Fatalf("unexpected provider summary: %+v", report.Providers)
    }
    for _, finding := range report.Findings {
        if finding.Code == "start_without_stop" || finding.Code == "stop_without_start" {
            t.Fatalf("complete lifecycle should not report gap: %+v", report.Findings)
        }
    }
}

func TestReportDoesNotTreatMissingStopAsDeath(t *testing.T) {
    project := t.TempDir()
    if _, err := Observe(project, "codex", strings.NewReader(`{"hook_event_name":"SubagentStart","agent_id":"a"}`), time.Now()); err != nil {
        t.Fatal(err)
    }
    report, err := Report(project, 10)
    if err != nil {
        t.Fatal(err)
    }
    found := false
    for _, finding := range report.Findings {
        if finding.Code == "start_without_stop" {
            found = true
            if finding.Severity != "info" {
                t.Fatalf("missing stop must not be classified as failure: %+v", finding)
            }
        }
    }
    if !found {
        t.Fatalf("expected start_without_stop finding: %+v", report.Findings)
    }
}

func TestObserveRejectsUnknownProvider(t *testing.T) {
    _, err := Observe(t.TempDir(), "other", strings.NewReader(`{"hook_event_name":"SubagentStart"}`), time.Now())
    if err == nil {
        t.Fatal("expected provider validation error")
    }
}


func TestResolveProjectWalksUpFromSubdirectory(t *testing.T) {
    project := t.TempDir()
    if err := os.MkdirAll(project+"/_task_mecca", 0o755); err != nil {
        t.Fatal(err)
    }
    nested := project+"/a/b/c"
    if err := os.MkdirAll(nested, 0o755); err != nil {
        t.Fatal(err)
    }
    resolved, err := ResolveProject(nested)
    if err != nil {
        t.Fatal(err)
    }
    if resolved != project {
        t.Fatalf("resolved=%q want=%q", resolved, project)
    }
}
