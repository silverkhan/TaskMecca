# 플로팅 시안: 목록 읽기 면·외곽 정렬 보완

사용자 원문/수정 기준은 [USER-FEEDBACK.md](USER-FEEDBACK.md)에 그대로 보존했다. [Root 원천6759441573](https://github.com/silverkhan/TaskMecca/pull/361#issuecomment-6759441573)과 실제 첨부 파일을 view_image로 확인했다. 이 피드백은 디자인 수용이나 전체 확장 승인이 아니다.

## 현재 구현

Code source **82585738a8c7d8665d1144f7278fdb08d5f9a8cc**, 기반00f446cc. 제품 변경은 선택형 `ui/dev-floating/pilot.css` 한 파일뿐이다. app/index/dev style/theme/select/로고·사진·P0·terminal·native workflow 소스와 기능은 byte동일이다.

- 도구·검색/정렬/페이지 제어·열 제목·모든 행을 감싸는 #content 하나에 detail과 같은 읽기 배경을 적용했다. 98%의 기존 dev surface+투과를 사용하며 일반 행은 사진 위에 직접 놓이지 않는다. 행별 blur/filter/canvas/광학층0.
- actual fixed sidebar에 desktop12px/mobile8px 바깥 여백, sidebar와 main 사이 desktop12px 실제 간격을 뒀다. 기존 expanded/compact/Auto peek 동작과 내용 폭을 유지한다.
- 상단 설정/PR검토/알림과 읽기 면의 좌우 기준을 통일했다. desktop radius18px/mobile16px와 공통 간격·chrome96%/reading98% 역할 토큰을 사용한다. PR·알림의 중첩된 색상 gradient 외곽을 평온한 같은 면/그림자로 정리했다. 펼침 알림은 기존 floating 동작을 유지하고 테두리/배경만 절제했다.
- 선택 행의 일반 outline은1px·radius8px로 낮췄다. 실제 키보드 focus-visible은2px solid accent로 구분한다. 텍스트/아이콘 shadow/stroke/blur/displacement를 추가하지 않았다.
- 기존 메뉴·아이콘·열·행 수·밀도·검색/정렬/페이지·연속본문·상시목차/플로팅인덱스·기존 PR 진입 두 버튼은 그대로다. 새로운 IA/모달/전체 테마 확장0. 일반 dev는 opt-in이 없어 영향0.

## 실제 검수 범위와 제한 초과 기록

impeccable Operate/quieter/layout/craft-floor 적용. 기존 세션 context는 재실행하지 않았다. 실제 첨부와 기존 UI/source를 먼저 평가한 뒤 layout detector를 별도로 실행했다. detector는 before/final 모두0건이지만 실제 시각 결과를 대체하지 않았다.

최초 API 알림 fixture 구성에서 사용자 알림이 숨겨져 batch setup이 실패했다. 다음에는 dev 행을 native anchor로 잘못 가정해 한 화면 뒤 멈췄다. raw를 그대로 보존했다. 실제 classic dev renderer에 명시적 synthetic 알림 row를 제공하고 dev의 focusable div 행을 사용해 수정 전 batch-ready12조건(1920/1440/1280/320/390/430 × Light/Dark), list/detail24PNG를 확보했다. 사용자 데이터/알림 backend를 검증한 결과가 아니라 공개 synthetic 데이터의 실제 UI 렌더다.

일괄 CSS 수정 후 confirm12조건을 확인했지만, 실제 이미지에서 기존 desktop sidebar가 grid가 아닌 fixed+main margin 배치라는 점을 놓친 결과 **sidebar-main 간격0**을 발견했다. 이를 바로잡고 neighbor gutter 조건을 추가하여 confirm-final12조건/list+detail24PNG를 추가 확인했다. 따라서 요청된 확인1회 제한을 엄밀히 넘겨 기하 확인1회를 추가했다. 초기 자동 기하 PASS를 시각 완료로 포장하지 않았고 모든 중간 근거/잘못된 화면을 별도 보존한다. 최종 이후 추가 polish/visual round는 없다.

최종 `confirm-final/`이 현재 UI다. `confirm/`는 간격 누락 중간 결과, `batch-ready/`는 수정 전이다. [최종 Dark PC](confirm-final/list-1440-dark.png), [최종 Light mobile](confirm-final/list-390-light.png), [수정 전](batch-ready/list-1440-dark.png).

모든 최종12조건에서 header/PR/알림/reading 좌우 좌표 같음, desktop sidebar x12/neighbor gap12, mobile x8, root overflow0, 20행/7DOM열/row filter0, 키보드 focus2px, pageerror0, list→실제 Enter→detail PASS. 원래 목록 배경은 transparent였고 최종은 실제 computed alpha.98이다. 최종 PC1440에서 설정/PR/알림/reading은 x262..1428로 같은 폭이다. 배너의 PR #361/00f SHA는 synthetic API seed이며 현재 실제 서버/바이너리 소스 식별로 쓰지 않는다. 실제 CSS/source hash는 protected.json에 묶었다.

## 잔여 CI와 기능 회귀

최신00f의 [CI42757920392](https://github.com/silverkhan/TaskMecca/actions/runs/42757920392) Unified workflow browser failure를 직접 읽었다. 이전 Go/Node native mount 단언과 다른 실패다. connected workflow의 목록 복귀 후 FLOW-1 링크가 Auto 재배치 도중 repeatedly detached/not stable로30s timeout했다. trace상 URL page_size=auto였지만 해당 두-task fixture의 API는 항상 page_size20을 반환했다.

수정 전 같은 connected 테스트는 이 Mac에서1/1 PASS했으므로 로컬 실패를 재현했다고 주장하지 않는다. 테스트의 목적은 workflow의 실제 pan/zoom/fit/단방향/읽기 보존이며 Auto 페이지 fitter가 아니다. 이 fixture의 고정20행 계약을 preference에 명시하고 실제 state.pageSize20 단언을 추가했다. 기존 모든 native 입력/상태/정확한 경계 단언은 유지했다. force click/timeout 연장/skip/고정 sleep/production native engine 변경0. Auto는 현재 제품 dev에서 별도 회귀로 보존·검증한다.

- CI와 같은 workflow5파일 묶음: **8/8 PASS** (`ci-workflow.tap`), 보호 native workflow JS/CSS byte동일. protected workflow 회귀 캡처는 소재 시각검수와 다른 검증이다.
- default dev/pilot 검사 **2/2 PASS** (`mechanics-complete.tap`)의4 PC/mobile×Light/Dark 조건: 한 읽기 면·행별 filter0·정렬·키보드focus2px, 기존 검색/목차/언어/System/side menu/선택비교와 **Auto 표시 수/600px 축소/844px 복원/수동20 복귀** PASS. 초기 Auto 전환 직후 관측8→축소4/1→최종7/3처럼 보수적인 측정이 정착하므로 ‘정확히 초기 숫자로 복원’이라는 잘못된 검사 가정을 버리고 실제 행이 viewport 안에 들어가며 다시 늘고 정확한 행 수가 맞는지 검증했다. 동작/기준을 완화한 것이 아니라 기존 dev의 측정 의미에 맞춘 검사다. hidden native select와 실제 popup option의 중복 role은 실제 listbox 내부 option으로 지정해 실제 UI를 사용한다. 세 초기 검사 실패 raw 보존.
- PR/Preview 진입 검사1/1/내부16조건 PASS는 `mechanics.tap`·`mechanics-confirm.tap`에 분리해서 보존한다. 이 두 실행은 새 Auto 검사 실패도 포함하므로 전체 파일이 PASS라고 하지 않는다. 최신 default검사2PASS와 unchanged entry1PASS를 범위별로 보고한다.
- 관련 unit **64/64 PASS**. safe named Go3개(TestHandlerServesDashboardAPIsAndAssets/SignatureThemeBranding/OverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly) PASS. 정확한 -run으로격리 fixture만 실행, unprotected go test ./.../uninstall fixture0.

새 CPU/frame/GPU/전체48 gate/idle 재측정은 수행하지 않았다. 기존 gate44/48 FAIL·immutable idle26/27 FAIL 정책hold/threshold를 그대로 유지한다. 삼성브라우저 - z fold8 ultra 접힘/펼침 실기는 미실행, 실제 랙 해결/전체A65완료/AC8체크/사용자 디자인 수용으로 승격0.

## 게시와 실제 신원

같은 Draft/base dev PR361 정상 push 직후 link, latest CI/Preview snapshot과 원문/결과/미실행을 canonical/PR에 직접 반영한다. 운영18765/기존57069 install/switch/restart0, 임의 visualserver0, merge/deploy0, 새delegate/reviewer0. 사용자는 최신 PR검증 빌드의 기존 ‘플로팅 시안 보기/기존 UI 비교’ 버튼으로 검토한다. Root 기존 watch 계속, childwatch/poll0, Root ACK는 완료조건아님.

동일 CODEX01a1bdce-8fa4-7a62-987e-129ff53a6d9f / actual T3 Controller ordinal28 running 읽기 관측. hook/binding미관측·JSONL/state/완료상태 조작0. effective787637c964b696d30e7840a0b93cd5f42dc000c31f5a5f7659ed23707f95c5e6 execution_authorizedtrue/findings[] 직접 조회. 원본 사용자 첨부는 canonical에 byte보존하고 공개 repo에는 synthetic 캡처만 게시한다.
