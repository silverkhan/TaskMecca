package rolepolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	Actor            string          `json:"actor"`
	WorkerMetadata   json.RawMessage `json:"worker_metadata"`
	EventID          string          `json:"event_id"`
	TaskID           string          `json:"task_id"`
	Kind             string          `json:"kind"`
	Text             string          `json:"text"`
	PolicyKey        string          `json:"policy_key"`
	Evidence         string          `json:"evidence"`
	UserConfirmed    bool            `json:"user_confirmed"`
	BaseSHA256       string          `json:"base_sha256"`
	RecordedAt       string          `json:"recorded_at"`
	RolePolicySHA256 string          `json:"role_policy_sha256"`
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
		b, e := readDecisionFile(project, file)
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
	var staleMetadata error
	for _, d := range events {
		if d.Kind == "worker_metadata" {
			if d.Evidence == "" {
				return nil, false, errors.New("worker_metadata_observation_evidence_required")
			}
			next, e := ValidateWorkerMetadataEvent(d.WorkerMetadata, d.Text, d.Actor, d.PolicyKey, d.RolePolicySHA256, d.UserConfirmed)
			if e != nil {
				return nil, false, e
			}
			if !matching(d) {
				staleMetadata = errors.New("worker_metadata_requires_current_contract")
				continue
			}
			if err := ValidateRequirementTransition(p, next); err != nil {
				staleMetadata = err
				continue
			}
			p = next
			staleMetadata = nil
		}
		if d.Kind == "supplement" && strings.Contains(d.Text, Fence) {
			if !d.UserConfirmed || !matching(d) {
				return nil, false, errors.New("role_policy_supplement_requires_current_contract_confirmation")
			}
			next, e := Parse(d.Text)
			if e != nil {
				return nil, false, e
			}
			p = next
			staleMetadata = nil
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
	if staleMetadata != nil {
		return nil, false, staleMetadata
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
	abs, e := filepath.Abs(path)
	if e != nil {
		return "", e
	}
	root, e := filepath.EvalSymlinks(filepath.Dir(abs))
	if e != nil {
		return "", e
	}
	f, e := openSnapshotPath(root, filepath.Base(abs))
	if e != nil {
		return "", e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return "", e
	}
	if !info.Mode().IsRegular() || info.Size() > defaultSnapshotLimits.fileBytes {
		return "", errors.New("evidence_file_nonregular_or_size_limit")
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, defaultSnapshotLimits.fileBytes+1))
	if e != nil {
		return "", e
	}
	after, e := f.Stat()
	if e != nil {
		return "", e
	}
	if n != info.Size() || n > defaultSnapshotLimits.fileBytes || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", errors.New("evidence_file_changed_or_size_limit")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReviewRevision pins the existing material contract and task-specific decision
// history, including consent/revocation and typed reviewer/reference updates.
// It is only a freshness fence, never another authorization/approval manager.
// Operational notes, other tasks and unrelated workspace files are excluded.
func ReviewRevision(project, task, raw string) (string, error) {
	h := sha256.New()
	h.Write([]byte(contractscope.NormalizeAcceptance(DecisionScopeRaw(raw))))
	files, e := filepath.Glob(filepath.Join(project, "_task_mecca", "data", "contract_decisions", "events", "*.json"))
	if e != nil {
		return "", e
	}
	for _, file := range files {
		b, e := readDecisionFile(project, file)
		if e != nil {
			return "", e
		}
		var d decision
		if e = json.Unmarshal(b, &d); e != nil {
			return "", e
		}
		if d.TaskID != task {
			continue
		}
		sum := sha256.Sum256(b)
		h.Write([]byte("\x00" + filepath.Base(file) + "\x00"))
		h.Write(sum[:])
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readDecisionFile(project, file string) ([]byte, error) {
	abs, e := filepath.Abs(project)
	if e != nil {
		return nil, e
	}
	root, e := filepath.EvalSymlinks(abs)
	if e != nil {
		return nil, e
	}
	path := filepath.ToSlash(filepath.Join("_task_mecca", "data", "contract_decisions", "events", filepath.Base(file)))
	f, e := openSnapshotPath(root, path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return nil, errors.New("decision_evidence_unbounded_or_nonregular")
	}
	b, e := io.ReadAll(io.LimitReader(f, (8<<20)+1))
	if e != nil {
		return nil, e
	}
	after, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if int64(len(b)) != info.Size() || len(b) > 8<<20 || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, errors.New("decision_evidence_changed_or_unbounded")
	}
	return b, nil
}
