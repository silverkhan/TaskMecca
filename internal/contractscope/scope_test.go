package contractscope

import (
	"strings"
	"testing"
)

func TestOnlyAcceptanceCheckStateIsOperational(t *testing.T) {
	a := "# Task\n## 요건 정의서\n### 요구사항\n- [ ] source requirement\n### 수용 기준\n- [ ] preserve data\n- [x] keyboard works\n### 제약 및 보존 조건\nno deployment\n## 실행 정보\nnotes"
	b := "# Task\n## 요건 정의서\n### 요구사항\n- [ ] source requirement\n### 수용 기준\n- [x] preserve data\n- [ ] keyboard works\n### 제약 및 보존 조건\nno deployment\n## 실행 정보\nnotes"
	if Fingerprint(a) != Fingerprint(b) {
		t.Fatal("check state invalidates approved wording")
	}
	for _, pair := range [][2]string{{"preserve data", "delete data"}, {"keyboard works", "keyboard fails"}, {"no deployment", "allow deployment"}, {"source requirement", "other requirement"}} {
		if Fingerprint(a) == Fingerprint(strings.Replace(a, pair[0], pair[1], 1)) {
			t.Fatal("meaningful change hidden")
		}
	}
}
