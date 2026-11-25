# Worker START and phase collection

The Controller owns canonical Runtime, lifecycle and handoff writes. The Worker
reports its own actual Activity through the existing verified CLI. Neither a
dispatch receipt nor a title proves an execution or phase.

1. After dispatch the Worker captures its actual native session ID and T3 thread
   ID from its managed environment, reads the exact T3 thread with the official
   tool, and sends START directly to Controller. Include task, assignment,
   internal Worker path, canonical project, source cwd and current actual phase.
   Naming verification is separate and never substitutes for identity.
2. Controller verifies the exact dispatch/assignment, approval and fresh access
   gate. For T3, use the existing observer with START integration:

   ```text
   task-mecca runtime observe-t3 --project CANONICAL_PROJECT \
     --thread-id ACTUAL_T3 --session-id ACTUAL_NATIVE --agent-id ACTUAL_NATIVE \
     --assignment EXACT_ASSIGNMENT --task-id TASK \
     --worker-path /root/controller/WORKER --source-worktree ACTUAL_CWD \
     --phase CURRENT_SUPPORTED_EVENT --json
   ```

   This reads the official local T3 projection, verifies native/T3 identity,
   observes the current execution and binds the exact existing assignment using
   the existing Runtime binder. It does not create an assignment, lifecycle
   transition or Activity. `--project` must explicitly name the canonical root;
   `--source-worktree` is context only. A missing assignment in the canonical
   project cannot be supplied by a copied worktree ledger. Existing role,
   attribution and handoff checks remain in force.
3. Deliver the returned exact attempt and `activity_reporting` result directly
   to Worker. Worker queries the exact native/task binding read-only, then runs
   the returned Activity report arguments for its **current actual phase**,
   with a stable event ID and actual evidence. Supported Worker events include
   `requirements_started`, `implementation_started`, `targeted_test_started`,
   `full_regression_started`, `build_started`, and `worker_completed`.
   `tests_started` is not a supported event: select the actual test scope.
   Preserve every success/failure receipt and send phase status to Controller.
4. Missing native/T3 identity, unsupported provider observation, ambiguous or
   mismatched binding, report failure and transport failure are collection gaps.
   Preserve stage/diagnostic, exact IDs and durable local report; directly notify
   Controller. Retry the supported operation with the same exact identities and
   stable report event ID after repair. Observation may persist before binding
   fails; retry does not imply rollback. Do not report a past phase after a late
   START, replay historical timestamps, or reuse a terminal attempt. A failed
   direct send is not proof of delivery; follow the approved transport recovery
   and retain the gap when unavailable. Do not wait for Root acknowledgement.
5. Before handing off, report `worker_completed`. Controller records its real
   review start and outcome through the existing claimed-handoff Activity path.
   Worker completion is not Controller acceptance.

`collection_received_at` is the Controller START CLI collection time, not the
original message transport/relay time (which lifecycle/transport receipts retain). The native receipt's `started_at`
and `completed_at` retain execution provenance; observation-time Runtime fields
are preserved separately. Lifecycle START is the reported boundary, which can
arrive late. A reported registration/dispatch wait therefore does not prove
idle time. The workflow read model compares persisted verified native execution
with reported waits, without provider scans or writes. Running receipts cover
only the observed prefix. Native runtime is never Worker phase Activity.

Activity coverage is the union of measured Activity intervals divided by total
elapsed time. Reported lifecycle waits, verified runtime and unmeasured Activity
are separate facts and may overlap. Completion does not fill missing intervals;
100% coverage is not an inferred goal. Legacy records with no exact verified
binding remain unmeasured. In particular, B-633 is read-only evidence: preserve
its original events and business/finance/DB state, use `observe-t3 --dry-run` for
any real terminal probe, and never backfill its missing Worker phase durations.

The single managed template is `goassets/template/_task_mecca`; this repository
has no second web app template to synchronize. Changes do not edit installed
project frameworks. Verification uses selected domain tests with temporary
ledgers, fixture T3 SQLite stores and offline DOM execution. Never run service
control or global install suites against an operating project.

## Managed collection and isolated verification

Use an explicit operation boundary START and confirm its durable ACK before
business entry. `runtime boundary run ... -- PROGRAM ARGS` executes a program
only after the current actor START is acknowledged; it records the real result
and END afterward. A dangling previous START is a recovery condition, not
permission to run the business command again. Task-linked collection entry also
requires an observed current actor and a successful last boundary ACK. Failed
Activity or boundary delivery retains its original event/key, error, owner and
resume condition. Repair and reconcile the same record before continuing.

`--collection-project OWNED_FIXTURE` separates the collection journal from
`--project REAL_CANONICAL`. Initialize a new owned `.artifacts/collection-fixture-*`
destination with the authorized source Worker. The source-owned initialization
grant and byte-matched marker establish destination provenance; neither grants
an actor a role. Every actor command rechecks its actual host/native execution,
project, source cwd, parent, current source role and authority. Worker admission
requires its current task/assignment/policy. An operational Controller also needs
the exact current claimed source-task handoff. Registrar keeps its real Root
lineage and dedicated registration provenance. Canonical/monitor destinations,
symlink escapes, altered markers and foreign actors fail before collection
writes. Fixture delivery and monitoring remain disabled.

Registration `--approve`, an execution-authorized handoff and a canonical task
execution decision are distinct receipts. Before implementation metadata and
Worker dispatch, record the actual existing user execution consent through
`contract decision record --kind execution_approved ... --user-confirmed` in the
owned destination, at the current time with the real approval source. Then record
observed owner/assignment/workspace facts through `worker_metadata`. Neither a
source assignment nor a fixture marker fills a missing destination decision.
A later recovery preserves the earlier refusal and never reconstructs a past
approval timestamp.

The isolated read-only preview uses the shipped handler, workflow projection and
assets. It validates the source-owned fixture before any boot reconciliation;
only scoped content/catalog, existing lifecycle snapshots and Activity reads
are admitted. Mutation, runtime, maintenance, attention, service, notification,
monitor and arbitrary project/backlog queries remain unavailable. A pending
lifecycle snapshot stays pending; it is not manufactured for a screenshot.

## Interruption and genuine same-native continuation

A verified native `cancelled` status maps to interrupted execution, preserving
its original native/run status and both completion timestamps. It does not mean
the task completed. Actual historical terminal source is required before the
observer admits the distinct new episode. A failed historical read retains its
underlying diagnostic. Never write a fabricated completed receipt or edit the
ledger to recover an observation.

After observing and binding the genuine current episode to the existing exact
task/role/assignment, start and acknowledge a current operation. Record a
`runtime boundary record --kind resume --previous-attempt OLD_ATTEMPT
--related-id EXACT_OLD_BOUNDARY ...` point. The CLI verifies the actual historical
T3 receipt, same native/session/role/task, continuing assignment, exact original
task-linked boundary and nonoverlapping native times. It derives the historical
source time itself. This links the real recovery without closing the old
operation or wait. The renderer labels an old operation whose native ended but
whose END was never reported as unmeasured, with that native bound and the
missing report visible. Resume is a separate fact, not Activity duration.

Native lifetime and task-state evidence have different bounds. Keep the original
native start/end and observation times in RuntimeTiming. For task-state coverage,
admit native evidence only from the later of native start and exact task binding;
when binding time is unknown, do not extend before the folded task observation.
An actual request/task-linked operation may independently evidence preparation.
This preserves established earlier scope while leaving unrelated prebinding
execution unknown for the task.

A new task cannot replace the live episode's task binding. Finish the actual
native episode, prepare the supported pending assignment, genuinely resume the
same native/role/group and START the distinct new attempt. Native turn completion
alone is not `worker_completed`, task acceptance or canonical done. Interrupted
and normal fixture runs remain separate evidence, including all failures and
missing stops; neither may backfill historical Activity coverage.
