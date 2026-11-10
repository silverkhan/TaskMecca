# A-65 플로팅 메뉴 수용·읽기면·소재 보완

제품 source **e3cde648f30cbe4e7a38925d76ce57d38b577d64**, 기반 **d82f5d343ae3729547cbf895872b872ee51136d1**. 실제 dev DOM·app/style·아이콘·앵커·데이터/기능 변경 없이 opt-in pilot CSS와 Auto 포커스 보정만 변경했다. [사용자 원문](USER-FEEDBACK.md). 사용자 첨부 원본은 canonical에 보존하고 공개 캡처는 synthetic fixture다.

## 실제 원인과 수정

Desktop의 mobile-sidebar-panel이 display:contents라 고정 sidebar 높이 안의 scroller가 없었다. 1440×500/768×420에서 Help bottom751/773px로 패널488/408px를 벗어나 pointer hit=false였다. 이제 기존 wrapper를 flex:1/min-height:0/overflow-y:auto scroller로 사용하며 메뉴/섹션을 축소하거나 숨기지 않는다. 헤더는 고정, 도움말·footer까지 패널 안으로 스크롤된다. sidebar 높이와 모바일 열린 메뉴 상한은 dvh와 safe-area를 따른다.

Auto focusin은 브라우저의 최초 focus-scroll 이후 기존 dev가 확장하여 메뉴 높이가 늘어나는 순서였다. 실제 active Help가 bottom550/panel386에 남는 trace를 보존했다. pilot.js에 desktop Auto의 sidebar focus만 queueMicrotask 후 nearest scroll로 보정한다. 1개 focus handler이며 pointer/mouse/scroll 추적, rAF, observer, loop가 없다. 일반 dev와 모바일에서는 no-op이다.

Hub·저장소 제목/설명이 투명 content 위에 있었다. 기존 content 자체를 하나의 연속 읽기면으로 보호한다. DOM/순서/기능·행별 면 추가 없음. chrome91%와 reading96%는 최악 흰/검정 배경 대비를 기존 중립 검증으로 확인하여 유지했다. reading에는 blur를 넣지 않았다. Light 상부 white .75/하부 black .10, Dark 상부 white .20/하부 black .45의 1px inset rim, chrome의 정적 .07 grazing reflection을 사용한다. desktop sidebar/topbar만 blur6px, 모바일은 열린 메뉴만 blur4px. 모바일 topbar/sidebar-header 및 목록·본문·행은 blur0. edge displacement/굴절을 구현했다고 주장하지 않는다. blur 미지원이면 neutral alpha/rim이 남는다.

## 실제 검수와 회귀

before 1batch → CSS 일괄 수정 → after 1confirm(중단 후 미실행 케이스만 이어서)으로 1440×500/768×420/390×600/320×480 × Light/Dark를 검사했다. 메뉴 top/end·Hub·저장소 64PNG, 동적 높이 -80px, compact/Auto geometry, Help pointer hit8/8, 읽기면8/8 확인. [PC 메뉴](after/768-light-menu-end.png), [mobile 메뉴](after/320-light-menu-end.png), [Dark Hub](after/390-dark-hub.png), [Light 저장소](after/390-light-storage.png). 시각 재수정/추가 시각 라운드는 없다. 이후 Auto 실제 keyboard defect는 기능 회귀로 교정했다.

첫 confirm은 768px root overflow가 0이라는 잘못된 새 단언에서 중단됐다. baseline도6px임을 원시값으로 확인하여 동일 viewport 대비 증가금지로 교정하고 이미 끝난 케이스는 재실행하지 않았다. 기존 6px는 미해결 잔여다. 첫 기능검사의 Auto focus 실패, 무효한 단순 대기 시도, search button-focus setup 실패, detail pathname을 Hub root처럼 쓴 setup 실패를 각각 로그로 보존했다. 실제 viewport·동작을 기다리고 올바른 root URL에서 검사하며 단언 삭제/skip/기준완화는 없다.

최종 browser **4/4 PASS**: exact dev entry/기능4조건, neutral16조건+System8반전, PR/Preview entry16조건. 새 short-height Help·Auto keyboard focus·모바일 메뉴·Hub/저장소 reading 배경 단언 포함. 관련 units64/64 PASS, 안전 named Go3 PASS. 전체 go test ./... 미실행. detect layout findings0. [회귀 원시결과](regression-final.tap), [unit](units.tap), [Go](safe-go.log), [15 보호자산 hash](protected.json).

## 비용 측정 경계

동일 owned fixture, 50행, Chrome desktop binary, touch/mobile/DPR3 surrogate, folded대체390/unfolded대체768 × Light/Dark, AB/BA8pairs/그룹 총64샘플. 실제 기기 viewport·사양으로 주장하지 않는다. 400px wheel 이동을 확인하고 500ms 동안 rAF interval과 CDP TaskDuration·input·heap을 측정했다. 마지막 pair의 trace에 Paint/PrePaint/Layerize/RasterTask를 보존했다. 본 검사는 scroll CSS 변경 후, Auto keyboard handler 추가 전이다. CSS는 최종 byte동일, JS 후속 범위는 별도 최종 기능검사로 확인했으며 전체 최종소스 성능 gate라고 주장하지 않는다.

| 대체조건 | scroll Task ms 전→후 | 변화 | input Task ms 전→후 |
|---|---:|---:|---:|
| 390 light | 5.930 → 6.244 | +5.29% | 32.026 → 32.087 |
| 390 dark | 5.738 → 5.491 | -4.30% | 30.403 → 29.296 |
| 768 light | 8.095 → 7.814 | -3.47% | 30.866 → 31.832 |
| 768 dark | 6.790 → 6.884 | +1.38% | 29.028 → 28.447 |

모든 조건 frame median16.7ms/P95≤16.8ms, >25ms interval0. JS heap 약3.67~3.69MB. input wall 약212~215ms는 debounce/API/Playwright 포함값으로 INP가 아니다. Paint/Raster 비용은 혼합 결과이며 단일 trace를 일반적 GPU 개선으로 확대하지 않는다. dropped-frame 이벤트/실제 GPU memory·energy·OS CPU는 계측하지 않았다. 별도 layer/GPU메모리 미측정. before visual harness의 frames는 wheel 전 idle interval이라 성능판정에 사용하지 않고 별도 performance.cjs 결과만 사용했다. [64 raw+요약](performance.json).

삼성브라우저 - z fold8 ultra 접힘/펼침 실기, 실제 touch fling/동적 주소창/키보드, Safari/Firefox 미실행. Chrome viewport resize는 대체검증이다. 전체48/8pair gate·immutable idle 재측정/면제0. 이전 통과·실패/idle26/27 FAIL 정책hold 보존, 사용자 실기 랙해소·전체디자인 수용·AC완료 주장0. 운영18765/57069·merge/deploy·새delegate0. owned test browsers/ephemeral fixtures 종료 확인.

## 사용자 확인

이 HEAD의 기존 PR검증모드/Preview 화면 배너에서 **플로팅 시안 보기 / 기존 UI 비교**로 진입한다. URL 수동 편집 불필요. 기존 project/task/hash 보존 동작 유지. 전체확장은 사용자 시안 수용을 기다린다. CI/Preview 최종HEAD 상태는 canonical 외부 snapshot과 PR 상단에 별도 기록한다.
