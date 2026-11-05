package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/silverkhan/TaskMecca/internal/contractscope"
	"os"
	"strings"
)

func ContractFingerprint(markdown string) (string, error) {
	fingerprint := contractscope.Fingerprint(markdown)
	if fingerprint == "" {
		return "", errors.New("canonical contract section not found")
	}
	return fingerprint, nil
}

func extractContractSection(markdown string) string {
	lines := strings.Split(markdown, "\n")
	start := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "## 작업 정의" || trimmed == "## 요건 정의서" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func reportFingerprint(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return "", errors.New("report file is empty")
	}
	var value any
	canonical := []byte(trimmed)
	if json.Unmarshal([]byte(trimmed), &value) == nil {
		if encoded, encodeErr := json.Marshal(value); encodeErr == nil {
			canonical = encoded
		}
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}
