# PR 검증에서 버튼으로 플로팅 시안·기존 UI 비교

사용자가 PR검증에 진입한 뒤 URL query를 수동으로 고쳐야 했던 불편을 보완했다. 실제 PR 전체 실행 배너와 standalone Preview 식별 배너 안에 **플로팅 시안 보기 / 기존 UI 비교** 버튼을 제공한다. 자동으로 일반 dev 기본 테마를 바꾸지 않는다. 검토자는 같은 화면에서 원하는 보기 버튼을 한 번 선택한다.

## 동작과 context 경계

`app.js`의 기존 /api/environment 결과(target.kind=pr, pr_number, head_sha) 또는 기존 /api/health의 검증된 Preview identity를 사용한다. 양의 PR번호·40자리 SHA가 확인될 때만 컨트롤을 만든다. 새 API/fetch/폴링/observer를 추가하지 않았다. 일반 dev나 미확인 identity에는 버튼을 표시하지 않는다.

버튼은 클릭 시점의 location.pathname/search/hash에서 floating 값만 설정하고, 시안 선택 시 background가 없으면 image를 추가한다. 기존 프로젝트·backlog·task·기타 query·hash를 유지한다. 초기 배너가 생성된 시점의 URL을 고정하지 않으므로 dev SPA 내비게이션 뒤에도 현재 화면을 대상으로 전환한다. 기존 pilot bootstrap의 session 선택으로 다음 내비게이션·새로고침에서도 비교 모드가 유지된다. 키보드 Enter/모바일 터치, aria-pressed, 최소44px 버튼과 wrapping을 제공한다. 새소재/IA/모달/배경 제어계를 만들지 않았다.

## source 경계

- 변경 source `a91a5f6694deb4360f27891b34e7dc9f121712fe`, 기반 `2bb105cdba180dbc98d6d2c009205243a1b3ecf9`.
- 소재 source3a066ed4의 기존 pilot CSS는 suffix 전체가 byte동일하다. 새 CSS는 배너의 진입 컨트롤 배치·44px 터치영역뿐이다. pilot.js/index/style/theme/select/사진/광학/terminal/P0 파일은 2bb와 그대로다.
- origin/dev app 전체는 두 배너 호출·범위가 표시된 컨트롤 함수/클릭 처리·초기화 binding을 제외하면 byte동일하다. 나머지6 핵심 dev 자산 byte동일 검증도 유지한다. 예전 ‘app 포함7파일 완전동일’ 문구는 초기 시안의 역사이며 현재 app에는 이 최소 진입 보완이 존재한다.
- production Go/API 변경0, 새로운 realtime굴절/mouse추적/rowblur/canvas/상시rAF0. 전체 UI 재작성/다른 테마 기본값 변경0. impeccable Operate/quieter/기존 craft floor를 적용했고 context는 같은 세션 기록을 재사용하여 재실행0.

## 새 CI 실패는 이전 Go 단언 실패와 다르다

555863bb의 CI42735770060·Preview42735769714는 TestHandlerServesDashboardAPIsAndAssets의 옛 native mount 단언으로 실패했고 2bb105cd에서 교정했다. 이 이력은 앞선 ci-repair 증거에 보존한다.

2bb105cd의 [CI42738529408](https://github.com/silverkhan/TaskMecca/actions/runs/42738529408)은 Go runtime job 안의 **Node tests/web_notification_center.test.cjs:58** ‘native packaged shell…’ 정적 검사가 현재 기본 index에 ui/main.js를 요구해 실패했다. Go 테스트/앞선 HTTP 단언이나 lifecycle 성능 gate의 새 실패라고 부르지 않는다. 같은 HEAD의 [Preview42738529409](https://github.com/silverkhan/TaskMecca/actions/runs/42738529409)는 직접 조회한 completed/success다.

이 정적 검사의 기존 native 메뉴/설정/중복attention 부재 단언은 동결 historical index와 실제 native shell 렌더러에서 그대로 수행한다. 별도로 실제 기본 dev의 app 엔트리·notifications 메뉴·pilot assets·native mount 부재를 강하게 검증한다. 단언 삭제/skip/기준완화가 아니다. 수정 전 로컬 실패와 원격 log-failed를 모두 보존했다.

## 수행 검증

- 실제 실패 CI의 동일 Node 검사 묶음 **190/190 PASS** (`ci-exact-node-final.tap`). PR/Preview/update/notification 관련 별도 묶음32/32 PASS.
- 신규 기능 browser1/1, 내부 **16조건**: normal dev /미확인 PR identity /실제형식 PR execution /standalone Preview × PC1440/mobile320 × Light/Dark. no manual URL edit, 실제 버튼 pointer/touch·PC Enter, 프로젝트·task·hash·추가query 보존, SPA 이동 뒤 현재 route 기반 비교, 선택/refresh 지속, ordinary dev unchanged, unconfirmed identity hidden, overflow0/pageerror0 PASS (`browser-confirm/review-entry.json`). 모두 격리 synthetic API이고 운영서비스나 실제 PR 전환을 실행한 것이 아니다.
- 초기 browser12조건도 PASS했으나 dev 응답 waiter를 응답 뒤에 만들던 불필요한 timeout 대기(131s)를 발견했다. 최종은 요청 전에 waiter를 준비해 검증 의미를 유지하고16조건 약12s에 완료했다. 초기/최종 결과와 cleanup 모두 보존했다. 새로운 제품 시각 batch/스크린샷/성능 측정은 수행하지 않았다.
- dev 소스 정적 보호1 PASS + 환경 없는 기존 browser1 SKIP은 `dev-source-static.tap`에 구분한다. SKIP을 PASS에 더하지 않는다. 초기 국소 검사에서 전역 click binding을 VM 정의 영역에 두어 fixture document에 addEventListener가 없던 실패는 기존 bootstrap 이후로 binding을 옮겨 교정했다. initial raw 보존, 해당 단언/fixture를 약화하지 않았다.
- safe named Go3개(TestHandlerServesDashboardAPIsAndAssets/SignatureThemeBranding/OverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly) PASS. t.TempDir 및 기존 isolated registry/mock 조건만 사용하고 unprotected go test ./.../uninstall fixture0.

기존 full48/idle 정책·threshold는 수정하지 않았다. 새 fullgate/CPU·frame/GPU/메모리 재측정0. 삼성브라우저 - z fold8 ultra 접힘/펼침 실기는 계속 미검증이며 체감 랙 해결·사용자 시안 수용·전체 AC8 완료를 주장하지 않는다. 사용자 시안 수용 전 전체 확장 금지, 기존 idle 정책hold 유지.

## 외부 게시와 운영

동일 Draft/base dev PR361 normalpush 후 바로 link한다. 최종 HEAD/CI·Preview snapshot/댓글은 canonical review-entry-20261110/external-final.json과 PR에 기록한다. 새 HEAD의 PR검증 빌드를 선택하면 배너 버튼으로 시안·dev 비교가 가능하다. 기존 실행 중인 2bb 인스턴스에 새 버튼이 이미 나타났다고 주장하지 않는다. 운영18765/기존57069 install/switch/restart0, 임의 visualserver0, 새delegate/reviewer0, merge/deploy0. Root 기존 watch가 CI를 이어받고 childwatch/poll0. Root ACK는 완료조건이 아니다.

actual 동일 CODEX01a1bdce-8fa4-7a62-987e-129ff53a6d9f / T3 Controller ordinal27 running 관측. 이전 completed run 조작0, hook/binding미관측·합성runtime기록0. 승인effective787637c964b696d30e7840a0b93cd5f42dc000c31f5a5f7659ed23707f95c5e6 authorizedtrue/findings[] 확인. 사용자 원문과 선택 근거는 USER-FEEDBACK.md에 보존한다.
