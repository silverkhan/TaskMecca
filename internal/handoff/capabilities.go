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
	if stored.Evidence == nil {
		stored.Evidence = map[string]string{}
	}
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
	if !ok {
		return RuntimeCapabilities{}, fmt.Errorf("unknown capability: %s", name)
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
		_ = os.Remove(tmp)
		return RuntimeCapabilities{}, err
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
	case "supports_completion_hook":
		return &caps.SupportsCompletionHook, true
	default:
		return nil, false
	}
}
