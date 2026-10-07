# Canonical assignment and direct completion protocol

이 문서는 SESSION_GUIDE 한국어·영어, collab 및 역할 문서의 공통 실행 절차다. Controller는 지정 원본 workspace의 canonical backlog/runtime의 단일 운영 writer다. Worker는 원본 계약을 읽고 구현·검증·실제 착수와 transport evidence를 Controller에 보고한다. Worker는 원본 또는 구현 worktree 사본 backlog/lifecycle/handoff/runtime 운영원장을 수동 생성·수정하거나 monitor에 등록하지 않는다. 승인된 CLI preflight의 ephemeral probe/cache 관측 쓰기와 runtime hook의 자동 관측은 허용된 예외이며 Worker의 운영원장 쓰기 권한이 아니다. Git 구현/지정 dev 통합과 원본 상태 기록은 별개다. Controller는 dev merge를 기다리지 않고 실제 배정·실행·대기·결과를 원본에 기록한다.

## Assignment checklist

모든 CLI 명령은 지정 원본 project를 cwd로 실행한다. 설치된 CLI의 옵션 지원을 확인하지 않고 다른 cwd에서 --project 동작을 가정하지 않는다. 원본 backlog root, 원래 repository, integration branch/base SHA, 구현 worktree/branch 및 금지 범위를 먼저 확정한다. 다음 순서는 생략하거나 뒤집지 않는다.

1. `task-mecca preflight --require-full-access --json`을 실제 dispatch 직전 실행하고 `access.orchestration_ready == true`를 확인한다. 과거 cache·UI label로 대신하지 않는다.
2. `task-mecca inspect <ID> --json`으로 원본 계약·현재 state·선행·Agent·변경범위를 fresh 확인한다. 기존 doing 복구는 기존 배정/완료 근거부터 확인한다.
3. native live agent 목록과 `coordinate --json`을 대조해 identity와 쓰기 scope를 결정한다. 신규는 `worker-name --used <live-path> --json`에 실제 live path들을 반복 전달하고 반환된 canonical path를 `agent <new-path> --new --json`으로 검증한다. 기존 재사용은 `agent <existing-path> --json`으로 검증한다. pool을 복제하거나 task ID/a19_impl/a20_impl을 새 이름으로 쓰지 않는다.
4. 원본을 doing으로 전환하고 Agent와 변경범위를 기록한 뒤 다시 inspect한다. state/Agent/scope가 일치해야 다음 단계로 간다. CLI에 없는 상태 변경 명령을 만들어 쓰지 말고 파일명과 metadata를 안전하게 편집한다.
5. `task-mecca runtime assign <ID> <worker-path> --json`의 실제 assignment_id를 보존하고 `task-mecca lifecycle record assigned <ID> <stable-event-id> /root/controller controller_report <evidence> <assignment_id>`를 기록한다. 배정은 착수가 아니다.
6. 실제 dispatch에 원본 계약 경로/ID, scope, assignment_id, contract_sha256와 Controller envelope를 전달한다. envelope는 semantic role, native target, runtime agent ID, attempt ID, provider/session scope를 구분한다. native alias가 다르면 실제 native target을 사용한다.
7. dispatch 결과/직접 hook에서 얻은 exact runtime agent ID를 `task-mecca runtime bind-assignment <assignment_id> <runtime-agent-id> --json`으로 연결한다. 후보가 0개/여러 개거나 첫 hook이 없으면 추정하지 않고 확인을 보류한다. role-only binding은 Go CLI에서 `task-mecca runtime bind <exact-attempt-id> '' /root/controller --json`처럼 빈 task-id를 사용한다. 단독 `-`는 공통 parser에서 거부될 수 있으므로 성공으로 기록하지 않는다.
8. 원본을 다시 inspect하고 state=doing, Agent, scope, assignment 및 exact binding을 대조한다. 실제 Worker started 보고/hook이 있으면 Controller가 `lifecycle record started`를 실제 evidence와 assignment/attempt로 기록한다. Worker는 waiting/resumed도 실제 보고하고 Controller가 기록한다. attempt를 얻지 못했으면 생략하고 제약을 남긴다.

dispatch 전 어느 gate라도 실패하면 dispatch를 중단하고 원본에 원인·남은 단계·재개조건을 남긴다. dispatch 뒤 누락이면 구현 결과/미커밋을 보존하고 안전한 복구 기록을 남긴다. 원본 확인 실패·미확정 binding·Worker 생성 미확인은 handoff applied의 근거가 아니다. 과거 착수 시각·assignment·attempt를 만들어 보충하지 않는다.

기존 비표준 active path/native alias는 강제 rename하거나 중복 spawn하지 않는다. 역사·archive·alias를 유지하고 새 배정부터 allocator 표준을 적용한다. 원본 Agent, runtime binding과 실제 dispatch identity의 대응을 검증한다.

## Worker DONE/BLOCKED transport checklist

Worker final 또는 Root 보고는 transport 성공이 아니다. 같은 결과에는 안정적인 report/handoff ID를 사용하고 재시도에도 유지한다.

1. 수용 기준별 결과, commit/push/PR/검증, 미커밋·ignored 파일, 잔여와 재개조건을 보고한다. `/tmp`는 임시 전달용으로만 사용한다. 실패/unknown 증적은 dispatch에서 Controller가 지정한 보존 경로 또는 Worker 구현 worktree의 별도 `handoff-evidence/` 경로에 안정적인 report/handoff ID로 durable report 파일을 보존한다. 이 경로는 운영원장이 아니며 backlog/runtime 사본 안에 두지 않는다. 원본 handoff prepare/mark와 durable 원장 기록은 Controller에게 위임한다.
2. dispatch envelope의 exact Controller native target을 종료 직전 native 목록에서 fresh 조회하고 runtime ID/attempt/provider/session과 대조한다. semantic path만 보고 stale completed target에 보내지 않는다.
3. running이면 `collaboration.send_message`로 Controller에 DONE/BLOCKED와 stable report ID를 직접 전달한다. completed이면 지정 원본의 fresh Full Access preflight 후 `collaboration.followup_task`로 같은 exact native Controller를 실제 재개한다. 이 작업의 실행 승인과 runtime resume 지원이 있어야 한다.
4. running 조회와 message 사이에 completed race가 생기면 같은 handoff ID로 bounded 확인한다. 즉시 조회하고 필요하면 한 번 더 확인하며 무한 polling하지 않는다. completed이면 fresh preflight 후 한 번의 followup_task를 실행하고 새 running turn/transport 결과를 확인한다. 이미 resume/소비가 확인된 report를 중복 dispatch하지 않는다. Controller claim/applied 상태도 중복 side effect를 막는다.
5. 종료 전에 실제 send/resume 반환값, fresh 상태, exact target, report ID 및 확인된 새 turn을 근거로 남기고 Controller에 전달한다. tool acceptance와 processing completion은 구분한다. 실패·unknown은 성공으로 쓰지 않고 근거, 남은 작업과 재개조건을 남긴다. Controller에 전달 불가능하면 durable report에 실제 transport 반환/오류와 fresh target state, stable report/handoff ID, 남은 작업·재개조건을 기록하고 native final에 그 경로와 실패 근거를 남겨 다음 recovery에서 읽을 수 있게 한다. /tmp만으로 durable 실패 기록을 충족하지 않는다. cleanup 전에 Controller가 보고서를 회수·보존해야 하며, 성공 전달 뒤 Controller가 원본 운영원장에 보고/transport 근거를 복사·기록한 것이 확인되면 durable 인계가 완료된다.

native runtime이 discovery/message/resume을 지원하지 않으면 capability/evidence와 fallback 제약을 명시한다. Task Mecca CLI는 native 자동 통지·재개 또는 turn 종료 강제를 보장하지 않는다. Root ACK/wake/지속 polling은 어느 단계의 gate도 아니다.

## Controller 완료 검토 기록 checklist

[Controller 역할의 완료 검토 절차](roles/controller.md#완료-검토-착수와-중단-복구)를 공통으로 적용한다.

1. DONE 인수 시 원본 계약·report/handoff·source Worker attempt·exact Controller target/claim을 확인하고, 긴 검증/merge 전에 원본 요약과 작업 노트에 실제 인수·검토 착수와 남은 일을 기록한다. Worker 완료만으로 done을 표시하지 않는다. 파일 상태는 `doing`, Worker `Agent`/assignment는 유지하고 Controller 연결은 handoff target/claim으로 보존한다.
2. 지원 projection은 `controller_review_pending` → `controller_review` → `controller_finalizing`이며 실패/중단은 `controller_recovery`다. canonical 파일명이나 lifecycle에 같은 이름을 새로 만들지 않는다. 검토 착수는 `claimed_at`과 정확한 실행 evidence, 검증 완료는 `acceptance=ok`로 확인한다. 요약·상태 편집 전용 CLI와 검토용 lifecycle event는 없다. Controller 검토에 Worker `started`를 재사용하지 않는다.
3. 검증 미충족은 원본에 차이·재개조건을 기록하고 Worker에게 직접 message/resume/reassign한다. completed Worker의 새 turn 직전에는 fresh Full Access preflight를 수행한다. 새 assignment/bind와 실제 started/resumed 근거를 기존 절차로 기록하고, 필요하면 canonical을 `doing`으로 환원한다. 단순 dispatch를 착수로 기록하지 않는다.
4. 5/15/10분 유예는 각각 완료 보고/인계·claim·acceptance의 고정 원천시각 기준이다. 현재 projection의 `since`/`grace_until`, 실제 native runtime/journal과 `coordinate`를 함께 읽는다. 조회·요약 갱신으로 유예를 재설정하지 않는다. completed/resume 대기/quiet/unknown 자체를 즉시 dead로 판단하지 않는다. 실제 중단·인계 실패·identity/계약 불일치·유예 만료는 Controller 복구 대상으로 구분한다.
5. 수용 기준 검증과 계약상 완료 처리를 마친 뒤에만 결과·검증·canonical done 및 lifecycle completed를 기록하고 external-synced/finalization을 마친다. Root ACK는 gate가 아니다. 현재 CLI/감시는 native 자동 wake/resume을 보장하지 않으므로, 중단 후 재개 시 아래 checklist부터 남은 일을 처리한다.

### 현재 지원 명령과 기록 한계

아래 `<...>`는 실제 원본/현재 identity/evidence 값으로 대체한다. 조회와 mutation 모두 canonical 원본을 `--project`로 지정한다.

```bash
task-mecca inspect <ID> --project <canonical-project> --json
task-mecca runtime list --project <canonical-project> --json
task-mecca handoff inspect <HANDOFF_ID> --project <canonical-project> --json
task-mecca coordinate --project <canonical-project> --json
task-mecca handoff claim <HANDOFF_ID> --as /root/controller --source-attempt <current-controller-attempt-id> --project <canonical-project> --json
task-mecca handoff mark <HANDOFF_ID> --step acceptance --result ok --evidence <verified-acceptance-evidence> --project <canonical-project> --json
```

claim/mark는 실제 인수·검증 시에만 실행한다. mark 결과는 `ok|failed|unknown`이며 기존 step 결과를 다른 결과로 덮어쓰지 못한다. 실패/unknown을 삭제하거나 재시도로 성공 근거를 만들어내지 않는다. 후속 복구의 새 report/handoff와 원천 연결을 보존한다. `handoff mark --step applied --result ok`는 자동 검증·done 처리 명령이 아니다. Controller가 필요한 backlog-finalized/external-synced와 결과 evidence를 직접 확인한 뒤 finalization에 사용한다.

lifecycle `record`는 `registered|assigned|started|waiting|resumed|completed`만 지원한다. 재작업의 실제 Worker 착수와 최종 종료에 기존 syntax를 사용한다.

```bash
task-mecca lifecycle record started <ID> <stable-event-id> <worker-agent-path> worker_report <actual-start-evidence> <assignment-id> <actual-worker-attempt-id> --project <canonical-project> --json
task-mecca lifecycle record completed <ID> <stable-event-id> /root/controller controller_report <verified-finalization-evidence> <assignment-id> <worker-attempt-id> --project <canonical-project> --json
```

별도의 `review_started` lifecycle 종류, `review` 파일 상태, 요약 수정용 `task update`, native 자동 재개 명령은 현재 없다. handoff claim/mark의 시각과 원본 작업 노트로 Controller 검토 착수 evidence를 기록한다. 추가 자동화가 필요하면 사용자 승인 후 별도 후속 작업으로 정의한다.

## Controller restart and finalization checklist

재개 Controller는 수신확인만 하고 종료하지 않는다. 다음 대조부터 시작하고 완료 결과와 merged PR 잔여를 ready 신규 구현보다 먼저 처리한다.

1. 원본 canonical inspect/계약·Agent·scope·assignment/lifecycle, native Worker state/exact attempt, handoff journal/claim, fresh `coordinate --json`의 recovery_queue를 함께 대조한다. recovery_queue가 비어 있어도 completed Worker와 merged PR를 별도로 확인한다. native/runtime 불일치나 누락을 완료로 추정하지 않는다.
2. 현재 exact Controller attempt로 handoff claim한다. already_applied는 반복하지 않고 already_claimed는 남은 단계부터 복구한다. claim_conflict/contract_changed는 evidence와 재개조건을 남긴다. 교체 Controller가 이전 claim을 임의 applied로 바꾸지 않는다.
3. 계약 수용 기준마다 코드·문서·테스트를 검증한다. 잔여 구현은 fresh dispatch 절차로 resume/reassign하고 실제 새 turn을 확인한다. 사용자 판단이 필요한 경우에만 durable hold와 재개조건을 만든다. 실행 가능한 후속이나 미처리 DONE/BLOCKED가 남아 있으면 조용히 종료하지 않는다.
4. 요구된 회귀/테스트 및 PR CI를 확인하고 최신 target·dirty·다른 writer를 점검해 지정 dev로 직렬 merge한다. remote merge만으로 끝내지 않고 지정 local dev에 merged SHA가 포함되는 ancestry 및 remote/local sync evidence를 기록한다. unsafe conflict는 보존하고 재개조건을 남긴다.
5. Go/Python 배포 지침의 동기화와 현재 설치 지침 refresh를 확인한다. 사용자 수정 managed 문서는 migration backup/동의 정책을 지키고 실제 설치 sync/재읽기 evidence를 기록한다.
6. 원본 결과·검증·핵심 요약 및 실제 lifecycle completed를 기록하고 done/archive한다. stale completed Agent를 doing 담당자로 방치하지 않는다. 전체 queue가 남아 있어도 충족된 개별 항목은 완료한다.
7. 출처가 있는 경우 승인된 외부 원천 writeback을 단일 writer로 수행한다. write 직후 local mark 전 중단이면 원격 evidence를 조회하고 blind retry하지 않는다. 확인 불가는 external-synced=unknown과 재개조건을 남긴다. 외부 도구 부재는 canonical 완료를 되돌리지 않는다.
8. exact worktree를 확인해 미커밋·untracked·ignored·사용자 데이터를 보존한 뒤 안전 cleanup한다. 필요한 파일과 handoff-evidence durable report는 cleanup 전에 Controller가 회수·보존·인계하고 정리 evidence를 남긴다. 원래 workspace나 canonical data를 삭제/reset하지 않는다.
9. backlog-finalized/external-synced 및 transport/검증 근거와 잔여 상태를 각각 mark한 뒤 `handoff mark <HANDOFF_ID> --step applied --result ok --evidence <finalization-evidence> --json`으로 handoff를 마친다. Root 통지는 선택적 best-effort이며 Root ACK/wake는 필요 없다.

## Regression evidence: A-19 / A-20

원본 todo/unassigned가 실제 실행·완료와 어긋난 사례, Worker 완료 후 직접 Controller 인계가 빠져 doing으로 남은 사례를 함께 검증한다. 복구 시 fresh canonical/native/PR/handoff evidence를 기록하고 현재 실제 시각으로 assignment/lifecycle를 기록한다. 가짜 과거 started·attempt·assignment를 만들지 않는다.

실제 Codex resume smoke: kkobugi는 2026-10-07 11:10:22 KST fresh preflight 뒤 completed Controller에 followup_task를 보냈고 running을 확인했다. 확인 토큰은 `TM-WORKER-RESUME-20261007-1110`, 실제 attempt는 `run-c644efc406c0ceba`, turn은 `01a11420-7e64-7b81-85ec-197e563a16a0`다. 이 값은 역사 evidence이며 다른 세션/재개 identity로 재사용하지 않는다. stale run3b4/turn01a113ba는 이 smoke의 근거가 아니다.
