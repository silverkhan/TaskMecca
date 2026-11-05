package contractscope

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const LegacyCheckCandidateLimit = 4096

// MatchLegacyAcceptanceHash proves compatibility by hashing exact variants of
// acceptance check state only. No wording or source quotation can be changed.
// A legacy event has no original text snapshot, so an exhausted bounded search
// returns exhaustive=false: callers must keep authorization unverified.
func MatchLegacyAcceptanceHash(scope, expected string) (matched, exhaustive bool) {
	if len(expected) != 64 {
		return false, true
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return false, true
	}
	hash := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	if hash(scope) == expected {
		return true, true
	}
	positions := []int{}
	acceptance := false
	offset := 0
	for _, line := range strings.Split(scope, "\n") {
		h := strings.TrimSpace(line)
		if h == "## 원천 전문 (변경 금지)" {
			break
		}
		if strings.HasPrefix(h, "## ") {
			acceptance = false
		}
		if strings.HasPrefix(h, "### ") {
			acceptance = h == "### 수용 기준"
		}
		if acceptance {
			if m := acceptanceCheck.FindStringSubmatchIndex(line); m != nil {
				positions = append(positions, offset+m[3])
			}
		}
		offset += len(line) + 1
	}
	if len(positions) == 0 {
		return false, true
	}
	limit := LegacyCheckCandidateLimit
	if len(scope) > 0 && len(scope)*limit > 64*1024*1024 {
		limit = 64 * 1024 * 1024 / len(scope)
		if limit < 1 {
			limit = 1
		}
	}
	candidate := []byte(scope)
	checked := 0
	seen := map[string]bool{}
	test := func() bool {
		value := hash(string(candidate))
		if seen[value] {
			return false
		}
		seen[value] = true
		checked++
		return value == expected
	}
	// Common legacy renderings and a single status toggle are cheap even when
	// a task has many criteria. A successful match is always an exact hash proof.
	for _, state := range []byte{' ', 'x', 'X'} {
		for _, p := range positions {
			candidate[p] = state
		}
		if test() {
			return true, true
		}
	}
	copy(candidate, scope)
	for _, p := range positions {
		original := candidate[p]
		for _, state := range []byte{' ', 'x', 'X'} {
			candidate[p] = state
			if checked >= limit {
				return false, false
			}
			if test() {
				return true, true
			}
		}
		candidate[p] = original
	}
	digits := make([]int, len(positions))
	states := []byte{' ', 'x', 'X'}
	for {
		for i, p := range positions {
			candidate[p] = states[digits[i]]
		}
		if checked >= limit {
			return false, false
		}
		if test() {
			return true, true
		}
		i := 0
		for i < len(digits) {
			digits[i]++
			if digits[i] < len(states) {
				break
			}
			digits[i] = 0
			i++
		}
		if i == len(digits) {
			return false, true
		}
	}
}
