package contractscope

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func legacyHash(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
func TestLegacyCheckHashProofPreservesTextAndBoundsWork(t *testing.T) {
	original := "# Task\n## 작업 정의\n### 수용 기준\n- [x] preserve data\n- [ ] keyboard works\n## 원천 전문 (변경 금지)\nsource"
	for _, current := range []string{original, strings.Replace(original, "[x]", "[ ]", 1), strings.Replace(original, "[ ]", "[X]", 1)} {
		ok, _ := MatchLegacyAcceptanceHash(current, legacyHash(original))
		if !ok {
			t.Fatal("legacy status-only change not proven")
		}
	}
	for _, pair := range [][2]string{{"preserve data", "delete data"}, {"keyboard works", "remove keyboard"}, {"source", "changed source"}} {
		ok, exhaustive := MatchLegacyAcceptanceHash(strings.Replace(original, pair[0], pair[1], 1), legacyHash(original))
		if ok || !exhaustive {
			t.Fatal("text/source change hidden")
		}
	}
	large := "## 작업 정의\n### 수용 기준\n" + strings.Repeat("- [ ] criterion\n", 20)
	if ok, _ := MatchLegacyAcceptanceHash(large, legacyHash(large)); !ok {
		t.Fatal("unchanged large legacy contract rejected")
	}
	if ok, exhaustive := MatchLegacyAcceptanceHash(large, strings.Repeat("0", 64)); ok || exhaustive {
		t.Fatal("bounded search pretended to prove unavailable original")
	}
}
