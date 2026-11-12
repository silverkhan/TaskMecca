# A-67 implementation DONE — verified T3 lifecycle recovery

Stable report ID: A67-KKOBUGI-RECOVERY-20261112. This reports implementation and
isolated validation, not canonical acceptance/applied, original-ledger recovery,
or A-66 dispatch. Controller remains sole operational writer.

## Actual identity, base, authority and transport

- Native CODEX_THREAD_ID: `01a1ce1e-3541-7733-8ad1-ab87bb4a320a`.
- Actual T3 Worker thread: `thread:delegated-task:command%3Amcp%3A17c02103-a2d4-43df-b4c5-96ede8dc66b1%3Adelegate-task%3Aa67-kkobugi-runtime-recovery-20261112`.
- Worker native episode: `run-attempt:run:run%3Athread%3Athread%253Adelegated-task%253Acommand%25253Amcp%25253A17c02103-a2d4-43df-b4c5-96ede8dc66b1%25253Adelegate-task%25253Aa67-kkobugi-runtime-recovery-20261112%3Aordinal%3A1:attempt:1`.
- Tool-prepared cwd: `/Users/hanati/.t3/worktrees/TaskMecca/t3-a5575212-76ca375289b1a21b`.
- HEAD at START and verified origin/dev base: `f102b6a22414b9262f4255f636f2950c4e4e9564`.
- Implementation commit: `a511caeb7a620e464643db2ba197967503a5a18b`. No push, PR, merge or deploy.
- START direct Controller send accepted: message `message:mcp:5356efb1-b2b0-4acd-8851-e7fa57a68029:thread-send:6e740d20-ea63-4ee5-9bd9-d6c34a074f17`, returned running/steered and ordinal 4. Acceptance is not proof of Controller processing.
- Fresh effective preflight before implementation returned orchestration_ready=true at 2026-11-12T13:58:45+09:00. A second actual effective preflight is preserved in `preflight.json`: checked_at=2026-11-12T14:13:11+09:00, orchestration_ready=True.
- A-67 user exception was used only for isolated implementation/testing. Original `handoff-36880e0cfe69d25a2d6b` was never claimed, changed or treated as recovered by this Worker. No extension to A-66.

Read the specified canonical framework contract, manifest and A-67 doing entry.
No original/copied Task Mecca operational ledger was written/registered, apart
from the expressly allowed original preflight ephemeral probe/cache. All runtime,
handoff/lifecycle/Activity test writes used generated temporary projects. No
primary dirty files, A-65/A-66 implementation, other worktrees, installed CLI,
operating Web or canonical framework were changed. No child agents were started.

## Acceptance evidence

1. **Exact verifiable current identity:** implemented `runtime observe-t3` using
   the available local T3 v2 structured projection and live server-owner lock,
   distinct `t3_structured` source, exact provider/native session/run-attempt,
   origin and observation timestamps. State is fetched from T3, not imported
   from arbitrary JSON. Wrong session/provider/runtime/owner, stale observation,
   unsupported schema/state, future timestamps, source races, old terminal
   attempts, ambiguous matches and unsupported sources fail closed. Repeated
   receipts are idempotent; retries and resumes create distinct execution
   episodes. Current binding is preserved on refresh. Tests also cover observing
   before binding and a prepared-but-not-yet-started resumed dispatch.
2. **Observed Hook gap and same held handoff recovery:** Codex CLI 0.167.2 and
   installed global/project Hook configuration were observed. Native diagnose
   returned `native_hook_observation_gap` with provider Hook count zero while native
   session metadata/start/turn counts were present. `controller-native-counts.json`
   contains event counts only, never native transcript content. This establishes
   an unobserved Hook invocation/delivery gap in this harness; no unsupported
   provider callback/API was invented, and no claim is made that Hook delivery
   itself was repaired. The explicit supported local T3 observation path closes
   the evidence gap. Same-attempt held Prepare now revalidates, and verified
   same-native-session episodes retarget the original unclaimed handoff.
   Contract reload/fingerprint, original authorization identity, exact binding
   and live Claim checks prevent approval or identity bypass. Actual candidate
   CLI evidence against this running Worker and a generated temporary project:
   pending attempt `run-3d84973186ef20ef` had no provider/session;
   observed attempt stayed `run-3d84973186ef20ef`;
   handoff `handoff-edd6fae38275d5b3fdf3` changed from hold to message_running under
   the SAME ID; Claim returned claimed=true. See `actual-t3-temp-project-cli.json`.
3. **True lifecycle/Activity/UI paths:** six-phase durable lifecycle integration,
   duplicates/retries, assignment without start, exact attempt resume, Activity
   identity/missing phase behavior and Web read-only projections pass. The UI
   diagnostic exposes separate T3 transitions and keeps Hook count zero. Runtime
   observations do not fabricate Activity phase measurement. The existing direct
   worker_report/controller_report lifecycle commands remain distinct sources.
4. **History preserved and A-66 readiness:** a generated A-66 regression fixture
   retains exactly registered+waiting through repeated recovery queries, without
   assigned/started/completed backfill. Candidate tests exercise the repaired
   normal registration gate through verified live Claim, followed by same-session
   resume/retarget. The Controller must now use the documented candidate on the
   original ledger, verify the original A-67/A-66 handoffs and actual approvals,
   and dispatch a real new A-66 Worker only through the repaired gate. This Worker
   was explicitly prohibited from doing that operational step; no A-66 Worker
   start or original Claim is represented as already observed.

## Candidate and precise operational workflow

Candidate: `/Users/hanati/.t3/worktrees/TaskMecca/t3-a5575212-76ca375289b1a21b/.artifacts/task-mecca-a67`

SHA-256: `018ef88617dc38422278f0342ecf9e7a0b502867000cc7dea8bd09b9da4b7acb`

Built from implementation commit `a511caeb7a620e464643db2ba197967503a5a18b` with explicit `go build -o`;
not installed globally. Actual candidate temp-project recovery was rerun after
this build. `.artifacts/` is deliberately untracked and retained.

Use `docs/cli-runtime-execution-recovery.md` for exact commands and receipt JSON
schema. It includes the real Controller T3 thread/native session
`01a1ce01-277d-73d0-8b25-b8992cc5ae2a`, current-run changes caused by direct
messages, the original A-67 handoff, A-66 original-ID selection, fresh Full Access,
Prepare contract/authorization preservation, running-source Claim and actual
lifecycle/Activity recording. No receipt JSON import exists. Save actual command
output; do not hand-author state/timestamps. A terminal target requires actual
native resume, fresh observation and same-handoff reprepare before Claim.

## Validation and limits

- `TMPDIR=/private/tmp go test ./internal/runtimeobs ./internal/handoff ./internal/backlog ./internal/activity ./internal/webui ./cmd/task-mecca`: passed.
- Same six packages with `-race`: passed; `race-suite.txt`.
- Final runtime/handoff `-race` after binding preservation hardening: passed; `final-identity-race.txt`.
- Final CLI `-race` after parser/command changes: passed; `final-cli-race.txt`.
- Explicit candidate build: passed. Actual T3 dry-run: verified=true/written=false.
- Actual T3 CLI temp-project pending -> observation -> same-handoff recovery -> Claim: passed, including taskless `bind-agent ... - ...`.
- `git diff --check`: passed.

The first broad CLI run using macOS's default `/var/folders` temporary path failed
existing worktree-cleanup tests because `/var` is a symlink. Using canonical
`TMPDIR=/private/tmp` passed those tests; no worktree protection was weakened.

The adapter intentionally supports the observed local Codex/T3 v2 schema and
requires `sqlite3` plus T3's live exclusive owner lock. Other providers, remote
machines or changed T3 schemas fail closed, rather than guessing. It trusts
T3's local owned projection, not a user-imported receipt; malicious replacement
of the local user's T3 files is outside that trust boundary. Native T3 can finish
between any two operations: revalidation narrows races but cannot atomically
fence an external scheduler. A stale/changed source must be re-observed, never
forced. Claim requires a running current T3 source and a <=2-minute receipt.
Hook delivery remains independently unobserved; counts are kept honest.

## Full implementation diff file list

- `cmd/task-mecca/main.go`
- `cmd/task-mecca/runtime_observe_t3_cli.go`
- `cmd/task-mecca/runtime_observe_t3_cli_test.go`
- `docs/cli-runtime-execution-recovery.md`
- `internal/backlog/lifecycle.go`
- `internal/backlog/lifecycle_recovery_test.go`
- `internal/handoff/held_revalidation_test.go`
- `internal/handoff/resolve.go`
- `internal/handoff/service.go`
- `internal/handoff/t3_recovery_test.go`
- `internal/runtimeobs/execution_evidence.go`
- `internal/runtimeobs/ledger.go`
- `internal/runtimeobs/t3_receipt.go`
- `internal/runtimeobs/t3_receipt_test.go`
- `internal/webui/activity_runtime_diagnostic.go`
- `internal/webui/activity_runtime_diagnostic_test.go`

Additional durable report/evidence files under `handoff-evidence/a67-kkobugi/`
are separate from every operational ledger. The subsequent documentation/evidence
commit contains this report and actual JSON/test receipts. The final direct DONE
transport receipt is saved separately after send acceptance; it cannot prove
Controller processing. No PR was requested or allowed.
