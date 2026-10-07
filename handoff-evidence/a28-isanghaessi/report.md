# A28 Worker handoff

- Stable report ID: A28-isanghaessi-20261007-01
- Worker: /root/controller/isanghaessi (native /root/controller_recovery/isanghaessi)
- Assignment: assignment-2d29d13a2e7f316aa5d3644b8a5aa2ae
- Attempt: run-cb7b6f13d2b86c1f
- Runtime: 01a11579-0096-7792-a078-473374637de0
- Turn: 01a115b1-ceb1-7403-951a-aa232f2465c3
- Parent: /root/controller_recovery, run-97594f71ee0f6961
- Contract SHA256: f40b7c94129fcde1b611155cc0733caf47734095ba77bf3d5326dc06ce949bda
- Worktree retained: /tmp/task-mecca-a28-isanghaessi
- Branch: codex/a28-resolved-assignment-warnings
- Base dev: bbc4d1919eb4da44edfbfba27bf3afe406d3a964
- Immutable implementation head: 4f9c8d7281b428c7373c3d8a208a2c18d80a8477
- PR: https://github.com/silverkhan/TaskMecca/pull/143 (base dev; Worker did not merge)

## 구현 결과

현재 unique canonical task와 authoritative Controller lifecycle, durable assignment 및 관련 runtime 경계를 매 scan에서 다시 대조한다. legacy controller_report의 task-wide 완료, parallel child assignment, explicit released user/external/dependency hold를 지원한다. obsolete assignment stage는 현재 pending 목록에서 제외하고 기존 경고 원문 및 별도 policy_resolution은 이력에 보존한다. hold는 완료로 표시하지 않으며 runtime terminal을 만들지 않는다.

실제 오류/중단/shutdown, unconfirmed terminal-error와 matching handoff_failed는 계속 actionable이다. 새 assignment/binding/start/activity, post-terminal activity, reopening, malformed/ambiguous/unavailable 근거, zero/future timestamp, 동일시각 상충 lifecycle, crossed assignment/attempt identity는 fail closed다. 같은 exact assignment/attempt의 실제 Hook completed terminal로 증명된 보고 마무리는 임의 grace 없이 구분한다. 성공적인 Controller claim/cleanup/applied bookkeeping은 새 실행으로 보지 않는다. 과거 exact-source suppression도 현재 doing ownership과 latest unique observed attempt를 다시 확인한다.

## 검증

- 최종 implementation head에서 단독 go test ./... PASS. 그 전 concurrent fullGo 한 번은 기존 Web shutdown context2초 timeout이 발생했으며 unrelated timeout 코드는 수정하지 않았다. 원형 실패 로그와 재실행 PASS를 local-validation.txt에 보존했다.
- 7 packages race PASS: webui/backlog/runtimeobs/handoff/maintenance/notify/projectguard.
- go vet ./... PASS; committed-range diffcheck PASS.
- Python 3.12 unittest 30 tests PASS. Node app.js/sw.js/terminal.js syntax PASS.
- Go/Python Web asset parity PASS, 양쪽 JS/CSS/HTML trees 미변경. Worker UI QA 추가 cycle 없음. 기존 impeccable visual/meaning 경계를 유지한 backend-only 변경이다.
- isolated synthetic scans/restarts에서 stable incident IDs/NextSequence 및 stale stage 재생성 방지 확인. canonical/runtime bytes unchanged, 실제 error와 matching handoff failed 보존, related/unrelated attempt scope 분리, hold 이후 still-running activity와 observed terminal reporting closure 구분 확인.

## 실제 read-only 사례

실제 project에 scanOperationProject, ReconcileCompletedOperation, monitor registration 또는 write API를 호출하지 않았다. 명시적 opt-in diagnostic은 Catalog/lifecycle/assignment/runtime/handoff read-only projection과 sanitized metadata만 사용했다. 기존 원본 8 canonical SHA256 전후 모두 동일(canonical-hashes.txt).

| Task | 분류 | Assignment / applicable attempt |
| --- | --- | --- |
| A7 | canonical_task_completed | 1 / 0 |
| A8 | canonical_task_completed | 3 / 0 |
| A10 | canonical_task_completed | 1 / 0 |
| A11 | canonical_task_completed | 1 / 0 |
| A19 | canonical_task_completed | 4 / 4 |
| B453 | canonical_task_completed | 3 / 1 |
| B456 | canonical_task_completed | 1 / 0 |
| A14 | canonical_released_user_hold | 1 / 1 |
| A26 | canonical_task_completed | 1 / 1 |
| A27 | canonical_task_completed | 1 / 1 |

A10 manual-bound legacy attempts는 assignment-only stage와 독립이다. 해당 attempt의 warning/error를 task policy로 숨기지 않는다. A11 handoff-98c11b793f35c7b93cd0의 source run-1366f4977107adb4, contract-verified failed 2026-10-06T17:53:13.43088Z 및 이후 applied 18:02:23.88756Z 원본을 그대로 보존한다. 현재 unrelated assignment_pending 재생성과 구분했으며 matching live handoff_failed 유지 회귀가 통과했다. A14의 사용자 결정 hold와 released claim은 그대로다. 실제 exact attempt의 observed completed terminal 08:16:13.760055Z는 종료 사실 근거이지 새 실행 추정이나 hold 완료 처리가 아니다.

## Immutable implementation CI

4f9c8d7281b428c7373c3d8a208a2c18d80a8477 exact head: 10/10 SUCCESS.

- [Platform build · macos-latest](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638043): SUCCESS
- [File-preserving project management · macos-latest](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638432): SUCCESS
- [Python compatibility · 3.12](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638714): SUCCESS
- [build](https://github.com/silverkhan/TaskMecca/actions/runs/37603602240/job/112733638229): SUCCESS
- [File-preserving project management · windows-latest](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638338): SUCCESS
- [Python compatibility · 3.11](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733637792): SUCCESS
- [Python compatibility · 3.13](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638319): SUCCESS
- [Go runtime](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638191): SUCCESS
- [Platform build · windows-latest](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638130): SUCCESS
- [Go ↔ Python compatibility smoke](https://github.com/silverkhan/TaskMecca/actions/runs/37603602052/job/112733638064): SUCCESS

Report-bearing final SHA and its independently completed exact-head CI are supplied in the final Worker DONE message, avoiding a self-referential commit SHA in this report.

## Controller handoff / 남은 역할

Controller가 PR merge/dev ancestry/sync, 실제 operational Web 배포·재시작 후 API/상단 경고 before/after 확인, canonical/Linear done/archive 및 안전 cleanup을 수행한다. Worker는 운영 설치/프로세스/Telegram gate·전송/알림 원장/원본 또는 사본 operational backlog/runtime/lifecycle/handoff/EMPFUND/user folders/OS Trash를 변경하지 않았다. A29 notify/delivery ledger, A14 source/hold, A27 done을 변경하지 않았다. Worktree와 committed evidence를 유지하며 Worker cleanup은 실행하지 않았다.
