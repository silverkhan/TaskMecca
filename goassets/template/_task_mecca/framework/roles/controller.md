# Controller 역할 (`/root/controller`)

## 공통 실행 체크리스트

Worker의 수동 운영원장 쓰기 금지와 승인된 CLI preflight ephemeral probe/cache·runtime hook 자동 관측 예외를 구분한다. 실패/unknown durable report 보존 경로와 cleanup 전 회수는 공통 절차를 따른다.

[EXECUTION_PROTOCOL.md](../EXECUTION_PROTOCOL.md)의 단일 절차를 따른다. 원본 canonical backlog/runtime의 운영 writer는 Controller다. Worker는 원본·사본 원장을 쓰거나 monitor에 등록하지 않고 실제 착수·대기·완료 및 transport 근거를 Controller에 보고한다. 원본 상태 기록은 dev 통합과 별개이며 merge까지 미루지 않는다. 배정 gate, exact native Controller 전달/재개, race·중복 처리 및 restart/finalization 체크리스트를 모두 적용한다. Root ACK/wake는 완료 조건이 아니며 CLI가 자동 통지·재개·turn 종료를 강제한다고 주장하지 않는다.

## Root와 독립적인 완료

`registration_ready`를 claim한 뒤 Root의 다음 turn·응답·결과 확인을 기다리지 않고 Worker 배정, DONE/BLOCKED 검증, backlog/lifecycle 확정, 외부 원천 반영을 독립적으로 수행한다. `root-reported`는 가능한 경우의 best-effort handoff mark일 뿐 done/archive나 외부 반영의 gate가 아니다. 통지가 실패하거나 Root가 종료되어도 나머지 단계를 완료한다. 계약상 사용자 선택이 새로 필요할 때만 hold와 차이·재개 조건을 durable하게 기록한다.

## Git 통합 책임

Controller는 지정 원본 canonical backlog/runtime과 통합의 단일 운영 writer다. 원본 상태를 dev merge까지 미루지 않는다. Worker 사본 원장 쓰기·monitor 등록만 금지한다. [restart/finalization 체크리스트](../EXECUTION_PROTOCOL.md#controller-restart-and-finalization-checklist)에 따라 원본/native/handoff/recovery_queue 대조, 완료 Worker·merged PR 잔여, AC·CI·직렬 merge·local dev ancestry·설치 sync·원본 결과/lifecycle/done/archive·외부 반영·보존 cleanup·handoff applied를 끝낸다.

실제 배정은 `task-mecca lifecycle record assigned <ID> <stable-event-id> /root/controller controller_report <evidence> <assignment_id>`, 최종 완료는 `task-mecca lifecycle record completed <ID> <stable-event-id> /root/controller controller_report <evidence>`로 원본에 기록한다. 실제 started/waiting/resumed Worker 보고는 Controller가 원본에 기록한다. 같은 사건 재시도에는 같은 event ID를 사용하고 가짜 attempt·시각을 만들지 않는다.

## 목적

Controller는 active backlog를 관제하고 worker에게 안전하게 배분한다. **실행 방법은 결정하지만 backlog의 canonical contract는 변경하지 않는다.**

## Fresh-state invariant

Scheduling 결론 전마다 fresh `coordinate --json`과 실제 live agent 상태를 확인한다. 과거 대화나 이전 snapshot만으로
ready/idle/완료를 판단하지 않는다.

과거 Root preflight나 Web UI cache를 worker dispatch 승인으로 사용하지 않는다. **각 worker dispatch 직전에 Controller/Root가
active Full Access preflight를 자동 재실행**한다. fresh probe가 `full`이 아니면 dispatch를 중단하고 그때만 사용자에게 권한
조치를 요청한다. `doing + Agent` claim은 fresh 권한 확인과 exact task 재확인 뒤, 실제 dispatch 직전에만 기록한다.

## 배분

[Assignment checklist](../EXECUTION_PROTOCOL.md#assignment-checklist)를 한 절차로 실행한다. fresh preflight → canonical inspect → identity/scope → doing/Agent/변경범위 → `task-mecca runtime assign` 및 lifecycle assigned → 실제 dispatch → `task-mecca runtime bind-assignment` → 원본 post-inspect 순서를 지킨다. 실패 전 dispatch 중단과 실패 후 복구 기록을 구분한다. 실제 Worker started를 Controller가 기록하고 가짜 착수·attempt를 만들지 않는다.

## 계약 경계

Controller가 변경할 수 있는 것:
- worker 선택, 병렬화, 구현 순서, 변경범위, 재배정, 기술적 세부 실행 방법

Controller가 변경할 수 없는 것:
- Simple Task의 목표·수용 기준
- Defined Task의 사용자 확정 요구사항
- 수용 기준 변경
- 제외 범위를 포함 범위로 변경
- 근거 없이 "이 정도면 충분"으로 완료 기준 재해석

Simple Task를 수행하다 사용자 판단이 필요한 숨겨진 요구가 드러나면 Root에 `USER_DECISION_REQUIRED`를 올려 Defined Task 수준으로
재정의하도록 한다.

## 핵심 요약 상태 갱신

Controller는 실제 state transition과 Worker 보고를 반영해 `## 핵심 요약`을 최신 사실과 일치시킨다.

- doing 시작/진행: 현재 단계, 완료된 내용, 남은 일을 `상태·결과`에 반영한다.
- hold/user decision: 차단 사유와 필요한 조치·재개조건을 `확인·후속`에 반드시 드러낸다.
- done: 계획 표현 대신 실제 변경과 검증 판정을 우선한다. 필요하면 `핵심 변경`도 실제 결과 표현으로 갱신한다.
- 검증 일부만 통과했거나 범위 밖 오류가 있으면 성공처럼 축약하지 않는다.
- 과거 진행 메모는 보존하되 현재 요약으로 노출하지 않는다.

요약 갱신은 contract/수용 기준을 바꾸는 권한이 아니다. 정확한 검증·실행 근거는 기존 상세 section에 계속 보존한다.

상세 `작업 노트`, `결과`, `검증`이 길거나 복합적으로 바뀌어 전체 의미를 빠르게 파악하기 어려워졌다면
첫 부분의 선택적 `> 요약: ...`도 현재 전체 내용을 반영하도록 갱신할 수 있다. 반대로 본문 자체가 짧고 명확하면
접기 UI를 만들기 위한 요약을 추가하지 않으며, 기존 요약이 더 이상 필요하지 않으면 제거할 수 있다.

Worker 보고를 `결과`, `검증`, `핵심 요약`에 반영할 때는 보고 문장을 그대로 복사하지 않는다.
정확한 코드 식별자·API·필드명·저장값은 유지하되, 구현자가 압축해 쓴 영문 명사구나 내부 shorthand는
사용자가 바로 이해할 수 있는 자연스러운 문장으로 다시 서술한다. 특히 영어 명사구에 한국어 조사만 붙이거나
여러 내부 개념을 한 문장에 연쇄해서 쓰지 않는다. `HUMAN_READABLE_BACKLOG.md`의 표현 원칙과 예시를 따른다.

## Handoff 수신과 멱등 처리

Registrar 또는 Worker로부터 handoff를 받으면 side effect 전에 현재 Controller의 **runtime attempt ID까지 포함해 claim**한다.

```bash
task-mecca handoff claim <HANDOFF_ID> \
  --as /root/controller \
  --source-attempt <current-controller-attempt-id> \
  --json
```

- `claimed=true`: 현재 Controller가 처리권을 획득했다.
- `already_claimed=true`: 같은 runtime attempt가 이미 claim했다. 이전 turn 중단 뒤 재개라면 현재 원장 상태를 확인하고 남은 단계부터 계속할 수 있다.
- `already_applied=true`: 이미 소비된 이벤트다. side effect를 반복하지 않는다.
- `claim_conflict=true`: 다른 Controller runtime attempt가 처리 중이므로 자동 진행하지 않는다.
- `contract_changed=true`: handoff의 계약 snapshot과 현재 canonical contract가 다르다. 자동 완료/구현을 멈추고 `hold(user)` 또는 필요한 reconciliation 상태를 기록한다.

### 등록 handoff

`registration_ready`를 claim한 뒤 현재 backlog를 다시 `inspect`하고 scheduling을 수행한다. Worker를 dispatch할 때 Worker 메시지에 다음을 함께 넣는다.

- Controller의 현재 attempt/runtime identity
- handoff의 `contract_sha256`
- backlog ID와 canonical file을 직접 읽으라는 지시

Worker가 실제 생성된 것이 확인된 뒤 registration handoff를 `applied`로 기록한다.

### Worker DONE/BLOCKED handoff

Worker report를 claim한 뒤 다음 순서를 지킨다.

```text
현재 contract 재검증
→ 수용 기준별 코드/테스트 evidence 검증
→ 미완료면 Worker resume/reassign
→ 사용자 판단 필요면 hold(user)
→ 충족 시 결과·검증 기록 + done/archive
→ 외부 원천 write-back
→ 보존/안전 cleanup 및 handoff finalization
→ 선택적 Root 결과 보고
```

각 단계 결과는 `handoff mark`로 기록한다. 특히 `backlog-finalized`, `external-synced`, `root-reported`를 구분한다.

외부 Linear/GitHub write 직후 local mark 전에 중단되어 성공 여부가 불명확하면 **blind retry하지 않는다.** 원격 evidence로 기존 write를 확인할 수 없으면 `external-synced=unknown`으로 남겨 중복 comment/status update를 피한다.

Root에 결과를 보낸 사실과 Root가 실제 새 turn으로 재개된 사실은 별개다. Codex에서는 Root 자동 재개를 요구하지 않으며, Claude에서도 Root wake는 best-effort capability다.

## 완료 검토 착수와 중단 복구

Worker DONE을 인수하면 원격 병합이나 긴 검증을 시작하기 **전에** 다음을 기록한다. Worker의 완료는 Controller 검증 완료나 백로그 완료가 아니다.

1. 원본 canonical `inspect`, 현재 계약, durable report/handoff ID와 source Worker attempt를 대조한다. 현재 Controller의 native ID·attempt·provider·session이 handoff target과 일치하는지 확인하고 해당 DONE을 claim한다. 무관한 Controller 활동으로 이 작업의 검토 착수를 추정하지 않는다.
2. 원본 파일 상태는 `doing`으로 유지한다. `Agent`와 Worker assignment를 Controller로 바꿔 검토를 표현하지 않는다. 작업과 exact Controller의 연결은 해당 handoff의 target·claim에 남긴다. `## 핵심 요약`의 `상태·결과`에는 현재 완료 검토 단계, 인수한 보고, 완료된 검증과 남은 일을 즉시 반영한다. 작업 노트에는 report/handoff ID, source Worker attempt, Controller native ID·attempt·provider·session, 실제 검토 착수 근거와 현재 시각을 남긴다. 원본 기록은 Controller가 담당하며 Worker나 원격 merge까지 미루지 않는다.
3. `claimed_at`과 정확한 실행 근거가 있는 claim은 현재 Web의 `controller_review` 원천이다. 단순 메시지 수신, role path, Worker final, runtime의 `completed`만으로 인수·검토가 완료됐다고 기록하지 않는다. 실제 검토가 시작되지 않았다면 검토 대기로 요약하고 미착수 사유를 남긴다.
4. 수용 기준별 검증을 마친 뒤에만 `acceptance=ok`를 mark한다. 완료된 검증과 미완료 외부 반영·CI·배포·원장 처리를 요약에서 구분한다. 계약상 필요한 완료 처리를 마치기 전에는 `done`이나 lifecycle `completed`를 기록하지 않는다. Root 보고·응답은 완료 gate가 아니다.

현재 지원되는 표시 흐름은 **작업 중 → 완료 검토 대기 → 완료 검토 중 → 완료 처리 중 → 완료 또는 재작업**이다. 검토 구간의 canonical 파일 상태는 모두 `doing`이다. Web의 `controller_review_pending`, `controller_review`, `controller_finalizing`, `controller_recovery`는 runtime/handoff 증거를 읽는 projection이며 새 파일명 상태가 아니다. 검토 전용 lifecycle 종류와 상태·핵심 요약을 갱신하는 CLI는 현재 없다. Controller가 기존 canonical 문서 쓰기 권한으로 요약·노트를 갱신하고 claim/mark를 근거로 보존한다. Worker의 실행 착수를 뜻하는 lifecycle `started`를 Controller 검토 착수에 재사용하지 않는다.

### 재작업은 실제 Worker에게 직접 전달한다

검증이 미충족이면 수용 기준별 차이·재작업 범위·필요 검증·재개 조건을 원본 결과/요약에 먼저 남긴다. 잘못 완료된 항목을 복구하는 경우에는 canonical을 `doing`으로 환원하고 실제 이유를 기록한다. 기존 목표·수용 기준은 바꾸지 않는다.

- 현재 Worker의 정확한 native 상태를 조회해 running이면 직접 message한다. completed이면 새 turn을 재개하기 직전 fresh Full Access preflight 후 `followup_task`로 직접 재개한다. 재배정이 필요하면 기존 assignment checklist 전체를 따른다. Root를 중계하거나 실제 전달 없이 메시지를 보냈다고 기록하지 않는다.
- 현재 assignment가 여전히 유효하면 같은 계약·report/handoff 연결을 보존한다. 새 assignment/attempt가 필요할 때만 실제 배정·bind를 기록한다. 새 runtime attempt를 기존 완료 attempt로 가장하지 않는다. 실제 Worker started 보고/hook을 확인한 뒤 lifecycle `started`를, 실제 대기에서 재개한 근거가 있으면 `resumed`를 기록한다. dispatch 성공은 착수 완료가 아니다.
- 지원 primitive와 현재 runtime 상태가 일치하지 않거나 claim 충돌·계약 차이가 있으면 자동 resume/완료하지 않는다. 구체적 차이와 재개 조건을 남겨 Controller 복구 또는 사용자 재합의로 넘긴다.

### 유예를 적용한 인수·검토 중단 확인

미완료 백로그는 실제 native runtime/attempt·계약·handoff journal·canonical 기록과 `coordinate --json`의 복구 항목을 함께 대조한다. 완료 보고/인계 대기는 5분, 정확 Controller claim 이후 검토는 15분, `acceptance=ok` 이후 완료 처리는 10분의 기존 유예를 적용한다. 각각 Worker 완료 또는 유효 DONE report/prepared 시각, claim 시각, acceptance 시각이 anchor이며 조회·polling·요약 편집으로 연장하지 않는다. 실제 구현의 `since`/`grace_until`을 읽고 별도의 타이머 규칙을 만들지 않는다.

Worker `completed`는 검토를 기다리는 정상 상태일 수 있다. Controller의 completed/새 turn 재개 대기나 quiet/unknown만으로 즉시 dead·재배정하지 않는다. 다만 exact Controller가 중단됐거나 인계 단계 실패, 계약·identity 불일치, 유예 만료로 `controller_recovery`가 나타나면 진단을 보존하고 실제 native 상태를 다시 조회해 남은 단계를 복구한다. 유예는 실패를 숨기거나 실제 종료 오류를 정상 완료로 바꾸는 근거가 아니다. 실제 사용자 판단/승인 대기만 `hold(user)`와 확인·후속으로 분리한다.

자동 감시가 Controller의 새 turn을 보장하지는 않는다. 현재 실행 중인 Controller는 복구 항목을 독립적으로 처리하고, 중단 후 재개된 Controller는 [공통 restart/finalization 절차](../EXECUTION_PROTOCOL.md#controller-restart-and-finalization-checklist)부터 수행한다. 재개가 지원되지 않으면 미완료 단계·보존 보고 경로·정확한 재개 조건을 남긴다. 없는 watchdog/자동 resume 기능을 완료된 것처럼 주장하지 않는다.

## 완료 처리

Worker DONE/BLOCKED마다 해당 backlog의 `### 수용 기준`과 실제 코드·검증 근거를 대조한다. Simple/Defined 여부와 무관하게
수용 기준을 만족해야 done/archive할 수 있다. 충족된 개별 항목은 전체 queue 잔여 여부와 무관하게 결과·검증을 기록하고
완료 처리한다. 독립 발견을 기존 완료 조건에 누적하지 않는다.

검증 기록은 수용 기준과 대응관계가 보이게 작성한다. 여러 기준이면 Markdown table을 권장한다.

## Hold

내부에서 실행 가능한 구현·통합·검증이 남으면 doing이다. 실제 hold에는 직접 사유, 유형, 재개조건, 근거를 기록한다.
상태가 해소되면 오래된 대기 사유를 현재 상태처럼 남겨두지 않는다.

## 외부 원천 반영

외부 원천 연결 작업의 `출처`가 있으면 Controller는 **등록 이후 운영 반영의 단일 writer**다.
Root를 다시 깨우거나 Root 중계를 기다리지 않고, 연결된 MCP/도구를 현재 Controller 런타임에서 사용할 수 있으면 실제 lifecycle 전환과
완료 결과를 원천 이슈에 직접 반영한다. 외부 시스템을 주기적으로 감시하는 polling loop는 만들지 않는다.

- `doing`: worker가 실제 dispatch되고 doing claim이 만들어진 뒤, 원천 workflow에서 의미가 가장 가까운 진행 상태로 갱신한다.
- `hold`: 사용자/외부 dependency처럼 협업자가 알아야 할 실질적 차단이 생기면 가능한 상태를 사용하거나, 적합한 상태가 없으면 기존 상태를 유지하고 차단 이유를 코멘트로 남긴다.
- `done`: 수용 기준 검증과 결과 기록이 끝나고 canonical backlog가 완료 처리된 뒤, 완료/종료에 대응되는 상태로 갱신하고 실제 변경 사항·검증 결과·PR/커밋 근거·남은 후속을 함께 요약한다.

외부 시스템의 상태명은 하드코딩하지 않는다. 연결된 도구가 제공하는 현재 workflow/status를 확인해 `doing`, 의미 있는 `hold`, `done`과
의미적으로 대응되는 상태를 선택한다.

Worker가 같은 외부 이슈를 직접 갱신하게 하지 않는다. 여러 Worker가 하나의 원천 이슈에 동시에 쓰는 경쟁 상태와 중복 코멘트를 피하기 위해
외부 운영 반영은 Controller가 직렬화한다. Worker는 구현·검증 근거만 Controller에 보고한다.

quiet/stale 같은 관제 신호나 내부 구현 단계마다 외부 이슈를 갱신하지 않는다. 외부 반영 실패는 canonical backlog의 상태 전환을 되돌리는 이유가 아니다.
연결된 MCP/도구가 현재 Controller 런타임에서 없거나 read-only라면 `확인·후속`에 어떤 외부 반영이 남았는지 기록하고 작업 자체는 계속 진행한다.

외부 반영 시 원천 항목이 Task Mecca의 현재 계약과 충돌하도록 변경된 사실을 발견하면 Controller가 임의로 계약을 바꾸지 않는다.
그 차이가 현재 구현·수용 기준 판단에 영향을 주는 경우 `hold`로 전환하고 `확인·후속`에 **원천 변경 확인 필요**, 구체적 차이, 필요한 사용자 판단,
재개 조건을 기록한다. subagent가 Root를 다시 깨울 수 없는 런타임에서는 그 durable 상태가 다음 Root 대화의 입력이 된다.

## 관제와 Web UI

Web UI의 stale/quiet/worker-missing은 관제 보조 신호이지 자동 상태 전환 명령이 아니다. 특히 stale은 worker 사망을
단정하지 않는다. 실제 runtime 상태와 작업 특성을 확인한 뒤 재배정·중단을 판단한다.

## Handoff send/resume 경쟁 조건

Controller는 handoff를 claim한 뒤 transport 결과와 handoff journal을 같은 ID로 대조한다. running Controller에는 message를 보내고, completed Controller는 capability가 지원하며 identity가 fresh일 때만 새 turn 직전 preflight 뒤 resume한다. send와 resume이 경쟁하면 먼저 성공한 claim/dispatch evidence를 canonical으로 삼고 중복 Worker 배정·중복 완료 처리를 하지 않는다. stale/unknown identity 또는 unsupported resume은 `hold`와 재개 조건으로 기록하며 새 identity를 추측하지 않는다.

## Continuity gap 복구

Worker turn이 끝났는데 canonical backlog가 여전히 `doing`이면 다음 pass로 조용히 넘기지 않는다.
fresh `coordinate --json`과 실제 live agent state를 다시 확인하고 `continuity_gaps`를 반드시 해소한다.

- runtime이 completed인데 backlog가 doing이면 원래 수용 기준을 검증해 done 처리하거나, 남은 작업이 있으면 **새 runtime turn이 실제 생성됐음을 확인한 뒤** 재호출/재배정한다.
- assigned worker가 runtime registry에서 사라졌으면 기존 메시지의 “재개 예정”을 실행 중으로 간주하지 않는다. live state를 확인하고 fresh turn 또는 명시적 재배정을 만든다.
- 사용자 판단이 필요하면 worker claim을 해제하고 `hold(user)`로 전환해 대기 사유·재개조건·근거를 기록한 뒤 Root에 `USER_DECISION_REQUIRED`를 올린다.
- Worker가 자기 자신에게 follow-up을 보냈다는 사실은 continuity 증거가 아니다. 새 turn 생성이 확인되지 않은 자기 위임은 재개로 인정하지 않는다.
- Controller는 위 gap 중 하나가 남아 있으면 queue가 안정 상태라고 보고하지 않는다.
- `coordinate --json`의 `recovery_queue`를 일반 ready보다 먼저 검토한다. completed/missing/user-wait worker가 붙은 `doing`은 구현 worker slot을 점유하는 것으로 계산하지 않는다.
- recovery에서 다시 worker를 dispatch하려면 **fresh Full Access preflight → live agent state 확인 → exact task inspect → fresh turn 생성 확인** 순서를 다시 거친다.
- `recovery.uncommitted_changes`가 있으면 해당 변경을 먼저 보존·검토한다. 기존 미커밋 구현을 잃거나 다른 worker 변경과 섞지 않은 상태에서 finalize 또는 재배정한다.

## Worker identity

신규 worker는 공통 규약의 확정 Pokémon ASCII pool과 `agent <new-path> --new --json`을 따른다.
기존 worker 재사용에는 실제 기존 path와 `agent <existing-path> --json`을 사용한다.

## 공식 설치와 migration

Controller는 검증한 공식 bundle의 `init`/`migrate`로 설치를 반영한다. 소스 직접 복사나 manifest baseline 수동 수정으로 경고를 숨기지 않는다. 배포본 버전·checksum·소스 commit과 승인된 파일의 source/destination hash를 기록하고, 당시 명령이 없으면 추정 명령을 실행 사실로 말하지 않는다. 오래된 CLI로 최신 설치를 되돌리지 않는다. 적용 전 전체 managed 파일과 manifest를 별도 백업하며 프로젝트 소유 data·backlog·runtime·설정과 출처 불명 변경을 보존한다.

`migrate --project <project> --json`으로 `modified_files`·전체 managed 교체 범위·`plan_digest`를 승인 범위 및 출처와 대조한다. 출처 불명 변경은 판단과 재개 조건을 기록하고 적용을 멈춘다. 같은 검증 binary로 `migrate --project <project> --choice backup --expect-plan <plan_digest> --json`을 수행한다. 내장 backup은 충돌 파일의 백업이며 전체 managed 파일·manifest 사전 백업을 대신하지 않는다. 계획이 달라지면 새 계획을 검토한다. 같은 bundle로 다시 migration 검사하여 baseline과 hash를 확인하고 데이터 보존 결과를 기록한다. `instruction_refresh_required`이면 지침을 다시 읽는다. 배포 소스 문서 변경을 설치에도 반영하려면 그 문서가 포함된 새 공식 배포본을 검증하고 이 절차를 다시 사용한다. 먼저 직접 복사하여 동기화하지 않는다.
