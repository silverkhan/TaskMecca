# 최신 dev UI를 보존한 작은 플로팅 외형 시안

새 합의 `dev-ui-floating-pilot-20261110.md`와 Root 원천 [댓글6755874276](https://github.com/silverkhan/TaskMecca/pull/361#issuecomment-6755874276)에 따른 첫 시안이다. 기존 재구현을 더 수선하거나 전체 확장한 결과가 아니다. 전체 A-65 완료/AC8체크·최종 디자인 수용을 뜻하지 않는다.

## 실제 소스와 범위

- 최신 origin/dev 기준 `f0a732e3dd8234a663dccf3c6937fc9dc674917b`.
- 시안 UI source `3a066ed44b1c52982b008230cfb9f4f1535847df`, 테스트·회귀 경계 commit `de7885b7d58ea54252b284e370297bdc06aa3f40`. 이후 evidence commit은 UI/production 변경0.
- 기본 엔트리를 dev로 복원했다. app.js/style.css/theme-boot.js/select-ui.js/logo.svg/manifest/perf-diagnostics 7개는 origin/dev와 byte 동일하다. index는 dev의 실제 DOM 그대로이며 opt-in script/style 3줄과 주석만 추가했다. Git 이력·이전 native UI 파일·모든 실패 raw는 보존했다. 무차별 삭제/reset/forcepush/dev merge0.
- dev의 메뉴·아이콘·표·입력·본문 순서·연속 화면·상시 앵커·플로팅 인덱스·프로젝트 메뉴·키보드 기능을 새로 만들지 않고 그대로 실행한다. sidebar/header/detail 바깥 면의 곡률·여백·그림자·배경만 바꾼다. 내부 summary/card/표/선택 UI는 dev를 유지한다.
- floating=1 모드만 적용한다. floating=0 또는 새 세션의 기본 화면은 dev 외형이다. Sidebar/header는 dev palette와 조화되는 기존 chrome을 기반으로 하며 Light Mecca의 기존 94–97% 그라데이션은 cascade상 유지된다. Dark/Slate의 기본 chrome token은91%, 문서 바깥 reading panel은96%다. 모든 palette에 동일91%가 적용됐다고 주장하지 않는다.
- 고정 사진 레이어 1개, 정적 alpha/둥근 모서리/그림자만 사용한다. 새 시안에서 realtime 굴절/SVG 광학/mouse 추적/CPU optics/canvas/row blur/상시 rAF0. dev 기존 logo/shortcut/update prompt의 원래 filter는 유지하며, pilot header는 backdrop none이다. 임의 뒤 DOM 굴절/7px blur를 이번 시안의 효과라고 부르지 않는다. 이 범위에서는 새 사용자 합의가 과거 optical engine 유지 요구를 대체한다.
- 사진은 기존 자산을 그대로 재사용했고 새 생성/외부 자산 추가0. 다른 기존 테마는 opt-in 해제 시 변경0. terminal UI는 이번 소재 시안 대상이 아니며 이전 자산을 변경하지 않았다. backend/API/Go production 변경0.

## 실제 검수와 회귀

impeccable Operate/quieter/optimize와 craft-floor 적용. 세션 context 기존1회 기록을 재사용했고 재실행0, DESIGN 수정0. 기존 GUI가 incumbent visual authority다.

동일 fixture·PC1440/1280·mobile320/390/430·대표 wide800 × Light/Dark: dev12조건과 pilot12조건의 DOM·아이콘·목차 signature 동일. 제품 시각 batch를 이어서 수행한 뒤 margin으로 모바일 가장자리가8px 잘리던 문제를 일괄 수정하고 최종 confirm12조건1회 완료했다. 신규 전면 검수/외부 reviewer0. 이미지 폴더 `batch/`(dev), `batch-pilot-completion/`(수정 전), `confirm/`(현재 UI). `confirm` errors0/rootOverflow0/sidebar·header실제bounds정상, anchor click/hash/refresh/resize/background image-solid/default-dev복귀 PASS.

초기 자동화의 기존 dev filter/undefined JSON key/중복query 가정 오류와 모든 실패 로그는 `inspection-notes.json`에 보존했다. 제품 기준을 완화하지 않았고 같은 화면을 계속 polish하지 않았다.

- 새 기본 dev/pilot 정적 동등성+실제 기능 browser: 2/2 PASS, 내부 PC/mobile×Light/Dark4조건. 검색20정확행·/키보드·실제combo pointer/Escape·System반응·sidebar열기닫기·anchor/refresh·default-dev동등성·reducedmotion PASS (`mechanics.tap`).
- 관련 unit36/36 PASS (`units-confirm.tap`). 첫35/36 실패는 기본 엔트리 교체 후 옛 native index 단정이었다. native assertion은 동결 과거 fixture에 그대로 유지했고, 새 default의 exact-dev assertion을 추가했다.
- 보존 native UI core browser1/1 PASS (`historical-core-confirm.tap`). 첫실패는 dev select helper와 historical native helper의 combobox 의미 차이였다. 과거 엔트리/4줄차이select helper snapshot을 historical fixture에만 사용하여 기존 assertion을 약화하지 않았다. 이 성공을 새 default dev의 검증으로 표기하지 않는다.
- safe named Go: TestSignatureThemeBranding / TestOverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly / TestPreviewGuardRejectsMutationsAndRiskyReadRoutes PASS. unprotected go test ./...0. CI는 새 dev/pilot test와 기존 native 회귀를 둘 다 수행한다.
- cmd/task-mecca build PASS. binary는 /private/tmp/task-mecca-a65-build/task-mecca-dev-floating-pilot이며 서비스로 실행/설치하지 않았다. source-protection.json에 hash 보존.

## 모바일 비용 대체 계측 — 혼합 결과

원문: “이제보니 모바일에서 느립니다. (삼성브라우저 - z fold8 ultra, 펼쳤을때, 접었을때 둘 다.)” 기기명은 원문 그대로다. OS/browser version/actual viewport·DPR/refresh rate/zoom/가속 미확인. 실제 삼성 인터넷 기기는 없다.

같은 Mac Chromium에서 synthetic Samsung UA·touch·360×800 DPR3 /800×900 DPR2.5 대표 viewport를 사용했다. 실제 기기의 접힘/펼침 치수라는 뜻이 아니다. 동일 JSON·정확히600px touch-scroll·8pair/조건·counterbalance·64유효샘플. Source/assets/Chrome stable=true. 원래 전체48scenario/10% gate와 별도인 작은 측정이며 전체 gate 면제가 아니다.

|대표 viewport/theme|scroll TaskDuration ms|차이|Paint ms|RasterTask ms|관측 DroppedFrame trace event 총수(8회)|
|---|---:|---:|---:|---:|---:|
|360 light|46.799 → 31.721|-32.22%|17.023 → 2.846|7.381 → 6.606|6 → 14|
|360 dark|47.660 → 31.550|-33.80%|19.729 → 2.577|5.708 → 7.075|0 → 14|
|800 light|39.211 → 32.516|-17.07%|13.994 → 3.091|0.728 → 1.336|6 → 18|
|800 dark|39.527 → 31.708|-19.78%|15.475 → 3.136|0.782 → 1.466|7 → 19|

Task/Paint 비용 감소는 관측됐다. frame P95 중앙값은 약16.7ms로 거의 같고, RasterTask와 DroppedFrame trace event는 여러 조건에서 증가했다. 따라서 전체 compositor/화면표시 개선·실기 랙해소·모든 성능 개선으로 일반화하지 않는다. DroppedFrame event의 분모와 실제 presented-frame율은 확보하지 못했으므로 기기 dropped-frame비율로 바꾸지 않는다. trace raw는 gzip으로 보존했다. CompositeLayers 이벤트 미관측을 합성비용0으로, LayerTree raw0을 실제 layer0으로 해석하지 않는다. GPU memory/energy/실제 화면표시 latency는 미측정이다. Heap 전후 값은 raw에 있으나 전체 앱 메모리나 GPU 메모리를 뜻하지 않는다. input TaskDuration·완료 wall도 summary/raw에 그대로 보존했다.

첫 계측 setup은 manual page-mode 누락으로 중단했고, 다음 3-way 시도는 이전 d5ac UI에 dev 연속 TOC가 없어 동일본문 비교 조건을 충족하지 못했다. 모두 failure/trace/raw/cleanup 보존, 최종 dev-pair와 섞지 않았다. 기존 실패를 noise로 재분류하거나 threshold를 바꾸지 않았다.

## Gate와 재개 조건

이전 f4/8728 후보 전체48 gate44/48 FAIL과 immutable idle26/27 FAIL(settings필수3GET/all-API-zero충돌)은 이전 근거와 정책hold로 보존한다. 새 작은시안의 전체48 gate·immutable idle은 재실행하지 않았다. 작은대체측정/기능검사를 전체 gate PASS로 승격하지 않는다. threshold/API 예외/정책hold 임의해제0. 전체 확장은 이번 시안 사용자 수용 이후에만 가능하다. 삼성 실기 해결 주장은 사용자 재확인 전 금지한다.

미실행: Samsung physical·Safari/Firefox·실제 OS zoom/주사율·screenreader·외부 PushTelegram·실 PR전환/운영설치·전체48 gate/idle재측정·전체 GPU/energy. 합의 범위에서 이 작은 시안의 구현·검증·PR 게시까지만 수행했다.

## 검토 경로

같은 PR361의 최신 HEAD Preview/PR검증 인스턴스를 선택한 뒤 그 인스턴스의 기존 A-65 상세 URL에 `floating=1&background=image`를 붙인다. theme/lang은 기존 dev 컨트롤을 사용한다. `background=solid`, `floating=0` 비교가 가능하며 선택은 해당 브라우저 세션에 보존된다. 직접 경로 예시는 `/tasks/A-65?project=%2FUsers%2Fhanati%2Fprojects%2FTaskMecca%2FTaskMecca&backlog=%2FUsers%2Fhanati%2Fprojects%2FTaskMecca%2FTaskMecca%2F_task_mecca%2Fdata%2Fbacklog&floating=1&background=image` 이다. Preview instance의 실제 project/backlog context가 다르면 기존 URL의 context를 유지해야 한다.

현재 로컬57069는 과거 f3 시안이며 새 dev-pilot이 아니다. 운영18765/기존57069 install/switch/restart0, 임의 visualserver0. 고정URL에 최신소스가 있다고 주장하지 않는다. PR Draft/base dev 유지, merge/deploy0. CI/Preview exact HEAD snapshot은 canonical external-final.json과 PR 본문에 별도 기록한다. Root ACK는 완료조건이 아니다.
