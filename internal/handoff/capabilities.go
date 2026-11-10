package handoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CapabilityState string

const (
	CapabilitySupported   CapabilityState = "supported"
	CapabilityUnsupported CapabilityState = "unsupported"
	CapabilityUnknown     CapabilityState = "unknown"
)

type RuntimeCapabilities struct {
	Provider                 string            `json:"provider"`
	CanMessageRunning        CapabilityState   `json:"can_message_running_agent"`
	CanResumeCompleted       CapabilityState   `json:"can_resume_completed_agent"`
	CanResumeRoot            CapabilityState   `json:"can_resume_root"`
	CanDiscoverAgents        CapabilityState   `json:"can_discover_agents"`
	CanCrossSessionMessage   CapabilityState   `json:"can_cross_session_message"`
	CanSpawnSubagent       CapabilityState   `json:"can_spawn_subagent"`
	CanCreateSession       CapabilityState   `json:"can_create_session"`
	CanMessageRoot         CapabilityState   `json:"can_message_root"`
	CanWakeRoot            CapabilityState   `json:"can_wake_root"`
	CanResumeSession       CapabilityState   `json:"can_resume_session"`
	SupportsCompletionHook   CapabilityState   `json:"supports_completion_hook"`
	IdentityPersistenceScope string            `json:"identity_persistence_scope"`
	Source                   string            `json:"source"`
	UpdatedAt                string            `json:"updated_at,omitempty"`
	Evidence                 map[string]string `json:"evidence,omitempty"`
}

func CapabilityPath(project, provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return filepath.Join(project, "_task_mecca", ".runtime", "handoffs", "capabilities", provider+".json")
}

func defaultCapabilities(provider string) RuntimeCapabilities {
	provider = strings.ToLower(strings.TrimSpace(provider))
	out := RuntimeCapabilities{
		Provider:                 provider,
		CanMessageRunning:        CapabilityUnknown,
		CanResumeCompleted:       CapabilityUnknown,
		CanResumeRoot:            CapabilityUnknown,
		CanDiscoverAgents:        CapabilityUnknown,
		CanCrossSessionMessage:   CapabilityUnknown,
		CanSpawnSubagent:       CapabilityUnknown,
		CanCreateSession:       CapabilityUnknown,
		CanMessageRoot:         CapabilityUnknown,
		CanWakeRoot:            CapabilityUnknown,
		CanResumeSession:       CapabilityUnknown,
		SupportsCompletionHook:   CapabilityUnknown,
		IdentityPersistenceScope: "unknown",
		Source:                   "builtin_baseline",
	}
	switch provider {
	case "codex":
		out.CanMessageRunning = CapabilitySupported
		out.CanResumeCompleted = CapabilitySupported
		out.CanResumeRoot = CapabilityUnsupported
		out.SupportsCompletionHook = CapabilitySupported
		out.IdentityPersistenceScope = "session"
	case "claude":
		out.CanMessageRunning = CapabilitySupported
		out.CanResumeCompleted = CapabilitySupported
		out.CanResumeRoot = CapabilityUnknown
		out.CanCrossSessionMessage = CapabilitySupported
		out.SupportsCompletionHook = CapabilitySupported
		out.IdentityPersistenceScope = "session"
	}
	return out
}

func LoadCapabilities(project, provider string) (RuntimeCapabilities, error) {
	baseline := defaultCapabilities(provider)
	if baseline.Provider == "" {
		return baseline, nil
	}
	payload, err := os.ReadFile(CapabilityPath(project, baseline.Provider))
	if errors.Is(err, os.ErrNotExist) {
		return baseline, nil
	}
	if err != nil {
		return RuntimeCapabilities{}, err
	}
	var stored RuntimeCapabilities
	if err := json.Unmarshal(payload, &stored); err != nil {
		return RuntimeCapabilities{}, fmt.Errorf("invalid capability evidence for %s: %w", baseline.Provider, err)
	}
	if strings.ToLower(strings.TrimSpace(stored.Provider)) != baseline.Provider {
		return RuntimeCapabilities{}, fmt.Errorf("capability evidence provider mismatch: %q", stored.Provider)
	}
	// Older runtime capability snapshots predate the execution-mode fields.
	// Missing means unknown, never implied support.
	for _, field := range []*CapabilityState{&stored.CanSpawnSubagent, &stored.CanCreateSession, &stored.CanMessageRoot, &stored.CanWakeRoot, &stored.CanResumeSession} {
		if *field == "" { *field = CapabilityUnknown }
	}
	if stored.Evidence == nil { stored.Evidence = map[string]string{} }
	return stored, nil
}

func RecordCapability(project, provider, name string, state CapabilityState, evidence string, now time.Time) (RuntimeCapabilities, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	name = strings.ToLower(strings.TrimSpace(name))
	if provider == "" {
		return RuntimeCapabilities{}, errors.New("provider is required")
	}
	if state != CapabilitySupported && state != CapabilityUnsupported && state != CapabilityUnknown {
		return RuntimeCapabilities{}, errors.New("capability state must be supported, unsupported, or unknown")
	}
	caps, err := LoadCapabilities(project, provider)
	if err != nil {
		return RuntimeCapabilities{}, err
	}
	field, ok := capabilityField(&caps, name)
	if !ok { return RuntimeCapabilities{}, fmt.Errorf("unknown capability: %s", name) }
	// Root delivery and native session creation are explicitly verified
	// runtime features, never guessed from a provider name.
	if state == CapabilitySupported && (name == "can_spawn_subagent" || name == "can_create_session" || name == "can_message_root" || name == "can_wake_root" || name == "can_resume_session") && strings.TrimSpace(evidence) == "" {
		return RuntimeCapabilities{}, errors.New("supported orchestration capability requires --evidence")
	}
	*field = state
	caps.Source = "runtime_evidence"
	caps.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	if caps.Evidence == nil {
		caps.Evidence = map[string]string{}
	}
	if detail := strings.TrimSpace(evidence); detail != "" {
		caps.Evidence[name] = detail
	}
	path := CapabilityPath(project, provider)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return RuntimeCapabilities{}, err
	}
	payload, err := json.MarshalIndent(caps, "", "  ")
	if err != nil {
		return RuntimeCapabilities{}, err
	}
	payload = append(payload, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return RuntimeCapabilities{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows does not replace an existing destination with os.Rename.
		// Capability evidence is ephemeral, so fall back to remove + rename.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			_ = os.Remove(tmp)
			return RuntimeCapabilities{}, err
		}
		if retryErr := os.Rename(tmp, path); retryErr != nil {
			_ = os.Remove(tmp)
			return RuntimeCapabilities{}, retryErr
		}
	}
	return caps, nil
}

func capabilityField(caps *RuntimeCapabilities, name string) (*CapabilityState, bool) {
	switch name {
	case "can_message_running_agent":
		return &caps.CanMessageRunning, true
	case "can_resume_completed_agent":
		return &caps.CanResumeCompleted, true
	case "can_resume_root":
		return &caps.CanResumeRoot, true
	case "can_discover_agents":
		return &caps.CanDiscoverAgents, true
	case "can_cross_session_message":
		return &caps.CanCrossSessionMessage, true
	case "can_spawn_subagent":
		return &caps.CanSpawnSubagent, true
	case "can_create_session":
		return &caps.CanCreateSession, true
	case "can_message_root":
		return &caps.CanMessageRoot, true
	case "can_wake_root":
		return &caps.CanWakeRoot, true
	case "can_resume_session":
		return &caps.CanResumeSession, true
	case "supports_completion_hook":
		return &caps.SupportsCompletionHook, true
	default:
		return nil, false
	}
}
