# A-65 실사용 피드백 및 보완 우선순위

## 원문과 실행 경계

Root 전달 사용자 원문: “UI구현된 사항 PR검증모드 진입해서 확인했는데 1. 랙이 심하다 2. 전체적으로 뭔가 정돈되지 않은 느낌이라 정신이없다”.

추가 명시 지시: “$impeccable 스킬을 같이 활용해 검수하면서 개선하기를 바란다.”

최신 핵심 기준 원문: “유념할 점은 지금 GUI THEME이 깔끔하고 간결한 성공 사례라는점이다”. 기존 GUI THEME의 깔끔함·간결함·정보 위계·컴포넌트 밀도·경계 및 여백의 절제가 incumbent visual authority다. 같은 화면·같은 내용을 비교하여 중복 패널/윤곽선/광택/효과 계층을 줄인다. 새 IA/장식 세계를 만들지 않는다. 이번 명확화는 실행범위 변경/hold 해제/기준완화가 아니다.

2026-11-09 Controller ordinal22에서 원격 PR361 HEAD `27dbe42e4fdb45be36ac660ac80ae09a4278fbc2`, Draft/open/base dev 확인. 이 HEAD의 production은 e646f475 교정과 동일하다. 사용자 검증모드의 실제 실행 binary/source SHA는 이번 전달에 없으므로 동일 HEAD라고 단정하지 않는다. 원래 57069는 f3 작은 시안이고 전체 후보 표시 주소가 아니다. 실사용 랙은 재현·원인분리가 필요한 독립적인 사용자 보고로 보존한다.

완료/전체 디자인 수용이 아니다. 10% 성능 기준·immutable idle 판정·기존 정책 hold를 해제하거나 API 예외로 해석하지 않는다. hold는 계약 충돌 판단만 제한하며 안전한 구현 개선을 중단하는 조건이 아니다. 기존 메뉴·위치·기능·내용·밀도와 승인된 곡률/투과/blur/rim을 유지한다. 새 외부 reviewer/Worker dispatch, 운영18765·57069 변경, merge/deploy 없음.

## 기존 근거와 현재 코드에서 좁힌 요소

- e215 gate46/48에서 scroll TaskDuration +32.5589%/+11.2783%. e646 native backdrop 교정 뒤 scroll은 통과했으나 1440 Light navigation +11.0531%로 전체47/48 FAIL. 카운터만으로 사용자 환경의 GPU/raster/입력 병목을 확정할 수 없다.
- e646 idle26/27 FAIL. settings 필수 초기3GET와 all-API-zero 판정 충돌은 별도 유지. 이를 사용자 랙 전체의 원인으로 주장하지 않는다.
- expansion.js는 body subtree의 모든 childList 변화마다 전체 surface 후보를 검색하고 optics.refresh 및 proximity.refresh를 호출한다. optics.js도 subtree 전체를 다시 검색한다. 광학층 자체 삽입과 일반 텍스트/로그 갱신까지 이 경로로 들어가므로 반복 갱신 범위를 측정·축소할 근거가 있다.
- 모든 크기의 heading/summary/section/control 묶음에 별개 곡면·rim·깊이·반사광을 부여하면 주 작업면과 보조 정보가 같은 시각적 무게를 갖는다. 중첩면의 사진 재샘플링은 부모 유리 안에서 배경 윤곽을 다시 강조한다. 코드와 기존 캡처에 근거한 복잡함 가설이며 새 실물 검수 완료 주장이 아니다.
- expansion.css의 depth/ground 반복, active capsule gradient/shadow, filters/detail-tabs의 테두리·inset, 섹션별24~26px·필터12~16px·모바일 별도 override가 겹친다. 주요 정렬선과 정보 계층을 우선하고 같은 그룹 내 중복 장식은 줄일 후보이다.

## 보완 방향과 검증 경계

1. 주요 shell/toolbar/작업면의 승인된 광학을 유지한다. 중첩 보조면은 부모의 투과를 공유하고 낮은 알파 tint와 절제된 rim으로 구분하여 겹친 사진/왜곡/깊이를 줄인다. 완전 opaque 또는 행별 blur로 대체하지 않는다.
2. 광학·근접광 registry는 실제 표면 추가/제거에만 갱신한다. 로그/텍스트/광학 장식의 자체 DOM 변경에는 전체 재탐색하지 않는다. class만 추가되는 readonly 표면은 명시 refresh 연결을 보존한다. 새 화면의 필수 초기 API는 삭제하지 않는다.
3. 같은 그룹의 경계/반사광은 한 번만, 선택/포커스와 주요 작업면에 시각적 우선순위를 준다. 메뉴그룹·정보량을 숨기거나 재배치하지 않고 여백·정렬·구분선 강도를 통일한다.
4. Impeccable Operate의 quieter+optimize 적용. 이번 세션 context 실행1회, quieter/optimize 및 craft-floor 읽기. DESIGN 임의 수정 없음. PC/mobile Light/Dark 한 배치 검수→발견 일괄수정→최대1 confirm. 전후 scroll/input/frame·CPU 측정은 scoped 근거이며 원래48group/8pair gate의 대체가 아니다.

현재 새 UI 보완·전후측정·실물검수는 아직 미완료. 기존 실제 캡처의 source12fb 경계, Safari/Firefox/Samsung 실기·전체 GPU memory/energy 미실행도 유지한다.

## 외부 상태 및 신원

최신 확인: CI42584066255 Go runtime FAIL, browser regression 등 나머지 표시 job PASS. Preview42584066261 build PASS. 이전 pending 기록을 이 관측으로 보완하며 CI 재실행/통과 재분류 없음.

동일 T3 Controller actual ordinal22/full-access/default/gpt-6.1-sol. hook/binding을 새로 관측하지 않았고 수동 조작하지 않았다. Root steer는 no-running-turn으로 거절되어 auto 직접 전달 수락·started 확인(처리 완료와 구별).
