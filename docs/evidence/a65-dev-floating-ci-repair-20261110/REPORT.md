# dev-floating-pilot CI 잔존 마운트 단언 수정

실패 HEAD: 555863bbf1379c0b6c0afad8feab936ad367e0bb. Root가 확인한 [CI42735770060](https://github.com/silverkhan/TaskMecca/actions/runs/42735770060)와 [Preview42735769714](https://github.com/silverkhan/TaskMecca/actions/runs/42735769714) 실패 로그를 직접 조회·보존했다. 둘 다 TestHandlerServesDashboardAPIsAndAssets, server_test.go:172 `native core module mount missing from Web shell`로 실패했다. 같은 이름의 로컬 테스트1개로도 수정 전 실패를 재현했다.

최신 합의는 실제 origin/dev 기본 DOM과 opt-in 소재 시안이다. 이 HTTP 테스트 한 곳에만 과거 ui/main.js/id=app 마운트 요구가 남아 있었다. 기존 native 엔트리의 회귀는 이전에 분리한 historical fixture 경계에 유지한다.

수정은 internal/webui/server_test.go만이다. 단언을 삭제하고 통과시키는 대신 다음 HTTP 계약을 검증한다.

- /, /tasks/A-1, /tasks/A-1?floating=1&background=image 모두 실제 embedded dev index와 응답 바이트가 동일하다.
- app-shell/sidebar/sidebarToggle/stateNav/topbar/content/shortcutBar의 필수 DOM이 각각 한 번 존재한다.
- theme bootstrap→dev stylesheet→pilot JS/CSS→select helper→dev app의 엔트리가 각각 한 번, 올바른 순서로 존재한다.
- 기본 shell이 재구현 ui/main.js를 로드하거나 pilot data attribute를 서버에서 미리 활성화하지 않는다.
- opt-in JS/CSS의 GET200·MIME과 JS/CSS/사진의 실제 embedded 응답 바이트를 검사한다. 기존 API·CSP·runtime·release 검증은 그대로 이어서 실행한다.

안전 검증: TestHandlerServesDashboardAPIsAndAssets / TestSignatureThemeBranding / TestOverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly 총3개 PASS. -run을 정규식으로 정확히 제한하고 -count=1 사용. 이 테스트의 프로젝트는 t.TempDir, registry는 registerWebFixture의 TASK_MECCA_HOME=t.TempDir, release provider는 기존 mock이다. hook API는 임시 프로젝트 fixture에만 작용한다. 실제 native hook/binding/완료 상태를 작성하지 않았다. unprotected go test ./...와 uninstall fixture는 실행하지 않았다.

production/goassets/실제 UI source3a066ed4·시안 캡처·기능·기존 fullgate/idle/threshold 모두 변경0. 새로운 visual round/브라우저 계측/전체 gate 재측정0. 새delegate/reviewer0, merge/deploy0, 운영18765/57069 변경0. 전체 확장은 사용자 시안 수용 이후이며 기존 immutable idle 정책hold는 유지한다. 삼성 실기는 미검증이다.

실제 Controller는 같은 CODEX01a1bdce-8fa4-7a62-987e-129ff53a6d9f, 같은 T3 thread ordinal26 running을 읽기로 관측했다. 과거 completed identity를 재사용·수정하지 않았다. 권한effective787637c964b696d30e7840a0b93cd5f42dc000c31f5a5f7659ed23707f95c5e6 / execution_authorizedtrue/findings[] 재확인. Root fresh preflight10:46KST는 전달된 실행근거이며 새 dispatch0. Root ACK 기다림 없이 PR/기록/직접 인계까지 처리한다.

정확한 후속 HEAD와 CI/Preview snapshot은 canonical ci-repair-20261110/external-final.json 및 PR 본문에 기록한다. Root는 기존 PR watch를 계속하며 child watch/poll0이다. 원격 전체 CI 성공으로 미리 표기하지 않는다.
