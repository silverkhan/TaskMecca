package webui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

// This explicitly scoped API must never fall back to the Backlog/service
// project when a saved session selection is removed or invalid.
func workloadSessionProject(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if !filepath.IsAbs(requested) || !maintenance.IsRegisteredProject(requested) {
		return "", fmt.Errorf("session project is not registered")
	}
	return filepath.Clean(requested), nil
}
