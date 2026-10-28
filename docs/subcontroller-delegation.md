# Sub-Controller delegation — initial decision contract

Status: **foundation only**. This module does not spawn agents or authorize canonical writes.

## Rationale
In observed Task Mecca usage, only **two or three overlapping** registrations and review completions can occupy the Main Controller for a substantial amount of time. Design and benchmark the small-concurrency case, rather than relying on large batches.

## Command

```sh
echo '{"backlog_id":"B-563","actor_role":"main_controller","event":"worker_completed","execution_authorized":true,"runtime_known":true,"pending_review":2,"max_subcontrollers":2}' | task-mecca subcontroller decide
```

This command reads one JSON input from stdin and emits a **side-effect-free** decision. No sub-agent is spawned; no handoff, approval, lifecycle or backlog is written. Inputs are assertions supplied by the caller, not independently observed canonical evidence. Never use a JSON result as dispatch authorization.

## Policy contract
- Empty backlog IDs or unknown actors are errors.
- Without explicit execution authorization, defer. With unknown runtime identity, escalate.
- Contract change or scope conflict overrides workload optimization and escalates.
- Two or more pending events may propose a delegation; one pending event stays with Main. A disabled pool stays with Main.
- Pool capacity prevents new proposals. Reuse and eventual queueing are future execution responsibilities.
- Sub-Controller review completion only **recommends** acceptance when evidence and mandatory independent review are ready. Main must re-check current contract and complete finalization.
- Canonical writes are forbidden by the output contract for all decision variants. This does not replace server-side policy checks.

## Next implementation boundaries
1. Durable delegation contracts and atomic lease/claim with exact provider/runtime identity; never trust agent-provided actor roles alone.
2. Real sub-agent creation and handoff, recovery/reclaim on interruption, scoped Worker dispatch.
3. Decision-based bounded pool, event deduplication and operational visibility.
4. Compare current Main-only handling against sub delegation at **1, 2 and 3 concurrent tasks** using Dev real operation. Keep empirical verification separate from the implementation-done definition of Linear issues.

Maintain the prior Main-only route until the execution layer is genuinely implemented and enabled.
