# Exact tool addresses and handoff continuity

An assignment/envelope is preparation. A dispatch request is a request. A real
creation receipt establishes the created tool identity, and an observed START
establishes execution. Reuse `runtime worker-start` and its A72 exact assignment,
native/T3 observation and binding checks; do not create a second START manager.
Never infer START or an interruption cause from `runtime query: not_found`.

Keep these namespaces separate:

| Identity | Evidence | Consumer |
| --- | --- | --- |
| Native session/runtime agent ID | Actual provider session and bound observed attempt | Runtime query/binding and official native naming |
| T3 thread ID | Official creation receipt, exact `t3_thread_read`, inherited structured T3 adapter | `t3_thread_send(threadId=…, mode=auto)` |
| Collaboration agent handle | Actual collaboration creation/discovery receipt in the current tool's scope | `collaboration.send_message` or `collaboration.followup_task` |
| Logical role, worker alias, title | Contract/display metadata | Human organization; never transport identity |

A native UUID is not evidence of a collaboration handle. If the handle is not
observable/addressable, report that limitation. Do not try a role/name fallback,
or create another agent to bypass a failed delivery. Current T3 direct delivery
is supported independently of whether a collaboration handle is available.

## Agent-host workflow

Prepare/inspect the existing content-bound DONE/BLOCKED handoff through the
existing Controller-owned workflow. Read its transport directive:

```sh
task-mecca handoff transport-plan EXACT_HANDOFF --project ORIGINAL --json
```

This new read-only command verifies the existing contract, source START and exact
assignment, target binding, current native episode and tool-specific address.
For native-only observations it returns `collaboration_handle_not_observed`, not
a UUID disguised as a handle. Holds, already claimed/applied handoffs, stale
episodes and unsupported resume capability are not send instructions. Repair
through the existing observe/prepare/retarget/transfer recovery, keeping the same
report/handoff identity. A claimed report continues its existing recovery.

For a supported plan, the agent must use its actual native tools. Re-read the
exact recipient thread, compare its current episode/native observation, and run
fresh canonical `preflight --require-full-access` immediately before a completed
target `t3_thread_send(mode=auto)`. An automatic-mode send can race with target
completion; the host adapter also checks Full Access when the fresh read says
completed. Preserve the actual tool result. A changed episode requires fresh
observation and existing-handoff recovery, not stale-attempt reuse.

`NativeTransport`/`DeliverReport` is an embedding interface for a host that
actually exposes those native tools; this repository does not install an MCP
bridge or pretend a Go process can invoke the conversation's tools by itself.
There is intentionally no CLI for importing arbitrary successful tool JSON.
The adapter's tests use offline tool-contract fixtures, not real delivery.
Hosts must pass the actual structured tool result and retain original errors.
Only explicit tool rejections use `RejectedSend`; timeout/disconnection remains
delivery-unknown. Unsupported host/runtime means a durable stop with an owner
and resume condition, never an unconditional wake claim.

The adapter writes a content-bound checkpoint under
`_task_mecca/.runtime/handoffs/transport/`. It saves `sending` before the effect,
then the exact receipt, original/last error, report hash, recipient, native ID,
attempt, action owner and resume condition. Concurrent calls have an exclusive
lock without timer takeover. Explicit rejection retries the same checkpoint
after repair; acknowledgement deduplicates DONE. `sending`/`delivery_unknown`
requires reconciliation and cannot be blindly resent. No creation API is called.

Transport acknowledgement proves acceptance by the transport only. It does not
prove that the Controller received/read the report, claimed it, reviewed it or
applied it. The Controller independently observes its current attempt, claims
the existing handoff, verifies the result and contract, records review, completes
canonical finalization, marks its existing handoff and checks `ready`/dependency
and recovery queues before the next authorized dispatch. Root ACK is not a gate.
The transport adapter cannot perform these Controller-owned operations.

## Historical diagnosis

Read-only empfund evidence on 2026-11-13 records B634 START/DONE
`collaboration.send_message(target=01a1c465-2fc0-7d62-b255-1ffc0e90d7ee)`
returning `agent with id … not found`; B636 followup transport also failed.
That UUID was a native runtime target, without an observed collaboration handle.
Offline fixtures reproduce the error and enforce the namespace boundary; no
message is sent to empfund.

The user's earlier report described an execution/handoff gap. Later B634/B636
archive-DONE and `controller-final-validation.json` show integration and remote
verification. Those later facts do not erase the earlier gap. B637 has an
assignment/envelope and an unobserved runtime; `not_found` alone does not establish
that it never ran. The technical cause of runtime interruption remains unknown.
These statements authorize no empfund business execution or remote action.
