# Activity Recorder · Task Mecca

Activity Recorder complements the durable Lifecycle and Runtime ledgers. It records agent-reported phases and aggregates only intervals with an observed begin and end. It **does not** convert an agent's self-description into authoritative native provider evidence.

## Current model

- Activity taxonomy: `requirements`, `research`, `implementation`, `targeted_test`, `full_regression`, `build_check`, `controller_review`, `controller_rereview`, `handoff`, `recovery`, `other`.
- Actor attribution includes task, role, runtime agent ID, exact attempt and either Worker assignment or claimed Controller handoff.
- `round=0` is initial work, `round=1..N` denotes subsequent rework. Round assignment is agent-reported and is **not yet automatically inferred from rejection evidence**.
- `start`, `switch`, `stop` capture recorder timestamps. The prior activity is closed at the switch boundary. Missing stops are left open and do not accrue assumed active time.
- Unsupported Registrar and Sub-Controller writes fail closed until role-specific authoritative bindings are available.
- Caller-provided phase names and timing transitions use `source=agent_report`, `confidence=inferred`. Runtime and dispatch/claim ledgers validate **attribution**, but same-OS-user CLI caller authentication is not cryptographic. Do not treat this as a hostile-process security boundary.

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

## APIs / screens

- `GET /api/activity?task=B-589&project=...`: append-only event history, interval summary, test-run summary and review-round summary.
- `GET /api/activity/project?project=...`: project rollup with observed-task coverage, agent-time, sum of per-task observed wall-time, P50/P90 and unclassified proposal summaries.
- Backlog detail: lazy Activity chart, round/type timings, test summary and agent suggestions.
- Project Workload: lazy, independent Activity statistics section.

Do not add parallel Agent durations and report them as elapsed wall-time. Open activities, invalid transitions and unattributed gaps are surfaced separately. These reports cannot by themselves prove Task Mecca caused a slowdown.

## Operational and migration safety

Project-owned data lives beneath `_task_mecca/data/activity/`, not the managed framework template. Activity events are immutable and original event IDs remain in the journal; the CLI cannot silently rewrite conflicting entries. Project guard refuses writes to paused/removed projects. Back up project-owned data before any manual migration or cleanup.

## Remaining limitations

- Trusted provider caller attestation, Registrar/Sub-Controller role support, complete Decision Engine/Hook phase transition integration and automatic round assignment are not yet available.
- Per-event file storage should be profiled under large histories before treating the metrics as zero-cost.
- The UI reports observed intervals only; it does not reconstruct unobserved historical phases.
- Full rollout should be judged after implementation and Dev real-use observation, not by synthetic charts alone.
