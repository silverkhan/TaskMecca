# 플로팅 시안 중립 팔레트·역할별 투명도

사용자 원문: “Mecca의 배색이 적용되어 있는데 직전에 Dark, Light의 검정, 하양 기준의 배색과 투명도를 적용해보자”. [Root 원천6760654161](https://github.com/silverkhan/TaskMecca/pull/361#issuecomment-6760654161)과 [lossless 합의](USER-FEEDBACK.md)를 적용했다. 전체 확장/시안 수용 승격이 아니다.

Code source **a83f6035dcebdf45ce72c32da928b9405d848ba9**, 기반2ea0098e. 제품 변경은 pilot.css뿐이다. CSS의 기존 읽기 면·sidebar12px/mobile8px·상단 공통 좌우선·gap/radius·선택1px/focus2px·밀도를 그대로 두고 paint 토큰만 변경했다. app/index/style/theme/select/사진/로고/기존PR진입/P0/terminal/native workflow·광학 파일은 byte동일이다.

## 실제 참조와 선택

|역할|실제 소스 참조|이번 값|이유|
|---|---|---|---|
|상단/sidebar/PR/알림 chrome|3a066ed4 dev-floating 선언: Light255/255/255, Dark23/26/33, alpha.91|White#fff / neutral#181818, alpha.91|직전 선언 alpha를 회복하고 RGB hue를 없앤다|
|연속 목록/본문 읽기 면|3a066ed4 선언alpha.96 →2ea에서Mecca surface 기반.98|White#fff / neutral#181818, alpha.96|투과4%를 남기면서 텍스트가 사진 위에 직접 놓이지 않게 한다|
|입력/선택·요약 내부 면|기존 dev surface/surface2 역할|white/light gray 또는black/dark gray의 불투명 내부 면|정보 밀도/입력 가독성 유지|
|외부 사진 tint|기존 .20 Light / .40 Dark|white.20 / black.40|alpha는 유지하고 푸른 색 성분만 제거|

3a의 값은 소스에 선언된 토큰이다. 당시 Light Mecca에서 일부 기존 signature CSS가 덮었던 역사와 실제 최종 적용값을 혼동하지 않는다. 이번에는 실제16조건 computed값으로91%/96%와 동일RGB채널을 검증했다. references.json에 immutable commit/file/excerpt를 보존한다.

기존 optical sample background-comparison.css의 Dark rgba24/25/29/.36, Lightwhite/.46는 별도 photo transmission/overlay/blur 층과 함께 쓰인 값이다. 그 숫자만 낮춰 새 한 겹 읽기 면에 적용하면 사진이 텍스트 바로 뒤에 드러나므로 복사하지 않았다. 이전 재구현 DOM·엔진·실시간굴절·mouse추적·rowblur/canvas·상시rAF 재도입0. 새opacity는 background에만 적용하며 텍스트/아이콘 전체 opacity나 필터를 걸지 않는다.

## 중립 범위와 보존

floating flag와 resolved data-theme 아래에서만 bg/surface/surface2/text/muted/border를 검정·흰색/회색으로 정의했다. Mecca에서 hardcode한 active navigation/theme/raw/filter 및 summary card·콤보의 장식 fill/border/gradient도 같은 중립 면으로 바꿨다. 기존 theme boot가 html에 inline background를 쓰므로 root의 background-color 한 곳은 scoped !important로 덮어 overscroll도 중립으로 유지한다. 해제하면 기존 inline/Mecca/Slate 색이 그대로 복원된다.

의미색 --ok/--warn/--danger/--accent는 변경하지 않는다. 작업 중 상태에 쓰이는 accent도 유지하고, 비의미 선택/focus는 별도 --floating-focus로 gray 경계를 사용한다. 따라서 완료/보류/경고/오류/작업 중 표식은 기존 의미색을 보존한다. logo SVG와 실제 src/filter는 그대로다. underlying Theme 선택값/일반 Mecca/Slate를 새 테마로 바꾸거나 rename하지 않았다. floating을 끄면 같은 기존 팔레트다.

## 묶음 검증

impeccable Operate/quieter와 craft-floor 적용. 중립 brief가 colorize 장식 조언보다 우선한다. 같은 세션 context는 재실행하지 않았다. detector의 color scope는 지원되지 않아 setup오류를 기록했고, 유효한 layout scan과 실제 computed색 검증을 사용했다. 지원되지 않은 scan을 PASS로 부르지 않는다.

한 candidate 시각 batch: PC1440/mobile390 × Light/Dark/System-Light/System-Dark, 목록·본문16PNG. Mecca/Slate까지 포함한 **16조건**에서 실제 dev renderer를 검사했다. 초기 fixture가 hold 행을 제공하지 않아 첫 시도가 screenshot 이전에 실패했고, 실제 hold/done fixture와 pseudo-element 읽기를 교정해 이어갔다. 제품 시각 수정/fix-loop/추가 screenshot confirm은 없었다. 캡처는 synthetic API이며 PR 배너2ea SHA는 identity seed이고 live binary/source 증거가 아니다.

- 실제 chrome/sidebar computed alpha.91, reading alpha.96, surface/selected/border/focus/ink의 RGB채널 동등 확인.
- 일반 Mecca/Slate mode-off 색은 pilot sheet를 껐을 때와 동등. 상태4토큰·실제 green/amber dot·logo src/filter는 mode-off/on 동일.
- 기존 geometry: PC sidebar 옆12px·mobile여백·header/PR/알림/읽기 면 좌우선·20행 유지, rowfilter0/root overflow0/pageerror0.
- System은8조건에서 OS colorScheme 반전까지 확인하고 preference=system을 유지한다.
- 사진 배경의 최악 black/white 합성을 가정한 읽기 면 contrast bound: 보조 텍스트 최소 **5.585:1**, 본문 최소14.213:1. 이 숫자는 해당 reading text/fill의 계산된 하한이지 모든 UI/상태색/실기 화면의 접근성 완료 판정이 아니다.
- 최종 CI 등록3파일 browser 묶음 **4/4 PASS**: 기존 dev/pilot 소스·4조건 기능/Auto, PR진입16조건, 신규 palette16조건. screenshot 없는 CI 실행에서도 detail summary의 중립 fill/border를 확인한다.
- 관련 unit64/64·safe named Go3 PASS. unprotected go test ./.../uninstall fixture0. 격리 fixture/browser 정리 완료.

[Dark PC](batch-complete/list-1440-dark.png), [Light mobile 본문](batch-complete/detail-390-light.png), [System Dark mobile](batch-complete/list-390-system-dark.png), 전체값 batch-complete/neutral-results.json 및 ci-regression/neutral-results.json.

## 남은 범위

직전2ea HEAD는 직접 조회한 원격 checks8/8 SUCCESS다. 새 후보의 exact HEAD CI/Preview는 별도 snapshot으로 기록하고 Root 기존watch에 인계한다. childwatch/poll0. CI 성공을 전체48 gate/immutableidle 정책hold 해소·삼성브라우저 z fold8 ultra 접힘/펼침 실기 해결·사용자 디자인 수용/전체AC8로 승격하지 않는다. 새CPU/frame/GPU/전체48/idle계측0, 기존실패/threshold/hold보존.

같은 Draft/base dev PR361 정상push후바로link, canonical/PR 직접갱신. 최신 빌드 PR검증의 기존 시안보기/기존UI비교 버튼을 사용한다. 운영18765/기존57069 install/switch/restart0, 새delegate0, 전체확장/merge/deploy0. RootACK는 완료조건이 아니다.

Actual 동일 CODEX01a1bdce-8fa4-7a62-987e-129ff53a6d9f / T3 Controller ordinal29 running 읽기관측·hook/binding미관측·state/JSONL조작0. 승인effective787637c964b696d30e7840a0b93cd5f42dc000c31f5a5f7659ed23707f95c5e6 authorizedtrue/findings[] 확인. 원문과 실제 neutral/alpha 출처는 canonical에도 보존한다.
