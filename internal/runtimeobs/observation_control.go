package runtimeobs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ObservationControl is device-wide and independent of project Hook files.
// A retained project Hook may still invoke the CLI, but cannot record events
// while its provider is disabled here.
type ObservationControl struct {
	Enabled   bool   `json:"enabled"`
	ChangedAt string `json:"changed_at,omitempty"`
}

var observationControlMu sync.Mutex

func observationControlPath() (string, error) {
	home, err := GlobalHookHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".task-mecca", "runtime", "observation-control.json"), nil
}

func readObservationControls(path string) (map[string]ObservationControl, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]ObservationControl{}, nil
	}
	if err != nil {
		return nil, err
	}
	controls := map[string]ObservationControl{}
	if err := json.Unmarshal(data, &controls); err != nil {
		return nil, err
	}
	return controls, nil
}

func ObservationStatus(provider string) (ObservationControl, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "codex" && provider != "claude" {
		return ObservationControl{}, errors.New("unsupported observation provider")
	}
	observationControlMu.Lock()
	defer observationControlMu.Unlock()
	path, err := observationControlPath()
	if err != nil {
		return ObservationControl{}, err
	}
	controls, err := readObservationControls(path)
	if err != nil {
		return ObservationControl{}, err
	}
	if control, ok := controls[provider]; ok {
		return control, nil
	}
	return ObservationControl{Enabled: true}, nil
}

func SetObservationEnabled(provider string, enabled bool) (ObservationControl, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "codex" && provider != "claude" {
		return ObservationControl{}, errors.New("unsupported observation provider")
	}
	release, lockErr := lockObservationControl()
	if lockErr != nil {
		return ObservationControl{}, lockErr
	}
	defer release()
	observationControlMu.Lock()
	defer observationControlMu.Unlock()
	path, err := observationControlPath()
	if err != nil {
		return ObservationControl{}, err
	}
	controls, err := readObservationControls(path)
	if err != nil {
		return ObservationControl{}, err
	}
	control := controls[provider]
	if control.Enabled == enabled && control.ChangedAt != "" {
		return control, nil
	}
	control = ObservationControl{Enabled: enabled, ChangedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	controls[provider] = control
	data, err := json.MarshalIndent(controls, "", "  ")
	if err != nil {
		return ObservationControl{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return ObservationControl{}, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".observation-control-*.tmp")
	if err != nil {
		return ObservationControl{}, err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return ObservationControl{}, err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return ObservationControl{}, err
	}
	if err := file.Close(); err != nil {
		return ObservationControl{}, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return ObservationControl{}, err
	}
	return control, nil
}
