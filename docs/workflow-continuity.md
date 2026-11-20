# Workflow continuity at actual boundaries

The existing Runtime, Activity, operation-boundary and handoff journals are the
sources. This change adds read diagnostics and repairs formal Reviewer terminal
attribution. It introduces no journal, heartbeat, polling, automatic summary or
additional per-tool reporting.

## Supported collection paths

| Actual boundary | Existing path and evidence | Interpretation |
| --- | --- | --- |
| Native execution | `runtime observe-t3`, supported provider hooks, exact native receipt and task binding | Native execution observation, independent of phase work |
| Registrar or Controller registration | `contract apply` → `beginRegistrationBoundary` → input/apply phases → task links → finish | One operation per real invocation; pre-ID operation and multiple task associations retain one shared operation time |
| Assignment and implementation | Exact assignment, current observed native binding, existing role-policy gate, `activity report` | Prepared assignment is not implementation START; missing phase is unmeasured |
| Coordination and integration | Existing current-native `runtime boundary` phase API and Activity/handoff prepare, delivery and claim projections | Record only the actual supported boundary; delivery/claim facts do not imply measured work or acceptance |
| Wait and resume | Operation wait scope plus reason, expected event, next owner, last evidence and resume condition; exact same-operation resume reference | Actor wait and task-next-stage wait are distinct; incomplete evidence remains unknown |
| Native terminal without phase STOP | Existing Workflow terminal provenance and terminal-hook `Abandon` | Execution ended; the phase end and successful outcome remain unreported |
| Completion | Existing exact handoff context, source/report/assignment checks and distinct acceptance/apply gates | Worker completion, review, integration, CI, release and final task acceptance remain separate |

See [execution boundaries](execution-coverage-boundaries.md),
[detail DTO](execution-coverage-detail-contract.md),
[Activity recording](activity-recorder.md) and
[completion context](task-completion-context.md) for existing commands and gates.
The corrected hook path recognizes the same formal Reviewer path as Activity
attribution before the broad Worker prefix. It retains `Abandon`, idempotence,
unknown confidence and zero confirmed duration. Read-only Workflow already
exposes `activity_unreported` with native terminal provenance without writing a
synthetic STOP. The two mechanisms remain distinct.

## Native start and first phase report

Each accepted `workflow.runtime_timing.intervals[]` now includes
`first_activity_status`, either `observed` or `not_recorded`. An observed match
also includes `first_activity_at`, `first_activity_event_id` and
`first_activity_delay_ms`. Zero is present only for an observed same-time match;
missing evidence has no timestamp, event ID or delay value.

The match uses effective Start/Switch events and the exact task, role, native
actor, attempt and assignment. Optional provider/session identifiers must match.
Candidates before native start, after known native completion or in the future
are excluded. Earliest matching occurrence wins, with event ID as a stable tie
break. A running receipt may have been collected before the report; the report
can be compared to native start without extending the receipt's observed prefix.

This is the delay between native start and the first recorded phase occurrence.
It does not establish the cause of the delay, idle time, an external blocker,
unreported phase work or successful completion. Native duration, lifecycle wait,
Activity totals, axis, coverage denominator and historical events are unchanged.

## Brief explanations

`WorkflowInterval.explanation` is optional. Closed Activity intervals copy the
effective originating segment's explanation; their existing exact originating
Start/Switch event ID, assignment, handoff and evidence association is preserved.
Open and native-terminal-bounded `activity_unreported` intervals copy the same
latest originating event's explanation. A STOP's comment does not overwrite its
preceding phase. Different actors or attempts at the same time never share a
comment. Missing comments remain absent. Native observations, lifecycle facts,
handoffs and unmeasured gaps do not receive inferred phase comments.

The existing explanation field is reused. `evidence` remains an independent
source reference. Frontend tooltip/accessibility presentation belongs to B-3;
this backend projection adds no alternative comment schema or inferred prose.

## Continue incomplete work before ending an episode

The Controller retains the original contract and each remaining AC, exact current
owner, source/report and executable next verification path. An implementation
report does not substitute for independent review; review does not substitute
for integration; CI and release do not establish final AC satisfaction or an
operating installation. Supported exact completion context checks continue to
validate these evidence boundaries.

For a remaining check, inspect supported safe isolated fixture or selected
regression paths first. If a real external event is essential, identify the
verified dependency, expected event, next owner, last evidence and resume
condition through the existing wait boundary. A task-next-stage wait does not
assert that its actor stopped working. Absence of a signal or elapsed time alone
cannot establish external hold, user action or inactivity. The next user turn or
Root ACK is not an execution gate.

This is future collection and current read projection. The overnight audit and
A-90–A-94 accepted reports are historical references. Their original failures,
unknowns, missing measurements and A-93 acceptance are preserved. Synthetic
selected regressions demonstrate behavior, not actual Registrar operations.
Controller acceptance, normal dev integration, CI/release and final AC evidence
are reported by their actual owners after the Worker candidate is returned.
