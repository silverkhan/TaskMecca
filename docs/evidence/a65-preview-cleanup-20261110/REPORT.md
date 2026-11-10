# A-65 Preview API 테스트의 fixture writer 수명 교정

수정 source **bf161355b9d0bea873b5870234995d5947b8cba9**, 기준 **8a2ee6772946dff16feae1760f9cad66d1f168c4**. 변경은 internal/webui/registered_fixture_test.go 하나이며 production server/attention 코드·Preview API 단언·전체 goassets tree는 byte동일하다. UI/시안/시각라운드/전체성능측정 변경0.

## 실패와 실제 원인

Root가 CI42787860946 Go runtime에서 확인한 TestPreviewBuildListServerDevOnlyAndNeverExecutes의 TempDir RemoveAll cleanup unlinkat directory not empty 실패 원문을 [raw](ci-42787860946-failed.log)에 보존했다. 이전 native shell 단언/브라우저 실패와 다르다. 같은HEAD의 나머지7check와 Preview는 성공이었다.

registerWebFixture는 isolated TASK_MECCA_HOME을 만들고 프로젝트를 monitoring에 등록한다. Handler는 관련 API를 호출하지 않아도 ensureDurableAttentionFeed를 시작한다. feed.run은 refreshActive의 projection/runtime/notification ledger writer를 소유한다. 기존 helper에는 종료 cleanup이 없어서 t.TempDir 정리·registry env 복원 뒤에도 goroutine이 살아 있었다. stopProjectAttentionFeeds의 stop 신호만으로는 진행 중 refresh 완료가 보장되지 않으며 doneCh는 run 종료 시 닫힌다.

원래 Preview named test40회에서는 동일 unlinkat가 재현되지 않았다(성공 기록 보존). 반면 실제 Handler의 feed를 소유하는 nested fixture 수명 회귀는 수정 전 **fixture returned before its background writer finished**로 결정적으로 실패했다. 이는 CI의 정확한 unlinkat 스케줄을 재현했다는 주장이 아니라 missing lifetime join의 재현이다.

## 최소 수정

fixture helper가 등록을 마친 직후 t.Cleanup을 등록한다. Go Cleanup LIFO에 따라 이 callback은 TASK_MECCA_HOME Setenv 복원 및 registry/project TempDir 삭제보다 먼저 실행된다. 해당 프로젝트 feed들을 mutex 아래에서 stop/remove하고 mutex를 푼 뒤 각 doneCh를 기다린다. active refresh가 종료되기 전에 디렉터리를 지우지 않는다. 다른 프로젝트/서비스에는 접근하지 않는다. 종료 대기 중 global mutex를 잡지 않으며 sleep/skip/retry/cleanup error 무시가 없다.

새 TestRegisteredWebFixtureJoinsAttentionFeedBeforeTempCleanup은 실제 Handler가 durable feed를 시작했는지 확인하고 nested fixture 종료 직후 doneCh 닫힘과 feed registry 해제를 함께 단언한다. 원래 Preview HTTP/status/실행불가/오류 단언은 수정하지 않았다. 이 수정은 테스트 fixture가 소유한 현재 feed join이며 전체 운영 Web shutdown/별도 detached Push 작업의 일반적 수명 해결로 확대하지 않는다.

## 검증

- 수정 전 원본 Preview40회 PASS, 새 수명 회귀1회 FAIL(raw 보존).
- 수정 후 Preview2개+새수명회귀1개 ×40회 =120 test executions PASS.
- -race 대상9개1회 PASS: Preview2·새수명1·attention lifecycle3·static/branding/readonly3. [정확한 이름](selected-tests.log), [반복 결과](after-repeat.log), [race](after-targeted-race.log).
- 각 명령은 별도 임시 TASK_MECCA_HOME을 사용하고 helper도 fixture별 isolated registry를 설정했다. uninstall fixture와 go test ./...는 실행하지 않았다. git diff --check PASS.
- [제품 byte보존](protected.json). 새 UI/브라우저/시각/성능검사는 요구 범위상 실행하지 않았다. 삼성 실기 미검증은 그대로다.

코드 수정은 사용자의 2026-11-10 18:27KST 기존CI실패 수정 명시 지시 범위다. 최신 CLI decision 조회에서는 운영 요약/추가 이력 뒤 legacy 승인 hash의 bounded proof가 소진되어 approval_required/LEGACY_CHECK_STATE_UNVERIFIED14개가 관측됐다. 이전 승인 snapshot과 현재 조회를 canonical에 모두 보존하고 승인/Hook/상태를 합성하지 않았다. 현재 사용자 명시 지시를 새 UI범위 확장/정책완화/AC수용으로 해석하지 않는다. 이 legacy verifier 정합성 잔여는 Root에 따로 전달한다.

Draft/base dev/open·기존 idle26/27 FAIL 정책hold·AC8미체크·사용자 시안/전체확장 검토대기 유지. 운영18765/57069·merge/deploy·새delegate0. 정상push 직후 link, 최신 exactHEAD CI/Preview 단회조회 후 pending은 Root 기존watch로 전달한다. 이전 모든 실패raw/증거를 보존한다.
