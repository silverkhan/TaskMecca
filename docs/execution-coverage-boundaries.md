# 실행 경계 수집 (A-93)

이 journal은 실행권한이나 Activity를 생성하지 않는다. 원본 등록 시각, lifecycle 축, Activity 원장과 coverage 분모는 그대로 사용한다. native 실행 관측과 실제 작업시간도 구별한다.

`_task_mecca/.runtime/execution-boundaries.jsonl`에 append-only로 저장한다. `provider + native_id + attempt_id + operation_id + key`가 멱등 키다. 같은 키의 재시도는 최초 `observed_at`을 보존하며, 내용 충돌은 오류로 남긴다. 손상된 줄/중복 충돌/쓰기 실패/지원되지 않는 신원은 성공이나 공백 없는 활동으로 바꾸지 않는다. 읽기에서는 정상 줄과 collection_errors를 함께 반환하고, 손상된 원장에는 추가 쓰지 않는다. symlink 저장 경로도 거부한다.

## 자동 연결

- 기존 `runtime observe-t3`는 검증된 실제 native START/종료의 **관측점**과 이후 task binding을 수집한다. native의 과거 `started_at`은 `source_at`으로 보존하고, 최초 현재 수집 시각을 과거로 옮기지 않는다. 이 관측점은 측정 작업시간이 아니다.
- 기존 지원 provider Hook도 검증 가능한 session/attempt의 관측점을 연결한다. 신원이 부족한 Hook 원문은 기존 journal에 남으며 새 identity를 추측하지 않는다.
- `contract apply --request-id ... --approve --expect-plan ...`은 실제 current native/T3/공식 역할을 확인한 뒤 등록 operation의 START, 입력 검증, apply 단계, 생성된 canonical task 연결, END를 자동 수집한다. 별도의 agent 보고나 heartbeat는 필요 없다. `plan/status`는 수집 원장도 쓰지 않는 읽기 전용이다.
- 안정적인 `request_id`는 등록 요청을 묶는다. 각 실제 apply 호출은 native attempt와 현재 관측에서 생성해 보존한 별도 `operation_id`를 사용한다. 같은 요청 재실행의 시간을 과거 호출에 덮지 않는다. 경계 내부 재시도는 같은 키다. 최초 신원 검증 이전 구간은 미측정으로 남긴다.
- 기존 Activity phase와 실제 handoff prepare/delivery/claim은 workflow의 기존 근거를 상세 DTO로 투영한다. 관측 사건으로 작업시간을 만들지 않는다.

자동 수집 오류는 원래 등록 결과/원본 오류를 바꾸지 않고 apply JSON의 `execution_collection`(status/started/ended/original_error)과 stderr에 `execution_boundary collection_failure/unmeasured`로 보존한다. 신원이 확인됐지만 단계 쓰기가 실패했다면 가능한 경우 실패 키/원오류를 별도 collection_failure로 남긴다. 저장소 자체가 고장 난 경우 stderr가 재시도 근거이며, journal에 실패를 남겼다고 주장하지 않는다. END가 없으면 START는 open이다.

## 현재 boundary API

`runtime boundary preview|record|list`를 제공한다. preview는 record와 동일한 필드/신원 검증을 하되 journal/lock을 생성하지 않는다. record는 현재 own `CODEX_THREAD_ID` 및 `T3CODE_THREAD_ID`, 실제 T3 running/attempt와 공식 native 역할 이름을 대조한다. 외부 actor, terminal/history, caller state JSON, caller timestamp, self-bind는 지원하지 않는다. Reviewer는 공식 Reviewer 이름이 지원되는 host의 실제 Reviewer만 수집하며 구현권한으로 사용하지 않는다. 미지원 host는 unmeasured/원오류다.

record 필수: `--project`, `--thread-id`, `--session-id`, `--attempt`, `--role`, `--operation`, `--key`, `--kind`, `--evidence`. task가 생기기 전에는 `--tasks`를 생략한다. 이후 `kind=task_link`와 canonical task 목록을 연결한다. explicit handoff는 기존 지원 journal의 실제 사건과 같은 actor attempt를 검증한다. API timestamp 인자는 없다.

대기는 `--wait-scope actor|task_next_stage`, `--reason`, `--expected-event`, `--next-owner`, `--last-evidence`, `--resume-condition`을 사용한다. 하나라도 부족하면 unknown이며 대기시간을 만들지 않는다. resume의 `--related-id`는 같은 native/attempt/operation의 실제 wait key 또는 ID다. 다른 actor/회차의 wait를 닫거나 다른 작업시간에서 빼지 않는다. task 다음 단계 대기는 actor의 작업중단을 뜻하지 않는다.

## Controller의 현재 maintenance 수집

sealed candidate binary 검토 후, Controller 자신의 실제 현재 native/attempt/공식 T3 thread로 preview를 먼저 실행한다. Worker는 canonical 새 journal을 직접 쓰지 않았다.

```sh
CANDIDATE=/ABS/WORKER/handoff-evidence/a93-implementation/candidate/task-mecca
"$CANDIDATE" runtime boundary preview --project /Users/hanati/projects/TaskMecca/TaskMecca --thread-id "$T3CODE_THREAD_ID" --session-id "$CODEX_THREAD_ID" --attempt CURRENT_VERIFIED_ATTEMPT --role controller --operation CURRENT_MAINTENANCE_OPERATION --key start --kind start --evidence CURRENT_SOURCE_REPORT_REFERENCE --json
```

preview 확인 후 같은 argv의 `preview`만 `record`로 바꾼다. 실제 현재 phase는 새 key와 `--kind phase --phase PHASE`로, 실제 종료는 `--key end --kind end`로 수집한다. 기존 task가 있으면 별도 `task_link --tasks A-93`을 사용한다. 첫 수집 이전의 분석/START를 추정하지 않는다. stable operation/key는 재시도 동안 보존한다. 오류가 나면 원문/예상 사건/다음 담당자를 보존하고 성공으로 보고하지 않는다. 다른 역할의 actual 수집은 해당 역할의 후속 실제 업무에서 검증한다.

## current T3 running/queued 선택

기존 max ordinal만 고르면 queued 후속 회차가 실제 running을 가린다. 유일한 running 뒤에 queued만 있을 때 그 running을 선택한다. 다중 running, 뒤쪽 terminal/starting 등 충돌은 거부한다. native identity/owner lock/project/최신 retry/terminal/historical 검증은 유지한다. caller가 원하는 과거 회차를 현재라고 선택하는 옵션은 추가하지 않는다.
