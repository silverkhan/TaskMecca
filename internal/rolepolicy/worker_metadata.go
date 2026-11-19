package rolepolicy

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var metadataWorkerPath = regexp.MustCompile(`^/root/controller/[a-z][a-z0-9_]*$`)

// ParseWorkerMetadata accepts only typed normal-Worker execution facts. It
// cannot contain prose supplements, changed requirements, an exception grant,
// or a review result. Review is an evidence reference, still verified by the
// existing actual-native/task/source gates rather than trusted as an outcome.
func ParseWorkerMetadata(raw []byte) (*Policy, error) {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	var p Policy
	if e := d.Decode(&p); e != nil {
		return nil, e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("worker_metadata_requires_one_json_object")
	}
	if p.SchemaVersion != 1 || p.Method != "worker" || p.Exception != nil {
		return nil, errors.New("worker_metadata_requires_normal_worker_policy")
	}
	if p.Implementation.Role != "worker" || !metadataWorkerPath.MatchString(p.Implementation.AgentPath) || !WorkerPath(p.Implementation.AgentPath) || p.Implementation.NativeID == "" {
		return nil, errors.New("worker_metadata_implementation_owner_invalid")
	}
	if p.Operational.Role != "controller" || p.Operational.AgentPath != "/root/controller" {
		return nil, errors.New("worker_metadata_operational_owner_invalid")
	}
	needsReviewer := true
	if p.ReviewRequirement != nil {
		d, err := p.ReviewRequirement.Decide()
		if err != nil {
			return nil, err
		}
		if err := ValidateRequirementScope(&p); err != nil {
			return nil, err
		}
		needsReviewer = d.Status != "omitted"
	}
	if needsReviewer || p.Reviewer.Role != "" || p.Reviewer.AgentPath != "" || p.Reviewer.NativeID != "" {
		if p.Reviewer.Role != Role(p.Reviewer.AgentPath) || p.Reviewer.Role == "unknown" || (p.Reviewer.Role == "worker" && !metadataWorkerPath.MatchString(p.Reviewer.AgentPath)) || p.Reviewer.AgentPath == p.Implementation.AgentPath || (p.Reviewer.NativeID != "" && p.Reviewer.NativeID == p.Implementation.NativeID) {
			return nil, errors.New("worker_metadata_requires_distinct_planned_reviewer")
		}
	}

	for _, owner := range []Owner{p.Implementation, p.Operational, p.Reviewer} {
		if strings.IndexFunc(owner.NativeID, unicode.IsSpace) >= 0 {
			return nil, errors.New("worker_metadata_native_identity_invalid")
		}
	}
	if !filepath.IsAbs(p.Workspace) || filepath.Clean(p.Workspace) != p.Workspace || !validScope(p.Scope) {
		return nil, errors.New("worker_metadata_workspace_or_scope_invalid")
	}
	if p.Review != nil && (strings.TrimSpace(p.Review.AttemptID) == "" || strings.TrimSpace(p.Review.ReportFile) == "" || !filepath.IsAbs(p.Review.SourceWorkspace) || filepath.Clean(p.Review.SourceWorkspace) != p.Review.SourceWorkspace) {
		return nil, errors.New("worker_metadata_review_reference_invalid")
	}
	return &p, nil
}

// ValidateWorkerMetadataEvent is shared by recording, consent projection and
// policy loading. Strict payload and envelope checks also apply on readback;
// arbitrary imported JSON or legacy prose is never promoted to typed metadata.
func ValidateWorkerMetadataEvent(raw []byte, text, actor, policyKey, roleSHA string, confirmed bool) (*Policy, error) {
	if text != "" || policyKey != "" || roleSHA != "" || confirmed || (actor != "/root/controller" && actor != "/root") {
		return nil, errors.New("worker_metadata_is_operational_facts_not_user_approval_or_supplement")
	}
	return ParseWorkerMetadata(raw)
}
