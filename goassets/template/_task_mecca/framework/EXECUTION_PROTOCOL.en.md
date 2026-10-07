# Canonical assignment and direct completion protocol

This is the shared procedure for both SESSION_GUIDEs, collab, and roles. The Controller is the sole operational writer of the designated original canonical backlog/runtime. Workers read the original contract, implement in their assigned worktree, and report actual start/wait/resume/completion and transport evidence. Workers must not manually create, edit, or monitor-register original or copied backlog/lifecycle/handoff/runtime operational ledgers. Authorized CLI preflight ephemeral probe/cache observations and automatic runtime hooks are permitted exceptions, not Worker operational write authority. Canonical state is recorded when facts occur; it is independent of Git/dev integration and must not wait for merge.

## Assignment checklist

Run CLI commands with the designated original project as cwd. Verify installed CLI support before assuming --project works elsewhere. Record canonical root, original repository, integration branch/base SHA, implementation worktree/branch and prohibited scope.

1. Run fresh `task-mecca preflight --require-full-access --json` immediately before dispatch. Require `access.orchestration_ready == true`; cached results and UI labels are insufficient.
2. Fresh `task-mecca inspect <ID> --json` checks the original contract, state, dependencies, Agent and scope. For recovery of doing, inspect existing assignment/completion evidence first.
3. Compare native live agents with fresh `coordinate --json`. Select identity and scope. Pass every actual live path through repeated `worker-name --used <live-path> --json` arguments, then validate a new allocator canonical path with `agent <new-path> --new --json`. Validate reuse with `agent <existing-path> --json`. Keep the existing Pokémon allocator pool; do not invent task-ID/a19_impl/a20_impl names.
4. Record original doing filename, Agent and change scope, then inspect again. All three must match before continuing. Safely edit filename/metadata; do not invent an unsupported state-change command.
5. Run `task-mecca runtime assign <ID> <worker-path> --json`, retain its actual assignment_id and record `task-mecca lifecycle record assigned <ID> <stable-event-id> /root/controller controller_report <evidence> <assignment_id>`. Assigned is not started.
6. Actually dispatch with original contract path/ID, scope, assignment_id, contract_sha256 and Controller envelope: semantic role, exact native target, runtime agent ID, attempt ID, provider/session scope. A native alias may differ from its semantic role.
7. Bind the exact ID from dispatch/direct hook using `task-mecca runtime bind-assignment <assignment_id> <runtime-agent-id> --json`. Zero/multiple candidates or an absent first hook require pending confirmation, never time-based guessing. Role-only Go binding uses `task-mecca runtime bind <exact-attempt-id> '' /root/controller --json`; a bare `-` may be rejected by the common parser and must not be logged as success.
8. Post-inspect original doing/Agent/scope and compare assignment/exact runtime binding. Only actual Worker started report/hook permits Controller lifecycle started. Workers report waiting/resumed; Controller records them. Missing attempt evidence is omitted with the limitation recorded, never fabricated.

A failed pre-dispatch gate stops dispatch and records cause, remaining steps and resume condition. Missing post-dispatch evidence requires preserving implementation/uncommitted changes and a safe recovery record. Failed canonical confirmation, uncertain binding or unconfirmed Worker creation cannot justify handoff applied. Never backfill fictional start time, assignment or attempt.

Preserve existing nonstandard active paths, native aliases and historical archives; do not force rename or duplicate spawn. Apply allocator naming to new assignments and verify original Agent/runtime binding/dispatch identity correspondence.

## Worker DONE/BLOCKED transport checklist

Worker final or a Root report alone is not transport success. Use one stable report/handoff ID for the same result and all retries.

1. Report each acceptance criterion, commit/push/PR/tests, uncommitted/ignored files, remaining work and resume condition. Use /tmp only for temporary delivery. Preserve failure/unknown evidence as a durable report file with a stable report/handoff ID in the retention path designated by the Controller at dispatch, or a separate handoff-evidence/ path in the Worker implementation worktree. This is evidence, not an operational ledger, and must not live in a copied backlog/runtime. Delegate original handoff prepare/mark and durable writes to the Controller.
2. Immediately before exit, freshly query native state for the exact Controller target from the dispatch envelope and compare runtime ID/attempt/provider/session. Do not send to a stale completed semantic-path target when an actual native alias was assigned.
3. If running, directly send DONE/BLOCKED and report ID with `collaboration.send_message`. If completed, run fresh Full Access preflight in the original project and actually resume the same exact native Controller using `collaboration.followup_task`, within existing execution authorization and supported runtime capability.
4. Handle a running-to-completed race with bounded confirmation under the same handoff ID: query immediately and at most once more, without endless polling. If completed, fresh preflight precedes one followup_task; confirm transport/new running turn. Do not redispatch a report already resumed or consumed. Controller claim/applied prevents duplicate side effects.
5. Before exit, preserve actual send/resume return value, fresh state, exact target, report ID and confirmed new turn; send evidence to Controller. Tool acceptance is distinct from completed processing. Record failure/unknown, cause, remaining work and resume condition rather than success. If delivery is unavailable, the durable report records actual transport return/error, fresh target state, stable report/handoff ID, remaining work and resume condition. Include its path and failure evidence in native final so the next recovery can read it. /tmp alone is not durable failure evidence. Before cleanup, Controller must retrieve and retain the report. After successful delivery, confirmed Controller copying/recording of report and transport evidence into the original operational ledger completes durable handoff.

Unsupported native discovery/message/resume requires capability evidence and explicit fallback limits. Task Mecca CLI does not guarantee native automatic notification/resume or force turn termination. Root ACK/wake/continuous polling is never a gate.

## Controller restart and finalization checklist

On resume, do not merely acknowledge and exit. Reconcile first and prioritize completed Workers and merged PR residue before new ready implementation.

1. Compare fresh original contract/state/Agent/scope/assignment/lifecycle, native Worker state/exact attempt, handoff journal/claim and fresh coordinate recovery_queue. An empty recovery_queue does not replace checking completed Workers and merged PRs.
2. Claim with the current exact Controller attempt. already_applied is not repeated; already_claimed resumes remaining steps. Record claim_conflict/contract_changed and resume conditions. A replacement Controller must not arbitrarily mark an old claim applied.
3. Verify code/docs/tests against every acceptance criterion. Resume/reassign remaining implementation through fresh assignment gates and confirm an actual new turn. Record durable hold only for an actual decision/dependency. Executable follow-ups or unprocessed DONE/BLOCKED prohibit quiet exit.
4. Verify required regressions/tests and PR CI; check latest target, dirty state and other writers before serial merge into designated dev. Verify merged SHA ancestry in designated local dev and remote/local sync; preserve conflicts and record resume conditions.
5. Verify Go/Python template synchronization and current installed instruction refresh. Follow migration backup/consent policy for customized managed files and record actual installation sync/re-read evidence.
6. Record original results, validation, summary and actual lifecycle completed, then done/archive. Do not leave a stale completed Agent on doing. Finalize satisfied individual tasks even when the wider queue remains.
7. For linked sources, perform authorized external writeback as sole writer. If interrupted after remote write but before local mark, check remote evidence rather than blindly retrying. Unverifiable writes remain external-synced=unknown with a resume condition; unavailable tools do not revert canonical completion.
8. Identify the exact worktree, preserve uncommitted/untracked/ignored/user data, retrieve and retain handoff-evidence durable reports before cleanup, then clean up safely and record evidence. Never delete/reset original workspace or canonical data.
9. Record separate backlog-finalized/external-synced/transport/validation evidence and residual state, then `handoff mark <HANDOFF_ID> --step applied --result ok --evidence <finalization-evidence> --json`. Optional Root notification needs no ACK/wake.

## Regression evidence: A-19 / A-20

Cover original todo/unassigned versus actual execution/completion, and missing direct Controller delivery leaving doing after Worker completion. Recover using fresh canonical/native/PR/handoff evidence and actual current observations, never fictional historical started/assignment/attempt values.

Actual Codex smoke: kkobugi sent followup_task to a completed Controller after fresh preflight at 2026-10-07 11:10:22 KST and confirmed running. Token: `TM-WORKER-RESUME-20261007-1110`; actual attempt: `run-c644efc406c0ceba`; turn: `01a11420-7e64-7b81-85ec-197e563a16a0`. These are historical evidence, not reusable identities. Stale run3b4/turn01a113ba are not evidence for this smoke.
