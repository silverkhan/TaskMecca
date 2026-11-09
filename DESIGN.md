---
name: Task Mecca — Accepted Transmissive Glass
description: Existing Task Mecca workflows in the user-accepted f3beec75 material.
---
# 수용된 재질과 기존 업무 흐름

사용자는 2026-11-09 f3beec75 시안에 대해 “좋습니다. 시안과 같이 진행해봅시다. 지금까지중 가장 원하던 품질에 가깝습니다.”라고 승인했다. `material-accepted-expand-existing-ux.md`가 현재 방향의 권위다. 과거 Smoke core/aa5의 5개 메뉴, 과거 finish verdict, SHIP 문서를 새로운 시각 정답으로 사용하지 않는다. 과거 원시 자료는 보존한다.

첫 화면은 Global Hub다. SYSTEM / BACKLOGS / OPERATIONS / HELP 그룹과 열린 프로젝트 직접 진입을 보존한다. PC와 모바일은 같은 업무와 전용 상세 화면을 사용한다. 모바일 보조 도구는 접을 수 있으나 제거하지 않는다. 원래 logo.svg, 한영, Light/Dark/System 및 저장된 설정을 유지한다.

공통 표면은 accepted `ui/material/material.css`, `background-comparison.css`, `transmissive.js`, `proximity.js`다. 이 원본 파일은 확대 작업에서 변경하지 않았다. `expansion.css`와 `expansion.js`가 실제 셸/목록/상세/관리 화면을 연결한다. rail 32px, 읽기 면 약30px의 곡률, 비대칭 rim, 내부 두께와 접지 그림자, 7px photo sampling blur, 공간 edge SVG map, 입력이 있을 때만 갱신하는 근접광을 공유한다. 작은 필드는16px, 메뉴·도구는 그 크기에 맞춰 구성한다. Pane의 실제 캡처는 형태와 깊이를 읽는 참조이며 데모의 숫자/사용자/타이머는 Task Mecca 기능이 아니다.

배경은 고정된 로컬 사진 또는 단색이다. 투과층은 **로컬 사진의 viewport 좌표를 각 면에 샘플링**한다. 임의의 뒤 DOM을 굴절하지 않는다. Chromium의 SVG photo distortion을 enhancement로 사용하고 다른 엔진에서는 7px blur·투과·rim을 유지한다. Safari/Firefox 실물 검증을 하지 않았다면 지원 선언과 실측을 구별한다. solid에서는 sampling layer를 숨기고 같은 rim/두께를 사용한다.

텍스트와 SVG 아이콘은 장식층의 형제이며 blur/displacement/filter/text-shadow/stroke 처리하지 않는다. dense backlog/Gantt/log의 각 행에는 blur/canvas/왜곡을 넣지 않는다. 큰 읽기 영역 한 면으로 묶고 행 구분·여백·상태 텍스트로 정보를 전달한다. 근접광은 기존 입력 batch이며 유휴 optical engine이나 continuous rAF를 추가하지 않는다. 고비용 합성의 실제 측정은 같은 환경 origin/dev와 8쌍/48scenario gate로 별도 검증한다.

버튼·필드·공통 combo·표·header·menu·evidence popover·native dialog는 같은 색/곡률/깊이를 사용한다. select는 기존 `select-ui.js`의 native value/change 모델과 keyboard listbox를 재사용한다. 팝업은 viewport 경계를 지키며 Escape/Tab/포커스 복귀를 유지한다. 사진 굴절을 표의 행마다 늘리지 않는다. 이미지가 정보를 대체하지 않는다.

프로젝트/부분 실패/unknown/stale/unmeasured는 0과 구별한다. 관리 API 응답 성공, 설치 요청 접수, 실제 실행 서버 전환, 기기의 알림 표시/읽음은 서로 다른 사실이다. PR identity와 Dev 복귀는 기존 P0 controller를 사용한다. 관리 쓰기 fixture 검증은 운영 실행의 증거가 아니다.

원본 비교와 화면 대응은 [기능 대응표](docs/a65-feature-map.md), 후보 검증·캡처·성능·잔여는 [Worker 보고서](handoff-evidence/accepted-expansion/REPORT.md)에 기록한다. Controller의 최종 실물 독립 검수와 사용자 최종 검토를 대신하지 않는다.
