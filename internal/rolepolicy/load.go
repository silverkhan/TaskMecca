package rolepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/silverkhan/TaskMecca/internal/contractscope"
)

// DecisionScopeRaw is the existing execution approval scope. Routine ledger
// notes and checkbox completion are excluded, while source quotations remain.
func DecisionScopeRaw(raw string) string {
	start := strings.Index(raw, "## 작업 정의\n")
	if start < 0 {
		start = strings.Index(raw, "## 요건 정의서\n")
	}
	if start < 0 {
		return raw
	}
	end := strings.Index(raw[start:], "## 실행 정보\n")
	if end < 0 {
		end = len(raw) - start
	}
	scope := strings.SplitN(raw, "\n", 2)[0] + "\n" + raw[start:start+end]
	if i := strings.Index(raw, "## 원천 전문 (변경 금지)\n"); i >= 0 {
		scope += "\n" + raw[i:]
	}
	return scope
}

type decision struct {
	EventID          string `json:"event_id"`
	TaskID           string `json:"task_id"`
	Kind             string `json:"kind"`
	Text             string `json:"text"`
	PolicyKey        string `json:"policy_key"`
	Evidence         string `json:"evidence"`
	UserConfirmed    bool   `json:"user_confirmed"`
	BaseSHA256       string `json:"base_sha256"`
	RecordedAt       string `json:"recorded_at"`
	RolePolicySHA256 string `json:"role_policy_sha256"`
}

func Load(project, task, raw string) (*Policy, bool, error) {
	// Only the contract section is authoritative; source quotations and notes
	// containing example policy blocks must never grant an exception.
	p, err := Parse(contractscope.RawSection(raw))
	if err != nil {
		return nil, false, err
	}
	files, err := filepath.Glob(filepath.Join(project, "_task_mecca", "data", "contract_decisions", "events", "*.json"))
	if err != nil {
		return nil, false, err
	}
	var events []decision
	for _, file := range files {
		b, e := os.ReadFile(file)
		if e != nil {
			return nil, false, e
		}
		var d decision
		if e = json.Unmarshal(b, &d); e != nil {
			return nil, false, e
		}
		if d.TaskID == task {
			events = append(events, d)
		}
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].RecordedAt == events[j].RecordedAt {
			return events[i].EventID < events[j].EventID
		}
		return events[i].RecordedAt < events[j].RecordedAt
	})
	matching := func(d decision) bool {
		ok, _ := contractscope.MatchLegacyAcceptanceHash(DecisionScopeRaw(raw), d.BaseSHA256)
		return ok
	}
	approved := map[string]string{}
	for _, d := range events {
		if d.Kind == "supplement" && strings.Contains(d.Text, Fence) {
			if !d.UserConfirmed || !matching(d) {
				return nil, false, errors.New("role_policy_supplement_requires_current_contract_confirmation")
			}
			next, e := Parse(d.Text)
			if e != nil {
				return nil, false, e
			}
			p = next
		}
		if d.PolicyKey == "implementation-role-exception" && d.UserConfirmed && d.Evidence != "" && matching(d) {
			if d.Kind == "policy_pending" {
				approved = map[string]string{}
			}
			if d.Kind == "policy_confirmed" {
				approved[d.EventID] = d.RolePolicySHA256
			}
		}
	}
	confirmed := p != nil && p.Exception != nil && approved[p.Exception.ApprovalEventID] == PolicyDigest(p)
	return p, confirmed, nil
}

// TaskMarkdown is a read-only lookup for runtime binding, which cannot import
// backlog (backlog itself consumes runtime). Duplicates/missing tasks fail closed.
func TaskMarkdown(project, task string) (string, error) {
	var matches []string
	for _, root := range []string{filepath.Join(project, "_task_mecca", "data"), filepath.Join(project, "_task_mecca")} {
		dirs, e := filepath.Glob(filepath.Join(root, "backlog*"))
		if e != nil {
			return "", e
		}
		for _, dir := range dirs {
			e = filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if !d.IsDir() && strings.HasSuffix(path, ".md") && strings.Contains(d.Name(), "."+task+".") {
					b, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					matches = append(matches, string(b))
				}
				return nil
			})
			if e != nil {
				return "", e
			}
		}
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("role_policy_task_lookup: expected one %s contract, found %d", task, len(matches))
	}
	return matches[0], nil
}

func HashFile(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// Snapshot hashes all Git source files in every declared scope, including new
// files and deletions. No commit ancestry or importer-selected subset can hide a
// later change. Paths crossing a symlink boundary are deliberately unsupported.
func Snapshot(workspace string, scope []string) (Source, error) {
	s := Source{Files: map[string]string{}}
	if !filepath.IsAbs(workspace) || !validScope(scope) {
		return s, errors.New("exact_workspace_and_scope_required")
	}
	root, e := filepath.EvalSymlinks(workspace)
	if e != nil {
		return s, e
	}
	args := append([]string{"-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}, scope...)
	b, e := exec.Command("git", args...).Output()
	if e != nil {
		return s, e
	}
	for _, path := range strings.Split(string(b), "\x00") {
		if path == "" {
			continue
		}
		if !Covers(scope, path) {
			return s, errors.New("source_outside_scope")
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		info, e := os.Lstat(full)
		if os.IsNotExist(e) {
			s.Files[path] = "absent"
			continue
		}
		if e != nil {
			return s, e
		}
		actual, e := filepath.EvalSymlinks(full)
		if e != nil {
			return s, e
		}
		if actual != full || !info.Mode().IsRegular() {
			return s, errors.New("source_symlink_or_nonregular")
		}
		h, e := HashFile(full)
		if e != nil {
			return s, e
		}
		s.Files[path] = h
	}
	if len(s.Files) == 0 {
		return s, errors.New("source_scope_empty")
	}
	return s, nil
}
