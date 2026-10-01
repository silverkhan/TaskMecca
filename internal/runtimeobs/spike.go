package runtimeobs

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "time"
)

const (
    spikeDirName  = "observability-spike"
    spikeFileName = "events.jsonl"
)

type SpikeEvent struct {
    Provider            string         `json:"provider"`
    ObservedAt          string         `json:"observed_at"`
    HookEventName       string         `json:"hook_event_name"`
    SessionID           string         `json:"session_id,omitempty"`
    TurnID              string         `json:"turn_id,omitempty"`
    AgentID             string         `json:"agent_id,omitempty"`
    AgentType           string         `json:"agent_type,omitempty"`
    ToolName            string         `json:"tool_name,omitempty"`
    ToolUseID           string         `json:"tool_use_id,omitempty"`
    PermissionMode      string         `json:"permission_mode,omitempty"`
    Reason              string         `json:"reason,omitempty"`
    StopHookActive      *bool          `json:"stop_hook_active,omitempty"`
    CWD                 string         `json:"cwd,omitempty"`
    Model               string         `json:"model,omitempty"`
    TranscriptPath      string         `json:"transcript_path,omitempty"`
    AgentTranscriptPath string         `json:"agent_transcript_path,omitempty"`
    RawBytes            int            `json:"raw_bytes"`
    RawSHA256           string         `json:"raw_sha256"`
    RawFields           []string       `json:"raw_fields,omitempty"`
    Extra               map[string]any `json:"extra,omitempty"`
}

type SpikeFinding struct {
    Severity string `json:"severity"`
    Code     string `json:"code"`
    Provider string `json:"provider,omitempty"`
    AgentID  string `json:"agent_id,omitempty"`
    Message  string `json:"message"`
}

type AgentSummary struct {
    Provider       string   `json:"provider"`
    AgentID        string   `json:"agent_id"`
    AgentTypes     []string `json:"agent_types,omitempty"`
    SessionIDs     []string `json:"session_ids,omitempty"`
    TurnIDs        []string `json:"turn_ids,omitempty"`
    FirstObserved  string   `json:"first_observed"`
    LastObserved   string   `json:"last_observed"`
    EventCount     int      `json:"event_count"`
    ActivityCount  int      `json:"activity_count"`
    SawStart       bool     `json:"saw_start"`
    SawStop        bool     `json:"saw_stop"`
    EventNames     []string `json:"event_names"`
}

type ProviderSummary struct {
    Provider                 string `json:"provider"`
    Events                   int    `json:"events"`
    Agents                   int    `json:"agents"`
    Starts                   int    `json:"starts"`
    Stops                    int    `json:"stops"`
    ToolActivityEvents       int    `json:"tool_activity_events"`
    AttributedToolActivity   int    `json:"attributed_tool_activity"`
    UnattributedToolActivity int    `json:"unattributed_tool_activity"`
}

type SpikeReport struct {
    JournalPath string            `json:"journal_path"`
    Exists      bool              `json:"exists"`
    Events      int               `json:"events"`
    Providers   []ProviderSummary `json:"providers"`
    Agents      []AgentSummary    `json:"agents"`
    Findings    []SpikeFinding    `json:"findings"`
    Recent      []SpikeEvent      `json:"recent"`
}

func JournalPath(project string) string {
    return filepath.Join(project, "_task_mecca", ".runtime", spikeDirName, spikeFileName)
}

func Observe(project, provider string, input io.Reader, now time.Time) (SpikeEvent, error) {
    provider = strings.ToLower(strings.TrimSpace(provider))
    if provider != "codex" && provider != "claude" {
        return SpikeEvent{}, fmt.Errorf("unsupported provider %q (use codex or claude)", provider)
    }
    raw, err := io.ReadAll(io.LimitReader(input, 4<<20))
    if err != nil {
        return SpikeEvent{}, err
    }
    if len(strings.TrimSpace(string(raw))) == 0 {
        return SpikeEvent{}, errors.New("hook input is empty")
    }
    var payload map[string]any
    if err := json.Unmarshal(raw, &payload); err != nil {
        return SpikeEvent{}, fmt.Errorf("invalid hook JSON: %w", err)
    }
    event := normalizeEvent(provider, payload, raw, now)
    path := JournalPath(project)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
        return SpikeEvent{}, err
    }
    line, err := json.Marshal(event)
    if err != nil {
        return SpikeEvent{}, err
    }
    line = append(line, '\n')
    f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
    if err != nil {
        return SpikeEvent{}, err
    }
    _, writeErr := f.Write(line)
    closeErr := f.Close()
    if writeErr != nil {
        return SpikeEvent{}, writeErr
    }
    if closeErr != nil {
        return SpikeEvent{}, closeErr
    }
    return event, nil
}

func normalizeEvent(provider string, payload map[string]any, raw []byte, now time.Time) SpikeEvent {
    hash := sha256.Sum256(raw)
    fields := make([]string, 0, len(payload))
    for key := range payload {
        fields = append(fields, key)
    }
    sort.Strings(fields)
    eventName := firstString(payload, "hook_event_name", "hookEventName", "event_name", "event")
    event := SpikeEvent{
        Provider:            provider,
        ObservedAt:          now.UTC().Format(time.RFC3339Nano),
        HookEventName:       eventName,
        SessionID:           firstString(payload, "session_id", "sessionId"),
        TurnID:              firstString(payload, "turn_id", "turnId"),
        AgentID:             firstString(payload, "agent_id", "agentId"),
        AgentType:           firstString(payload, "agent_type", "agentType"),
        ToolName:            firstString(payload, "tool_name", "toolName"),
        ToolUseID:           firstString(payload, "tool_use_id", "toolUseId"),
        PermissionMode:      firstString(payload, "permission_mode", "permissionMode"),
        Reason:              firstString(payload, "reason"),
        CWD:                 firstString(payload, "cwd"),
        Model:               firstString(payload, "model"),
        TranscriptPath:      firstString(payload, "transcript_path", "transcriptPath"),
        AgentTranscriptPath: firstString(payload, "agent_transcript_path", "agentTranscriptPath"),
        RawBytes:            len(raw),
        RawSHA256:           hex.EncodeToString(hash[:]),
        RawFields:           fields,
    }
    if value, ok := firstBool(payload, "stop_hook_active", "stopHookActive"); ok {
        event.StopHookActive = &value
    }
    extra := map[string]any{}
    for _, key := range []string{"source", "trigger", "notification_type", "error_type"} {
        if value, ok := payload[key]; ok && isScalar(value) {
            extra[key] = value
        }
    }
    if len(extra) > 0 {
        event.Extra = extra
    }
    return event
}

func Report(project string, recentLimit int) (SpikeReport, error) {
    path := JournalPath(project)
    report := SpikeReport{JournalPath: path, Recent: []SpikeEvent{}, Providers: []ProviderSummary{}, Agents: []AgentSummary{}, Findings: []SpikeFinding{}}
    f, err := os.Open(path)
    if errors.Is(err, os.ErrNotExist) {
        return report, nil
    }
    if err != nil {
        return report, err
    }
    defer f.Close()
    report.Exists = true

    events := []SpikeEvent{}
    scanner := bufio.NewScanner(f)
    scanner.Buffer(make([]byte, 64*1024), 1024*1024)
    line := 0
    for scanner.Scan() {
        line++
        text := strings.TrimSpace(scanner.Text())
        if text == "" {
            continue
        }
        var event SpikeEvent
        if err := json.Unmarshal([]byte(text), &event); err != nil {
            report.Findings = append(report.Findings, SpikeFinding{
                Severity: "warning",
                Code:     "invalid_journal_line",
                Message:  fmt.Sprintf("journal line %d is invalid JSON: %v", line, err),
            })
            continue
        }
        events = append(events, event)
    }
    if err := scanner.Err(); err != nil {
        return report, err
    }
    report.Events = len(events)
    if recentLimit <= 0 {
        recentLimit = 10
    }
    if len(events) > recentLimit {
        report.Recent = append(report.Recent, events[len(events)-recentLimit:]...)
    } else {
        report.Recent = append(report.Recent, events...)
    }

    type mutableAgent struct {
        summary   AgentSummary
        types     map[string]struct{}
        sessions  map[string]struct{}
        turns     map[string]struct{}
        eventNames map[string]struct{}
    }
    agents := map[string]*mutableAgent{}
    providers := map[string]*ProviderSummary{}
    startSeen := map[string]bool{}

    for _, event := range events {
        ps := providers[event.Provider]
        if ps == nil {
            ps = &ProviderSummary{Provider: event.Provider}
            providers[event.Provider] = ps
        }
        ps.Events++
        isStart := strings.EqualFold(event.HookEventName, "SubagentStart")
        isStop := strings.EqualFold(event.HookEventName, "SubagentStop")
        isTool := strings.EqualFold(event.HookEventName, "PreToolUse") || strings.EqualFold(event.HookEventName, "PostToolUse") || strings.EqualFold(event.HookEventName, "PostToolUseFailure")
        if isStart {
            ps.Starts++
        }
        if isStop {
            ps.Stops++
        }
        if isTool {
            ps.ToolActivityEvents++
            if event.AgentID != "" {
                ps.AttributedToolActivity++
            } else {
                ps.UnattributedToolActivity++
            }
        }
        if (isStart || isStop) && event.AgentID == "" {
            report.Findings = append(report.Findings, SpikeFinding{
                Severity: "error",
                Code: "subagent_event_missing_agent_id",
                Provider: event.Provider,
                Message: event.HookEventName + " did not include agent_id",
            })
        }
        if event.AgentID == "" {
            continue
        }
        key := event.Provider + "\x00" + event.AgentID
        a := agents[key]
        if a == nil {
            a = &mutableAgent{
                summary: AgentSummary{
                    Provider: event.Provider,
                    AgentID: event.AgentID,
                    FirstObserved: event.ObservedAt,
                    LastObserved: event.ObservedAt,
                },
                types: map[string]struct{}{},
                sessions: map[string]struct{}{},
                turns: map[string]struct{}{},
                eventNames: map[string]struct{}{},
            }
            agents[key] = a
        }
        a.summary.EventCount++
        if event.ObservedAt < a.summary.FirstObserved || a.summary.FirstObserved == "" {
            a.summary.FirstObserved = event.ObservedAt
        }
        if event.ObservedAt > a.summary.LastObserved {
            a.summary.LastObserved = event.ObservedAt
        }
        if event.AgentType != "" {
            a.types[event.AgentType] = struct{}{}
        }
        if event.SessionID != "" {
            a.sessions[event.SessionID] = struct{}{}
        }
        if event.TurnID != "" {
            a.turns[event.TurnID] = struct{}{}
        }
        if event.HookEventName != "" {
            a.eventNames[event.HookEventName] = struct{}{}
        }
        if isStart {
            a.summary.SawStart = true
            startSeen[key] = true
        }
        if isStop {
            a.summary.SawStop = true
            if !startSeen[key] {
                report.Findings = append(report.Findings, SpikeFinding{
                    Severity: "warning",
                    Code: "stop_without_start",
                    Provider: event.Provider,
                    AgentID: event.AgentID,
                    Message: "SubagentStop was observed without an earlier SubagentStart in this journal",
                })
            }
        }
        if isTool {
            a.summary.ActivityCount++
            if !startSeen[key] {
                report.Findings = append(report.Findings, SpikeFinding{
                    Severity: "info",
                    Code: "activity_without_start",
                    Provider: event.Provider,
                    AgentID: event.AgentID,
                    Message: "tool activity was observed before a SubagentStart in this journal",
                })
            }
        }
    }

    for _, ps := range providers {
        count := 0
        for _, a := range agents {
            if a.summary.Provider == ps.Provider {
                count++
            }
        }
        ps.Agents = count
        report.Providers = append(report.Providers, *ps)
    }
    sort.Slice(report.Providers, func(i, j int) bool { return report.Providers[i].Provider < report.Providers[j].Provider })

    for _, a := range agents {
        a.summary.AgentTypes = sortedKeys(a.types)
        a.summary.SessionIDs = sortedKeys(a.sessions)
        a.summary.TurnIDs = sortedKeys(a.turns)
        a.summary.EventNames = sortedKeys(a.eventNames)
        if a.summary.SawStart && !a.summary.SawStop {
            report.Findings = append(report.Findings, SpikeFinding{
                Severity: "info",
                Code: "start_without_stop",
                Provider: a.summary.Provider,
                AgentID: a.summary.AgentID,
                Message: "SubagentStart is present but no SubagentStop has been observed yet; this is not proof of failure",
            })
        }
        if len(a.summary.AgentTypes) > 1 {
            report.Findings = append(report.Findings, SpikeFinding{
                Severity: "warning",
                Code: "agent_type_changed",
                Provider: a.summary.Provider,
                AgentID: a.summary.AgentID,
                Message: "the same agent_id was observed with multiple agent_type values",
            })
        }
        if len(a.summary.SessionIDs) > 1 {
            report.Findings = append(report.Findings, SpikeFinding{
                Severity: "warning",
                Code: "agent_id_multiple_sessions",
                Provider: a.summary.Provider,
                AgentID: a.summary.AgentID,
                Message: "the same agent_id was observed under multiple session_id values",
            })
        }
        report.Agents = append(report.Agents, a.summary)
    }
    sort.Slice(report.Agents, func(i, j int) bool {
        if report.Agents[i].Provider != report.Agents[j].Provider {
            return report.Agents[i].Provider < report.Agents[j].Provider
        }
        return report.Agents[i].AgentID < report.Agents[j].AgentID
    })
    sort.SliceStable(report.Findings, func(i, j int) bool {
        rank := func(s string) int {
            switch s {
            case "error":
                return 0
            case "warning":
                return 1
            default:
                return 2
            }
        }
        ri, rj := rank(report.Findings[i].Severity), rank(report.Findings[j].Severity)
        if ri != rj {
            return ri < rj
        }
        if report.Findings[i].Provider != report.Findings[j].Provider {
            return report.Findings[i].Provider < report.Findings[j].Provider
        }
        return report.Findings[i].Code < report.Findings[j].Code
    })
    return report, nil
}

func FilterReport(report SpikeReport, provider string) SpikeReport {
    provider = strings.ToLower(strings.TrimSpace(provider))
    if provider == "" {
        return report
    }
    filtered := report
    filtered.Providers = nil
    filtered.Agents = nil
    filtered.Findings = nil
    filtered.Recent = nil
    filtered.Events = 0
    for _, row := range report.Providers {
        if row.Provider == provider {
            filtered.Providers = append(filtered.Providers, row)
            filtered.Events += row.Events
        }
    }
    for _, row := range report.Agents {
        if row.Provider == provider {
            filtered.Agents = append(filtered.Agents, row)
        }
    }
    for _, row := range report.Findings {
        if row.Provider == "" || row.Provider == provider {
            filtered.Findings = append(filtered.Findings, row)
        }
    }
    for _, row := range report.Recent {
        if row.Provider == provider {
            filtered.Recent = append(filtered.Recent, row)
        }
    }
    return filtered
}

func firstString(payload map[string]any, keys ...string) string {
    for _, key := range keys {
        if value, ok := payload[key]; ok {
            if text, ok := value.(string); ok {
                return strings.TrimSpace(text)
            }
        }
    }
    return ""
}

func firstBool(payload map[string]any, keys ...string) (bool, bool) {
    for _, key := range keys {
        if value, ok := payload[key]; ok {
            b, ok := value.(bool)
            return b, ok
        }
    }
    return false, false
}

func isScalar(value any) bool {
    switch value.(type) {
    case string, float64, bool, nil:
        return true
    default:
        return false
    }
}

func sortedKeys(values map[string]struct{}) []string {
    out := make([]string, 0, len(values))
    for value := range values {
        out = append(out, value)
    }
    sort.Strings(out)
    return out
}
