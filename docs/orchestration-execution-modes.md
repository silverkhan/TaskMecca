# Orchestration execution modes (A/B/C/D)

Task Mecca keeps one logical Root, Registrar, Controller, and Worker protocol regardless of how a provider runs agents. A session title is not an immutable transport identity. The canonical backlog, approval history, handoff evidence, role ownership, and exact runtime attempt binding remain authoritative.

## User-facing options (stable letter labels)

| Code | Label | Execution | Root communication |
| --- | --- | --- | --- |
| A | **A. 기본 서브에이전트** | Subagent | Off; durable hold / next Root turn |
| B | **B. 서브에이전트 + Root 통신** | Subagent | On, only when verified |
| C | **C. 독립 세션 + Root 통신** | Independent sessions | On, only when verified |
| D | **D. 독립 세션 (Root 통신 없음)** | Independent sessions | Off; durable hold / next Root turn |

Present A/B/C/D at startup; discover actual provider capabilities before enabling options. Root selection is explicit, scoped to the Root/workgroup, and must not silently change on session resume. An unsupported/unknown capability is NOT a supported capability. Never auto-switch to a different mode without user consent.

## Decision function (implemented)

```sh
task-mecca orchestration modes
task-mecca orchestration decide --mode B --event root_initialized \
  --capabilities '{"spawn_subagent":"supported","message_root":"unknown"}' --json
task-mecca orchestration decide --mode B --event user_decision_required \
  --capabilities '{"spawn_subagent":"supported","message_root":"supported","wake_root":"unknown"}' \
  --durable-recorded --json
```

The `decide` command returns read-only policy decisions; it does not discover capabilities, spawn sessions, send messages, or wake Root. The separate `configure` command persists explicitly confirmed selections. The caller must furnish fresh, observed capability evidence; do not label a guessed feature supported. The native runtime and existing handoff/decision/recovery owners execute, mark, and reconcile operations.

For a user decision, Controller first records the hold, reason, and resume condition in the canonical backlog. Only after that does the decision function permit `send_to_root`. Transport success is not a user acknowledgement or approval. If the message feature is unknown/unsupported, use `await_next_root_turn`. Root wake is separate from send.

For registration/Worker completion, continue the existing handoff prepare/claim/mark contract; `handoff_via_native_runtime` is a policy outcome, not proof of delivery. In independent-session modes, missing session message capability causes `retain_handoff_for_recovery`. Resume must re-read canonical state and runtime identity; a previous attempt cannot be blindly reused.

## Independent-session names

Use `DMMDD_NN_` as the common **display** prefix per workgroup, e.g. `D1109_01_Controller`, `D1109_01_Registrar`, `D1109_01_Worker_Digeuda_G1`. Reserve a non-colliding sequence within the project and keep the initial group prefix across calendar days. Root name may be unchangeable, in which case associate Root using internal group identity. Never use the display title to authorize handoff: use project, group, exact session/native identity and runtime attempt.

## Implemented persistent Root selection and evidence-driven notification

New runtime capabilities are recorded through the existing handoff capability registry, with evidence mandatory when marking a new orchestration capability as supported. Provider names never imply native session creation or Root messaging.

```sh
task-mecca handoff capability record --project . codex can_spawn_subagent supported --evidence NATIVE_SMOKE_RESULT --json
task-mecca handoff capability record --project . codex can_message_root supported --evidence NATIVE_SMOKE_RESULT --json
task-mecca orchestration modes --project . --provider codex --json
task-mecca orchestration configure --project . --provider codex --root-session-id EXACT_ROOT_SESSION --mode B --confirmed --timezone Asia/Seoul --json
task-mecca orchestration status --project . --provider codex --root-session-id EXACT_ROOT_SESSION --json
task-mecca orchestration name --project . --group D1109_01 --role worker --worker Digeuda --generation 2 --json
```

Selections are persisted under `_task_mecca/data/orchestration/groups/` and are bound to `provider + exact root session ID`; they survive Root resumes and are not overwritten by framework updates. Group allocation is concurrency-safe and gives all displayed independent-session names a common prefix. A change requires `--change-mode --decision-evidence USER_REFERENCE --confirmed`. The native provider must still use the returned name when it creates/renames an actual session.

`orchestration notify` checks the exact canonical user hold, resume condition, bound Controller and Root attempts, saved mode and verified capabilities. It prepares the **existing handoff journal**, and returns the native message/resume action if safe. It never asserts a send, receipt, wake or user approval without provider-side evidence.

```sh
task-mecca orchestration notify --project . --provider codex --root-session-id EXACT_ROOT_SESSION --task B-620 \
  --controller-attempt EXACT_CONTROLLER_ATTEMPT --root-attempt EXACT_ROOT_ATTEMPT --json
# The live provider must actually send/resume, then record the result using:
task-mecca handoff mark HANDOFF_ID --project . --step dispatch --result ok --evidence ACTUAL_NATIVE_TRANSPORT_PROOF --json
```

## Direction-driven operation (selected design)

Task Mecca intentionally does **not** implement provider-native session creation or messaging transports. Its existing decision functions and handoff/recovery contracts direct the executing Root, Registrar, Controller or Worker to call the collaboration tools actually supplied by that environment. Do not build a duplicate Codex/Claude session manager.

For Root decisions, run `task-mecca flow next --role controller --event user_decision_required ...` to select the next action. Once the canonical user hold and exact runtime identities are available, run `task-mecca orchestration notify ... --json`. When the handoff is safe, its `directive` includes:

- `actor_role`, `actor_attempt_id`: who must act
- `target_role`, `target_provider`, `target_session_id`, `target_attempt_id`, `target_runtime_agent_id`: whom to contact (never select by the display title)
- `transport_action`: send to running session, or resume completed Root after fresh preflight if *wake capability is verified*
- `message`: task ID, exact handoff ID, blocking reason, resume condition, instructions to ask the user
- `after_actual_delivery_command`: command to record actual transport evidence **only after** the Agent has performed it
- `on_transport_failure`: durable fallback when the native tool is not available or reports failure

A returned directive is a task instruction for the live Agent; it is not evidence that a provider message was sent, received, or approved. The CLI must never manufacture a native result or set `delivered=true` before transport proof. Agents must call the decision function again at role/event transitions rather than relying on stale conversational memory.

The same boundary applies to independent-session creation: the decision engine directs a running Root/Controller to use the available native create-session tool with the group's display name, then records the observed native session and attempt identities via existing runtime procedures. If the environment does not support creation/communication, reject that mode instead of pretending a session exists.

## Remaining integrations and verification

1. Provide provider-capability evidence from actual native smoke tests; automatic capability discovery is optional, not a prerequisite for direction-based orchestration.
2. Expand event/role coverage in the existing `flow next` decision interface and ensure role instructions call it at every relevant transition; the binary cannot force the host Agent to invoke tools.
3. Extend observability to project the selected mode, group and message acknowledgement alongside live sessions.
4. Validate that Agent-executed native session creation, Root delivery, resume and fallback obey returned directives in each supported environment.

Policy, mode persistence, display-name generation and exact-identity Root handoff **preparation with explicit Agent directives** are implemented. Native tool execution is deliberately owned by the runtime Agent; message acknowledgement and Root wake are not claimed without observed evidence.

## Official metadata naming workflow

`orchestration name` remains the only name generator. Run `orchestration session-name plan` before **both newSession and existingSession dispatch**, then include all generated before/after/resume directives in the actual agent task. Apply/verify in the exact current native session after dispatch. Codex uses the installed official app-server schema and metadata-only `thread/name/set` plus `thread/read`; Claude uses documented SDK `renameSession` plus `getSessionInfo().customTitle` when those exports are actually installed. No creation, model turns, messaging, authorization, or operational ledger writing is added.

T3 `delegate_task.title` uses the same generated string at creation. The acting agent must separately read the exact current T3 thread with official tools, update only if needed and reread. The CLI never trusts caller JSON as T3 or provider verification. It emits pending T3 verification even after successful native verification. Same native session resumes preserve group/role/worker/generation; generated-name identity conflicts fail without a write. No bulk historical rename is provided.

Commands, mandatory role steps, status meanings and primary source citations are in the managed-source [execution protocol](../goassets/template/_task_mecca/framework/EXECUTION_PROTOCOL.md#공식-세션-이름-적용과-확인-필수). Update the canonical `goassets/template` sources only; use the existing installer/updater consent and backup policy for installed projects. No global or running-project framework sync is implied by a source edit.


## Environment-aware dispatch boundary (A-76)

At Root startup and resume, collect current process metadata with
`orchestration environment --provider codex --root-session-id EXACT_CURRENT_NATIVE --json`.
The command reads only the selected provider's current identity and the current T3 environment marker.
It does not enumerate MCP tools or prove native/T3 linkage: tools and tracking remain unknown.
An absent T3 process marker leaves the host unknown; it does not prove a native environment.
The host agent independently reads its actual callable schemas and exact native/T3 linkage,
then supplies a minimal `Environment` profile: host (`t3`, `native`, `other`, `unknown`),
provider, exact Root native session and T3 thread IDs, separately observed collaboration handle,
creation/communication tool support (`supported`, `unsupported`, `unknown`), tracking visibility,
source and checked-at timestamps. A missing collaboration handle remains missing.
Collect no prompts, secrets, transcripts or raw provider audit files. Do not copy identities from a display name.

Use the existing decision engine, rather than deciding transport from conversational memory:

```sh
task-mecca orchestration decide --project CANONICAL --provider codex \
  --root-session-id EXACT_NATIVE --root-t3-thread-id EXACT_T3 \
  --event dispatch_requested --environment PROFILE.json --separate-worktree \
  --source-cwd ABSOLUTE_SOURCE_CWD --base EXACT_COMMITTED_SHA --json
```

`PROFILE.json` accepts either an Environment object or the environment command's wrapper.
This is **diagnostic policy input** even if it contains `verified`, `created` or `execution_authorized` assertions.
`dispatch.execution_authorized` remains false. `allowed` means the requested policy route can be planned;
it does not mean that a tool exists, a session was created, Full Access passed, or work may start.
Saved exact-Root mode is reused and a conflicting requested mode is rejected. Existing A–D selections,
name allocation, direction-based communication and recovery stay in their original owners.
Legacy startup calls remain compatible but explicitly require environment evidence before actual dispatch.

T3 separate worktree selects `delegate_task` with `isolation=worktree`, including independently observed
schema support for that isolation. Missing support blocks the request with evidence/resume guidance;
there is no native, provider, model, settings or mode fallback. Session modes C/D have no assumed
T3 subagent/worktree route. Shared subagent work selects the actual native tool only when observed supported.
For non-T3 environments the same explicit support checks apply; native separate-worktree support is not invented.
Native children may be absent from T3 Thread Details/Lineage. Verify them through the actual native
creation receipt and the supported status/read tool on its exact returned handle. Record that visibility limit.
T3 dispatch requires an independent exact `t3_thread_read` after creation and verified parent linkage,
native ID, actual cwd/worktree and committed base. A collaboration handle is not a native ID or T3 ID.

`HostAdapter` is the typed boundary for an actual tool-owning host integration. It is not a CLI-loaded JSON
adapter, provider manager or an implementation of MCP transport. `CheckDispatchBoundary` re-observes the
exact environment and current tool schema, then fresh canonical Full Access within the same check.
The host must call it immediately before the actual tool call, while preserving contract/runtime/assignment/handoff gates.
No concrete host dispatch is performed by the CLI. Follow the decision's instructions with the real host tools;
unsupported or unobserved host integration stays unverified rather than becoming an apparent success.

`VerifyCreation` takes a real creation receipt and independently reads it through the host adapter.
It checks exact native/T3 IDs, selected provider, source cwd, separate worktree and committed base.
Created-but-unlinked is a partial result, never START, tracked success, or proof of a technical interruption.
START/binding and Activity remain the existing A-72 procedures; typed communication and source/display
separation remain A-74/A-75. Dispatch request, assignment preparation, creation, START and tracking proof
are distinct evidence boundaries. Re-observe after resume, environment change or identity conflict;
profiles expire after five minutes, and cached support is never dispatch authority.

An existing instruction violation, missing product environment input, and an unknown historical interruption
cause are different findings. Preserve the evidence for each; do not infer a historical cause or phase from
missing collection. Dev integration/CI/release/operating installation also remain separate Controller-owned results.
