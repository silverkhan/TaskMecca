package notify

import (
	"path/filepath"
	"regexp"
	"strings"
)

func telegramProjectPrefix(project string) string {
	return "[" + filepath.Base(filepath.Clean(project)) + "]"
}

// Only redundant leading task labels are removed. Project names, URLs,
// meaningful references in the rest of the title, and message bodies stay intact.
func telegramTaskTitle(title, taskID string) string {
	if strings.TrimSpace(taskID) == "" {
		return title
	}
	id := regexp.QuoteMeta(taskID)
	label := regexp.MustCompile(`(?i)^(?:\[` + id + `\]|\(` + id + `\)|` + id + `)(?:[ \t]*[·:|–—][ \t]*|[ \t]+-[ \t]+|[ \t]+|$)`)
	remainder := title
	for {
		candidate := strings.TrimLeft(remainder, " \t")
		match := label.FindStringIndex(candidate)
		if match == nil {
			return remainder
		}
		remainder = candidate[match[1]:]
	}
}
