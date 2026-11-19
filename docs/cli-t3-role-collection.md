# Formal T3 role collection and B-639 recovery

Managed creation, official thread read/name verification, execution observation,
assignment binding, direct delivery, handoff claim, acceptance and applied are
separate facts. A title or a successful `t3_thread_send` (`steered` included) is
neither execution collection nor claim. Keep the original error and exact IDs.

## Role-specific collection

The original Worker START route remains supported:

```sh
task-mecca runtime observe-t3 --project CANONICAL \
  --thread-id EXACT_MANAGED_T3 --session-id EXACT_NATIVE --agent-id EXACT_NATIVE \
  --assignment EXISTING_ASSIGNMENT --task-id TASK \
  --worker-path /root/controller/WORKER --source-worktree ACTUAL_CWD --json
```

It requires a Worker child path and retains Worker implementation role policy,
scope/workspace and exact current execution/assignment gates. `/root/controller`
is not a Worker child. Its original generic identity error does not establish
that the T3 ID failed a UUID parser. T3 IDs are opaque host-returned strings;
native session IDs, T3 IDs, collaboration handles and role names are different
namespaces. A provider-native wrapper is not proof of managed creation.

An operational Controller uses the explicit formal route. Workers may also use
this route with `--role worker --agent-path /root/controller/WORKER`:

```sh
task-mecca runtime observe-t3 --project CANONICAL \
  --thread-id EXACT_MANAGED_T3 --session-id EXACT_NATIVE --agent-id EXACT_NATIVE \
  --run-id EXACT_CURRENT_HOST_RUN --assignment EXISTING_ASSIGNMENT --task-id TASK \
  --role controller --agent-path /root/controller \
  --parent-thread-id EXACT_PARENT_T3 --source-worktree ACTUAL_CWD --json
```

Use an existing role-appropriate assignment; do not reassign or create a second
session to bypass a collection failure. Formal collection independently reads
the live T3 projection with its server ownership lock, verifies exact parent and
subagent relationship, project workspace, source worktree, shared Git identity,
native identity and current execution episode. It rejects changed scope, wrong
ID kinds, foreign project/worktree, stale run, ambiguous runtime and conflicting
assignment. Parent/project/worktree inputs are expectations, never caller proof.
`--worker-path` and formal role flags cannot be mixed. Formal `--dry-run` is
unsupported and fails without writing; use exact official host reads and
read-only `runtime query` / `runtime diagnose` for investigation.

The Controller result binds its **operational** role only. It grants no product
implementation exception, writes no Activity, creates no Hook events and makes
no handoff claim. A Worker formal result still passes the existing Worker role
policy. Query the returned exact attempt independently, then report only the
current real Activity phase using the supported role/assignment context.

## Diagnose the actual boundary

| Evidence/failure | Owner and resume condition |
| --- | --- |
| Legacy Worker identity error with Controller path | Controller: preserve original error; use explicit formal Controller route with existing exact assignment |
| `role_path_mismatch` / identity-kind mismatch | Controller: compare actual native, exact managed T3 and logical assigned role; never substitute Root IDs or invented UUIDs |
| Host parent/project/worktree mismatch | Controller/host owner: official exact child and parent read plus actual cwd/repository identity must agree |
| `host_scope_unavailable` / structured observation unavailable | Host bridge owner: establish supported local projection schema and live server lock; preserve unsupported/error result |
| Hook configured, zero events | Collector owner: configuration is not delivery proof; inspect minimal exact-session counts and collector errors; cause remains unknown |
| Native events exist, no matching runtime | Controller/collector owner: use current supported observation path; never inject Hook/attempt/START or rewrite user hooks |
| Observation persisted, binding failed | Controller: repair exact assignment/current identity and retry same START; partial observation is retained |
| Observed attempt unbound/ambiguous/stale | Controller: diagnose binding/current episode separately; provider-wide Hook counts cannot establish exact-session collection |
| Direct send succeeded; no claim | Recipient Controller: refresh current context and execute supported claim; transport ACK is not acceptance/applied |

`runtime diagnose` preserves an existing attempt's binding/staleness diagnosis
even if Hook count is zero. It does not conclude that Hook configuration caused
the gap. This change neither modifies real hook settings nor injects events.

## Same-handoff recovery; no EMPFUND operation performed here

B-639's recorded assignment was
`assignment-bcd3c8d10faf1d2b6c64123933bed5aa`, role `/root/controller`, source
`/Users/hanati/.t3/worktrees/empfund/t3-ca90345a-1e83532959ea792b`, native
`01a1e2c1-7d76-7360-9fab-434be8de3265`, and managed T3
`thread:delegated-task:command%3Amcp%3Aeef2e499-d425-4776-b1cc-5da583543e73%3Adelegate-task%3Aa65f3b07-2420-4814-a93d-27aec7600e62`.
The original first error was:

```text
exact canonical project, source worktree, task, assignment, worker path and distinct native/T3 identities required
```

The supplied evidence does not retain the original executable VERSION or every
literal argv token. Do not label a reconstruction as the original command log.
The offline B-639 regression reproduces this exact first rejection at the
inherited source guard for both opaque and UUID-looking T3 IDs, before host or
project access. It proves the Controller-path rejection in this source, not a
historical executable's full behavior or the unknown Hook cause.

Only the EMPFUND owner may resume B-639: fresh canonical Full Access preflight,
fresh exact host/native/current-run and approval reads, formal operational
collection with current identities, independent exact-attempt query, then
recover the **same** `handoff-bcab2b71ef76829edbfc` using existing
prepare/retarget/verified-transfer as applicable. Read `handoff context` and
`handoff transport-plan`, deliver through the supported exact tool address,
and have the actual current recipient claim. Existing hold and original errors
remain until that owner verifies the resume conditions. Historical attempts
and expired assignments are not instructions to replay these values now.

Native collaboration handles work only in the tool's observed agent tree.
`live agent path ... not found` and provider-wrapper `message parent` rejections
are transport scope failures, separate from runtime binding. Use the actual
supported exact T3 address when available; do not guess native handles or create
another agent. See [typed handoff transport](typed-handoff-transport.md) and
[completion context](task-completion-context.md) for existing claim/report gates.

## Limits

This CLI uses the existing local schema-bound T3 bridge; it cannot invoke the
conversation's MCP tools, control arbitrary host versions, or establish managed
creation from a JSON profile. Unsupported bridge schemas fail closed. Offline
SQLite/mock tool fixtures prove local validation and retry behavior, not real
MCP collection/delivery, external Hook execution or EMPFUND recovery. Actual
maintenance Worker collection, independent review, Controller acceptance, dev
integration, CI/release and operating installation need their own evidence.
# A-90 Reviewer 수집 경계

`role=reviewer`와 `/root/controller/reviewer/<name>`을 정식 역할로 사용한다. 이름은 `DMMDD_NN_Reviewer_<name>_G<n>`이며 같은 세션 재개 시 세대는 유지한다. exact own-native `session-name plan/apply/verify`와 별도의 공식 T3 title update/readback을 수행한다. 관리 경로·공식 이름만으로 native/배정/현재 실행을 추정하지 않는다.

Reviewer START는 기존 원 프로젝트/부모/실제 checkout/배정/현재 native 검증에 읽기 전용 검수 gate를 추가한다. 지정 Reviewer native와 다른 실제 구현 native·구현 이력·전체 소스 준비를 확인하며 구현용 Evaluate 권한을 열지 않는다. Activity attribution과 `runtime query --role reviewer`가 Worker와 구분한다. 과거 Worker 검수의 역할·이름·시각은 소급 수정하지 않는다.

T3 신규 검수/재검수는 실제 새 managed delegate로 수행한다. 이 CLI가 외부 MCP 생성·권한을 수행했다고 주장하지 않는다. 정책·fixture 지원과 실제 호스트 실행 검증은 구분한다. 초기 미측정 구간은 backfill하지 않는다.
