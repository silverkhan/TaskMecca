# A-65 수용 재질 전체 확대 Worker 인계 보고

Report ID: `a65-accepted-expansion-20261109-kkobugi-t3-310a9f74`

상태: **BLOCKED — 원래 성능 gate 2개 미충족**. 구현·잔여 메뉴 수정·최종 scoped 검증·전체 성능 계측을 마쳤으나 전체 기술 완료로 선언하지 않는다. 이 보고서는 AC 체크, 전체 A-65 done, 운영 승인 또는 사용자 최종 수용 선언이 아니다.

## 실제 신원과 작업 연속성

- assignment: `assignment-2054988e647522abe875bb9b7e81a61f`, semantic `/root/controller_recovery/kkobugi`.
- cwd `/Users/hanati/.t3/worktrees/TaskMecca/t3-310a9f74`, branch `t3/uiux-redesign-implementation`.
- actual Codex thread `01a1be98-799f-7580-9ec8-2c8b38630b8f`.
- actual T3 thread `thread:delegated-task:command%3Amcp%3Ab47f2958-5d29-4728-b41e-ca53598882b3%3Adelegate-task%3A7c50090a-7991-4124-85df-edb6723c501e`, active run ordinal1. `native-identity.json`, `runtime-environment.json`, `network-continuity.json` 참조.
- 네트워크 upstream502 뒤 같은 thread/run·checkout·HEAD63cf에서 재개했다. 대체 Worker/run 생성0. 오류를 테스트·성능 성공으로 계산하지 않았다. configure에서 codex/gpt-6.1-sol/full-access/default, 자기 thread read에서 동일 active ordinal1/running을 관측했다.
- native hook·canonical binding·Activity event는 관측하지 못했다. JSONL/runtime/state를 만들거나 조작하지 않았다. 과거 completed 신원을 재사용하지 않았다.
- Controller에 연속성 및 최종 성능 freeze를 직접 전송했다. transport `started`/`queued`는 처리 완료 증거가 아니다. 최종 전송 기록은 별도 handoff 파일에 둔다.

## 권한과 보존 경계

canonical framework SESSION_GUIDE/collab/worker/EXECUTION_PROTOCOL 및 지정 7개 계약 원문을 읽었다. effective `84674de5f2872d2c5173bb98a21ebdc27efded35aab854701a9c7caa17aed280`, execution_authorized=true/findings[]는 Controller 배정 근거이며 Worker가 운영 승인 원장을 수정하지 않았다. 계약 읽기 전용 SHA는 `contracts-read-only.json`에 있다. 작은 시안 수용 gate는 이미 충족됐고, 새 중간 질문·승인 gate를 추가하지 않았다.

subagent/checkout/worktree 생성0, 타 WT 수정0, canonical 원장/runtime/lifecycle/계약 수정0, push/PR 작성0, 서비스 설치/restart0, 운영18765·57069 접근/변경0. sample f3 binary/PID60445도 Controller 전용으로 접근하지 않았다. 모든 실패·raw·과거 캡처를 보존했다. unprotected `go test ./...`는 실행하지 않았다. 관리 쓰기는 격리 fixture만 검증했다.

## 기반과 통합할 커밋

시작 `d315e2f184c156cafe97d4d2c3129fa2a71733f9` clean. fetch 후 승인 원격 b326으로 정상 merge했고, 구현 전 `HEAD^{tree}=b326^{tree}=3ab9038fd810bfdb81801c97a84340c36b2ab452` 및 clean을 확인했다(`START.json`). **기반 merge `6c1fcc08afe66a25fe518b7e4bc92ebcac03d2bd`는 cherry-pick 대상이 아니다.** 전체 dev merge/reset/force/delete를 하지 않았다.

Controller가 순서대로 cherry-pick할 소스 커밋:

1. `7eab22bc9534fd43a3ae0d86043640c77d946f61` — feat: 승인된 투과 유리 재질을 기존 UX와 실제 관리 화면으로 확대
2. `5b54b09ad8de3dfd27b822ba2fa9366c5458b420` — fix: 프로젝트 문맥과 복원 동작을 보존하고 유리 스크롤 좌표 비용 축소
3. `cf460036c16ac5a8da11bd244d051566b167083a` — fix: 운영 관측의 활성·복구 상태와 선택한 백로그 범위를 구분
4. `f5afe4f346af230793604d84d9ae58db3816e53b` — fix: 확대 시 공통 셸을 재배치하고 유리 윤곽과 상태 문구 정리
5. `c37ce93a5651a33731e6d562bc7bb08d00796271` — fix: 독립 터미널도 첫 화면부터 승인 재질의 화면 모드 적용
6. `b9b849f0de9cf6145e1bcd582b3ce7f7102b2cf3` — fix: 읽기 전용 표면을 연결하고 문서 면적·대비·native reader 상태 보존
7. `12fb5dd9c6c20d377a138c33765da44581919ea6` — fix: 동일 근거 갱신 중 pinned reader의 배치와 읽기 스크롤 유지
8. `6d78210cbfe71a5bdbfbc94b4f716f9a56789844` — fix: 실행 identity 표면에 native DOM을 보존하는 공통 유리 fallback 적용
9. `63cfecb95ccf33e59f071645f1b76709a1c47bd1` — fix: Auto 행 수·저장 설정·확대된 메뉴의 접근성 복원
10. `e21594e8b4cc1484b76b51258ffbbfeb68771468` — fix(webui): keep expanded mobile navigation below menu controls

최종 production source SHA는 `e21594e8b4cc1484b76b51258ffbbfeb68771468`다. evidence/docs 커밋은 이후 별도 인계한다. 바이너리 provenance는 최종 성능 run의 `binary-provenance.json`에 있다.

## 실제 기능 대응

전체 대응표는 [docs/a65-feature-map.md](../../docs/a65-feature-map.md)다. 실제 `origin/dev=f0a732e3dd8234a663dccf3c6937fc9dc674917b` 원본 `app.js`를 read-only 대조했다. 함수 목록·해시를 `legacy-functions.txt`, `legacy-reference.sha256`에 보존했다. legacy application 전체 런타임/old-new 전환을 추가하지 않았고 native ES modules를 유지했다.

- SYSTEM/BACKLOGS/OPERATIONS/HELP 그룹과 위치, Global Hub 첫 화면, sidebar 프로젝트 직접 진입·닫기/재열기·저장값을 연결했다.
- backlog 검색/복수 상태/태그 catalog·namespace/위치/filter/sort/page/Auto, 목록 키보드/선택/scroll 및 복귀·deepURL을 연결했다. Auto는 실제 행·viewport·sidebar resize에 따라 재계산하며 manual 저장값을 복원한다.
- 모든 기기 전용 상세와 slim lifecycle/connected bars/개별 tooltip·pinned reader/fit/zoom/wheel/pinch를 유지했다. pinned reader refresh 시 배치가 뒤집히면서 scroll이 clamp되는 실제 결함을 원래 assertion 그대로 교정했다.
- workload/현재·이전 Root/실행 history, 운영 감시·복구 사건, 프로젝트 등록·감시·보관·복원·삭제·지침 갱신, 저장소 다중프로젝트·전역 중복제거·보호/정리를 기존 API와 확인 흐름으로 연결했다.
- 알림 현재 canonical 사용자조치/history, Push 연결/시험, Telegram 공통·개별 수신자/종류/설정, canonical claim/ack/fail을 연결했다. API 응답 성공과 실제 기기 전달은 다른 사실로 표시한다.
- 실제 terminal document iframe과 원래 terminal runtime, 매뉴얼/prompt/업데이트·page/detail/읽음, diagnostics·복원요청 복사, 실제 Mermaid11.10.0 SVG/원문복귀·코드복사를 연결했다. URL placeholder로 끝내지 않았다.
- Preview SHA-pinned build/instance/rebuild/stop/진입과 실제 재시작 identity 확인, 기존 P0 actual PR identity/Dev 복귀 거부·mismatch·unknown false-success 방지를 유지했다. 운영 설치·실제 PR/채널전환은 실행하지 않았다.

## 수용 재질과 접근성

수용 f3의 곡률/rim/두께/광택/photo 투과/7px blur/공간 SVG edge map/근접광을 실제 공통 셸·input/select/combo/table/header/menu/popover/modal·화면에 확대했다. 원본 f3 material 5개 소스와 photo는 바꾸지 않았다(`protected-after.json` 및 최종 보호 해시). opaque fallback으로 회귀하지 않았다.

굴절은 **로컬 photo의 viewport 샘플**이며 임의 뒤 DOM 굴절이 아니다. fixed backdrop 좌표가 scroll400 전후 같음을 실제 캡처에서 확인했다. 다른 엔진은 blur/투과/rim fallback이다. Chromium 강제 fallback 비교를 Safari 실측으로 표시하지 않는다. 글자·아이콘에 shadow/stroke/filter/blur/displacement를 넣지 않았고 dense row blur/canvas/왜곡, CPU optics engine, continuous rAF를 추가하지 않았다. 기존 xterm canvas는 터미널 렌더러다. 긴 문서·소스는 bounded reading region으로 합성 면적을 줄였다.

Impeccable SKILL·PRODUCT/DESIGN·대표 source·overdrive·craft-floor를 적용했다. 프로젝트 cwd `context.mjs --target material-review` 1회와 detector1회 결과를 보존했다. 과거 aa5 DESIGN/SHIP 대신 사용자 수용 f3 방향으로 PRODUCT/DESIGN을 갱신했다. 새 comp/사용자 질문/추가 승인/독립 reviewer 생성0.

320px/CSS200에서 legacy `.rail nav` fixed bottom 규칙이 닫기 버튼을 덮는 hit-test 결함을 최종 수정했다. position/inset/z-index와 column 방향을 명시 해제했다. 원래 로고·accessible brand name을 유지하고 좁은 경우 wordmark만 compact 처리한다. 실제 hit/열기/Escape/aria/다시열기/마우스닫기 및 최종 focused 캡처를 보존했다. native OS/browser zoom200% 실측 주장은 아니다.

## 검증 명령과 실제 coverage

공통 browser env: `WORKFLOW_PLAYWRIGHT_MODULE=/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright`, `CHROME_EXECUTABLE=/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, `WORKFLOW_REQUIRE_BROWSER=1`, 매 실행 고유 `WORKFLOW_EVIDENCE_DIR`. workflow group은 `WORKFLOW_NO_CAPTURE=1`.

| 명령 / 범위 | 실제 결과 | 근거 / source 경계 |
|---|---|---|
| `node --test tests/a65_core_browser.test.cjs tests/a65_expansion_browser.test.cjs` | 2/2 PASS, skip0 | `exact-final-core.tap` at12fb; Auto 후 `auto-final.tap`2/2 |
| `node --test tests/web_workflow_browser.test.cjs tests/web_workflow_connected.test.cjs tests/web_workflow_gestures.test.cjs tests/web_workflow_navigation.test.cjs tests/web_workflow_popover.test.cjs` | 8/8 PASS, skip0 | `exact-final-workflow.tap`,12fb 원래 assertion 유지 |
| `node --test tests/a65_preferences.test.cjs tests/a65_task_data.test.cjs tests/a65_return_dev.test.cjs tests/web_select_ui.test.cjs tests/web_terminal_theme.test.cjs tests/web_terminal_service_restart.test.cjs` | 66/66 PASS | `exact-final-unit.tap`, 후속 `final-preference-unit.tap`66/66 |
| `node --test tests/a65_return_dev_browser.test.cjs` | 1/1 PASS; positive/denied/mismatch/unknown | `return-dev-delivery.tap`,6d identity CSS 포함; 캡처는 confirm 폴더 |
| terminal theme/service tests | 11/11 PASS | `terminal-final.tap`,prepaint 보완 별도 증거 |
| `go test ./internal/webui -run 'Test(Preview\|Environment\|LogStorage\|HubProject\|NotificationHistory\|Workload)' -count=1` | PASS | `named-go.log`; 안전 named 범위 |
| `node --test handoff-evidence/accepted-expansion/document-tools-contract.cjs` | 1/1 PASS; 실제 SVG/theme/source/copy | `document-tools-final.tap`, `document-tools-contract.json` |
| `node --test handoff-evidence/accepted-expansion/auto-layout-contract.cjs` | 1/1 PASS; 실제 resize/manual restore | `auto-layout.tap`, `auto-layout-contract.json` |
| `node --test handoff-evidence/accepted-expansion/zoom-brand-contract.cjs` | 최종1/1 PASS | `zoom-menu-final-confirm.tap`, e215 source와 동일 수정, open/closed PNG |
| `node scripts/check-native-ui.cjs`, syntax, `git diff --check`, exact source `go build -o /tmp/a65-accepted-expansion-frozen-310a9f74-menu-final ./cmd/task-mecca` | PASS | 도구 실행·최종 static/provenance 기록 |

기존 b326 helper await Promise 수정 및 독립9groups/core1 pass·실패2attempt 이력을 그대로 보존했다. fixed sleep/skip/assertion 완화/맹목 rerun을 도입하지 않았다. 실제 combo 구조에 맞춘 locator 변경은 semantics·predicate를 보존한다. 모든 `attempt`/reader trace/이전 실패는 삭제하지 않았다. 실제 Mermaid classic bundle SHA256 `0755e39315802b1f9644afbc95ef1eccd4015a47399deae280bf1c1fd33835c2`; clipboard는 test-local 대체로 사용자 clipboard를 건드리지 않았다.

### 캡처와 상태

`visual-batch1` → 필요한 수정 `visual-batch2-final` → 허용 최종 `visual-confirm-final`을 보존했다. 전면 새 polish batch를 추가하지 않았다. 마지막 잔여 320 메뉴의 focused confirm은 네트워크 연속성 지시 범위다.

`visual-confirm-final/report.json`: 실제 owned synthetic Go API(라우트 mock0), source12fb, home/backlog/detail/lifecycle/workload/notifications/terminal/projects/storage/operations/manual/release-notes/settings/preview 14화면 × 1440/1280/320/390/430 × Light/Dark =140. English CSS200 추가10=150케이스. pageerror0/external0/root overflow0/material 연결 누락0/filtered text0, 총190PNG. image/solid 실제 설정 조작, scroll400 고정좌표, forced fallback·System/reduced-motion/heap·DOM·layers·SystemInfo supplemental을 같은 batch에 보존했다.

**12fb 뒤 identity CSS·Auto/저장값·narrow brand/menu 변경이 있어 전체150케이스를 최종HEAD 캡처라고 부르지 않는다.** 후속 변경은 위 P0/Auto/unit/focused menu로 검증했고 일반 manual20 화면의 전체 캡처는12fb 증거다. `contrast-exact-before.json` 실패를 보존하고 공통 veil/ink 교정 후 `contrast-exact-final.json`의 실제 raster metadata/status 표본6.46–8.94를 얻었다. 이는 전체 페이지 WCAG 인증이 아니다.

empty/loading/error/partial/stale/unmeasured/wrong0·누락 구분, exact project/header/body, 403 rollback/보호, project scope/filter/page/선택/scroll 복귀, keyboard/touch/reader native gesture는 해당 core/expansion/data/workflow fixture 검사와 캡처로 범위를 확인했다. 모든 실제 운영 데이터·모든 API 조합을 전수 실행했다는 주장은 아니다. 합성 sample을 실데이터로 표시하지 않았다.

## 최종 성능 gate와 raw

원래 `docs/evidence/a65-performance/{RUNBOOK.md,protocol.json,paired.cjs,idle.cjs,idle-verdict.cjs,report.cjs}`를 직접 읽고 바꾸지 않았다. 실제 dev archive `/private/tmp/task-mecca-a65-baseline-f0a732e3`의 tracked frontend13개 byte 일치는 `performance-final-7eab22bc/baseline-proof.json`에 있다. 작은 sample로 baseline을 대체하지 않았다.

첫7eab full8pair/48은44PASS/4FAIL:1280 Light scroll+20.044%,Dark+16.209%,390 Light+12.901%,Dark+13.004%. 기존 run/raw를 보존했다. 이후 scroll마다 좌표쓰기 제거·fixed viewport attachment·합성 면적 교정 후 최종 후보를 별도 고유 run에서 다시 전체 측정했다. 10% gate를 면제·완화하지 않았다.

최종 source e215와 exact binary/protocol/harness/browser SHA를 고정하고 다른 owned fixture/browser/build를 종료했다. `owned-confirm-cleanup.json` 참조. 실행:

```sh
A65_PERF_HEAD=e21594e8b4cc1484b76b51258ffbbfeb68771468 \
A65_PERF_RUN=../../../handoff-evidence/accepted-expansion/performance-final-e21594e8 \
node docs/evidence/a65-performance/paired.cjs
# 같은 env로 이어서 idle.cjs, idle-verdict.cjs, report.cjs
```

최종 동일환경 8쌍/48그룹 결과: **FAIL**. PASS 46/48, 미통과 2/48.

| 폭 | 테마 | 동작 | 지표 | dev 중앙값 | 후보 중앙값 | 변화 | 판정 |
|---|---|---|---|---:|---:|---:|---|
| 1440 | light | scroll | TaskDurationMs | 20.0145 | 26.5310 | +32.559% | FAIL |
| 390 | dark | scroll | TaskDurationMs | 23.0265 | 25.6235 | +11.278% | FAIL |


최종 idle verdict는 **PASS_IN_SCOPED_HARNESS**, overall **FAIL**, incomplete0이다. manifest의 source/assets/browser hash 안정성과 cleanup을 확인했다. 실패 두 그룹은 `failure-breakdown.json`에 동일 raw의 Script/Layout/Recalc/frame/readiness 중앙값을 함께 기록했다. 후보 scroll의 ScriptDuration은 각각2.473/2.4555ms, Layout·Recalc 중앙값0이며 frameMedian은 두 제품 모두16.7ms다. 그럼에도 TaskDuration 초과는 남는다. 이 카운터만으로 raster/compositor/SVG/backdrop 중 정확한 원인을 분리할 수 없어 원인 특정이나 noise 면제를 주장하지 않는다. 앞선 좌표쓰기 교정은 적용됐지만 gate를 해결하지 못했으며, **엄격10% gate 미충족**을 Controller에 인계한다.


raw/summary/manifest/protocol·harness snapshot/binary provenance/request·frame·counter/cleanup/idle/final-verdict/report/evidence SHA는 `performance-final-e21594e8/`에 있다. 네트워크 단절은 이 결과의 PASS 근거가 아니다. 전체 후보 scope의 원래 gate에 최종 결과를 그대로 적용한다.

CDP timeTicks TaskDuration은 renderer main-thread wall task time이다. input/idle precise coverage·SSE·bounded pending·hidden/route/cleanup은 별도 측정이다. heap/layer/SystemInfo는 캡처 snapshot이며 continuous memory profile이 아니다. 전체 CPU%, GPU memory/기기 에너지/배터리는 측정하지 않았다. rAF cadence는 GPU presentation·물리 touch가 아니다. synthetic hidden 이벤트는 실제 OS background 검증이 아니다.

## 미실행·소유 정리·Controller 후속

물리 Samsung/phone, Safari/Firefox, OS gesture·native browser200% zoom, screen reader, 외부 Telegram/Push 기기 전달, 실제 shell 명령, 운영 install/channel/restart/PR 진입·Dev 복귀, 대규모 실제 프로젝트 filesystem/backend scan latency, total CPU/GPU memory/energy는 미실행이다. fixture 성공을 운영 쓰기 승인·배포 성공·외부 전달 성공으로 확대하지 않는다.

Controller가 소스/evidence를 serial cherry-pick하고 자기 소유 sample 후보 반영·한국어 PR rewrite·fresh reviewer1명 독립 실물 검수·최종 사용자 검토 hold·canonical 후속을 담당한다. Worker는 push/PR/서비스/원장 최종화/AC 체크를 하지 않는다. 최초 진행보고는 `REPORT-progress-preserved.md`에 보존했다. 최종 dirty/untracked·protected hash·owned cleanup과 evidence commit SHA/직접전송은 아래 인계 기록으로 보완한다.


### 최종 보호·작업 트리 기록

`protected-final.json`에서 f3 material5개/photo1개의 시작 대비 SHA256 전부 동일함을 재확인했다. `final-workspace-before-commit.json`에는 기존 P0 `environment.js`와 b326 `tests/helpers/workflow_native.cjs`가 byte 동일함을 기록했다. evidence commit 직전 tracked clean, untracked는 이 보고서 폴더와 adapter-validation 증거뿐이며 evidence 내부 ignored0이다. `owned-confirm-cleanup.json` 및 최종 paired/idle cleanup은 소유 서버·브라우저 종료와 stream0을 기록한다. 운영 서비스·57069·원본 f3 binary/PID는 접근하지 않았다.

최종 증거 내용은 별도 evidence commit에 보존한다. 자기 commit SHA를 내용에 재귀 삽입할 수 없으므로 exact evidence SHA와 clean 검증은 후속 인계 receipt 및 Controller 직접 메시지로 전달한다. 실패·네트워크·초기 progress·모든 visual batch를 삭제하지 않았다. 안정 Report ID와 본 REPORT 경로로 Controller가 자료를 찾을 수 있다.


### 증거 커밋 및 인계 상태

최초 전체 증거 snapshot commit: `dc55a04ee5712d68a01c4b31ab592c585b8dd5f9`. 이 커밋 직후 tracked/untracked clean을 확인했다. 최종 receipt/evidence-index 보완 커밋의 exactSHA는 Controller 직접 메시지로 전달한다. `HANDOFF.json` 참조. 인계 상태는 **BLOCKED_PERFORMANCE_GATE**이며, 성능46/48 PASS·2FAIL·incomplete0 및 scoped idle PASS다. 사용자 최종검토 hold나 전체AC 완료를 Worker가 선언하지 않는다.
