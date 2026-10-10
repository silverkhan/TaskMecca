package runtimeobs

import "time"

// AssignmentObservationGrace is the existing assignment_pending observation
// window. Anchor it to durable AssignedAt, never to a poll or render time.
const AssignmentObservationGrace = 2 * time.Minute
