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

The command returns read-only policy decisions; it does not discover capabilities, persist the selection, spawn sessions, send messages, or wake Root. The caller must furnish fresh, observed capability evidence; do not label a guessed feature supported. The native runtime and existing handoff/decision/recovery owners execute, mark, and reconcile operations.

For a user decision, Controller first records the hold, reason, and resume condition in the canonical backlog. Only after that does the decision function permit `send_to_root`. Transport success is not a user acknowledgement or approval. If the message feature is unknown/unsupported, use `await_next_root_turn`. Root wake is separate from send.

For registration/Worker completion, continue the existing handoff prepare/claim/mark contract; `handoff_via_native_runtime` is a policy outcome, not proof of delivery. In independent-session modes, missing session message capability causes `retain_handoff_for_recovery`. Resume must re-read canonical state and runtime identity; a previous attempt cannot be blindly reused.

## Independent-session names

Use `DMMDD_NN_` as the common **display** prefix per workgroup, e.g. `D1109_01_Controller`, `D1109_01_Registrar`, `D1109_01_Worker_Digeuda_G1`. Reserve a non-colliding sequence within the project and keep the initial group prefix across calendar days. Root name may be unchangeable, in which case associate Root using internal group identity. Never use the display title to authorize handoff: use project, group, exact session/native identity and runtime attempt.

## Next integration boundaries

1. Add provider-specific capability probes and evidence, including separate `message_root`, `wake_root`, `message_session`, `resume_session`.
2. Persist the selected mode and group identity with a schema and safe session-resume reconciliation.
3. Plug decisions into Root bootstrap and handoff/recovery actions; provider-native action results must feed existing durable journal.
4. Include mode, group and delivery evidence in observability, and test end-to-end against each supported provider.

These integrations are **not** implemented by the read-only mode decision CLI.
