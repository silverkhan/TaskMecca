package main

import (
	"github.com/silverkhan/TaskMecca/internal/runtimeobs"
	"testing"
)

func TestA81HookConfigurationIsNotRuntimeCollection(t *testing.T) {
	if !nativeObservationGap("observed_native", 1, true, true, nil) {
		t.Fatal("configured but no matching observation gap hidden")
	}
	for _, a := range []runtimeobs.Attempt{
		{AttemptID: "observed", BindingState: runtimeobs.BindingUnbound, StateEvidenceSource: runtimeobs.EvidenceT3},
		{AttemptID: "bound", BindingState: runtimeobs.BindingBound, StateEvidenceSource: runtimeobs.EvidenceT3},
		{AttemptID: "hook", BindingState: runtimeobs.BindingBound, StateEvidenceSource: runtimeobs.EvidenceHook},
	} {
		if nativeObservationGap("observed_native", 1, true, true, []runtimeobs.Attempt{a}) {
			t.Fatalf("actual observation masked as collection gap %+v", a)
		}
	}
	if nativeObservationGap("not_found", 0, true, true, nil) || nativeObservationGap("observed_native", 1, false, true, nil) {
		t.Fatal("unknown native/disabled collector misdiagnosed")
	}
}
