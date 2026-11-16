# Exact task context at resume and completion

A finished provider turn is not task completion. The existing handoff gates now
associate the current Controller attempt, original registration handoff, contract,
normalized acceptance criteria, actual bound Worker/assignment, report and source.
This adds evidence collection, not another approval, warning or session manager.
Normal Worker work retains its existing execution approval and A78 review policy.

The canonical Controller is the operational writer. Workers create their report
and context candidate in their implementation checkout and send them directly to
the exact Controller. They do not prepare/claim/mark canonical handoffs themselves.

## Start and resume

After observing the real current native/T3 episode, verifying the existing binding
and fresh Full Access, the Controller reads:

```sh
task-mecca handoff context ORIGINAL_HANDOFF --project CANONICAL --target-attempt EXACT_CURRENT --json
task-mecca coordinate --project CANONICAL --json
```

`context` is read-only. It checks the original task/contract/AC and current target,
reports the latest actual Worker phase on the current assignment and returns known
same-task reports with their changes/tests/errors. Empty collections are unknown
evidence, not successful implementation. Another task's report remains a historical
reference, never an implicit result for this task. Old attempts are not silently
reused. Existing unclaimed retarget or verified claim transfer applies to pending
handoffs. An already-applied registration is immutable intake history: the typed
same-native lineage check below connects it without a transfer or rewrite. The
original report's receiving attempt remains visible.

### Applied registration intake and current Controller episodes

A registration `applied` receipt records intake; it is not a completion report.
For a later current episode, `context` and each distinct completion gate verify
the same task/contract/AC and a typed `registration_lineage` proof. The old receipt
must be registration_ready, applied and claimed by its exact Controller target.
That exact old attempt must have observed, bound terminal native evidence matching
the immutable target's provider/session/runtime identity. The separately verified
fresh current Controller must have that same native/session/provider identity.
The response preserves original target/claim/applied time, current attempt, and
`historical_target_mismatch: true`; it never pretends the old receipt targets the
current episode. Old unobserved, foreign-native/provider, unclaimed, pending,
wrong-task, changed-contract and stale-current evidence still refuse. Pending
handoffs continue using their existing retarget/transfer rules.

No new registration, approval, transfer of an applied row, or canonical repair is
created by this read-only link. Completion uses a distinct report/source/assignment
handoff with its own current receiver. Repeated current episodes retain this
original lineage while existing pending-completion retarget/retry stays idempotent.
Normal Worker metadata continues preserving existing execution consent.

### Implementation attribution during review

`StartContext` resolves the implementation owner from the current role policy,
including supported worker_metadata, or an explicit canonical Agent path for a
legacy task. It then selects assignments only for that owner and verifies the
actual bound observed native identity. A later reviewer assignment is returned
under `other_actors`; it cannot become the implementation source/phase or erase
the completed implementer's collection gap. Explicit implementation reassignment
uses the new declared owner and its exact assignment. Missing/multi-owner legacy
metadata, ambiguous assignments/attempts and policy/native mismatch remain a
collection gap with a Controller verification stage; latest task assignment is
never an ownership fallback. No reviewer name heuristic is used.

The existing `coordinate.recovery_queue` and `recovery event` paths include finished
Worker turns without completion evidence and assignment/START collection gaps after
the existing assignment grace. They preserve runtime state and measured phase:
assigned/prepared, requirements, implementation, test/build, Worker completion,
Controller review and waiting remain different observations. Actual creation, START
and current-phase Activity are collected through their existing supported tools.
No crash cause is inferred from a finished turn or missing evidence. No daemon,
automatic wake, new provider capability or Root ACK requirement is introduced.

## Completion report

New `worker_done` and `completion_report` handoffs require `--context-file` and
`--report-file`, exact source/target attempts and the actual `--source-worktree`.
The context is JSON with `schema_version: 1`, `controller_attempt_id`, `targets`
and optional `references`. Each target has:

| Field | Evidence |
| --- | --- |
| `task_id` | Exact completion target, distinct from historical references |
| `contract_sha256`, `acceptance_sha256` | Current contract fingerprint and normalized AC digest from `handoff context` |
| `original_handoff_id` | The original same-task registration handoff, currently targeted/claimed by this Controller |
| `phase` | `worker_completed` for completion; actual supported phases for context checks |
| `source_attempt_id`, `assignment_id` | Actual observed bound implementer and its existing exact assignment |
| `source_workspace` | Absolute implementation checkout |
| `source` | `{"files":{"relative/source":"SHA256"}}`; complete policy scope when A78 is configured, including additions/deletions |
| `report_file`, `report_sha256` | Absolute durable report path and actual full file SHA-256 |
| `changes`, `tests` | Known scoped changes and `{command,result,evidence}` records; disclose not-run/limitations honestly |
| `original_errors` | Original errors, retained with actor/task/report/source evidence |

The durable report contains exactly one applicable machine declaration, on one line:

```text
TASK_MECCA_REPORT {"targets":[{"task_id":"EXACT_TASK","contract_sha256":"ACTUAL_CONTRACT_SHA","acceptance_sha256":"ACTUAL_AC_SHA","source_attempt_id":"EXACT_WORKER_ATTEMPT","source":{"files":{"relative/source":"ACTUAL_SHA"}}}]}
```

Use actual values. Write this declaration before hashing the full report. Its
declaration excludes the report's own hash to avoid a self-reference. Prose below
it may include past tasks, limitations and errors; no natural-language keyword
engine determines success/failure. A multi-target report declares each target's
own matching contract, Worker and source. Each completion handoff applies to one
explicit task and rechecks only that task. Historical targets need not still have
their old checkout available merely to be referenced.

```sh
task-mecca handoff context-check TASK --project CANONICAL --target-attempt EXACT_CURRENT --context-file CONTEXT.json --json
# Controller writer, after actual DONE receipt and existing authorization checks:
task-mecca handoff prepare TASK --project CANONICAL --event worker-done --from WORKER_PATH --to /root/controller --source-attempt EXACT_WORKER --target-attempt EXACT_CURRENT --source-worktree ABSOLUTE_SOURCE --report-file ABSOLUTE_REPORT --context-file CONTEXT.json --json
```

`context-check` checks start/resume linkage; completion preparation additionally
requires the completion phase and matching report/source/assignment evidence.
Prepare, claim/review, acceptance and applied recheck the applicable evidence.
Applied requires acceptance. A78 still requires the real different bound native
reviewer's completed official final statement, full source scope and report hash;
context JSON, `independent:true` or self `review_accepted` cannot substitute for it.
Policy/source/report changes invalidate the affected pending review/completion.

## Failure, retry and historical compatibility

Mismatch/missing evidence refuses the current completion, preserves the original
report and immutable journal receipts, and returns the Controller owner, remaining
stage and resume conditions through the context diagnostic. Repair the exact
evidence and retry the same task/report. Duplicate preparations and applied retries
remain idempotent. A verified same-native current-attempt replacement uses existing
retarget; different operational ownership requires existing verified transfer.
The reported receiving attempt is retained separately from the current one.

Already applied history is not revoked, and an unrelated done task is not edited.
Old completion receipts without context remain readable as unverified history;
new acceptance of one requires matching scoped evidence, rather than a fabricated
retroactive START/review. Supported CLI gates do not fence every manual/legacy
Markdown/journal write. Tool ACK, Controller acceptance, local/dev integration,
remote commit/CI/release and operating installation remain separate outcomes.

Backlog runtime projection and `coordinate` use the same authoritative implementation
owner rule as start context: structured role policy (including existing Worker
metadata) overrides the legacy operational Agent field. Bound implementation
progress requires that owner's path, observed native identity and exact current
assignment. A later reviewer observation stays in `other_actors`; it cannot erase
an implementation collection gap or supply its recovery owner. A newer prepared
implementation assignment remains a START collection gap rather than reusing an
older completed episode. Unknown, conflicting or unverifiable ownership produces
an implementation-owner verification recovery entry. Without structured policy,
only an explicit single legacy Worker path can correlate execution evidence.
These observations authorize no automatic dispatch, approval or completion.

Legacy Worker paths remain supported for exact assignment/runtime correlation and
real matching completion reports. A terminal hook without a report retains its
owner-aware collection gap; it cannot provide a pending-review or DONE fact.
The legacy terminal-only regression verifies that distinction across runtime,
coordination and task detail without manufacturing a completion report.
