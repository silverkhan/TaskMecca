package main

import "github.com/silverkhan/TaskMecca/internal/runtimeobs"

// Missing Hook delivery is not equivalent to missing runtime collection. A
// structured T3 observation can exist even when configured hooks delivered zero
// events. Preserve binding/staleness diagnoses instead of masking them as a Hook
// cause. Provider-wide event counts cannot establish exact-session delivery.
func nativeObservationGap(status string, taskStarted int, enabled, configured bool, attempts []runtimeobs.Attempt) bool {
	return status == "observed_native" && taskStarted > 0 && enabled && configured && len(attempts) == 0
}
