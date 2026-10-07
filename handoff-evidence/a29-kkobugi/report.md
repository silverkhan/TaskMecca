# A29 꼬부기 인계

Stable report ID: `A29-kkobugi-20261007-01`.

## 정확 실행 연결과 경계

- assignment: `assignment-1e80088d8c40eba84a21ce2ad8a2b8f1`
- Worker native `/root/controller_recovery/kkobugi`, semantic `/root/controller/kkobugi`
- exact attempt `run-cf4306837c9ab9e4`, runtime `01a11564-d065-71a2-a433-590ef83cd442`, turn `01a115b3-052e-7130-8807-1ac335f9210b`
- parent Controller native `/root/controller_recovery`, semantic `/root/controller`, attempt `run-97594f71ee0f6961`
- worktree `/tmp/task-mecca-a29-kkobugi`, branch `codex/a29-telegram-no-replay-resume`, base `bbc4d1919eb4da44edfbfba27bf3afe406d3a964`
- original dispatch contract SHA `d355469aca1138d60d90d57426c364896bacd8ceedf4bc17ca47ef4d2fce01ae`
- 원본 계약 전체 직접 읽기 완료. 착수/runtime note 변경 이후 live SHA `02205d272f308a0a5141df340563bbbc7751e53e02c28ccf2b433eeec0599da2`도 별도 확인/Controller 보고. Worker는 원장에 쓰지 않았다.
- 실제 STARTED를 exact native Controller에 직접 회신했다. 추가 agent/Orca/model override 사용 없음.

## 조사 결론과 구현

기존 process maintenance gate는 no-network/no-ledger-write early-return이므로 과거 backlog가 재개 시 재생될 수 있다. 기존 설정 토글/ActivatedAt 변경은 설정 보존·미확정 attempt audit·known ID chronology 방어를 독립적으로 충족하지 않는다. 따라서 별도 durable resume boundary가 필요하다.

새 `notifications suppress-before-resume --project ABS --confirm-project ABS --cutoff RFC3339 --json`은 process transport가 disabled일 때만 동작한다. configuration/token/chat/Enabled/recipient/ActivatedAt/Delivered를 읽거나 변경하지 않고 event journal도 읽기만 한다. gate를 해제하거나 서비스를 재시작하지 않는다. `notifications resume-status`는 credential-free read-only 진단이다.

notify ledger의 별도 `resume_boundary`(cutoff/evidence ID/established timestamp/reason/backup path+SHA)와 `suppressed_before_resume` metadata(previous state/reason/evidence/cutoff/classified timestamp)로 감사한다. 기존 pending/failed/sending/uncertain/unknown은 초기·전진 cutover snapshot에서 억제하지만 sent로 위장하지 않으며 attempts/error/retry time/duplicate uncertainty/attempt history를 보존한다. 동일 cutoff 재실행은 정상 신규 failed retry를 소비하지 않는다. cutoff rewind와 미래 cutoff를 거부하고 전진 시 resume_history를 보존한다.

매 Deliver에서 restart/late source discovery에도 `event_at <= cutoff`, invalid/missing/future time, 기존 timestamp evidence가 과거인 event를 sending/retry보다 먼저 억제한다. 알려진 event ID의 시각을 바꿔도 resurrection되지 않는다. 기존 sent/consumed/suppressed와 opaque root/record/attempt 확장 facts를 디스크에 보존하며 opaque facts는 공개 진단으로 내보내지 않는다.

process mutex+동일 cross-process delivery file lock으로 cutover/delivery를 직렬화하고 byte-exact0600 backup+SHA 확인 후 fsync/atomic persist한다. backup/commit 실패·corrupt ledger/boundary/journal·symlink ancestor는 fail-closed, gate/config/기존 ledger는 유지한다. backup은 notification ledger만 포함하며 Controller의 repository 밖 secret configuration backup과 구분한다.

## 변경 파일과 회귀

- `internal/notify/delivery_resume.go`: suppression/cutover API/status 및 safety.
- `internal/notify/delivery_evidence.go`: opaque/settled history preservation, projection 분리.
- `internal/notify/delivery_ledger.go`: narrow metadata/validation/fsync/pre-retry reconciliation only.
- `cmd/task-mecca/notification_cli.go`, `_test.go`: 명시 CLI·gate/path/confirmation/snapshot 조건.
- `cmd/task-mecca/main.go`: early dispatcher **1줄**. compact source whole-file formatting 없음.
- `internal/notify/delivery_resume_test.go`: old/equal/late/missing/invalid/future IDs, timestamp substitution, sending/unknown truthfulness, sent/history/config byte preservation, exact backup digest, malformed boundary network0, samecutoff idempotency+newfailedretry, monotonic history, backup/commit failure, symlink, concurrency/child restart.
- docs: `docs/A-29-telegram-no-replay-resume.md` Controller-only exact procedure와 실패 대응.

Cross-process synthetic 회귀는 4개 동시 cutover child, 8개 동시 delivery child, 2개 fresh restart child에서 past0/new1 및 Attempts1을 검증했다. fake HTTP sender와 fake cfg/temp home만 사용했다. 실제 Telegram 발송·설정·서비스·프로젝트 event/ledger/registry에는 접근/쓰기하지 않았다.

## 검증

- targeted Resume/Notification: PASS.
- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- race: notify/cmd/backlog/projectguard/maintenance/handoff/install/runtimeobs PASS. Web 기존 `TestOperationWebProcessContinuesAfterBrowserClose`에서 service shutdown2초 context deadline으로 first six-package, isolated, Web전체, full `go test -race ./...`, latest isolated 재현 FAIL. operations/server timeout 소스·기대값은 수정하지 않았다.
- untouched base `bbc4d191`를 `git archive`로 `/tmp/task-mecca-a29-baseline-jFLEqF`에 복사해 같은 synthetic test 확인: isolated count1 PASS(4.194s), count3은 같은 operations_monitor_test333/434 shutdown deadline FAIL(7.703s). A29 이전에도 재현되는 timing-sensitive shutdown 한계를 검증했다. macOS arm64/현재 Go race runtime, 동일 로컬 환경; 실제 운영 서비스 아닌 httptest/subprocess fixture. 원형 로그: `baseline-race.log`, `baseline-race-repeat.log`, `a29-targeted-race.log`.
- latest `go test -race ./internal/notify ./cmd/task-mecca -count=1`: PASS(6.972s/1.900s). 위 Web failure를 이 targeted PASS로 숨기지 않는다. Controller/CI가 최종 integration 판단한다.
- bundled Python3.12.14 `PYTHONPATH=src python3 -m unittest discover -s tests`: 30 PASS.
- `git diff --check`: PASS. JS/UI 파일 변경 없음; 추가 시각 QA/JS 변경 테스트 불필요. 기존 CI JS syntax checks는 수행한다.
- A28 operation projection/A14 held renderer/notify.go/shared server/UI 미변경.

## 운영 및 보존 인계

Controller 관측 actual enabled2projects/missing2projects와 gate disabled는 Worker 검증으로 대체하지 않는다. 현재 ledger pending0이어도 late discovery suppression을 위해 모든 approved enabled target에 exact cutoff가 필요하다. missing2projects에 파일 생성/registry 변경하지 않는다.

안전 절차: A28 실제 보정 검증 → 모든 기존 writer gate 유지/quiesce → 새 exact binary/hash/version 검증 → 단일 승인UTCcutoff를 각 approved target에 명시 commit/status 검증 → 실패 시 전체 disabled 유지 → Controller만 supported Web restart로 process gate 해제 및 binary/API/UI 확인 → cutoff 이후 새 정상 검증 알림 최소1개 실제 수신/dedup 근거 확인. 이전 binary는 resume boundary를 모르므로 gate 해제 후 이전 writer가 실행되어서는 안 된다.

원본/사본 backlog·runtime/lifecycle/handoff·실제 event/delivery/config ledger·monitor registration·운영 설치/서비스·실제 Telegram/Trash/userfolder·Linear/dev/canonical writes는 Worker 수행 없음. A27 done/A14 userhold 불변. 운영 절차/실제 수신·dev 통합·Linear·canonical done/archive·cleanup은 Controller 소유다.

ignored local `src/task_mecca/__pycache__/`, `tests/__pycache__/`와 baseline evidence directory `/tmp/task-mecca-a29-baseline-jFLEqF`를 보존했다. 구현/테스트/docs/report/raw verification logs 외 새 파일 없음. 합성 fixture는 test TempDir lifecycle로 종료됐으며 장기 실행 서비스/브라우저 없음. 실제 사용자 폴더 삭제 없음.

## Delivery

실제 PR: https://github.com/silverkhan/TaskMecca/pull/142 (base dev, OPEN/MERGEABLE).

독립 A29 implementation head `45758f6c0bdaa3310af323772702a371f32b4b13`: CI37603355554 + standalone37603355549 모두 SUCCESS(10checks).

A28 PR143 dev 통합 이후 origin/dev `1f4cbeee8e402fc0463d2172194e0df2c7aa4b2e`를 ordinary merge했다. A28 final head `3ac91a5` ancestry 포함, merge head `3902816b72112ad2762b969b4a363bb59cd61eb4`, conflict 없음/force reset/rebase 없음. A29-vs-dev에는 12개 notify/CLI/docs/evidence 파일만 있고 A28 source는 merge로 상속했을 뿐 추가 편집하지 않았다.

combined head에서 fullGo PASS(Web29.237s), vet PASS, notify/cmd race PASS(6.725s/1.570s), Resume/Notification 경계 회귀 PASS(0.788s/0.824s), Python30 PASS(1.587s), diffcheck PASS. 환경 go1.27.1 darwin/arm64, Python3.12.14. 원형 stdout/command: `combined-validation.log`. 기존 full Web race shutdown 한계는 위 기록 그대로이며 combined ordinary tests 통과로 숨기지 않는다.

combined exact head3902816의 CI37604991732 9jobs SUCCESS와 standalone37604991709 SUCCESS, all10checks를 직접 확인했다. https://github.com/silverkhan/TaskMecca/actions/runs/37604991732 및 https://github.com/silverkhan/TaskMecca/actions/runs/37604991709 . 실제 exact-head rollup은 `combined-ci.json`에 고정했다.

이후 갱신은 evidence/report-only commit으로 code 변경이 없다. 자기 report commit SHA 삽입 순환을 피하기 위해 exact final report head/checks는 native DONE에서 함께 전달한다. original contract SHA d355469a 및 exact run/turn은 위 경계와 동일하다. Controller-only 운영 AC(실제 cutoff/config/gate/restart/minimal receive/Linear/dev/canonical/cleanup)는 Worker 완료로 주장하지 않는다.
