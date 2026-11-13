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
