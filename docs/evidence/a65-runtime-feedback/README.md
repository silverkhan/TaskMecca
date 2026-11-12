# 실사용 피드백에 따른 제한된 보완 — timing 통과, idle 기준 충돌 유지

원천 피드백: “UI구현된 사항 PR검증모드 진입해서 확인했는데 1. 랙이 심하다 2. 전체적으로 뭔가 정돈되지 않은 느낌이라 정신이없다”.

“$impeccable 스킬을 같이 활용해 검수하면서 개선하기를 바란다.” / “유념할 점은 지금 GUI THEME이 깔끔하고 간결한 성공 사례라는점이다”.

기존 GUI의 명료함을 기준으로 같은 합성 데이터의 백로그를 PC/mobile Light/Dark 한 배치로 비교했다. before의 gui는 origin/dev f0a732e3 정적 자산, candidate는27dbe42e다. 사용자 실제 PR검증모드의 실행 SHA는 미관측이며 이를27dbe42e로 단정하지 않는다. 합성 fixture를 실제 사용자 데이터로 주장하지 않는다.

Impeccable Operate quieter+optimize/craft-floor 적용. 메뉴·기능·정보 내용은 유지하고 요약 띠의 별도 광학층 제거, 중첩 깊이/보조 경계·광택 축소, 간격 정돈, 배경 대비 절제. 곡률·주요면의 투과/7px blur/edge22 유지. 원본 material/photo 자산 변경0. 전경 text/icon filter0. DESIGN 변경0.

텍스트·장식 노드의 childList 갱신은 표면 registry를 바꾸지 않으므로 전체 surface/광학/proximity 검색을 생략한다. 신규 readonly 화면과 표면 제거는 계속 반영한다. mutation.json의 단일 대조 실험(60회 합성 텍스트 갱신)에서 전체 selector 호출126→3, ScriptDuration4.014→1.885ms, TaskDuration18.242→15.277ms. 실제 화면의 병목 전체를 설명하거나 통계적인 성능 gate로 삼지 않는다.

before/after의 각 4회 1초 스크롤·포인터 측정은 mixed result다. 포인터는 mobile viewport에서도 합성 mouse이며 실제 touch가 아니다. frame 중앙값16.7ms. task/스크립트/heap 원시는 scoped-summary.json과각raw.json. 전체 CPU·GPU memory·energy/삼성/Safari/Firefox 미실행. 사용자 랙 해결 선언0.

before8장(기존GUI4+후보4)→수정→after4장(후보PC/mobile Light/Dark) bounded 검수. after 캡처·측정 후 모바일 heading/summary의 좌우14/12px 정렬만 교정했으며 추가 시각라운드는 열지 않았다. after 캡처/manifest는 그 수평정렬 직전 소스이고 최종 source와 구별한다.

회귀: 첫 CSS수정은 mobile 첫행450px조건 실패해 padding을 바로 교정했다(regression.tap 보존). 이후 core/expansion/P0 3/3 PASS(final-regression.tap). 알림 테스트는 실제 nav-label span과 기존부터 있는 정확한 한국어 '알림센터'를 검증하도록 정정. 기존 CI의 class없는 span/알림 기대 불일치 이력을 보존하고 notification-final.tap11/11 PASS. assertion 삭제·성능 threshold/하네스 수정 없음.

원래e646 full47/48·idle26/27 실패와 정책hold는 그대로다. 이 작은 보완을 전체 gate 통과/전체UX수용/AC체크로 확대하지 않는다. 새 frozen source의 전체 gate는 별도로 기록한다. 운영Dev18765/소유57069/merge/deploy 변경0, 새 Worker/reviewer0.

## 새 frozen source 결과

source **7eab6fa8d17399a26fc54c3bc8a49eae0fa1b7a6**. [원래 하네스·raw·manifest·idle](../a65-feedback-gate/frozen-7eab6fa8/)는 기존 pairi의 protocol/fixture/paired/idle와 SHA256 byte동일이다. 새 production 소스의 full gate는 한 번만 실행했다. 48/48 timing PASS, 54 raw pairs(6warmup 포함), invalid0, assets/head/Chrome binary stable=true. 이전 e646 FAIL을 지우거나 PASS로 재분류하지 않는다.

기존 잔여1440Light navigation은 이번 run의 baseline83.445→candidate81.0505ms(-2.8696%). 각 run의 baseline 자체도 달라졌으므로 e646의82.6585ms와 이번81.0505ms 차이를 전부 수정효과라고 주장하지 않는다. 이번 최악 TaskDuration 그룹은1280Light navigation+0.8512%로 원래10%기준 안이다. mixed scoped 결과와 별도 기록한다.

동일source immutable idle는 **26/27 FAIL**. pendingStopsOnRoute에서 `/api/version`, `/api/runtime/hooks`, `/api/channel-options` 각1회, streams0. 필수 settings bootstrap과 all-API-zero 충돌은 해결/승인된 것이 아니다. 기술 BLOCKED/정책hold 유지, 전체AC체크0/사용자최종수용0. 필수조회삭제·하네스변경·API예외·load타이밍편법0. 최종 독립검수는 아직 시행하지 않았다.

gate owned browsers/servers 종료·streams0, idle 프로세스정상종료/cleanupStreams0. source-linked binary는 build만 했고 서비스설치/전환은 하지 않았다. 별도 임의 visual 서버가 아니라 기존 synthetic browser-test fixture만 사용했다.

화면표시source 추가조회: 운영18765에 readonly GET `/api/preview/instances` 결과 `instances:[]`. `/api/preview/builds`는5초 timeout으로 응답body0. 따라서 사용자가 보았던 검증모드의 실제 실행SHA를 확인하지 못했다. GET 관측과 timeout을 실패 그대로 보존하고 build/설치/전환 POST0. 57069 listenerPID60445는 유지되며 전체 후보로 갱신하지 않았다. 이 주소는 원래 f3 작은시안이다.

protected.json: 승인된3파일 제외 현재 tracked web49파일은27dbe와byte동일. API/Go/원본photo/material/P0environment/workflow_native 변경0. CI의 알림nav-label 기대 교정은 실제 기존 labels를 정확히 assert하며 성능기준 완화와 무관하다.

실제 신원은 identity.json의 동일 Controller ordinal22/CODEX01a1bdce…/full-access. hook 미관측·수동state조작0. context는 이번 세션1회(초기 canonical cwd의 material-review target) 실행했고 재실행하지 않았다. 이후 실제 integration checkout source·기존GUI 동일자료·quieter/optimize/craft-floor를 직접 읽고 대조했다. context target 실행 위치 한계는 숨기지 않는다.
