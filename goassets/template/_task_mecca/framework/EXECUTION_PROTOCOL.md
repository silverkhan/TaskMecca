# Canonical assignment and direct completion protocol

> 실행·승인·등록 CLI의 사용자 근거 및 재시도 정책은 [CLI_OPERATIONS_PLAYBOOK.md](CLI_OPERATIONS_PLAYBOOK.md)를 따른다. 기존 canonical Writer/Worker의 권한 경계를 변경하지 않는다.

이 문서는 SESSION_GUIDE 한국어·영어, collab 및 역할 문서의 공통 실행 절차다. Controller는 지정 원본 workspace의 canonical backlog/runtime의 단일 운영 writer다. Worker는 원본 계약을 읽고 구현·검증·실제 착수와 transport evidence를 Controller에 보고한다. Worker는 원본 또는 구현 worktree 사본 backlog/lifecycle/handoff/runtime 운영원장을 수동 생성·수정하거나 monitor에 등록하지 않는다. 승인된 CLI preflight의 ephemeral probe/cache 관측 쓰기와 runtime hook의 자동 관측은 허용된 예외이며 Worker의 운영원장 쓰기 권한이 아니다. Git 구현/지정 dev 통합과 원본 상태 기록은 별개다. Controller는 dev merge를 기다리지 않고 실제 배정·실행·대기·결과를 원본에 기록한다.

## 사용자 판단 시 Root 연락 · 필수 Decision 호출

기존 역할·서브에이전트·인계 경로는 변경하지 않는다. **Controller가 실제 사용자 판단이 필요한 `hold(user)`를 기록한 직후, 이미 존재하던 `hold(user)`를 재개/복구하면서 발견한 때, 해당 사건을 처리하고 종료·대기하려는 직전**에는 반드시 `task-mecca flow next --role controller --event user_decision_required --project PROJECT --task ID --provider PROVIDER --root-session-id EXACT_ROOT_SESSION --json`을 호출한다. 해당 시점은 Worker의 blocked 보고, 계약·승인 충돌, 외부 원천 충돌, 미아 작업 복구 등 **발생 원인과 무관하다.** Worker/Registrar는 기존 방식으로 Controller에게 인계하며 Root에 직접 우회 전송하지 않는다.

1. Controller 단일 writer가 정확한 backlog를 `hold`로 전환하고 `- 대기유형: user`, 구체적인 `- 대기:`, `- 재개조건:`을 기록한다. 일반 internal/external/dependency hold는 Root 요청으로 오분류하지 않는다.
2. 위 `flow next` 정책 함수를 **호출하고 반환된 `NextAction`을 실제로 따른다.** B/C이며 capability와 대상 신원을 확인할 수 있으면 `task-mecca orchestration notify --project PROJECT --provider PROVIDER --root-session-id EXACT_ROOT_SESSION --task ID --controller-attempt SOURCE --root-attempt TARGET --json`을 호출한다.
3. `notify`가 `directive`를 반환했을 때에만 Controller가 현재 Agent 환경의 native message/resume 도구로 **정확한 Root session/attempt에 `directive.message`를 전송**한다. 단순한 CLI 성공/문구를 실제 전달로 기록하지 않는다. `resume_completed_after_preflight`는 fresh Full Access preflight와 실제 wake 지원이 필요하다.
4. 실제 native 전송 결과를 확인한 뒤 기존 Handoff의 `dispatch` 단계를 `ok/failed/unknown`으로 기록한다. 이미 `dispatch=ok`, 수신 Claim/Applied, 혹은 unresolved failed/unknown이면 중복 전송을 지시하지 않으며 기존 handoff/recovery로 확인한다.
5. A/D, Root 신원 미확인, 설정 미구성, 메시지 기능 상실, 또는 `notify`에 지시가 없는 경우에는 **기존 방식 그대로 canonical `hold(user)`와 재개 조건을 유지**하고 다음 Root 사용자 대화에서 회수한다. Root ACK/응답·임의 polling을 Controller 완료조건으로 추가하지 않는다.

동일 이벤트 재호출 시 과거 대화 기억보다 canonical backlog와 기존 Handoff journal의 확인된 사실을 우선한다. 안내문이나 지침만으로 네이티브 메시지가 도달했다고 선언하지 않는다.

## Activity Recorder · 작업시간 경량 기록

Activity는 Lifecycle/승인 원장과 별개의 **측정 전용 이벤트**다. 가장 적은 호출로 실제 활동을 구분할 수 있도록, 역할별 Decision 결과를 실행한 다음 실제 사건을 `task-mecca activity report TASK --project PROJECT --role ROLE --agent EXACT_ID --attempt EXACT_ATTEMPT --event PHASE_EVENT --event-id STABLE_ID --evidence SHORT_REASON ... --json`으로 **한 번만** 보고한다. Recorder가 역할·회차를 참고해 start/switch/stop 및 표준 활동 유형을 선택한다. Decision이 다음 행동을 권고했다는 이유만으로 Activity를 시작하지 않는다.

- **Worker:** `requirements_started`, `research_started`, `implementation_started`, `worker_completed` 등을 실제 착수·완료 시각에만 보고한다. 검증/빌드 명령은 `activity run ... -- PROGRAM ARGS`로 실행해 시작·종료·실행 결과를 자동 계측한다. 전체 회귀 테스트를 추가로 자동 실행하지 않는다.
- **Main Controller:** 실제 Handoff 인수 후 `review_started`, 검토 후 `review_accepted` / `review_rejected` / `review_needs_changes`를 보고한다. 이 명시적인 검토 결과로만 재작업 회차를 증가시킨다. Sub-Controller의 검수 보고는 Main 최종 수용과 다르다.
- **Registrar/Sub-Controller:** 본인의 정확한 Runtime 바인딩에서만 보고하며, 분류 권한이나 Activity 원장 기록이 원본 backlog writer·최종 승인 권한을 부여하지 않는다.
- **미분류:** 사전에 없는 사건은 `other --explanation DESCRIPTION --proposed-type CANDIDATE` 형태로 Agent 판단을 남긴다. 필요한 경우 `activity suggest`에서 유형만 미리 조회할 수 있다.

Worker는 실제 `--assignment ID`, Controller는 claimed `--handoff ID`를 함께 전달한다. Runtime/attempt/등록 ID가 불명확하면 실행 사실이나 시간을 추측하지 않는다. `event-id`는 같은 사건 재시도 때 동일하게 사용한다. 모든 tool call마다 기록하거나 주기적 heartbeat를 생성하지 않는다. 기록 실패를 성공처럼 주장하지 않되, Activity 관측만 실패했다고 구현·검수·인계를 취소하지도 않는다.

현재 기록된 활동 유형은 `agent_report / inferred`이며 CPU/배터리 사용량이나 운영 병목의 인과적 증거가 아니다. 세부 명령과 한계는 `task-mecca help activity`, 저장소의 `docs/activity-recorder.md`를 참조한다.

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

Tool addressing precedes the checklist's message/resume choice. Native UUIDs
belong to runtime observation/binding; they are not collaboration handles.
`handoff transport-plan EXACT_HANDOFF --project ORIGINAL --json` returns the
verified current tool/address, or an explicit hold/unsupported reason. The
collaboration calls below require that tool's observed exact agent handle.
When only an exact T3 route is supported, use `t3_thread_read` + current native
episode verification, then fresh canonical Full Access immediately before a
completed-target `t3_thread_send(threadId=EXACT_T3, mode=auto)`. Never infer a
handle from a name, role, UUID or transport thread. Preserve original failure,
stable report ID, exact recipient, report path, checkpoint owner/resume condition.
An actual transport ACK does not claim/apply the handoff; Controller recovery
continues independently. Unknown delivery must be reconciled before retry.

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
2. 현재 exact Controller attempt로 handoff claim한다. already_applied는 반복하지 않고 already_claimed는 남은 단계부터 복구한다. `claimant_attempt_mismatch` 또는 `claimed_by_other_runtime_attempt`이고 과거 Controller가 종료된 경우, **자동 재작성/새 handoff 생성이 아니라** 아래 안전한 `handoff transfer` 경로로 기존 Claim을 이전한다. 이전 Controller의 실제 terminal 상태, 신규 Controller의 observed running·exact binding, 다른 동시 Controller 부재, 미완료 handoff 및 계약 해시 일치를 검증하지 못하면 transfer를 보류한다. `contract_changed`는 evidence와 재개조건을 남긴다. 교체 Controller가 이전 claim을 임의 applied로 바꾸지 않는다.

### 종료된 Controller Claim의 안전한 이전 (B-575 회귀)

```bash
task-mecca handoff inspect HANDOFF_ID --project PROJECT --json
task-mecca runtime query --project PROJECT --json
task-mecca handoff transfer HANDOFF_ID --from-attempt EXACT_OLD --to-attempt EXACT_NEW --evidence "observed old terminal; new Controller observed running; contract rechecked" --project PROJECT --json
task-mecca handoff claim HANDOFF_ID --as /root/controller --source-attempt EXACT_NEW --project PROJECT --json
task-mecca handoff mark HANDOFF_ID --step acceptance --result ok --source-attempt EXACT_NEW --evidence "verified current review" --project PROJECT --json
```

인수권 이전은 원래 Handoff ID와 승인·계약·기존 Claim·검토 단계 기록을 보존하는 `claim_transferred` append-only 전이이며, 이전 종료 및 신규 실행의 Provider/Runtime 근거와 독립적 Controller 한 명을 확인해야 한다. `duplicate=true`인 `prepare`를 강제 새 Handoff로 만들지 않는다. **Transfer 이후의 모든 `handoff mark`에는 현재 Claimant의 `--source-attempt`가 필수**이며, 이전 Controller의 `mark`는 거부된다. 과거 기록의 이름·runtime attempt를 추측하거나 원장 JSONL을 수동 변경하지 않는다. 이 작업은 네이티브 메시지 실제 전달·CLM 전역 파일 writer fencing을 증명하지 않는다.
3. 계약 수용 기준마다 코드·문서·테스트를 검증한다. 잔여 구현은 fresh dispatch 절차로 resume/reassign하고 실제 새 turn을 확인한다. 사용자 판단이 필요한 경우에만 durable hold와 재개조건을 만든다. 실행 가능한 후속이나 미처리 DONE/BLOCKED가 남아 있으면 조용히 종료하지 않는다.
4. 요구된 회귀/테스트 및 PR CI를 확인하고 최신 target·dirty·다른 writer를 점검해 지정 dev로 직렬 merge한다. remote merge만으로 끝내지 않고 지정 local dev에 merged SHA가 포함되는 ancestry 및 remote/local sync evidence를 기록한다. unsafe conflict는 보존하고 재개조건을 남긴다.
5. Go/Python 배포 지침의 동기화와 현재 설치 지침 refresh를 확인한다. 사용자 수정 managed 문서는 migration backup/동의 정책을 지키고 실제 설치 sync/재읽기 evidence를 기록한다.
6. 원본 결과·검증·핵심 요약 및 실제 lifecycle completed를 기록하고 done/archive한다. stale completed Agent를 doing 담당자로 방치하지 않는다. 전체 queue가 남아 있어도 충족된 개별 항목은 완료한다.
7. 출처가 있는 경우 승인된 외부 원천 writeback을 단일 writer로 수행한다. write 직후 local mark 전 중단이면 원격 evidence를 조회하고 blind retry하지 않는다. 확인 불가는 external-synced=unknown과 재개조건을 남긴다. 외부 도구 부재는 canonical 완료를 되돌리지 않는다.
8. **Decision Engine P1 (설치판 지원 시):** Controller는 Worker 산출물을 통합하고 원본 backlog done을 확인한 뒤 `task-mecca flow next --role controller --event final_requested --project REPO --task ID --into REF --worktree EXACT --archive EXTERNAL --json`으로 **실제 작업/워크트리/보존 증거**를 읽는다. `EXIT_BLOCKED`이면 정리 완료를 가장하지 않고 남은 단계와 담당자·이유·재개조건을 원본 canonical 노트 및 후속 backlog에 남긴다. 현 P1은 advisory/read-only로 모델의 final/exit를 기술적으로 인터셉트하지 않는다. `cleanup_status=removed_verified`도 다른 인계·외부 원천·미완료 Worker 확인 없이 전체 종료 승인이 아니다.

8. exact worktree를 확인해 미커밋·untracked·ignored·사용자 데이터를 보존한 뒤 안전 cleanup한다. 필요한 파일과 handoff-evidence durable report는 cleanup 전에 Controller가 회수·보존·인계하고 정리 evidence를 남긴다. 원래 workspace나 canonical data를 삭제/reset하지 않는다.
9. backlog-finalized/external-synced 및 transport/검증 근거와 잔여 상태를 각각 mark한 뒤 `handoff mark <HANDOFF_ID> --step applied --result ok --source-attempt <CURRENT_CONTROLLER_ATTEMPT> --evidence <finalization-evidence> --json`으로 handoff를 마친다. Root 통지는 선택적 best-effort이며 Root ACK/wake는 필요 없다.

## Regression evidence: A-19 / A-20

원본 todo/unassigned가 실제 실행·완료와 어긋난 사례, Worker 완료 후 직접 Controller 인계가 빠져 doing으로 남은 사례를 함께 검증한다. 복구 시 fresh canonical/native/PR/handoff evidence를 기록하고 현재 실제 시각으로 assignment/lifecycle를 기록한다. 가짜 과거 started·attempt·assignment를 만들지 않는다.

실제 Codex resume smoke: kkobugi는 2026-10-07 11:10:22 KST fresh preflight 뒤 completed Controller에 followup_task를 보냈고 running을 확인했다. 확인 토큰은 `TM-WORKER-RESUME-20261007-1110`, 실제 attempt는 `run-c644efc406c0ceba`, turn은 `01a11420-7e64-7b81-85ec-197e563a16a0`다. 이 값은 역사 evidence이며 다른 세션/재개 identity로 재사용하지 않는다. stale run3b4/turn01a113ba는 이 smoke의 근거가 아니다.

## 공식 설치와 migration

Controller는 검증한 공식 bundle의 `init`/`migrate`로 설치를 반영한다. 소스 직접 복사나 manifest baseline 수동 수정으로 경고를 숨기지 않는다. 배포본 버전·checksum·소스 commit과 승인된 파일의 source/destination hash를 기록하고, 당시 명령이 없으면 추정 명령을 실행 사실로 말하지 않는다. 오래된 CLI로 최신 설치를 되돌리지 않는다. 적용 전 전체 managed 파일과 manifest를 별도 백업하며 프로젝트 소유 data·backlog·runtime·설정과 출처 불명 변경을 보존한다.


`migrate --json`은 읽기 전용 조회가 아니다. modified 파일이 있으면 `choice_required`와 계획을 반환하여 선택을 기다리지만, modified 파일이 없으면 첫 호출에서도 자동 적용한다. 따라서 공식 bundle의 전체 교체 범위·출처 대조, 전체 managed 파일과 manifest 백업, 프로젝트 소유 데이터 보존 스냅샷을 어떤 `migrate` 호출보다 먼저 준비한다. modified가 있을 때 반환된 계획을 확인하고 승인된 선택에 `--expect-plan`을 사용한다. `--choice cancel`은 계획을 만들지 않고 즉시 취소하므로 계획 조회 명령으로 사용하지 않는다.

`migrate --project <project> --json`으로 `modified_files`·전체 managed 교체 범위·`plan_digest`를 승인 범위 및 출처와 대조한다. 출처 불명 변경은 판단과 재개 조건을 기록하고 적용을 멈춘다. 같은 검증 binary로 `migrate --project <project> --choice backup --expect-plan <plan_digest> --json`을 수행한다. 내장 backup은 충돌 파일의 백업이며 전체 managed 파일·manifest 사전 백업을 대신하지 않는다. 계획이 달라지면 새 계획을 검토한다. 같은 bundle로 다시 migration 검사하여 baseline과 hash를 확인하고 데이터 보존 결과를 기록한다. `instruction_refresh_required`이면 지침을 다시 읽는다. 배포 소스 문서 변경을 설치에도 반영하려면 그 문서가 포함된 새 공식 배포본을 검증하고 이 절차를 다시 사용한다. 먼저 직접 복사하여 동기화하지 않는다.

## 공식 세션 이름 적용과 확인 (필수)

모든 관리 역할은 기존 `orchestration.SessionName` / `orchestration name`의
`DMMDD_NN_` 그룹·역할·Worker·세대 규칙 하나만 사용한다. 작업 ID·작업 제목을
세션 이름으로 대신하지 않는다. 이름은 표시 metadata이며 claim/dispatch/권한의
근거가 아니다. provider/session ID, runtime agent ID, attempt, 내부 Worker 경로,
handoff 연결은 그대로 보존한다. 사용자 다른 세션과 완료된 과거 세션을 일괄 변경하지 않는다.

**native dispatch 직전** fresh canonical contract/Full Access gate와 함께 다음을 실행한다.
새 생성은 `newSession`, 정확한 기존 관리 세션의 resume/reuse는 `existingSession`이다.

```sh
task-mecca orchestration session-name plan --project ORIGINAL_PROJECT --group D1112_01 \
  --role worker --worker pikachyu --generation 1 --provider codex \
  --execution newSession --worker-path /root/controller/pikachyu --json
# 기존 세션에는 --execution existingSession --session-id EXACT_NATIVE_ID 사용
```

반환된 `directive`의 before/after/resume 지시를 실제 dispatch prompt에 포함한다.
`naming_support=supported`는 설치된 공식 인터페이스의 지원 확인일 뿐 적용 증거가 아니다.
`plan`과 `name`은 `verified=false`, `dispatch_authorized=false`이며 실행 승인을 대신하지 않는다.
기존 세션의 plan은 exact ID를 공식 조회하여 다른 생성 이름/세대면 `identity_conflict`로 거부한다.
기존 legacy 표시 이름은 `mismatch`로 남기고 해당 관리 세션이 실제 재개된 뒤 적용한다.
T3 delegate 생성 시 `delegate_task.title`에 **생성된 이름 그대로** 넣는다.
Codex native ID는 실제 spawn 결과/현재 환경에서 확인한 후 적용하며 제목에서 추측하지 않는다.
Claude native 생성이 지원되면 공식 `--name`을 사용한다. 모드/provider를 임의 전환하지 않는다.

**실제 생성 또는 재개 직후**, 대상 세션 자신이 exact native ID로 실행한다.
아래 `apply`와 `verify`는 원본 project를 읽기만 하고 공식 provider 표시 metadata만 처리한다.
operational backlog/runtime/handoff/Activity에 기록하지 않는다.

```sh
task-mecca orchestration session-name apply --project ORIGINAL_PROJECT --group D1112_01 \
  --role worker --worker pikachyu --generation 1 --provider codex \
  --execution existingSession --session-id "$CODEX_THREAD_ID" --json
task-mecca orchestration session-name verify --project ORIGINAL_PROJECT --group D1112_01 \
  --role worker --worker pikachyu --generation 1 --provider codex \
  --execution existingSession --session-id "$CODEX_THREAD_ID" --json
```

Controller 등은 해당 `--role`을 쓰며 worker/generation 옵션은 Worker에만 해당한다.
Claude 경로는 실제 선택된 Claude 세션의 `CLAUDE_SESSION_ID`와 `--provider claude`를 쓴다.
현재 native 환경과 다른 ID, caller JSON receipt/ownership/capability 선언은 증거로 받지 않는다.
Codex는 설치 CLI가 생성한 공식 schema를 확인하고 app-server의
`initialize` → `initialized` → `thread/read(includeTurns:false)` → 필요한 경우만
`thread/name/set(threadId,name)` → `thread/read`로 exact ID와 `thread.name`을 비교한다.
Claude는 설치 공식 SDK의 `renameSession`/`getSessionInfo` export를 확인하고
`customTitle`과 `sessionId`를 비교한다. SDK query/model turn은 실행하지 않는다.

T3를 썼으면 **별도로** 제공된 `t3_thread_read`로 exact current thread의 `title`을 확인한다.
다르면 `t3_thread_update`로 생성된 제목을 적용하고 다시 `t3_thread_read`한다.
이미 같으면 변경하지 않는다. CLI는 caller JSON으로 T3를 검증했다고 주장하지 않으며
`t3_status=agent_tool_read_required`, `naming_complete=false`를 유지한다.
Controller는 실제 native receipt와 실제 T3 tool readback 둘 다 확인한 후에만 완료로 보고한다.
최소 보고는 생성 이름, exact native ID/읽힌 이름, exact T3 ID/읽힌 제목, 실제 사용한
공식 도구·시각·결과이며 transcript/secret은 보관하지 않는다. Worker는 이 근거를 Controller에
직접 보내고 Controller만 canonical 운영원장에 반영한다.

동일 native 세션의 resume/reuse는 기존 group/role/worker/generation을 유지한다.
새 turn이라는 이유로 G를 증가시키지 않는다. 다른 생성 이름으로 변경하는 apply는
`identity_conflict`다. 매 resume 직후에도 provider와 T3를 fresh 재조회하며 옛 receipt를
현재 증거로 재사용하지 않는다. 이름을 바꾸어 runtime/attempt/handoff 신원을 바꾸지 않는다.

`unsupported`, `timeout`, `error`, `partial`, `mismatch`, `identity_conflict`는 성공이 아니다.
API ack만으로 `applied/verified=true`가 되지 않는다. 쓰기 요청 후 readback이 없으면
`requested_not_confirmed=true`이고, 재시도 전 verify로 실제 상태를 확인한다.
미지원/미검증은 원인·현재 근거·재개 조건을 직접 보고하고 provider/mode를 바꾸지 않는다.
이 절차는 Agent 지시이며 native spawn/메시지 session manager를 새로 만들지 않는다.

공식 계약 확인 출처 (2026-11-12):
[Codex app-server](https://learn.chatgpt.com/docs/app-server),
설치 `codex app-server generate-json-schema --out TEMP_DIR`의
`ClientRequest`/`ThreadSetNameParams`/`ThreadReadParams`,
[Claude CLI --name 및 /rename](https://code.claude.com/docs/en/cli-reference),
[Claude SDK renameSession/getSessionInfo/customTitle](https://code.claude.com/docs/en/agent-sdk/typescript#renamesession).
Claude 지원은 실제 설치 SDK capability와 실제 세션 readback이 있을 때만 주장한다.
문서/fixture 확인과 실제 Claude 실행 검증은 구분하여 보고한다.
# Worker START collection boundary

Follow the required START/phase sections in `roles/worker.md` and
`roles/controller.md`: actual environment native/T3 IDs -> official exact
observation -> exact canonical assignment/task binding -> Worker current-phase
`activity report` receipt. Send START and each phase success/failure directly to
Controller. Failed transport, unbound identity and report failure remain visible
collection gaps with supported retry; no Root ACK gate or historical backfill.
The Controller owns Runtime/lifecycle/handoff writes; the Worker may report only
its own real Activity through the verified CLI. Keep canonical project separate
from source worktree, and native start separate from START report/relay time.
Reported lifecycle wait, verified Runtime, total elapsed and Activity coverage
are different quantities. Missing Activity remains unmeasured after completion.
