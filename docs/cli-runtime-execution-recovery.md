# Verified T3 execution recovery (A-67 candidate)

`runtime observe-t3` observes the installed local T3 v2 structured projection. It
is an explicit alternative to Hook delivery, with `evidence_source=t3_structured`
and `native_receipt.origin=t3_local_projection_v2`. It never writes Hook events,
never starts/resumes a provider, never accepts a JSON state assertion, and never
approves or assigns a task. Global/project Hook configuration still does not
prove that Hooks fired.

The adapter uses the available local `sqlite3` executable in read-only mode,
against `$HOME/.t3/userdata/statev2.sqlite`. It requires the live T3 server's
exclusive `server-owner.sqlite` lock. There is deliberately no database-path or
receipt-import flag. The adapter supports the observed Codex/T3 v2 schema only;
missing SQLite, changed schema, unlocked/absent owner, ambiguous/missing rows,
archived/deleted thread, conflicting native session, inconsistent run/attempt
state, missing/future timestamps and unsupported states fail closed. This is a
local structured-store adapter, not an invented T3/provider HTTP API. A local
user able to replace T3's own files is outside its trust boundary.

It selects the latest T3 run ordinal and latest execution retry, joins its
provider-thread owner, and verifies the Codex `nativeThreadId` recorded by T3.
For this adapter, `runtime_agent_id` is the exact native `CODEX_THREAD_ID`, not the
T3 thread ID and not T3's shared provider-session ID. Each native run-attempt
(including retries within one run) has its own Task Mecca execution episode.
An exact pending dispatch can be enriched in place once. Subsequent episodes
preserve the same native identity and existing dispatch binding, close only
independently verified terminal predecessors, and cannot reopen old attempts.
The source is sampled twice under the observation lock. Dispatch/binding writes
use that same lock. Source changes require a retry. Hook observation controls
also apply to this explicit observation path.

## Controller workflow with an explicit candidate

Only the canonical Controller writes the original operational ledgers. Workers
must use temporary test projects. Do not install the candidate globally. The
following commands do not imply approval or native delivery:

```sh
candidate=/absolute/worktree/.artifacts/task-mecca-a67
project=/Users/hanati/projects/TaskMecca/TaskMecca
controller_thread='thread:delegated-task:command%3Amcp%3Ae9d69a54-b3b2-4159-8c18-243fb36a0168%3Adelegate-task%3A8e21d087-7a4d-402e-a389-fe73c8003fca'
controller_native=01a1ce01-277d-73d0-8b25-b8992cc5ae2a
"$candidate" preflight --project "$project" --require-full-access --json
"$candidate" runtime diagnose --project "$project" --provider codex --session-id "$controller_native" --json
"$candidate" runtime observe-t3 --thread-id "$controller_thread" --session-id "$controller_native" --agent-id "$controller_native" --dry-run --json
"$candidate" runtime observe-t3 --project "$project" --thread-id "$controller_thread" --session-id "$controller_native" --agent-id "$controller_native" --json
```

Read `attempt.attempt_id` from the actual observation output. No caller-authored
state file is consumed. `--run-id EXACT_T3_RUN` optionally pins a just-observed
run and rejects a concurrent wake; omit it to resolve the current latest run.
A `thread_send(mode:auto)` can wake the same thread into a new run. Re-observe
and use the new returned attempt instead of copying a prior run ID. A terminal
receipt is terminal evidence, not permission to call the session running.

Inspect the **existing** handoff and its contract/authorization. For A-67 retain
`handoff-36880e0cfe69d25a2d6b`; for A-66 use its existing registration handoff,
not a newly generated replacement. Re-run its original Prepare arguments,
including its original source, event, authorization and exact contract hash:

```sh
"$candidate" handoff inspect EXISTING_HANDOFF --project "$project" --json
"$candidate" handoff prepare A-67 --event registration_ready --from /root/registrar --to /root/controller --target-attempt ACTUAL_CURRENT_ATTEMPT --contract-sha256 ORIGINAL_CONTRACT_SHA --execution-authorized --project "$project" --json
# For A-66 repeat with A-66 and its own original contract and approval.
"$candidate" handoff claim EXISTING_HANDOFF --as /root/controller --source-attempt ACTUAL_CURRENT_ATTEMPT --project "$project" --json
```

Confirm the returned handoff ID equals the inspected original, and its action is
appropriate. Do not reuse the example source if the original differs. Recovery
preserves original approval identity and contract; a false authorization remains
report-only. Same-attempt holds revalidate; verified same-native-session resumes
retarget the same unclaimed handoff. Claimed/applied handoffs are not stolen.
Contract changes require their own authorized resolution. Existing claim transfer
semantics remain unchanged. Prepare reloads the contract under the handoff lock.
Claim rereads the live source and refuses stale (>2 minutes), changed, terminal,
foreign-role or foreign-session T3 execution. A native execution can finish at
any time; no filesystem journal can fence the external T3 scheduler atomically.
A refusal means inspect/observe/retry, not edit a journal.

If the target is completed, honor the existing `resume_completed` requirement:
fresh Full Access preflight, actual native/T3 resume, observe the new running
execution, prepare the same registration again for that new attempt, then claim.
A-67's one-off exception does not authorize bypassing the repaired A-66 gate.
Do not mark acceptance/applied until actual review/finalization is done.

## Evidence output and lifecycle

Successful observation returns `verified`, `written`, `attempt`, and `receipt`;
dry-run returns `verified`, `written=false`, and `receipt`. The receipt schema is:

```json
{
  "thread_id": "exact T3 thread",
  "run_id": "actual current T3 run",
  "native_attempt_id": "actual T3 run-attempt/retry",
  "session_id": "native CODEX_THREAD_ID",
  "provider": "codex",
  "state": "running|completed|interrupted|errored",
  "started_at": "T3 native attempt start RFC3339Nano",
  "completed_at": "optional actual T3 terminal RFC3339Nano",
  "observed_at": "fresh local query RFC3339Nano",
  "origin": "t3_local_projection_v2"
}
```

Save this **actual command output** as durable transport/recovery evidence if
needed. `observed_at` is the actual new observation boundary. Native historical
start/end timestamps remain separate receipt fields; the observer does not
backdate lifecycle records. Repeating the same receipt/observation boundary is
idempotent, while fresh observations can update runtime freshness.

The Controller records separate true observations with stable IDs using the
existing `lifecycle record registered|assigned|started|waiting|resumed|completed`
commands. Use `worker_report` or `controller_report` for the actual report; cite
the exact assignment, attempt and receipt/transport evidence. Receipt collection
alone does not manufacture any of these lifecycle reports. Assignment is not a
start; a receipt is not Activity phase measurement. Report actual Activity phase
boundaries via existing `activity report` with exact role/agent/attempt/assignment.
The read-only Activity diagnostic exposes `state_evidence_source`, separate
`recent_t3_transitions`, and unchanged Hook counts. Missing Activity remains
missing; elapsed runtime does not become measured work.

A-66's historical events remain exactly registered + waiting. No assigned,
started or completed history is backfilled during Controller recovery. Only
after a real new A-66 Worker is dispatched through the repaired gate should the
Controller record its new assignment and actual Worker START/receipt, followed
by true waiting/resumed/completed facts as they occur. Numeric shortcut
implementation belongs to that A-66 Worker, outside A-67.
