# Handoff and Controller review warning boundaries

The operations monitor projects immutable assignments, bound attempts, and worker_done handoffs into current stages. It persists the stage name, assignment/attempt/handoff identity, source evidence and original timestamp in the operations journal. It never creates a lifecycle transition, alters a runtime state, or completes a backlog file.

| Stage | Clock source | Grace |
| --- | --- | --- |
| assignment_pending | latest unique assignment AssignedAt | 2 minutes |
| worker_report_pending | observed completed hook EndedAt | 5 minutes |
| handoff_pending | matching worker_done PreparedAt | 5 minutes |
| controller_review | Controller claim ClaimedAt | 15 minutes |
| review_complete | acceptance=ok ObservedAt | 10 minutes |
| handoff_failed | failed step ObservedAt | none |

Grace ends at the boundary, not after it. Polling, server restarts and repeated scans cannot move its clock. A new assignment invalidates an older worker report. Ambiguous bindings, malformed journals, future timestamps and unobserved terminal claims fail closed, rather than earning review grace. Handoff applied is not itself a verified backlog completion.

Only runtime_unknown/no_signal observations are deferred. Actual errors/interruption/shutdown remain actionable and retain their runtime history. Handoff failure is immediate; missing report, undelivered handoff and review/finalization stalls are separately observable after their bounded grace. Recovered warnings are retained in history with their original evidence. Execution-ID-less old assigned warnings resolve only against the latest assignment's bound observed execution; arbitrary backlog done does not qualify.

Notification episodes keep their stable incident IDs during repeat scans. The existing notification delivery ledger deduplicates these IDs; no transport/configuration or message formatter is changed. Telegram maintenance gate must remain disabled for production verification. No real task ledger or transport is required for the synthetic tests.

Old sources resolved by a later observed assignment are compared by exact project/task/attempt/agent/kind/evidence/last-observed identity on subsequent scans. The unchanged historical source cannot start another notification episode even if the latest execution later fails; that new actual failure remains independently actionable. A changed historical observation is not silently suppressed.

Web keeps the incumbent visual system: current actionable incidents in the top summary, expandable evidence/history and current handoff/review stages. Worker report and Controller review completion explicitly do not imply backlog completion.
