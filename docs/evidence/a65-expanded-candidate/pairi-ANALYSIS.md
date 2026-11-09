# A-65 성능 원인분리와 채택 근거

안정 ID: `a65-performance-correction-pairi-20261109-t3-32ea130b`.
이 문서는 진단 기록이다. 최종 판정은 `REPORT.md`와 원본 48그룹 gate/idle 원시를 따른다. 진단군의 소재 제거, trace overhead, 불완전 sample을 수용 근거로 승격하지 않는다.

## 기반과 권한

- 실제 시작: 준비된 `/Users/hanati/.t3/worktrees/TaskMecca/t3-32ea130b`, 시작 HEAD `d315e2f184c156cafe97d4d2c3129fa2a71733f9`, clean.
- 정상 merge `ff5e3b6b607c9efeda15fb43832ebdb2e681060c`: 충돌에서는 승인된 `75749827a94ea515dc2442d5767d8ad9cfa2d1dd` 전체 tree를 보존. 두 tree 모두 `1d6a2f709157b3ecfec04c2bafbc84f7c6538213`. reset/force/delete/새 worktree 생성 없음.
- 교정 source commit: `e646f475870aa1dbfc4c4f43d88fcaa317768183`. Controller는 이 교정 commit만 통합한다. 기반 merge는 통합 대상이 아니다.
- production 변경은 `ui/optics.js`, `ui/expansion.css` 두 파일. API/Go/backend, 원본 photo, f3 시안 파일, material 5개, environment.js, workflow_native.cjs, 테스트/성능 기준 변경 없음.
- Controller가 최종 통합/독립 검수/PR/원장/사용자 검토를 담당한다. Worker는 subagent, canonical/runtime/lifecycle/계약 쓰기, 타 WT 쓰기, push/PR, 운영 서비스, 18765/57069 접근을 하지 않았다.

## 실패 이력을 보존한 출발점

canonical `accepted-expansion-20261109`의 원본 실패를 읽었다. e215 최종은 46 PASS / 2 FAIL, 1440 Light scroll +32.5589%, 390 Dark scroll +11.2783%; 앞선 7eab 실행은 44/4이다. Controller 1bb6f181 native scroll-timeline 시도는 43/5로 더 나빠 정상 revert된 상태가 757이다. 이 실패를 성공으로 재사용하거나 동일 소스를 blind full rerun하지 않았다. 본 교정은 scroll timeline, inner photo transform, will-change:transform promotion을 채택하지 않는다.

## 원인 분리

`diagnostic-01`은 1440 Light / 390 Dark, variant마다 제외 warmup 1 + 반복 6, 순서 정/역 counterbalance로 진행했다. baseline은 지정 f0a732e3 read-only archive, candidate는 준비된 checkout, 동일 Chrome binary/기기/합성 data를 사용했다. native source 변경 전 controlled CSS로 filter만 제거, rim만 제거, fixed sampling만 제거, opacity promotion을 각각 분리했다. 진단에는 규정 full gate와 다른 setup이 있으므로 gate가 아니다. 각 sample에서 정확한 Needle IDs를 기다리고 실제 400px 이동과 pageerror 부재를 확인했다.

1440 반복에서 fixed sampling 제거만 명확히 비용이 낮았다. filter 제거와 rim 제거, opacity promotion은 지속적인 개선 근거가 없었다. `diagnostic-01/summary.json`의 median 함수는 짝수 sample에서 upper median을 사용하므로 정확한 중앙값 판단에는 `raw.json`을 이용한다. 이 부정확한 요약은 그대로 보존했고 최종 gate에는 원본 중앙값 함수를 byte 그대로 사용한다. 일부 단일 trace 요약에 n=0 행이 있어도 유효한 비교로 보지 않는다.

`scoped-trace-01` controlled trace에서 current는 1440/390 각각 Paint 60회, noFixed는 각각 1회였다. current RasterTask는 86/53회, noFixed 13/9회였다. trace의 TaskDuration은 instrumentation overhead를 포함하므로 합격 수치로 사용하지 않는다. fixed sampling을 제거하면 사진 좌표가 바뀌므로 이 진단 CSS 자체를 ship하지 않았다.

`native-trace-01`에서 native SVG backdrop 경로의 Paint는 각각 6회, RasterTask 32/26회였다. `numeric-trace-01`은 실제 face 크기의 map을 쓴 경로에서도 이 Paint 감소가 유지됨을 확인했다. 원인은 GaussianBlur나 rim 자체라고 단정하지 않고, 반복 대조와 Paint 횟수로 확인한 **각 decorative face의 fixed-background paint 경로**를 교정 대상으로 삼았다.

## 광학 검증에서 발견한 문제

새 native backdrop 이식의 controlled probe에서 공용 `.material-optical-defs`는 0×0 viewport다. 이 backdrop 문맥에 `feImage width="100%" height="100%"`를 주면 map의 유효 크기가 0이 되어, displacement가 edge field 대신 전역 이동처럼 보였다. `backdrop-support`, `edge-support`, `map-support-02`, `map-support-http`는 syntax support만으로 실제 지원을 선언할 수 없음을 보여 준다. data SVG를 PNG나 same-origin HTTP로 바꾸는 것만으로 해결되지 않았다. 이 probe를 기존 수용 f3의 별도 CSS-filter 렌더링 전체에 대한 부정 판정으로 일반화하지 않는다. inline 참조와 percentage primitive map 실험도 채택하지 않았다.

`map-support-numeric`에서는 동일 map의 width=320, height=230 숫자 지정만으로 중앙의 line 좌표가 정확히 유지되고 좌우 edge line만 이동했다. feFlood 중립 대조도 함께 보존했다. 별도 PNG/수정 photo/CPU optics engine을 ship하지 않고 원래 code-native SVG map을 그대로 쓴다.

## 채택 구성

Chromium UA와 CSS syntax 지원이 있는 top-level face는 실제 뒤 DOM을 native `backdrop-filter`로 처리한다. blur(7px) 후 원래 map/scale 22 displacement 순서다. 각 face의 ResizeObserver가 map 폭·높이만 갱신하며 scroll 좌표 읽기/재쓰기와 continuous rAF는 없다. 기존 backdrop veil(.30 dark/.20 white)을 고려해 face tint를 보정해 수용된 .68 dark/.60 white의 최종 tint를 유지한다. rim, 곡률, 두께 shadow, 근접광은 기존 source 그대로다.

nested face는 parent의 tint나 foreground를 다시 blur하지 않도록 기존 viewport-fixed **동일 local photo 샘플링**을 유지한다. Chromium nested sample의 map에도 face 숫자 크기를 준다. non-Chromium UA의 기존 blur/photo/rim fallback은 유지하고 edge refraction 지원으로 과장하지 않는다. 실제 Safari/Firefox 실측이 아니라 Chromium에서 UA branch 활성화만 검증했다.

초기 native SVG 전체-graph 후보는 비용 감소가 있었지만, 중앙 좌표를 보존하는 numeric map을 적용한 다음 모바일의 baseline 대비 비용이 아직 10% 근처/위에 있었다. `split-diagnostic-01`에서 native CSS blur를 먼저 하고 SVG는 displacement만 맡기는 순서로 같은 optical operation을 분리했다. 390 Dark 8회 중앙값은 baseline 18.5805ms, 이전 native 후보 22.458ms, split 20.0415ms (+7.86% baseline, 약 -10.76% 이전 native). 이 개선 근거로 source를 교정했으며 최종 합격은 별도 원본 full gate로 판정한다.

## 재질/행동 보존 근거

`optics-regression-final/result.json`은 1440/390 × Light/Dark 네 경우에서 map dimensions, overflow, scroll 400px, viewport resize, foreground style 불변, foreground filter/backdrop-filter/text-shadow/stroke 효과 부재, nested fallback, selection, solid 전환, detached filter cleanup, pageerror 0을 검사했다. `pixel-proof.json`은 production이 생성한 map을 사용해 blur만 진단상 0으로 놓고 rim/glint confound 없이 실제 뒤 DOM grid를 비교했다. 중앙 RGB 차이는 0이고 양 edge는 양수, 뒤 DOM을 바꾸면 중앙도 반응한다. 이는 반사광이나 동일 photo 재샘플링을 실제 DOM refraction으로 오인한 증거가 아니다. 진단상 blur 제거는 production 소스에 들어 있지 않다.

`source-diagnostic-01`과 `split-diagnostic-01`의 0/400px raster/geometry 및 원시 비용을 보존했다. whole-viewport RGB 평균은 국소 edge 품질의 단독 합격 기준이 아니며, blur 순서 분리의 raster 차이도 `blur-order-raster.json`에 숨김없이 남겼다. 새 전면 aesthetic review/시안/레이아웃 재설계는 하지 않았다.

기존 core+expansion+P0+workflow browser 11/11, unit 66/66이 통과했다. `regression-browser.tap`, `regression-unit.tap`, `regression-return-dev/`를 보존한다. Impeccable context를 이 세션에서 material-review 대상으로 1회 실행하고 PRODUCT/DESIGN·대표 source·optimize·craft-floor를 읽었으며 detector 결과는 `[]`다. 독립 최종 검수는 Controller가 수행한다.

## 측정 한계와 실패 보존

모든 진단/실패/불완전 실행과 cleanup을 별도 디렉터리에 보존한다. 첫 map-support 실행은 SVG document에 HTML을 setContent한 fixture timeout으로 실패했으며 브라우저를 닫았다. Pillow가 없는 시스템 Python 시도는 PNG 분석 전에 실패했고, 이후 isolated uv dependency로 실제 분석했다. 이를 source 실패나 성공 gate로 바꾸지 않는다.

최종 측정은 기존 하네스/프로토콜을 동일 깊이 디렉터리로 byte 동일 복사해 쓴다. full measurement 동안 다른 browser/build/race를 실행하지 않는다. threshold, assert, skip, baseline 동작 변경, noise 면제, 추가 fixed sleep은 없다. 원본 하네스의 timing window와 입력 간격은 그대로다.

검증 범위는 동일 Mac의 synthetic static frontend뿐이다. 새 source의 실제 Go backend/producer, 물리 Samsung, Safari/Firefox, OS zoom 200%, 스크린리더, 전력, 전체 CPU/GPU/메모리, active-task 규모 성능은 실측하지 않았다. 이전 Worker의 실제 Go 150case/190PNG 및 UX 검증 기록은 보존/참조했으며 본 Worker가 재실행한 것으로 주장하지 않는다.


## 최종 frozen-source 결과와 BLOCKED

`e646f475870aa1dbfc4c4f43d88fcaa317768183`의 원래 full timing gate는 **47 PASS / 1 FAIL**이고 idle은 **26/27**이다. source/binary/asset stable, invalid 0, baseline asset과 Chrome binary hash는 이전 e215 실행과 동일하다. 기존 모든 scroll 그룹은 통과했다. 잔여 timing FAIL은 1440 Light navigation TaskDuration 74.4315→82.6585ms (+11.053115952251403%)다. threshold 완화·noise 면제·같은 source full rerun을 하지 않았다. source는 기술 완료/merge-ready가 아니다.

idle 실패 `pendingStopsOnRoute`의 raw는 settings bootstrap `/api/version`, `/api/runtime/hooks`, `/api/channel-options` 각 1회이며 SSE는 0이다. 원래 frozen runner는 `page.goto(settings)`의 page load 직후부터 모든 API를 0으로 요구한다. `idle-route-diagnostic-01`에서 legacy optics/e646/observer guard 각각 4회(총12회), 순서 counterbalance로 대조했다. 세 variant 모두 매번 같은 합법적 settings 초기 GET 3개가 page load 뒤 들어왔으며 stale `/api/overview` 요청과 SSE는 항상 0이었다. 따라서 새 optics만의 pending-poll 퇴행으로 단정할 수 없다. Worker는 이 실패를 PASS로 재분류하거나, 필수 settings API/프로젝트 경로 검증을 삭제·우회하거나, immutable runner의 readiness/assertion을 변경할 권한이 없다. 이 충돌을 해결하지 않은 채 full gate를 반복해 운 좋은 결과를 고르는 것도 하지 않았다.

navigation 후속 대조는 전부 별도 진단이다. `nav-target-02`의 batch geometry 초기화는 개선을 재현하지 못해 source에 채택하지 않았다. 이 실행의 탐색용 원래 summary는 두 theme를 pool하므로 사용하지 않고 `derived-by-theme.json`의 정확한 분리를 보존했다. `observer-nav-target-01`은 1440 Light 8쌍에서 baseline 90.825ms, e646 92.891ms, guard 88.705ms였다. `observer-nav-trace-01`은 material `surfaces` 호출 8→4, optics `refresh` 16→6을 확인했다. 그러나 guard는 full gate나 전체 동작 검증을 거친 source가 아니며 관련 없는 DOM 변화가 근접광 위치 캐시를 dirty로 만드는 기존 행동도 보존해야 한다. 실험 파일로만 남기고 production에 채택하지 않았다. 최종 production은 여전히 exact e646이다.

최종 상태는 **BLOCKED**다. nav의 strict FAIL이 남고 immutable idle 판정과 필수 bootstrap 조회의 충돌도 남는다. AC/전체 기술 완료/최종 사용자 hold로 승격하지 않는다. Controller는 이 실패 source와 원시를 직접 인수해 기존 계약 범위 안의 해결 근거 또는 상위의 명시적 측정 정의 정합성 결정을 확보해야 한다. Root ACK를 기다리지 않는다.

최종 handoff 전 Full Access probe는 새로 만든 owned `access-probe` fixture에서만 실행했다. `final-preflight.json`의 full/orchestration_ready=true와 자동 cache를 보존한다. canonical/사본 운영 원장이나 운영 runtime/lifecycle을 수동으로 수정하지 않았고 monitor 등록도 하지 않았다. 이 승인된 CLI ephemeral probe/cache와 canonical runtime 운영 쓰기를 구별한다.
