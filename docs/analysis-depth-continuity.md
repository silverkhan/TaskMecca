# 작업에 맞는 분석 범위와 재개

`flow next --role worker|controller --event analysis_requested --task A-94 --analysis-input FILE --json`은 읽기 전용 advisory다. `execution_authorized=false`, `written=false`를 항상 반환한다. 실제 배정, 현재 native/attempt 결속, fresh 권한·strict gate, 필수 독립검수, 원계약 완료를 대체하지 않는다. A-91/A-92/A-93 구현·수용·측정 완료도 의미하지 않는다.

원계약/확정 AC, 실제 변경 source/hash, 영향도·복잡성·가역성·검증 가능성, 기존 분석과 미해결 문제를 먼저 확인한다. `risk`는 A-90 `rolepolicy.ReviewRisk` 구조를 재사용한다. 단순하고 영향이 작은 작업은 최소 확인 후 구현한다. 복잡 참조는 필요한 의존성과 질문으로 범위를 제한한다. 삭제·금융/중요 데이터·신뢰 경계·높은 영향·되돌리기 어려운 변경은 `safety_checks`를 명시하고 해결할 때까지 필수 조사를 유지한다. unknown은 낮은 위험으로 간주하지 않는다.

분석 종료는 계약/AC 이해, 필요한 안전·작업 확인의 근거 해결, 남은 불확실성과 다음 구현/검증 행동의 명확화로 판단한다. 파일 수나 고정 분 단위는 종료 기준이 아니다. 진행 제안은 caller가 명시한 근거에 한정되며 실제 구현 허가나 전체 계약 충족 판정이 아니다.

## 입력과 재사용

`current`는 `task`, SHA-256 `contract_sha256`, `acceptance_sha256`, `sources`(경로→해시), 명시적 `decisions`·`open_issues` 배열, `context_id`를 가진다. 생략된 배열, unknown context, 비정상 해시는 유효 분석의 증명이 아니다. `required_checks`, `safety_checks`, 실제 `completed_checks`를 제공한다. `prior`는 같은 context 구조와 risk, 실제 분석 `artifact`, `resolved_checks`를 가진다. 호출자는 현재 계약·파일 해시·분석 산출물·기록을 읽어 정확히 대조해야 한다. JSON 입력만으로 실제 근거를 인증하지 않는다.

task/계약/AC/context/이전 결정/미해결 문제/risk와 모든 source hash가 같으면 exact 재사용한다. source만 변하면 추가·삭제·변경 경로를 `changed_sources_only`로 반환하며 이전 완료 체크를 자동 재사용하지 않는다. 기존 분석은 참고로 유지하고 변경분과 영향받는 결정/체크를 재검토한다. 계약·context 교체나 불일치·오래된 hash에는 재사용을 승인하지 않는다. 현재 지침/권한/실제 dispatch gate는 항상 fresh로 확인한다.

## 원인 구분과 현재 경계

`evidence`에 원계약 `contract_ref`, 실제 산출물 `artifact_ref`, 조사 참조 `reference_ref`, 원래 `phase`, `purpose`, 대조 결과 `finding`을 연결한다. 필요한 안전 조사, 중복 확인, 재개 복구 비용, 요구사항 이해, 자료 조사를 구분한다. requirements의 자료조사 의심은 별도 분류하되 원 phase·시간을 바꾸지 않는다. 근거가 부족하면 `insufficient_evidence`다. 분류는 제출된 근거에 대한 advisory이며 과잉분석 자동 판정이 아니다.

실제 현재 경계에서 requirements / research / implementation / verification / actual_wait / coordination_handoff를 기존 지원 API로 기록한다. 실제 대기는 원인·담당·재개 조건을 보존한다. 전환/종료 누락과 unknown phase는 collection gap으로 남기고 과거 시간을 재작성하거나 미측정을 활동으로 채우지 않는다. 이 CLI는 Activity를 생성하지 않는다.

기대 범위와 진행의 차이는 `drift`의 원인 `cause`, 현재 산출물 `current_artifact`, 남은 확인 `remaining_checks`, 다음 행동 `next_action`으로 Controller에게 직접 조율한다. 경과시간만으로 사용자 개입/정체 알림으로 승격하지 않는다. 새 poll·heartbeat·토큰 추정을 추가하지 않는다.

## B651과 검증 한계

B651 audit의 requirements 932901ms(15분33초), 제안서 21대상/7933 메타데이터 참조는 원계약·실제 산출물·참조·phase 대조의 출발점이다. 숫자만으로 비효율·난도를 확정하지 않는다. 안전/계보 조사 필요, 반복 확인·재개 비용, 요구사항과 자료조사의 오분류, 전환/종료 누락을 각각 근거에 연결한다. 미측정 Controller/Registrar와 열린 구현은 확정 측정과 분리한다. 동등 범위 전후 실측 없이는 효율 개선 수치를 주장하지 않는다.

선택 fixture: 작은 수정, 복잡 참조, exact 재개, 계약/AC 변경, stale/hash 불일치, context unknown/교체, risk 변경, phase gap, 불완전 drift, B651 숫자만 있는 근거. 실제 현재 maintenance는 현재 source hash와 선택 테스트 산출물을 별도로 보존한다. fixture 통과는 실제 독립검수·Controller 수용·CI 릴리스가 아니다.
