# A-65 전체 UX 연결 후보 — 기술 검증 보류

2026-11-09. 이 후보는 **merge-ready·기술완료·최종 디자인 검토본이 아니다**. 전체 A-65 AC 8개는 미체크다.

사용자 승인: [수용 재질의 기존 UX 전체 적용](https://github.com/silverkhan/TaskMecca/pull/361#issuecomment-6732301992). 기존 UX의 메뉴 그룹·Global Hub·사이드바 프로젝트 직접 진입을 유지하며, 백로그 검색/필터/정렬/페이지/복원, 전용 상세와 라이프사이클, 프로젝트/저장소/워크로드/운영/알림/터미널/도움말/업데이트/Preview 관리 기능을 네이티브 모듈에 연결한 후보다. [기능 대응표](../../../a65-feature-map.md), [구현 보고서](implementation-report.md) 참조.

## 현재 소스와 판정

- 전체 구현 원본 `e21594e8b4cc1484b76b51258ffbbfeb68771468`, 성능 교정 원본 `e646f475870aa1dbfc4c4f43d88fcaa317768183`.
- 통합 소스 `d4cec02886ac202b07341029adc84ae31675378b`의 production/internal/tests는 e646과 바이트 동일하다. 기존 primary merge는 가져오지 않고 구현 커밋만 직렬 cherry-pick했다.
- 전체 8쌍/48그룹: **47 PASS / 1 FAIL**, invalid 0. 모든 scroll 그룹은 통과했다. 남은 1440px Light navigation TaskDuration은 `74.4315 → 82.6585ms`, **+11.0531%로 10% 기준 초과**다.
- idle은 **26/27 FAIL**. `pendingStopsOnRoute`가 settings로 이동한 뒤의 필수 초기 조회 세 건을 실패로 센다. 아래 충돌은 통과로 재분류하지 않았다.
- 원시·manifest·고정 프로토콜·binary provenance·cleanup·판정은 [performance](performance/)에 있다. 숫자·기준·assertion·하네스를 바꾸지 않았다. 동일 소스 full rerun도 하지 않았다.

## 재질의 실제 동작

첫 계층 표면은 Chromium의 native backdrop `blur(7px)`와 SVG edge displacement 22를 사용한다. 중앙 RGB 변화 0과 가장자리 국소 변화, 뒤 DOM 교체에 따른 출력 변화를 분리해 확인했다. 따라서 이 경로는 f3 작은 시안의 사진 복제 방식과 달리 **실제 뒤쪽 DOM을 샘플링**한다. 중첩 면은 부모의 글자까지 재차 흐리지 않도록 기존 사진 샘플링을 유지한다. 미지원/비Chromium 엔진은 투과·흐림·rim 대체 표현이며 실제 Safari/Firefox 실측은 아니다. [광학 원시 증거](optics/pixel-proof.json), [4개 viewport/theme 검사](optics/result.json).

원본 사진·f3 material 5파일·P0 environment.js·b326 workflow_native.cjs·API/Go는 교정 전후 동일하다. 전경 텍스트·아이콘에 그림자/스트로크/필터를 추가하지 않았다. CPU 픽셀 엔진·canvas·행별 blur·상시 rAF는 없다.

## 회귀와 캡처의 경계

- 교정 소스의 core/expansion/P0/workflow 실제 브라우저 **11/11**, unit **66/66**, scoped optics **4/4** 통과. 하네스의 합성 API 검증을 운영 백엔드나 물리기기 검증으로 확장하지 않는다.
- 전체 UX의 150-case/190-PNG 행렬은 실제 owned Go 합성 서버의 `12fb5dd9` 소스다. [PC/모바일 Light/Dark 주요 화면](screens/)은 그 캡처의 일부다. 이후 Auto·메뉴 hit·native optics 교정은 각 focused 검사로 검증했으며, 이 이미지를 e646의 exact 전체 캡처라고 하지 않는다. [파일별 출처](screenshots-source.json).
- Samsung 실기, Safari/Firefox, native OS zoom 200%, 스크린리더, 외부 Push/Telegram, 운영 설치/실제 PR·채널 전환, 전체 CPU/GPU 메모리·에너지는 미검증이다.
- f3/e7/a41 원본 및 모든 실패 실험·원시 자료는 canonical evidence와 원래 Worker 커밋에 보존했다. 전체 원시 약 470MB를 이 공개 요약과 혼동하지 않는다.

## 검증 기준 충돌 — Root 판단 필요

고정 `idle-verdict.cjs`의 `pendingStopsOnRoute`는 settings의 `page.load` 이후 **모든 `/api/` 요청 0개**를 요구한다. settings는 등록 프로젝트를 먼저 검증한 다음 `/api/version`, `/api/runtime/hooks`, `/api/channel-options`를 각 1회 조회한다. 이것은 새 화면의 정상 초기 조회다.

기존 광학/e646/observer 대조 각 4회, 총 12회 모두 세 초기 GET이 발생했다. 이전 화면의 `/api/overview` 재요청과 SSE는 모두 0이었다. 필수 기능·프로젝트 경로 검증을 제거하거나, readiness/assertion을 임의 변경해 통과시키지 않았다. [상세 분석](pairi-ANALYSIS.md), [원래 실패 판정](performance/idle-verdict.json).

Root가 이 판정의 의도와 settings 필수 초기 조회의 관계를 확정해야 한다. 그 결정과 별개로 navigation 성능 실패의 교정은 남는다. **10% 기준 완화나 성능 면제를 요청하는 것이 아니다.**

기존 57069는 수용된 f3 작은 시안을 그대로 유지한다. 이 전체 후보로 바꾸지 않았다. Draft PR만 갱신하며 Dev/Main/Stable merge·배포·운영18765 설치/전환/restart는 없다. 검증 기준이 정리되고 남은 성능 교정·최종 독립 검수가 끝나기 전에는 최종 사용자 디자인 수용을 요청하지 않는다.
