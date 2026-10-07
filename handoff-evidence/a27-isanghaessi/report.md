# A27 Worker DONE · A27-isanghaessi-20261007-01

## 최종 A26 결합 검증

Controller 요청으로 최신 origin/dev 71b9801c4390ebba08d58ec2de18680e8f34bb1b (A26 PR140)를 rebase/force-push 없이 일반 merge했다. 충돌은 없었으며 결합 구현 head는 87b0a27948f1e65215f9e93d2c2a23c23cc6a585이다. A26 Hub/Archive/migration 동작은 변경하지 않고 inherited registered fixture 경계를 유지했다.

결합 fullGo PASS(webui28.838s), vet PASS(0 bytes), race8(webui/handoff/runtimeobs/backlog/notify/projectguard/maintenance/install) PASS, Python3.12 30/30 PASS(1.336s), both app.js Node syntax 및 Go/Python app/style/index parity PASS, origin/dev..HEAD committed range diffcheck PASS. UI는 Controller 요청에 따라 3차 QA/추가 polishing 없이 기존2 bounded rounds JPEG 증거를 유지한다.

원래 구현 head CI10/10은 아래에 보존했다. 결합 후 report-bearing 최종 PR head 및 새10개 exact-head CI가 모두 완료된 상태에서만 실제 DONE envelope를 전달하며, commit 자체의 self-referential SHA는 보고서에 위조하지 않고 Controller 전달 메시지·PR checks로 별도 명시한다. 결합 검증 출력은 combined-verification.md에 보존한다.

- 계약: A-27 / Linear AID-115; SHA256 4f1210ebffc8d68ab73e5a037a9bb5fad753e793db2fe4b9c6c946d5ee5b2fbd.
- semantic /root/controller/isanghaessi; native /root/controller_recovery/isanghaessi.
- assignment assignment-e347769600483fe21904c138f9a1cb0f; actual attempt run-3ca758cf24dd6563; runtime 01a11579-0096-7792-a078-473374637de0.
- worktree /tmp/task-mecca-a27-isanghaessi; branch codex/a27-handoff-review-warning-grace; base f261fa59.
- 검증된 구현 head: e8f4bf04304a8cd815b56c693d71f2af754a4f5f.
- PR: https://github.com/silverkhan/TaskMecca/pull/141 (base dev, merge 금지; Controller 직렬 통합).
- 이 보고서 추가 커밋은 문서/증거 안정화이며 구현 head와 runtime 자산을 변경하지 않는다. 최종 PR report-bearing head 및 해당 exact-head CI는 DONE 전달 시 별도 실제 Git/CI 조회로 명시한다.

## 결과와 경계

최신 unique assignment와 exact bound attempt / worker_done source / 실제 Controller target·claim을 대조하여 stage/evidence/since/grace_until을 operations journal에 보존한다. 배정2분, observed worker 종료 후 보고5분, 보고 인계5분, Controller claim 검토15분, acceptance 이후 최종처리10분 유예는 immutable 원천시각 기준이고 polling으로 새로 시작하지 않는다. failed step은 가장 이른 유효 failure 시각(잘못된 시각은 prepared fallback)으로 즉시 경고한다.

실제 errored/interrupted/shutdown 및 관측 미확정 terminal 오류는 정상 유예로 숨기지 않는다. runtime_unknown/no_signal의 identity/관측 gap만 유예하고, 최신 bound observed running에 의해 해소된 old source는 정확한 project/task/attempt/agent/kind/evidence/last-observed signature로 재출현을 막는다. 최신 실행이 나중에 실제 오류로 바뀌어도 그 새 오류만 별도로 actionable이다.

controller_verified completion은 exact Controller actor·assignment/attempt·canonical done·실질 결과/검증·시각·later assignment/observation gate를 모두 확인한다. runtime_unknown/no_signal/handoff_stalled와 ID 없는 오래된 배정 경고를 근거로 이력 이동하되 실제 오류/handoff_failed는 단순 done으로 해소하지 않는다. runtime terminal/status를 조작하거나 Worker report/acceptance만으로 backlog done을 만들지 않는다. observed completed 근거가 있으면 역사 UI 문구도 미확정 실행으로 오기하지 않는다.

상단 active incidents만 요약하고, 단계·근거·해소 이력은 펼쳐 확인한다. 회복된 monitor_gap을 1시간 동안 현재 actionable처럼 재노출하던 UI는 제거했다. A26 Hub/Archive/migration와 A14 알림 문구/hold·notify.go/ledger 생성/메시지 formatter는 수정하지 않았다. 공유 파일 변경은 app.js 상단 운영 render, style.css operation-stage 추가, operations backend에 한정했고 server.go/index.html은 그대로다.

## 검증

- 최종 구현 full Go: go test ./... PASS (webui 26.163s; 이후 source-bound 보강 exact head 재확인은 /tmp/a27-final-go.log).
- race: Go webui/handoff/runtimeobs/backlog/notify/projectguard 6 packages PASS (/tmp/a27-final-race.log).
- go vet ./... PASS, output 0 bytes (/tmp/a27-final-vet.log).
- Python3.12 unittest 25/25 PASS (0.881s); local Python3.14도25/25 PASS.
- Go/Python app/style/index byte parity PASS; both app.js Node syntax PASS; committed range f261fa59..implementation head diffcheck PASS.
- Regression: grace boundary / polling 고정 clock / claim / acceptance / failed / deterministic failed map / malformed failure clock / 재배정 / ID 없는 assigned warning / actual error·unconfirmed terminal exclusion / old source scan2·3 재출현 방지 / 새 actual error 유지 / episode ID 안정성 / doing 문서 불변 / controller_verified 완료 후 stall 해소와 runtime 불변.
- 기존 legacy 회귀에서 단순 새 attempt만으로 confirmed errored를 숨기던 기대를 명시적 실제 오류 보존 계약으로 수정했다. assertion을 삭제하지 않고 역방향 보존 기대를 검증한다.
- Controller independent fullGo/race4/vet/Python25 및 stage regressions PASS가 별도로 전달됐다.
- 구현 head exact CI 10/10 SUCCESS: CI https://github.com/silverkhan/TaskMecca/actions/runs/37596340590 ; artifacts build https://github.com/silverkhan/TaskMecca/actions/runs/37596340588. native macOS/Windows recycle+restore, Python3.11/12/13, Go/platform/smoke 포함. final report-bearing head도 exact CI 완료 후 DONE 전달한다.

## 실제 사례 보존

A19/A24/A25 및 EMPFUND B457/B458의 필터링한 metadata만 read-only 확인했고 완료 문서 before/after SHA256 모두 동일하다. case-preservation.md에 digests와 synthetic 대응을 기록했다. 실제 ongoing operations/runtime/lifecycle/notification 원장 전체 불변을 주장하지 않으며, Worker는 해당 원장이나 EMPFUND 구현을 변경하지 않았다. 실제 정상 완료를 위조하지 않았다.

## 실제 UI 증거와 skill 사용

impeccable SKILL 전체/harden/craft-floor 직접 읽기, incumbent app/style 확인. 정확 target context.mjs 호출 후 도움말 인수 확인을 1회 더 했으나 이 스크립트가 loader를 재실행했으므로 session-once 지침의 부주의한 추가 호출을 명시한다. 추가 context 호출·drift repair는 하지 않았다.

실제 Chrome CUA paused synthetic fixture http://127.0.0.1:18927 /private/tmp/a27-isanghaessi-ui-pDyu78에서 총2 bounded batched rounds만 수행했다. native AX로 단계 펼침/테마 제어를 실제 사용했다. desktop light/dark와390×844 mobile light/dark에서 단계시각·긴 근거 wrapping·가로 overflow 없음·desktop sidebar 가림 없음·Telegram maintenance 표기를 확인했다. final capture 뒤 historical observed-completion 설명 분기만 의미적으로 보완했고 Node syntax/parity와 CI로 검증했으며 3차 visual round는 하지 않았다.

CUA tab.screenshot({fullPage:false}) actual JPEG bytes를 node:fs/promises.writeFile로 다음 경로에 보존했다:

- handoff-evidence/a27-isanghaessi/ui/desktop-light.jpg
- handoff-evidence/a27-isanghaessi/ui/desktop-dark.jpg
- handoff-evidence/a27-isanghaessi/ui/mobile-light.jpg
- handoff-evidence/a27-isanghaessi/ui/mobile-dark.jpg

Controller가4뷰를 view_image로 독립 확인했다. viewport override reset, agent tab close, opt-in fixture process만 Ctrl-C로 종료했다(serve fixture의 intentional interrupt는 unit regression failure가 아니다). 실제 production process·launchd·설치·Trash는 조작하지 않았다.

mechanical detector 1회: 새 운영영역 finding 없음. incumbent 범위 밖 CSS4건(side-tab line909, font line5, sidebar layout-transition lines250/256)은 기존 identity 보존을 위해 수정하지 않았다. 전체 결과 /tmp/a27-ui-detector.json. UI 스타일4뷰 재설계/추가 polish는 하지 않았다.

## 잔여·인계

Tracked implementation/evidence clean 이후 이 stable report commit만 추가한다. ignored src/task_mecca/__pycache__/ 및 tests/__pycache__/는 테스트 생성이며 보존한다. /private/tmp/a27-isanghaessi-ui-pDyu78 및 /tmp/a27-*.log는 synthetic/generated evidence로 보존한다. worktree/branch는 Controller cleanup 전까지 유지한다.

운영 설치/서버 재시작/실제 Telegram 송신 또는 process delivery gate 해제/외부 Linear 상태 반영/원본 canonical/backlog/runtime/lifecycle/handoff 수동 생성·편집은 하지 않았다. Telegram transport 차단은 변경하지 않았다. A14 사용자 판단 hold와 구현 보존 경계는 그대로다.

한계: 명시적인 source assignment/attempt 및 durable handoff 없이 보고·검토 단계를 추정하지 않는다. malformed/ambiguous ledger는 grace를 얻지 못한다. 역사적 actual failures는 명시적 검토가 필요한 active 근거로 남는다. Controller가 PR exact final head/CI, 최신 dev ancestry·결합 회귀·운영 적용, 외부 writeback, verified completion·archive, 보존·cleanup을 맡는다. Root ACK는 gate가 아니다.
