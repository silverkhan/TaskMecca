# A-65 성능 교정 Worker 인계 — BLOCKED

안정 ID: `a65-performance-correction-pairi-20261109-t3-32ea130b`  
배정: `assignment-e17b301517088a37defdac1a151424ec`  
semantic Worker: `/root/controller/pairi`

**최종 상태는 BLOCKED다. 성능 gate를 충족하지 못했으며, 기술 완료·merge-ready·A-65 AC·최종 사용자 hold로 승격하지 않는다.** 기존 scroll 실패를 교정했지만 새 frozen source의 전체 timing은 47/48, idle은 26/27이다. 같은 source를 blind full rerun하거나 noise 면제하지 않았다.

## 정확한 인계 SHA와 작업 트리

- 교정 source: **`e646f475870aa1dbfc4c4f43d88fcaa317768183`**.
- 원시/진단/실패/회귀 evidence commit: **`786ffe3496d0897116cc16898876c6b9d3c77cc7`**.
- 별도 기반 merge: `ff5e3b6b607c9efeda15fb43832ebdb2e681060c`. 시작 d315와의 정상 merge 충돌에서 승인 기준 `75749827a94ea515dc2442d5767d8ad9cfa2d1dd`의 tree를 보존했다. merge와 757의 tree는 모두 `1d6a2f709157b3ecfec04c2bafbc84f7c6538213`다.
- ancestry: `757 → ff5e3b6b → e646f475 → 786ffe34 → 인계 metadata commit`. 원격 b326을 개발 기준으로 잘못 사용하지 않았다. Controller가 source를 검토할 때 기반 merge/evidence를 source 교정과 구별한다. **현재 실패 source를 완료본으로 통합하라는 권고가 아니다.**
- checkout: `/Users/hanati/.t3/worktrees/TaskMecca/t3-32ea130b`, branch `t3/optics-performance-correction`.
- source freeze/측정 종료 때 tracked production clean이고 source worktree가 commit과 byte 동일했다. freeze 시 untracked는 새 owned evidence뿐이었다. 최종 metadata도 별도 commit하며 최종 HEAD/clean 상태는 직접 인계 메시지로 전달한다.
- integration WT와 원본 Worker WT는 읽기만 했다. reset/force/delete/새 worktree 생성·타 WT source 수정은 없다.

## 실제 신원과 전달

CODEX_THREAD_ID=`01a1bf10-eeee-7cc0-9366-011ff76fde0c`. 실제 T3 Worker는 `final-native-worker.json`의 codex/gpt-6.1-sol/full-access/default, ordinal 1, 준비된 branch로 관측했다. 실제 run ID와 최초 env/기반은 `start-identity.json`에 있다. runtime hook은 Worker가 독립 관측하지 못했으며 추정 binding이나 수동 JSONL/state를 만들지 않았다.

Controller exact native target은 다음이다.

<t3-thread://v2/3a4cd808-c663-4625-923d-8f6e2713c5f5/thread:delegated-task:command%3Amcp%3A39b825cc-e32b-4f03-8716-c7353d39e936%3Adelegate-task%3Aa65-transmissive-glass-controller-20261109>

시작/진행/freeze/실패를 이 target에 직접 보냈다. 최종 직전 native 조회는 completed/ordinal20/full-access였으므로 지원되는 native 연속 메시지로 재개한다. `final-preflight.json`은 owned isolated probe fixture에서 full/orchestration_ready=true였다. canonical 운영 runtime/lifecycle/계약/원장 쓰기나 monitor 등록이 아니다. 최종 수락 반환은 `controller-final-transport.json`에 별도 보존한다. **transport 수락/새 turn 시작과 Controller 처리 완료를 구별하며, Root ACK를 기다리지 않는다.**

## 채택한 교정과 재질 지원

production 변경은 `ui/optics.js`, `ui/expansion.css` 두 파일이다. 원본 photo, 수용 f3 시안 및 material 5개, environment.js, b326 workflow_native.cjs, Go/backend/API/기존 UX/한영/Light-Dark-System/image-solid/scroll restore/테스트/CI repair는 변경하지 않았다.

각 decorative photo의 `background-attachment:fixed` 경로가 root scroll마다 Paint를 만드는 것을 controlled 비교로 분리했다. filter/rim 제거와 promotion은 해법으로 채택하지 않았다. trace current Paint 60/60회가 noFixed 진단 1/1회, native backdrop 진단 6/6회로 줄었다. trace timing은 instrumentation overhead가 있어 gate 근거로 쓰지 않는다.

현재 Chrome의 top-level 면은 native CSS **blur(7px) → SVG edge displacement(scale22)** 순서로 실제 뒤 DOM을 처리한다. 수용 map·rim·곡률·두께·근접광을 유지하고 기존 backdrop veil을 감안해 최종 face tint를 보정했다. 각 face의 ResizeObserver는 map 크기만 갱신한다. 새 CPU optics/canvas/row blur/continuous rAF/scroll 좌표 재쓰기는 없다.

native 이식 probe에서 zero-sized SVG defs의 percentage map 크기가 실제 edge field로 동작하지 않는 문제를 확인했다. map에 실제 숫자 폭·높이를 주어 중앙 좌표 보존과 양 edge 변위를 증명했다. 이를 기존 수용 f3의 별도 CSS-filter 렌더링 전체에 대한 부정 판정으로 일반화하지 않는다.

nested 면은 기존 viewport 좌표의 **동일 local photo 재샘플링**을 유지한다. 이것은 임의 뒤 DOM 굴절과 다르다. non-Chromium UA는 기존 blur/photo/rim fallback이며 edge 지원을 주장하지 않는다. fallback 검사는 Chromium의 UA branch 활성화일 뿐 Safari/Firefox 실물 검증이 아니다.

`optics-regression-final/pixel-proof.json`은 production-created map에서 blur만 진단상 0으로 분리하고 rim/반사광 없이 실제 뒤 DOM grid를 비교했다. 네 viewport/theme 모두 중앙 RGB 차이 0, 좌우 edge 약37~72, 뒤 DOM 변경 시 중앙59.5였다. foreground filter/backdrop-filter/text-shadow/stroke 효과 부재, 선택, resize, 400px 이동, solid 전환, nested fallback, detached map cleanup도 확인했다. 진단상 blur 제거는 ship source에 없다.

## 전체 gate의 정확한 실패

원본 RUNBOOK/protocol/paired/fixture/idle/idle-verdict/report/adapter-validation을 동일 깊이 `docs/evidence/a65-pairi-performance`로 byte 동일 복사했다. 기존 evidence/binary-provenance는 덮어쓰지 않았다. source freeze 후 원래 8 counterbalanced pairs/48 groups 전체 gate는 **1회** 실행했다. warmup은 제외했다.

`docs/evidence/a65-pairi-performance/gate-frozen-e646f475/`의 전체 결과:

| 항목 | 실제 결과 |
| --- | --- |
| TaskDuration/frameMedian 48 groups | **47 PASS / 1 FAIL** |
| 남은 timing FAIL | 1440 Light navigation TaskDuration |
| baseline → candidate 중앙값 | 74.4315ms → 82.6585ms |
| 변화 | **+11.053115952251403%** — 10% gate FAIL |
| 기존 scroll 그룹 | 모두 PASS |
| invalid/readiness/ID/400px | invalid 0 |
| source/asset/Chrome binary | stable |
| 이전 e215 대비 baseline assets/Chrome hash | 모두 동일 |
| idle27 | **26 PASS / 1 FAIL** |
| idle 실패 | `pendingStopsOnRoute` |
| overall | **FAIL** |

raw/counter/frame/request/manifest/source snapshots/protocol/binary hash/invalid/cleanup/idle coverage/전체 verdict/evidence-sha256를 모두 보존했다. `final-verdict.json`, `summary.json`, `idle-verdict.json`, `raw.json`, `idle.json`과 원본 report가 판정 근거다. 앞선 e215 46/2, 7eab 44/4, Controller 1bb 43/5 및 정상 revert 이력은 canonical의 기존 원시를 읽고 보존했다. 실패 timeline/transform/promotion 방식을 성공이라고 재사용하지 않았다.

## 차단 근거와 후속 대조

idle raw의 route-exit 후 요청은 필수 settings bootstrap GET `/api/version`, `/api/runtime/hooks`, `/api/channel-options` 각 1회다. SSE는 0이다. 고정 runner는 `page.goto(settings)`의 page load 직후부터 **모든 API 0**을 요구하므로 이 세 요청을 실패로 센다.

`idle-route-diagnostic-01`에서 legacy optics/e646/observer 대조군 각각 4회(총12회), 순서 counterbalance를 했다. **세 군 모두 매번 같은 settings 초기 GET 3개가 page load 뒤 발생했고, stale `/api/overview`와 SSE는 항상 0**이었다. 이 대조는 원래 idle 판정을 대체하거나 PASS로 면제하지 않는다. 새 optics만의 pending-poll 퇴행이라고 단정할 수도 없다.

Worker는 필수 settings 조회나 프로젝트 경로 검증을 삭제·우회할 수 없고, immutable runner의 readiness/zero-API assertion도 바꿀 수 없다. 이 충돌이 남은 상태에서 full gate를 반복해 우연한 결과를 고르지 않았다. strict navigation FAIL도 별도로 남는다. **Controller의 인수/정합성 판단과 승인 범위 안의 추가 해결 근거가 필요하며, Worker가 AC나 최종 사용자 hold로 바꾸지 않는다.**

navigation의 batch geometry 초기화는 개선 재현이 없어 기각했다. observer 대조는 material `surfaces` 8→4, optics `refresh` 16→6 호출과 targeted 8쌍 비용 감소를 확인했지만, 근접광 위치 dirty 행동 보존/전체 검증을 거친 source가 아니다. `optics-batch-init.js`, `optics-observer-gate.js`, `expansion-observer-gate.js`는 **진단 파일로만** 보존했고 production에 넣지 않았다. 탐색 summary의 upper-median/pooling 한계와 올바른 raw-derived 분리는 `ANALYSIS.md`에 명시했다.

## 회귀·한계·cleanup

- 기존 core/expansion/P0/workflow browser **11/11 PASS**. fixture 쓰기만 사용했다.
- preference/task-data/return-dev/select/terminal unit **66/66 PASS**.
- scoped optics desktop/mobile × Light/Dark **4/4 PASS**, fallback UA branch PASS.
- Impeccable context 1회, PRODUCT/DESIGN/대표 source/optimize/craft-floor 적용. detector `[]`. 새 전면 시각 라운드와 subagent는 0이다.
- 실제 Go backend/producer, 물리 Samsung, Safari/Firefox, OS zoom200, 스크린리더, 전력/전체 CPU/GPU/메모리/active-task 규모는 미측정이다. 과거 Worker의 실제 Go150case/190PNG를 본 Worker 재실행으로 주장하지 않는다. 로컬 `go test ./...`는 실행하지 않았다.
- owned browser/context/HTTP server/SSE cleanup은 23개 cleanup 기록과 `EVIDENCE-INDEX.json`에 있다. 모든 실행은 종료했고 외부 process kill은 0이다. 운영18765/57069 접근/서비스 변경/PR/push/canonical 운영 쓰기는 0이다.
- 고유 Go binary `/private/tmp/task-mecca-a65-pairi-e646f475870aa1dbfc4c4f43d88fcaa317768183`와 source/build/hash 근거를 보존했다. timing은 이 binary의 backend 실측이 아니라 실제 정적 frontend+합성 API 비교다.
- primary evidence 358개(약120MB), 실패/불완전 probe 포함, 삭제/덮어쓰기 없이 commit했다. final native/REPORT/HANDOFF/transport는 후속 metadata다.

전체 원인·채택/기각 역학은 `ANALYSIS.md`, 요약 기계 판정은 `RESULT.json`, immutable raw와 index는 evidence commit을 따른다. 독립 최종 reviewer/통합/PR/원장화/사용자 검토는 Controller의 후속 범위다.
