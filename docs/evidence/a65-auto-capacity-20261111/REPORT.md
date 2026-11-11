# A-65 Auto 준비 조건·fixture 종료 순서 최소 수정

대상 실패: a92e1c3bea8c50363141d04bd71c654a5c073151 / CI42874906497 / artifact13792085468. 제품 변경 없이 테스트 두 파일만 수정했다. 수정 commit d46e44ab. 전체 실패 로그와 Root 회수 JSON, 원래 artifact를 함께 보존한다.

Auto 실패는 390/light Linux Chrome156.0.8078.4에서 복원 후 capacity=3인데 임시 초기값8 대비 증가를 요구했다. 기존 준비 조건은 canonical 응답 대기 중인 8행(pending/revalidating=true, budget=0)을 받아들였다. 제어 재현에서는 해당 응답을 보류해 이 상태와 기존 조건의 오판을 확인했다. 응답 해제·폰트 준비·현재 기하의 측정 capacity 일치 후에는 초기값3, 축소1, 복원3으로 정상 검증됐다. desktop Light/Dark는7→4→7, mobile Dark도3→1→3이다. Linux 최초 DOM/폰트의 모든 시간순서를 직접 관측했다는 뜻은 아니다.

초기 동기화만 강화했다. 원래 복원 증가·행수·viewport 적합 단언과 30초 timeout은 byte 동일하다(source-proof.json). 제품 Auto 및 모든 goassets/internal은 a92 대비 변경0이다. 제품 회귀로 단정하거나 growth 조건을 삭제하지 않았다.

같은 run의 neutral 실패는 별도 프로세스에서 route.fetch가 context 종료 뒤 계속된 unhandledRejection이었다. 원시 결과는16개 검증 완료를 기록했다. unrouteAll({behavior:'wait'})로 진행 중 route를 합류한 뒤 context를 종료한다. 제어 재현은 수정 전 closed-context 오류, 수정 후 응답1건 완료→route-finished→context-closed 순서를 확인했다. 오류 무시·고정 sleep·assertion 완화0.

검증: WORKFLOW_REQUIRE_BROWSER=1, 기존 Playwright1.64.1 / macOS Chromium156으로 dev-floating/neutral/review-entry named suite5/5 PASS. neutral16조건 및 System8, controlled Auto와 route lifetime 검증 포함. backlog_list_controls unit PASS. diff-check PASS. commands와 raw는 final-named.log, auto-unit.log 및 JSON에 보존했다. setup-error/client-cancel-assumption 로그는 테스트 개발 과정의 잘못된 설정·과도한 client 요구였으며 제품 결함으로 계산하지 않는다.

새 visual/full performance/Go/서비스·57069/삼성 실기 검증은 수행하지 않았다. 기존 성능 역사와 immutable idle26/27 FAIL 정책 hold, 사용자 디자인 불만·미수용·8AC 미체크를 유지한다. 현재 계약 verifier execution_authorized=false/legacy14 findings를 보존한다. 이번 명시 지시는 좁은 CI 진단·최소 테스트 수정이며 승인 상태를 조작하지 않았다. 원격 CI 성공은 별도 확인 대상이며 Root watch에 인계한다.

실제 Codex identity 01a1bdce-8fa4-7a62-987e-129ff53a6d9f, 기존 T3 Controller ordinal33 연속 실행. hook/binding 미관측; 과거 completed identity 재사용·수동 hook/state 생성0.
