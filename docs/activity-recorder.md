# Activity Recorder · Task Mecca

Activity Recorder complements the durable Lifecycle and Runtime ledgers. It records agent-reported phases and aggregates only intervals with an observed begin and end. It **does not** convert an agent's self-description into authoritative native provider evidence.

## Current model

- Activity taxonomy: `requirements`, `research`, `implementation`, `targeted_test`, `full_regression`, `build_check`, `controller_review`, `controller_rereview`, `handoff`, `recovery`, `other`.
- Actor attribution includes task, role, runtime agent ID, exact attempt and either Worker assignment or claimed Controller handoff.
- `round=0` is initial work, `round=1..N` denotes subsequent rework. Round assignment is agent-reported and is **not yet automatically inferred from rejection evidence**.
- `start`, `switch`, `stop` capture recorder timestamps. The prior activity is closed at the switch boundary. Missing stops are left open and do not accrue assumed active time.
- Unsupported Registrar and Sub-Controller writes fail closed until role-specific authoritative bindings are available.
- Caller-provided phase names and timing transitions use `source=agent_report`, `confidence=inferred`. Runtime and dispatch/claim ledgers validate **attribution**, but same-OS-user CLI caller authentication is not cryptographic. Do not treat this as a hostile-process security boundary.

## One-call Decision-to-Activity reporting

Use `activity report` immediately **after the real phase begins or ends**. It
uses the same role/event taxonomy as the read-only `activity suggest` command
but also writes the observed transition under the existing Runtime/assignment
or claimed-handoff evidence gate. It never treats a suggestion as execution.

```bash
task-mecca activity report B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event requirements_started \
  --event-id B589-analysis-start --evidence "began requirement analysis" --json
task-mecca activity report B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event implementation_started \
  --event-id B589-implementation-start --evidence "began implementation" --json
task-mecca activity report B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event worker_completed \
  --event-id B589-worker-completed --evidence "work returned to Controller" --json
```

For a Main Controller use the exact claimed `--handoff`. Actual
`review_started` plus a later `review_accepted`, `review_rejected`,
or `review_needs_changes` records the review outcome. These are Activity
facts only; they do not update canonical acceptance or backlog status.
Unsupported phase names become `other` and require `--explanation`.
A stable event ID can be retried without adding another measurement.

## CLI examples

Get exact IDs from `task-mecca runtime query`, assignment/handoff, and native evidence; do not guess. Each logical call needs a stable unique `--event-id` for retries.

```bash
task-mecca activity start B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event-id B589-analysis-01 \
  --type requirements --evidence "Worker requirements analysis started" --json

task-mecca activity switch B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event-id B589-implementation-01 \
  --type implementation --evidence "Worker started implementation" --json

task-mecca activity stop B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --event-id B589-worker-stop-01 \
  --evidence "Worker finished and reported outcome" --json
```

For Controller review, use `--role controller --handoff EXACT_CLAIMED_HANDOFF` and the Controller's exact runtime identity, instead of `--assignment`. On a review stop, add `--review-outcome accepted|rejected|needs_changes`. `other` requires `--explanation`; optionally provide `--proposed-type`.

For tests/builds, use a test activity type and supply `--command`, `--git-sha`, `--scope`, `--run-id`, `--purpose`; on stop supply `--result passed|failed|cancelled|unknown`. The same SHA/command/scope produces a *repeat candidate*, not a declaration that the retest was unnecessary.

## Immutable correction and audit

A mistaken classification can be corrected **without modifying the original
event** while the original actor still has verified live Runtime attribution.
Use a new stable ID:

```bash
task-mecca activity correct B-589 --project /path/to/project \
  --role worker --agent EXACT_RUNTIME_AGENT_ID --attempt EXACT_ATTEMPT \
  --assignment EXACT_ASSIGNMENT_ID --supersedes-id ORIGINAL_EVENT_ID \
  --event-id CORRECTION_EVENT_ID --type research \
  --evidence "corrected phase classification from requirements to research" --json
```

The original and correction are both returned in raw history. Time summaries
use the effective event, and report invalid correction chains rather than
silently accepting broken or double-applied records. Corrections cannot change
the task/role/agent/attempt/assignment/handoff ownership, override terminal
Hook observations, or increase invalid time transitions. Corrected occurrence
times require an explicit `--occurred-at` timestamp. Correction is not
available once the original actor's Runtime identity can no longer be
verified; future administrator-authorized historical corrections would need
a separate explicit authorization design.

## APIs / screens

- `GET /api/activity?task=B-589&project=...`: append-only event history, interval summary, test-run summary and review-round summary.
- `GET /api/activity/project?project=...`: project rollup with observed-task coverage, agent-time, sum of per-task observed wall-time, P50/P90 and unclassified proposal summaries.
- Backlog detail: lazy Activity chart, round/type timings, test summary and agent suggestions.
- Project Workload: lazy, independent Activity statistics section.

Do not add parallel Agent durations and report them as elapsed wall-time. Open activities, invalid transitions and unattributed gaps are surfaced separately. These reports cannot by themselves prove Task Mecca caused a slowdown.

## On-demand Runtime versus Activity reconciliation

The Activity detail screen has a **Compare runtime evidence** button. It requests
`GET /api/activity?task=TASK&project=...&runtime_diagnostic=1` only when
clicked. The ordinary lazy Activity read folds persisted Runtime journals through
`BuildLedger`, using its process-local fold cache. It does not scan provider
sessions, reconcile bindings, acquire the monitor lock, or write observations.
Terminal evidence therefore remains available when the separate Control Tower
Lifecycle snapshot is pending or stale. The Workflow response cache also includes
Runtime evidence, so a new terminal or resumed execution invalidates the response.

Workflow matches the exact task, attempt, native actor and assignment binding.
Verified terminal evidence carries `hook` or `t3_structured` provenance in
`runtime_terminal`; it remains visible even if an earlier Worker handoff supplies
the execution-report boundary. Runtime `ended_at` is the persisted observation
boundary; a T3 receipt's native `completed_at` can precede it. Neither timestamp
is a measured Activity phase-end report.

An Activity start without a phase-end report becomes `activity_unreported` in
Workflow when its exact Runtime attempt is terminal. It has no measured duration
and is excluded from measured activity ribbons and totals. Immutable Activity
events and the ledger summary remain unchanged: `summary.open_activities` counts
unclosed ledger records, not running Workers. Missing/untrusted terminal evidence
leaves the execution end unobserved. Reads never append synthetic stops or
backdate Activity events.

Backlog detail renders Summary → visible Lifecycle → separate Workflow. Existing
Lifecycle evidence/retry anchors remain available; asynchronous Lifecycle refresh
patches its evidence and refreshes Workflow without replacing document content.

This comparison matches the exact task-bound Runtime attempt and native agent
identifier to the immutable Activity events. It distinguishes a matched
Activity record, no matching Activity record, an unverified binding, and an
identity mismatch. Unbound attempts without a verified task are excluded
rather than assigned speculatively. Diagnostic rows are capped at 60 and
errors leave the standard Activity summary available. A missing record is
**not evidence that the Worker never ran**, nor does the comparison reconstruct
unobserved time or authenticate same-user CLI callers.

## Recorder overhead measurement (on demand)

`task-mecca activity diagnose --project PROJECT --json` returns the count,
total bytes and average size of immutable Activity event files, along with
the wall-clock cost of scanning **metadata only**. It does not read event
contents or run repository tests. It is not a CPU, battery or memory energy
measurement.

For individual activity writes, use `recorder_elapsed_ms` in the CLI JSON
response. For a standalone Activity Web API query, inspect the `Server-Timing`
header for the `activity_read` and `activity_aggregate` phases. These metrics
are not comparable with total time consumed by all running Agents, and no
background polling or periodic benchmark is added.

## Opt-in performance diagnostics

Use the read-only diagnostic only when comparing an observed slow project:

```bash
task-mecca activity diagnose --project /path/to/project --samples 5 --json
task-mecca activity diagnose --project /path/to/project --task B-589 --samples 5 --cpu-profile /tmp/task-mecca-activity.cpu.pprof --json
```

The command performs 1–10 explicitly requested direct ledger reads, reporting
p50/p95 read wall-time, event-file count, stored event bytes, and diagnostic
process heap before and after. An optional Go CPU profile captures work
performed by the diagnostic process only. It does not run in the background
or on every backlog click. It does not claim that a change in process heap
equals Activity-attributed memory or that elapsed time measures battery drain.

Compare like-for-like workloads (same task count, events, OS, storage medium,
and warm/cold cache state) before and after changing ledger internals. For
normal production Web reads, the existing 2-second bounded cache and
compact project aggregates remain the low-cost path.

## Operational and migration safety

Project-owned data lives beneath `_task_mecca/data/activity/`, not the managed framework template. Activity events are immutable and original event IDs remain in the journal; the CLI cannot silently rewrite conflicting entries. Project guard refuses writes to paused/removed projects. Back up project-owned data before any manual migration or cleanup.

## Remaining limitations

- Trusted provider caller attestation, Registrar/Sub-Controller role support, complete Decision Engine/Hook phase transition integration and automatic round assignment are not yet available.
- Per-event file storage should be profiled under large histories before treating the metrics as zero-cost.
- The UI reports observed intervals only; it does not reconstruct unobserved historical phases.
- Full rollout should be judged after implementation and Dev real-use observation, not by synthetic charts alone.
