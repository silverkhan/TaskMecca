# A70 raichyu implementation evidence

Stable DONE ID: `a70-raichyu-done-20261112-g1`. Implementation DONE; acceptance, canonical done/applied and integration remain Controller actions.

Implementation commit: `3211cffce85b6b8ca16ff7c92e4242173363659a`.
Base and observed origin/dev: `ace1e3dc167478999d3457f2c49c95ebe147ae42`.
Branch: `t3/lifecycle-terminal-projection`.
Checkout: `/Users/hanati/.t3/worktrees/TaskMecca/t3-4ec6223e-1d998ea9614380d1`.
The evidence-only follow-up commit contains this report and two minimal JSON receipts. Final branch HEAD is sent in the direct DONE transport; no push/PR/merge/install/deploy occurred.

## Actual identity and contract

- Actual `CODEX_THREAD_ID` / native runtime ID: `01a1cebe-fd3a-7672-bd3f-3108ea17f44b`.
- Fresh runtime query at `2026-11-12T08:09:39.332692Z`: `run-3ac8827fe12a4c99`, task A-70, `/root/controller/raichyu`, bound, running, terminal=false, t3_structured. This is observation, not a process-liveness guarantee.
- Actual T3 thread: `thread:delegated-task:command%3Amcp%3A17c02103-a2d4-43df-b4c5-96ede8dc66b1%3Adelegate-task%3Aa70-raichyu-workflow-terminal-lifecycle-20261112`; direct tool rereads verified title `D1112_01_Worker_raichyu_G1`.
- Direct START delivered to exact Controller using `t3_thread_send(mode=auto)`: message `message:mcp:0a504f8d-ef4c-44dc-b5ff-d42cdeead613:thread-send:368f0023-38d7-42d3-8593-efb609cb3300`, delivery=started. Later progress messages delivered directly; these are transport, not Hooks.
- Required absolute request manifest and canonical A-70 item were read. Candidate `contract decision show --task A-70` returned execution_authorized=true, findings=[], effective hash `bfce7b02d6445bc0facdd52d3ccfdae5dd08e00e494efd781216d8542cec25e5`; official supplement `registrar-lossless-constraints-workflow-terminal-lifecycle-20261112` was included. Source, 4AC, constraints and tags preserved.
- Worker/protocol/session/playbook read from canonical framework. Fresh effective preflight before implementation at `2026-11-12T16:54:19+09:00`: full_access_confirmed=true, orchestration_ready=true. The only preflight write was its authorized ephemeral cache/probe.
- Canonical inspect confirms doing, current assignment, agent `/root/controller/raichyu`. Normal A70 handoff `handoff-8ae9915772236d839e36` was claimed by Controller after fresh current episode observation (`run-e9f150dd08fdada3`); no A67 exception was used or invented by Worker.

## Findings and implementation

Installed GET `/api/activity` obtains execution observations only from `backlog.ReadLifecycleProjection`. That is a process-local Control Tower publication, and the actual A68 response has `control_snapshot.trust=pending`, no published execution observations. `HasObservedExecution` already validates T3 receipts; it was not a missing-Hook-only predicate.

`readActivityWorkflow` now independently reads persisted Runtime journals through `activity.ReadWorkflowExecutions` / `runtimeobs.BuildLedger`. This is the read-only fold, not `ReconcileLedger`: no provider-session scan, binding repair, event append, monitor lock, or Control Tower refresh. Complete current Runtime evidence participates in the response-cache identity, so terminal/resume changes invalidate a cached response even when the Control Tower snapshot remains pending/stale. Read errors fail closed and are displayed as evidence errors.

Terminal matching requires the exact task/attempt, bound native identity, actor/assignment when provided, nonambiguous candidate, trusted observed execution provenance and a valid nonfuture terminal boundary. T3 receipt identity/state are verified by the existing predicate; thread/run and valid completion/observation ordering are checked additionally. A replacement attempt never inherits an earlier end.

`runtime_terminal` preserves independent Runtime provenance even if an earlier Worker handoff supplies the execution-report end. `end_evidence_source` distinguishes Hook and t3_structured when Runtime supplies that boundary. A missing phase-end report becomes Workflow `activity_unreported`, measured=false, no duration. It is absent from measured activity ribbons/totals. The raw Activity summary remains honest: `open_activities` counts unclosed ledger events, not running Workers. No old events or timestamps are mutated.

The CLI `observe-t3` path does not call Activity `RecordTerminal`, whereas the Hook write path in `cmd/task-mecca/main.go` does. This was verified by source inspection. An append bridge is unnecessary for this correction: it would not supply a measured phase-end report, and reads can express the trusted Runtime terminal plus missing Activity report directly. No synthetic stop or abandonment was appended; neither command source nor main.go changed.

Lifecycle is now an independent visible section immediately after the human summary. Workflow has its own `#workflow` section and keeps all controls. `#lifecycle`, `#workflowLifecycleEvents`, evidence/retry anchors and `#activity-metrics` remain unique. TOC/deeplinks include both sections; existing responsive classes remain. Async refresh patches Lifecycle evidence, refreshes the separate Workflow and preserves document content, scroll handling, retry and cache behavior. Impeccable layout/harden guidance was applied to the existing design; no PRODUCT/redesign/browser preview/server was introduced. Mechanical detector returned `[]`.

## 4AC evidence

1. **PASS:** Actual A68 Runtime metadata and installed symptom were read. Candidate original-data projection resolves the exact Worker attempt with native_terminal/t3_structured. Provenance is visible separately from Activity phase-end reporting.
2. **PASS:** No measured Worker duration, fabricated zero, historical stop, backdated event or API write. Tests cover missing terminal, different attempt/task/Worker/binding, untrusted source, missing/invalid receipt, native completion before observation, duplicate candidates, resume/replacement, open Activity and earlier handoff plus independent terminal evidence. Raw ledger open count intentionally remains 1.
3. **PASS:** Default-visible independent Lifecycle DOM order, unique anchors, TOC/deeplinks, preserved Workflow controls and independent async refresh are tested. Existing responsive structure retained; no live browser/mobile rendering was run under the explicit no-preview/server constraint.
4. **PASS:** Read-only original-data probe plus installed GET comparison; narrow domain/source-file Web cache tests and 70 pure Node tests passed. Installed Web remains old until a separately authorized install.

## Actual A68 comparison and no-write proof

Minimal results: `actual-data-check.json`. Full derived projection is preserved locally at `.artifacts/a70-actual-projection.json`, excluded from commits; no raw operational journals/transcripts were copied into Git.

- Task A-68; native `01a1ce73-7221-7ad1-adf6-47728b52348d`; attempt `run-ad478c65e15a5d8a`; assignment `a68-cycle-badges-20261112`; bound `/root/controller/isanghaessi`; state completed, terminal=true, source t3_structured; `HasObservedExecution=true`.
- Persisted `ended_at`: `2026-11-12T06:48:19.952955Z`, an observation boundary. Receipt native `completed_at`: `2026-11-12T06:46:40.368Z`. Candidate retains the persisted observation boundary; neither is promoted to a measured Activity phase end.
- Installed GET still reports phase completed, execution_report end_evidence=unobserved, requirements activity_open, open_activities=1, trust=pending.
- Candidate reports phase completed; execution_report end_evidence=native_terminal/source=t3_structured; requirements activity_unreported. Both intervals measured=false and lack duration_ms. Worker metric partial, segment_count=0, no duration. Controller measured work remains unchanged.
- Probe hashed **43,370 files** before and after, including canonical backlog, executions, handoffs, lifecycle and Activity inputs. Exact path→SHA maps equal. Aggregate source digest: `d5bd00c4266f0da04a84d7263fa1ef4b1f6bb5dddf334e59daf98aeabc1c4ff9`. `read_no_write_equal=true`.
- Original protected completion_review_test.go SHA remains `bb2b5c4d08819a00b24d2a170814922217e255c9c54f8b3263d124e1d7a9e655`. Checkout's base version SHA is `8dc7239121161ce313bf197befea88cdacdf666393819bf1938aaa68f4903db3`; neither version was edited.

## Safe audit and independent commands

Audited `internal/activity` test sources/helpers: no TestMain/init, service-control or subprocess service calls. Selected Workflow tests use pure data or `t.TempDir`. Runtime `BuildLedger`, handoff `BuildLedger`, Activity `List`, and `ReadLifecycleTransitions` read paths were inspected: their write/reconcile counterparts are not invoked. The Web cache test compiles only the two read-model source files and its own temporary-project test: no Handler/server, package-wide Web/cmd test helpers or service init is compiled. The production Web package's diagnostic-only init was inspected but not executed by that test.

Run independently from the exact checkout:

```sh
cd /Users/hanati/.t3/worktrees/TaskMecca/t3-4ec6223e-1d998ea9614380d1
go test ./internal/activity -run '^(TestWorkflow|TestReadWorkflowExecutions)' -count=1
```

Result: `ok github.com/silverkhan/TaskMecca/internal/activity 0.603s`.

```sh
go test internal/webui/activity_workflow.go internal/webui/activity_lifecycle.go internal/webui/activity_workflow_read_test.go -run '^TestActivityWorkflowReadPendingSnapshotAndTerminalCache$' -count=1
```

Result: `ok command-line-arguments 0.224s`. Pending snapshot and unchanged response reuse, terminal cache invalidation and same-attempt resume tested without a server.

```sh
node --test tests/web_detail_lifecycle_live.test.cjs tests/web_task_detail_prefetch.test.cjs tests/web_workflow_detail.test.cjs tests/web_activity_detail.test.cjs
```

Result: 70 passed, 0 failed, 0 skipped. Logs preserved in `.artifacts/a70-node-tests.txt`. An earlier additional navigation test invocation passed 30 tests and skipped its browser-gated test; it was not used as browser evidence.

```sh
go build -o .artifacts/a70-readonly-probe ./handoff-evidence/a70-raichyu/probe
```

Build passed. Explicit binary: `/Users/hanati/.t3/worktrees/TaskMecca/t3-4ec6223e-1d998ea9614380d1/.artifacts/a70-readonly-probe`.
SHA-256: `1c7b74da46ce65766f5f189ee13a07e31bade75e41e5a385efaa56e1eceadaf8`.

```sh
/Users/hanati/.t3/worktrees/TaskMecca/t3-4ec6223e-1d998ea9614380d1/.artifacts/a70-readonly-probe --project /Users/hanati/projects/TaskMecca/TaskMecca --task A-68
```

This one-shot command prints a projection and before/after hash equality, then exits. It does not start or contact a server. Concurrent Controller writes can legitimately make equality false; do not interpret that as a probe write or override the failure.

Installed READ comparison uses exactly the handler's `project` and `task` query keys, without diagnostics or admin routes:

```sh
curl --get --data-urlencode 'project=/Users/hanati/projects/TaskMecca/TaskMecca' --data-urlencode 'task=A-68' http://127.0.0.1:18765/api/activity
```

`git diff --check` passed. No go test ./..., full CLI, six-package, package-wide cmd/Web, service-control, launchctl, actual admin writes, candidate installation, global installation, service start/stop/restart or cleanup was run. A69 was neither canceled nor modified.

## Official current-session name verification

Minimal actual receipt: `name-verification.json`. Used only the user-specified A69 adapter path and SHA `3a033cb5f3415ae1015df05dd996438758a5cdb5018d8ed0a637a5b85901faa0`; no naming source was edited.

Own-current-session apply at `2026-11-12T08:00:35.319366Z` and verify at `2026-11-12T08:00:47.831943Z` returned official `codex_app_server:thread/read`, native name `D1112_01_Worker_raichyu_G1`, matching session, verified=true, operational_written=false. CLI naming_complete=false / agent_tool_read_required is intentionally retained; it cannot certify a T3 tool response. Independent actual `t3_thread_read` returned the exact title on the exact thread. No T3 rename was necessary. Names are labels, never identity/authority.

## Changed files and preserved artifacts

Implementation commit files:

- docs/activity-recorder.md
- goassets/template/_task_mecca/framework/web/app.js
- goassets/template/_task_mecca/framework/web/style.css
- internal/activity/workflow.go
- internal/activity/workflow_execution.go
- internal/activity/workflow_execution_test.go
- internal/webui/activity_workflow.go
- internal/webui/activity_workflow_read_test.go
- tests/web_detail_lifecycle_live.test.cjs
- tests/web_workflow_detail.test.cjs
- handoff-evidence/a70-raichyu/probe/main.go

Evidence follow-up files: this report, actual-data-check.json, name-verification.json. Preserved untracked `.artifacts/`: a70-readonly-probe, a70-actual-projection.json, a70-installed-read.json, a70-domain-tests.txt, a70-web-read-tests.txt, a70-node-tests.txt. No implementation changes remain uncommitted after the evidence commit; artifacts are deliberately untracked and retained, not cleaned up.

No remote push, dev integration, PR, deploy, canonical acceptance/done/applied, or installation is claimed. Controller must review and perform any separately authorized integration/install/finalization.
