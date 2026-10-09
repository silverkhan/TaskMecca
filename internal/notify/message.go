package notify

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// telegramMachineName identifies the device producing this message, not the
// Telegram bot or the destination chat. A per-installation alias can override
// the OS hostname when several installations use the same bot.
func telegramMachineName() string {
	name := strings.TrimSpace(os.Getenv("TASK_MECCA_MACHINE_NAME"))
	if name == "" {
		name, _ = os.Hostname()
	}
	var safe strings.Builder
	space := false
	count := 0
	const maxRunes = 48
	for _, r := range name {
		if unicode.IsSpace(r) {
			if count > 0 && !space {
				safe.WriteRune(' ')
				space = true
				count++
			}
			continue
		}
		if !unicode.IsPrint(r) {
			continue
		}
		if count >= maxRunes {
			break
		}
		safe.WriteRune(r)
		space = false
		count++
	}
	cleaned := strings.TrimSpace(safe.String())
	if cleaned == "" {
		return "알 수 없는 컴퓨터"
	}
	return cleaned
}

func telegramMachineLine() string {
	return "💻 컴퓨터: " + telegramMachineName()
}

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
