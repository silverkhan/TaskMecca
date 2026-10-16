# 에이전트 실행 조회 — Runtime query (AID-203)

기존 `task-mecca runtime list --json`은 원장 전체를 보여준다. 역할, 세션, 특정 native runtime agent의 **최근 실행**을 찾기 위해 별도 Python/JavaScript 정렬·필터를 작성할 필요가 없도록 `runtime query`를 제공한다.

## 예시: 사용자 운영에서 반복되던 최신 2건 조회

```bash
task-mecca runtime query \
  --agent-id 01a113a5-5be7-7292-b35d-17f2060b3d3d \
  --limit 2 \
  --fields attempt_id,binding_state,current_state,runtime_agent_id,session_id,turn_id \
  --json
```

## 다른 조회

```bash
task-mecca runtime query --role controller --state open --json
task-mecca runtime query --role worker --state unknown --json
task-mecca runtime query --session-id SESSION_ID --limit 5 --json
task-mecca runtime query --agent-path /root/controller --provider codex --json
task-mecca runtime query --help
```

- 필터: `--provider`, `--agent-id`, `--agent-path`, `--role`, `--session-id`, `--task-id`, `--state`.
- `--state any|open|terminal|unknown`: open은 현재 관측 원장 기준으로 terminal이 확인되지 않은 상태다. **실제 실행이 살아 있음을 증명하지 않는다.** unknown은 runtime_unknown 또는 ambiguous binding을 포함한다.
- **필터 → 최신 시작 시각 정렬 → 개수 제한** 순서로 적용한다. 시작 시각이 없다면 최초 관측 시각을 사용하고 이조차 없다면 가장 오래된 쪽으로 놓는다.
- `--fields`에는 JSON에서 쓰는 정식 필드명을 지정한다. 잘못된 이름은 명시적 오류를 낸다. 없으면 전체 attempt를 반환한다.
- JSON 응답: `status=found|not_found`, `count`, `total_matches`, `attempts`, `findings`, `generated_at`, `observation_scope`.
- `not_found`는 *일치하는 관측 기록이 없음*이며 *실행한 적이 없음/프로세스 사망*을 뜻하지 않는다.
- 조회는 쓰기 없이 canonical runtime evidence만 읽는다. `reconcile`·`hooks`·`bind` 등 기존 명령은 유지한다.
- 관측 실패 원인 분석과 안전 복구 가이드는 AID-209에서 별도 제공한다.
