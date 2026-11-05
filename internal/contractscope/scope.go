// Package contractscope separates approved requirement text from operational
// acceptance check marks. It never changes the immutable source quotation.
package contractscope

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var acceptanceCheck = regexp.MustCompile(`^(\s*(?:[-*+] |[0-9]+[.)] )\[)[xX ](\].*)$`)

// NormalizeAcceptance only removes completion state in the acceptance section.
// Requirement wording, scope, constraints and quoted source remain byte-sensitive.
func NormalizeAcceptance(markdown string) string {
	lines := strings.Split(markdown, "\n")
	acceptance := false
	for i, line := range lines {
		heading := strings.TrimSpace(line)
		if heading == "## 원천 전문 (변경 금지)" {
			break
		}
		if strings.HasPrefix(heading, "## ") {
			acceptance = false
		}
		if strings.HasPrefix(heading, "### ") {
			acceptance = heading == "### 수용 기준"
		}
		if acceptance {
			lines[i] = acceptanceCheck.ReplaceAllString(line, "${1} ${2}")
		}
	}
	return strings.Join(lines, "\n")
}

func RawSection(markdown string) string {
	markdown = strings.ReplaceAll(strings.ReplaceAll(markdown, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(markdown, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		h := strings.TrimSpace(line)
		if start < 0 && (h == "## 작업 정의" || h == "## 요건 정의서") {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(h, "## ") {
			end = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < end; i++ {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func Section(markdown string) string { return NormalizeAcceptance(RawSection(markdown)) }

func MatchesFingerprint(markdown, expected string) (bool, bool) {
	if Fingerprint(markdown) == expected {
		return true, true
	}
	return MatchLegacyAcceptanceHash(RawSection(markdown), expected)
}

func Fingerprint(markdown string) string {
	section := Section(markdown)
	if section == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(section))
	return hex.EncodeToString(sum[:])
}
