# 실행 추적 상세 DTO v1 (A-93 → A-92)

경로: `workflow.reporting.roles[].execution_coverage`. A90 Workflow type과 axis/measurement는 불변이다. 기존 response cache에는 detail을 저장하지 않으며, 읽기 요청에서 최신 저장 근거를 결합한다. 이 작업은 UI 제품 파일을 편집하지 않는다.

```json
{"version":1,"items":[],"collection_errors":[],"allocation":"operation_wall_time_once; task links are associations, never per-task exclusive time"}
```

각 item: `id`, `category`, `role`, 선택적 `native_id/actor_id/attempt_id/operation_id/task_ids/scope/handoff_id`, 원본 `start`, 선택적 `end/duration_ms`, `reason`, 선택적 원문 `boundary`. 기존 actor path를 native ID라고 바꾸지 않으며 원래 actor_id를 보존한다. boundary에는 stable request_id/operation_id/key, 현재 observed_at, 선택적 원본 source_at과 wait 상세가 있다. duration이 없으면 0으로 바꾸지 않는다.

| category | 의미 |
|---|---|
| confirmed_closed_activity | 기존 Activity가 실제 양쪽 경계를 제공한 구간 |
| open_start | 현재 START만 확인; now까지 작업시간을 만들지 않음 |
| open_phase / closed_phase | 명시적 operation 단계 경계; 전환/END가 없으면 open |
| closed_operation | 현재 START/END로 관측된 operation; task 전용 시간 아님 |
| evidenced_wait | 5개 필수 근거가 있는 실제 대기; scope 별도 |
| observation_handoff | native/Hook/등록연결/실제 요청·전달·claim의 관측 사건 |
| unmeasured | 기존 축에서 확정 활동으로 덮이지 않은 구간 |
| collection_failure | 수집 원오류, 종료만 있고 START가 없음, 대응 없는 resume 등 |
| unknown | 실제 대기 근거 부족; 비활동/미수행을 뜻하지 않음 |

동일 등록배치의 task_ids가 A-1/A-2이면 두 화면에서 연관 사실을 보여도 동일 duration을 각 task 전용시간으로 합산하지 않는다. native/attempt/operation별 겹침과 회차를 유지한다. actor 대기와 task_next_stage 대기는 별도 표시하며 task 대기로 actor 시간을 덮지 않는다. unknown/unmeasured/collection_failure를 확정 대기로 칠하지 않는다.

fixture: `handoff-evidence/a93-implementation/fixtures/execution-coverage-v1.json`. fixture는 격리된 합성 회귀 근거이며 actual 유지보수 수집 성공이 아니다. A92의 화면 구현·접근성·브라우저 검증은 해당 계약에 남는다.

B651의 932901ms requirements, 21대상/7933참조만으로 과다분석/미수행을 판정하지 않는다. 조사 snapshot의 closed 933965ms와 open 구현, Controller/Registrar not_recorded, 1155319ms unmeasured는 서로 다른 근거다. 이 DTO는 그 과거 공백을 소급 채우거나 분모를 바꾸지 않는다.
