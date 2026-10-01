# AID-41 Execution Lifecycle Ledger

AID-41은 AID-40의 provider hook evidence를 Task Mecca 내부의 provider-neutral execution model로 승격한다.

## 저장 방식

canonical execution journal:

```text
_task_mecca/.runtime/executions/events.jsonl
```

매 hook마다 하나의 현재 상태 JSON을 덮어쓰지 않는다. runtime event와 binding evidence를 append-only로 기록하고 현재 상태는 journal을 folding하여 재구성한다.

이 구조는 병렬 hook의 stale write를 피하고, Task Mecca 재시작 후에도 동일 원장에서 실행 상태를 복원하기 위한 것이다.

## Hook 설치

Task Mecca가 provider 설정을 덮어쓰지 않고 기존 JSON을 보존하면서 lifecycle hook만 병합한다.

```bash
task-mecca runtime hooks status all
task-mecca runtime hooks install codex
task-mecca runtime hooks install claude
task-mecca runtime hooks install all
```

경로:

- Codex: `.codex/hooks.json`
- Claude Code: `.claude/settings.json`

설치는 idempotent하며 이미 같은 Task Mecca command hook이 있으면 중복 추가하지 않는다.

## Production hook

AID-40의 `runtime-spike observe`는 실측/진단용으로 남긴다.

실제 lifecycle 원장에는 다음 명령을 연결한다.

```bash
task-mecca runtime observe codex
task-mecca runtime observe claude
```

기본 stdout은 비어 있으므로 hook 실행 자체가 모델 context에 불필요한 문자열을 추가하지 않는다.

## 조회

```bash
task-mecca runtime list
task-mecca runtime list codex
task-mecca runtime list claude
task-mecca runtime list --json
```

`--limit N`으로 Agent별 최근 의미 있는 transition 개수를 제한한다.

## Canonical state

- `runtime_unknown`
- `starting`
- `running`
- `waiting_user`
- `waiting_approval`
- `interrupted`
- `completed`
- `errored`
- `shutdown`

Hook adapter의 초기 매핑:

| Runtime evidence | Canonical |
| --- | --- |
| SubagentStart | running |
| PreToolUse / PostToolUse / PostToolUseFailure | activity evidence |
| SubagentStop | completed |
| Stop reason에 interrupt/cancel | interrupted |
| Stop reason에 error/fail | errored |
| Stop reason에 shutdown/terminate | shutdown |

`SubagentStop`이 없다는 사실만으로 `interrupted`나 사망을 추론하지 않는다.

## Attempt와 Binding

runtime attempt는 기본적으로 provider + session_id + runtime agent_id로 구분한다. 동일 runtime agent id가 다른 session에서 나타나면 다른 attempt다.

Task Mecca의 Pokémon worker identity와 runtime identity는 별개이므로 최초 상태는:

```text
binding_state = unbound
```

이다.

실측 또는 dispatch evidence로 연결이 확인되면:

```bash
task-mecca runtime bind <attempt-id> <task-id> <agent-path> [parent-attempt-id]
```

로 binding event를 추가할 수 있다.

동일 attempt에 상충하는 task/agent binding이 들어오면 `binding_state=ambiguous`로 바꾸고 기존 task/agent 표시도 제거한다. 이 상태는 자동 backlog 전환 근거로 사용할 수 없다.

## 시간 지표

- `elapsed_ms`: Start → terminal 또는 현재 시각
- `waiting_ms`: 명시적인 waiting state가 관측된 구간만 합산
- `observed_active_ms`: provider가 authoritative active interval을 제공할 때만 사용

현재 Codex/Claude hook만으로는 내부 reasoning 전체의 active interval을 확정할 수 없으므로:

```text
active_time_available = false
observed_active_ms = null
```

로 둔다. Tool event 사이의 공백을 작업시간으로 임의 보간하지 않는다.

## Recent history

Raw tool activity를 사용자 이력으로 그대로 노출하지 않는다.

`PreToolUse/PostToolUse`는 `last_activity_at`과 activity/evidence count 갱신에 사용하고, 사용자-facing history에는 state/binding transition만 최근 N개 남긴다.

## Reconciler

```bash
task-mecca runtime reconcile
task-mecca runtime reconcile --json
```

현재 Reconciler는 보수적으로 동작한다.

- terminal evidence가 있으면 해당 terminal state 유지
- terminal evidence 없이 stale window 동안 새 runtime evidence가 없으면 `stale`
- current state 자체를 확정할 근거가 없으면 `runtime_unknown`
- timeout만으로 Agent 사망/중단을 확정하지 않음
- binding 충돌은 `binding_ambiguous`

향후 Codex app-server 또는 Claude background-session query가 확정되면 동일 journal에 `reconciled / authoritative` evidence를 추가하는 방식으로 확장한다.

## Provider event의 session_id 누락

일부 tool event가 `session_id` 없이 들어오는 경우에는 같은 provider + runtime agent id를 가진 **현재 non-terminal attempt가 정확히 하나일 때만** 해당 attempt에 연결한다.

후보가 둘 이상이면 시간 순서로 추정하지 않는다.
