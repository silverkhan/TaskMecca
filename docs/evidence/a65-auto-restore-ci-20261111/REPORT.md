# A-65 Auto 높이 복원 CI 실패: 진단 근거 보강, 원인 미확정

기준 HEAD e4918b61a01999f9ec67c57b70aa203323e41b29, 진단 source ae08104209f3c5c325c2062470f636844a597a8a. 이번 변경은 tests/a65_dev_floating_pilot.test.cjs의 실패 경로뿐이다. 제품·시안·기존 단언은 변경하지 않았다. **CI 실패 해결 완료가 아니다.**

## 실제 외부 실패

CI42791585728 Unified workflow browser regression에서 Auto 높이600→844 복원 predicate가30초 후 timeout. [실패 로그](ci-failed.log), 원본 [CI artifact](ci-artifact/dev-floating-mechanics.json)를 보존했다. artifact cases=[]라 첫1440/light 케이스의 어느 하위조건이 미충족인지 알 수 없다. neutral/진입 검사 성공은 Auto 통과가 아니며 앞선 Go fixture cleanup 실패와 별개다. 당시 Go runtime은 성공했다.

## 소스와 진단

실제 dev updateAutoListPageSize는 canonical/revalidation이 끝난 뒤 list의 page-top 좌표·shortcut높이·12px 여유로 available을 계산한다. 같은 project/filter/width에서 첫5행 중앙값의 최대 rowHeightBudget을 유지하고 floor(available/budget), 최소1·최대50으로 정한다. 높이 변경 때 보수적 budget을 초기화하지 않으므로 최초 행수의 정확한 복귀는 계약이 아니다. 기존 테스트도 정확한 최초크기 대신 증가·최종fit을 검사한다.

현재 Auto controller 구간은 최신 origin/dev(f0a732e3)의 원문과 byte동일이다. 이것만으로 pilot CSS의 실제 geometry 회귀를 배제할 수는 없다. 복원 predicate의 auto/settled/growth/rowcount/last-fit 중 무엇이 실패했는지 CI artifact에는 없다. 네트워크 race/행높이 추정/폰트·플랫폼/스크롤 중 어느 원인도 확정하지 않았다.

Mac Chrome157 대상기능1회 PASS(4 viewport/theme cases), CI와 같은 Playwright1.64.1의 headless shell156 macOS 대상기능1회 PASS. Linux CI 자체를 재현한 결과는 아니다. 최종 수정은 같은 headless shell에서 named 파일2/2 PASS, 기존 backlog_list_controls 단위 검사 PASS. browser version을 맞추기 위한 isolated Playwright browser 설치만 했으며 서비스 설치/재시작은 없다. named 실험 이유가 다른 두 browser 비교이며 blind rerun/전체suite 반복이 아니다. [최종 결과](final-named.log), [기존 Auto 단위검사](auto-controller-unit.log).

## 변경과 다음 판별

기존 waitForFunction predicate·30초 timeout·600→844 시퀀스를 그대로 둔 채 try/catch로 감쌌다. 실패하면 auto-restore-failure.json에 browser/platform, autoBefore/autoSmall, Auto size·mode·pending/revalidating, 모든 행높이·수·lastBottom·listTop·scrollY·viewport·shortcutHeight·rowBudget·available/capacity, 각 기존 하위조건의 참/거짓, 최근12개 실제 list요청을 기록하고 **동일한 원래 오류를 다시 throw**한다. 추가 wait/sleep/skip/강제클릭/단언완화·제품변경이 없다. 기존 CI always-upload가 이 파일을 보존한다. [predicate/hash 원문 보존](source-proof.json).

다음 정확HEAD Linux CI가 실패하면 이 artifact로 원인을 좁혀 최소 교정할 수 있다. 성공하더라도 현재 실패 원인이 규명·해결됐다고 승격하지 않는다. 새 디자인/시각라운드/fullperf/Go전체검사/외부review delegate/새delegate0. 삼성실기·운영18765/57069·merge/deploy0.

원본 decision 검증은 approval_required/execution_authorized=false/LEGACY_CHECK_STATE_UNVERIFIED14건이며 canonical에 원문 보존했다. 사용자 최신 명시 지시의 기존CI 원인진단·안전한 최소기록 범위만 수행했고 승인/Hook/상태 합성·정책해제0. “이정도면 새UI포기해야하나”라는 품질 불만과 Root 중단 권고를 실제 취소/폐기/PR close 승인으로 해석하지 않는다. Draft/hold/8AC미체크/전체확장·사용자수용 대기 유지. 새 CI/Preview 대기는 Root 기존watch에 직접 인계하고 childwatch/poll0.
