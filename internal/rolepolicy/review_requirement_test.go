package rolepolicy

import "testing"

func a90Requirement() ReviewRequirement {
	return ReviewRequirement{Scope: []string{"product.go"}, Rationale: "isolated reversible correction, existing checks cover behavior", Risk: ReviewRisk{Impact: "low", Complexity: "simple", Reversibility: "easy", Verifiability: "established"}}
}

func TestA90RiskDecisionAndOmission(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*ReviewRequirement)
		want   ReviewLevel
		denied bool
	}{
		{"small", func(r *ReviewRequirement) {}, ReviewOptional, false},
		{"explicit", func(r *ReviewRequirement) { r.Risk.ExplicitRequired = true; r.Omit = true }, ReviewRequired, true},
		{"authority", func(r *ReviewRequirement) { r.Risk.BehaviorChanged = true; r.Risk.TrustBoundary = true; r.Omit = true }, ReviewRequired, true},
		{"finance", func(r *ReviewRequirement) { r.Risk.BehaviorChanged = true; r.Risk.FinancialIntegrity = true }, ReviewRequired, false},
		{"irreversible", func(r *ReviewRequirement) { r.Risk.BehaviorChanged = true; r.Risk.Reversibility = "difficult" }, ReviewRequired, false},
		{"documentation_in_sensitive_area", func(r *ReviewRequirement) { r.Risk.TrustBoundary = true }, ReviewOptional, false},
		{"unknown", func(r *ReviewRequirement) { r.Risk.Impact = "unknown" }, ReviewRecommended, false},
		{"previous_defect", func(r *ReviewRequirement) { r.Risk.PriorFindings = []string{"race"} }, ReviewRecommended, false},
		{"recommended_waiver", func(r *ReviewRequirement) {
			r.Risk.Complexity = "complex"
			r.Omit = true
			r.OmissionReason = "Controller verified narrow impact using existing invariant checks"
		}, ReviewRecommended, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := a90Requirement()
			tc.change(&r)
			d, err := r.Decide()
			if (err != nil) != tc.denied || d.Level != tc.want || d.CreateReviewer {
				t.Fatal(d, err)
			}
		})
	}
	r := a90Requirement()
	r.Omit = true
	if _, err := r.Decide(); err == nil {
		t.Fatal("unexplained omission")
	}
	r.OmissionReason = "known scope"
	d, err := r.Decide()
	if err != nil || d.Status != "omitted" {
		t.Fatal(d, err)
	}
	r.Risk.Impact = "low because I say so"
	if _, err := r.Decide(); err == nil {
		t.Fatal("free prose accepted as risk enum")
	}
}
