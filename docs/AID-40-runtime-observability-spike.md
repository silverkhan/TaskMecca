# AID-40 Runtime Observability Spike

이 문서는 AID-39의 구현 전제인 Codex / Claude Code subagent lifecycle 관측 가능성을 실제 실행 환경에서 검증하기 위한 Spike 절차다.

## 원칙

- 이 Spike는 heartbeat를 만들지 않는다.
- Hook 입력은 Task Mecca Go CLI가 stdin으로 받아 `_task_mecca/.runtime/observability-spike/events.jsonl`에 기록한다.
- `tool_input`, `tool_response`, 마지막 assistant message 같은 큰/민감한 nested payload는 저장하지 않는다.
- 원본 payload의 필드 이름, byte 길이, SHA-256은 남겨 schema 차이를 확인할 수 있게 한다.
- `SubagentStart`가 있고 `SubagentStop`이 없다는 이유만으로 Agent 사망으로 판정하지 않는다.
- 병렬 실행의 Task Mecca attempt ↔ runtime agent 연결은 시간 순서로 추정하지 않는다.

## Collector

Hook에서 다음 명령을 호출한다.

```bash
task-mecca runtime-spike observe codex
task-mecca runtime-spike observe claude
```

두 명령은 성공 시 stdout을 출력하지 않는다. 따라서 hook이 모델 context에 불필요한 텍스트를 추가하지 않는다.

현재 수집하는 공통 필드:

- provider
- hook_event_name
- session_id
- turn_id
- agent_id
- agent_type
- tool_name / tool_use_id
- permission_mode
- reason
- stop_hook_active
- cwd / model
- transcript 경로
- raw field names / raw byte size / raw SHA-256

## Report

```bash
task-mecca runtime-spike report
task-mecca runtime-spike report --json
task-mecca runtime-spike report codex --json
task-mecca runtime-spike report claude --json
```

기본 최근 이벤트 수는 CLI의 `--limit` 값(기본 10)을 사용한다.

Report는 provider별 start/stop/tool activity 수와 agent별 lifecycle continuity를 보여준다.

다음 finding은 의도적으로 **사망 판정이 아니다**.

- `start_without_stop`: 아직 실행 중일 수도 있고 terminal hook이 누락됐을 수도 있음
- `activity_without_start`: collector가 실행 중간부터 시작됐을 수 있음

## Codex hook 설정 예시

공식 Codex Hooks는 repo의 `.codex/hooks.json`을 지원한다. Spike 동안 다음과 같이 등록할 수 있다.

```json
{
  "description": "Task Mecca AID-40 runtime observability spike",
  "hooks": {
    "SubagentStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe codex",
            "timeout": 3
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe codex",
            "timeout": 3
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe codex",
            "async": true,
            "timeout": 3
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe codex",
            "async": true,
            "timeout": 3
          }
        ]
      }
    ]
  }
}
```

Codex 프로젝트 hook은 trust 승인이 필요할 수 있다. `/hooks`에서 활성 source와 trust 상태를 확인한다.

## Claude Code hook 설정 예시

프로젝트의 `.claude/settings.json`에 다음 hook을 추가한다. 기존 설정이 있으면 `hooks`를 병합한다.

```json
{
  "hooks": {
    "SubagentStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe claude",
            "timeout": 3
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe claude",
            "timeout": 3
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe claude",
            "async": true,
            "timeout": 3
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "task-mecca runtime-spike observe claude",
            "async": true,
            "timeout": 3
          }
        ]
      }
    ]
  }
}
```

Claude Code의 project/user/plugin hook은 subagent 내부 tool call에도 실행되며, 해당 tool event가 subagent 소유라면 `agent_id` / `agent_type`을 통해 attribution할 수 있는지 이 Spike에서 확인한다.

## 실제 검증 시나리오

### 1. 단일 Worker

- Worker 1개 생성
- 최소 2개의 tool call 수행
- 정상 완료
- report에서 동일 `agent_id`에 Start → activity → Stop이 이어지는지 확인

### 2. 병렬 Worker

- Worker 3개를 가능한 한 동시에 생성
- 각 Worker가 서로 다른 파일/명령을 사용하도록 한다.
- `agent_id`가 서로 구분되는지 확인
- tool activity가 각 Start identity에 안정적으로 귀속되는지 확인
- Task Mecca `agent_path` / backlog attempt와 runtime identity를 명시적으로 연결할 근거가 있는지 별도로 기록한다.

### 3. Interrupt

- 장시간 실행 Worker를 중단한다.
- SubagentStop 또는 다른 terminal evidence가 실제로 발생하는지 확인한다.
- Start만 남을 경우 이를 failure 확정으로 해석하지 않는다.

### 4. Host/runtime abnormal termination

- 테스트용 세션에서만 runtime 프로세스를 비정상 종료한다.
- terminal hook 누락 여부 확인
- 재기동 후 provider history/runtime query로 해당 실행을 복원할 수 있는지 확인

### 5. Task Mecca restart

- 이벤트 수집 후 Task Mecca Web/CLI를 재시작한다.
- journal이 그대로 읽히는지 확인
- runtime provider의 현재 상태와 과거 journal을 reconciliation할 수 있는 범위를 기록한다.

## AID-40 완료 시 남길 결과

Codex와 Claude 각각 다음 표를 채운다.

| 항목 | 결과 |
| --- | --- |
| Start identity | |
| Tool activity attribution | |
| Normal terminal evidence | |
| Interrupt terminal evidence | |
| Hard-kill evidence | |
| Runtime query surface | |
| Task Mecca attempt binding | |
| Restart reconciliation | |
| 알려진 ambiguity | |

이 표가 확정되면 AID-41의 Runtime Adapter 계약으로 승격한다.
