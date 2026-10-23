# Decision Engine P0/P1 — Controller Worktree Finalization Gate

## Why

B-541: a Controller copied 456 output files, verified their hashes and recorded `source_worktree_retained=true`, then exited without a cleanup follow-up. Six of seven worktrees remained after completed work; B-542 is still in progress. `git status` appeared clean but ignored outputs survived. Five branches were cherry-picked, so their HEAD is not an ancestor of the integration branch.

This is a **continuation/decision omission**, not a reason to weaken Git deletion protections or delete an active worker's files.

## Contract and state ownership

| Source | Ownership | Interpretation |
|---|---|---|
| Canonical backlog | Original project Controller writer | `done` proves original task finalized, **not** cleanup |
| Git worktree plan | Existing read-only scanner | exact path, HEAD ancestry, dirty/ignored/untracked, usage unknown |
| Protected archive manifest | Existing `worktree preserve` CLI | files copied, SHA-256; **not** removal |
| `removal.json` + absent worktree + archive SHA | Existing safe `worktree remove` CLI | exact worktree removal evidence |
| Controller report | Natural-language context | not authoritative for Git state or deletion |
| Cleanup follow-up task/hold | Original canonical writer | owner/reasons/evidence/next attempt; not implemented as durable structured engine record in P1 |

The engine does **not** touch Git or operator files, start agents, make approvals, create backlog items or intercept agent final. `flow next` is advisory/read-only, and never reports a general Controller `exit_allowed=true` in this P1 slice. Provider-native exit enforcement, structured blocked-followup receipts and full role policies remain planned under AID-221~223.

## CLI contract

```bash
task-mecca flow next --role controller --event final_requested \
  --project /original/repo --task B-541 --into main \
  --worktree /exact/worker/worktree --archive /external/archive --json
```

Result schema: `schema_version=1`, `decision`, `exit_allowed`, `task_id`, `task_state`, `worktree`, `cleanup_status`, `evidence_quality`, `reasons[]`, `next_action`, `commands[][]`, `plan_digest`, `written=false`.

| Input/snapshot | Decision | Cleanup status | Required next action |
|---|---|---|---|
| Unknown/duplicate task or Git unreadable | EXIT_BLOCKED | unknown | Correct task/source identity |
| B-542 doing | CONTINUE | not_due | Finish real work, no cleanup |
| B-541 done, archive missing | EXIT_BLOCKED | retained/unknown | Plan then preserve and verify |
| B-541 done, copied 456 files and archived but source exists | EXIT_BLOCKED | preserved_retained | Investigate cleanup next action |
| `git status` clean but ignored/untracked payload remains | EXIT_BLOCKED | preserved_retained | Do not force-remove; reconcile and record follow-up |
| Worker commit cherry-picked into main (`unmerged_head`) | EXIT_BLOCKED | preserved_retained | Check integration by independent evidence, do **not** fabricate ancestry or force remove |
| B-541 exact target missing but no valid receipt | EXIT_BLOCKED | missing_removal_receipt / unknown | Investigate and preserve evidence |
| Git target absent, exact receipt valid, archive hash verified | CONTINUE | removed_verified | Cleanup **subgate** cleared; evaluate other handoffs, workers and external sync |
| Unsupported role/event | EXIT_BLOCKED | unknown | Use existing role protocol until implemented |

**No automatic deletion.** Normal cleanup remains `worktree plan → preserve → verify → remove`; removal requires exact target, matching plan digest, clean/non-detached/merged Git state, protected archive and explicit `--approve --confirm-unused`.

## P2/P3 required next development

1. Durable, idempotent `cleanup_deferred` records with task/worktree/HEAD/plan digest, assigned owner, reason, source evidence, created timestamp, due/review trigger, and resumption/closure receipt.
2. Controller final gate reads **all** outstanding cleanup obligations for its Worker worktrees and blocks exit if any has neither removal receipt nor verified delegated follow-up. A tracked deferred obligation must not keep the completed task in `doing`.
3. Recovery queue observes remaining worktrees independent of active agents, and alerts once per meaningful change. B-542 must remain protected.
4. Support verifiable cherry-pick equivalence only through a separate explicit integration-evidence contract; it must not convert `unmerged_head` into automatic safe-to-delete without fresh safety checks.
5. Tests against 6 retained completed worktrees, B-542 active, ignored artifacts, 456-file preservation, crashes, duplicate calls, user locks and TOCTOU. No real worktree deletion in CI except isolated fixtures.

## Compatibility and release policy

- Phase-one CLI **does not** implement the whole AID-220~224 decision engine. Unsupported role/event is fail-closed.
- Existing project framework documents are distributed as templates; installed projects require their approved migration/refresh process, not a silent replacement.
- Dev-first; Stable requires separate approval. Never claim user environmental UAT without evidence.
