# 에이전트 실행 진단 — AID-209

`runtime list`는 전체 원장, `runtime query`는 역할/세션/에이전트별 최근 실행, `runtime diagnose`는 **설정과 실제 관측을 분리한 읽기 전용 진단**을 담당한다.

```bash
task-mecca runtime diagnose --project /PROJECT --provider codex \
  --agent-id EXACT_NATIVE_AGENT_UUID --json

task-mecca runtime diagnose --project /PROJECT --provider claude \
  --role controller --json
```

구조화 출력의 `checks`에는 global/project Hook 설정, provider 관측 control, 실제 Hook journal 사건 수, 해당 canonical runtime attempt 발견 여부가 각각 별도로 담긴다.

`diagnosis`:
- `observation_disabled`: 관측 중지 설정이 확인됨
- `hooks_unconfigured`: Hook 등록이 확인되지 않음
- `not_observed`: 설정은 있으나 실제 관측을 확인하지 못함
- `no_matching_runtime`: provider Hook 기록은 있으나 요청한 Agent/Session/Role이 없음
- `binding_unbound`, `binding_ambiguous`: 연결 또는 exact identity 확인 필요
- `terminal_history`: 종료된 과거 실행
- `observation_stale`, `runtime_unknown`: 실제 생존·사망을 단정하지 못함
- `observed_nonterminal`: 비종료 관측 근거가 있음. **실제 프로세스 생존의 확증은 아님**

안전 원칙:
- 진단은 lifecycle·Hook 설정·Provider 원본 세션 파일을 **수정하지 않는다**.
- API가 보장하지 않는 native resumed/message/spawn은 호출했다고 주장하지 않는다. 필요한 외부 동작을 `recovery_steps`로 제시한다.
- 이전 completed 실행을 재사용하거나 합성 시작/완료 이벤트를 넣어 복구하지 않는다.
- 0.3.1에서 해소된 과거 연결 장애를 재발했다고 주장하지 않으며 미래 진단에 사용한다.
