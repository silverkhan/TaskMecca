# Reviewer 공통 인터페이스 (A-90 소유)

상태: A-90 구현 계약. A-91 연속성/CLM 및 A-92 화면 구현 완료를 뜻하지 않는다.

`rolepolicy.ReviewRequirement`는 실제 영향도(low/moderate/high/unknown), 복잡성(simple/complex/unknown), 가역성(easy/difficult/unknown), 검증 가능성(established/limited/unknown), 실제 동작 변경, 명시적 필수 요구, 신뢰 경계/금융 무결성/파괴적 영향, 이전 지적사항을 구조화한다. `Decide()`는 required/recommended/optional과 이유 코드를 반환한다. 파일 수·역할 이름·자유문구만으로 결정하지 않는다. explicit/high-risk 필수는 생략할 수 없다. 권장/선택 생략에는 Controller 사유가 필요하다. 어떤 결정도 Reviewer를 자동 생성하지 않는다.

`rolepolicy.ReviewRequest`는 task_id, assignment_id, contract_sha256, requirement, 완전한 source.files, implementation/reviewer 실제 신원, report_file을 연결한다. continuity_ref/context_ref는 A-91 소유의 선택적 근거 참조이며 사용률/차수/교체 성공을 추정하지 않는다. CLI는 외부 MCP 생성·권한 수행을 하지 않는다.

정식 역할은 reviewer, 관리 경로는 `/root/controller/reviewer/<name>`, 공식 이름은 `DMMDD_NN_Reviewer_<name>_G<n>`이다. G는 세션 세대이며 검수 차수가 아니다. provider native ID, opaque T3 thread, 관리 경로, 공식 이름은 서로 대체하지 않는다. 같은 native의 다른 turn/역할은 독립 검수가 아니다. 과거 제품 구현 native는 새 역할로 자기검수할 수 없다.

화면 소비 상태: omitted(정책상 생략), unassigned, requested, in_progress, needs_changes, accepted, unknown. necessity와 actual review outcome은 별도다. running/START/요청만으로 accepted를 만들지 않는다. source/report/contract/실제 신원 결속이 확인된 수용만 독립 수용으로 표시한다. 소스 변경 후 이전 accepted는 새 소스에 복사하지 않는다.

과거 Worker 검수의 저장된 역할·공식 이름·시각과 기존 승인·검수 gate를 보존한다. 새 metadata가 없으면 기존 경로 그대로 동작한다. optional 생략은 independent accepted가 아니다. 미측정 context/초기 Activity interval은 unknown/unmeasured이며 보간하지 않는다. T3 새 검수/재검수는 새 managed delegate를 사용하고 동일 세션 메시지를 새 검수로 주장하지 않는다.
