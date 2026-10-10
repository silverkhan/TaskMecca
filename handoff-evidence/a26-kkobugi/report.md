# A26 꼬부기 구현·검증 인계

## 작업 경계

- assignment: `assignment-a087e8051609ea5b50657be557eb5930`
- Worker: `/root/controller/kkobugi` (native `/root/controller_recovery/kkobugi`)
- attempt: `run-2feb6271b8055852`, turn: `01a11564-d141-79e1-a49f-55609c28bb48`
- 격리 worktree: `/tmp/task-mecca-a26-kkobugi`, branch: `codex/a26-project-archive-migration-choice`
- base: `f261fa59e61821fd9dbc07bdcc6cac95fb83c3b5`
- 계약 SHA256: `cc6af5a8ef4797e2c8e636ade50da34725093ab7052ac4ea4105525e93aef18a`
- 운영 설치/dev113, 실제 프로젝트, 실제 registry, Telegram, OS Trash, canonical/runtime/lifecycle/handoff 원장은 수정하지 않았다. A14 및 A27 operation-banner 영역은 범위 밖이다. Controller가 dev 통합, canonical 완료, 보존/정리를 담당한다. A26 운영 배포 없음.

## 8AC 구현

1. Go 최초 init은 framework 설치 후 Web 감시 등록을 한다. 기존 설치에는 migrate 안내를 유지한다. 등록 실패해도 설치된 framework를 지우지 않는다. Python init은 기존 framework-only 호환 역할을 유지한다.
2. migrate는 framework-only다. 사용자 backlog/config/Telegram/auth/runtime/backups manifest 경로를 거부하며 registry 등록·tag 생성 부수효과를 제거했다.
3. Go/Python/Web 공통 overwrite/backup/cancel 선택과 수정 파일 목록을 제공한다. 선택 전 무쓰기/무백업, 백업 실패 중단, 비TTY/JSON exit3 choice_required, digest stale 선택 재요구, 읽기 실패·symlink ancestor·사용자 소유 경로 차단을 검증했다.
4. CLI/Web 감시 등록/해제, 보관/다시 편입을 공통 maintenance 경계로 처리한다. Archive는 registry 상태만 변경하고 monitoring/notification/runtime writer를 차단한다. Restore는 현재 canonical identity가 역사적 boundary와 같은지 확인한다.
5. 보관 목록은 이름·전체 경로·복사·접기·다시 편입·목록 제거를 제공한다. Forget은 파일이나 suppression tombstone을 지우지 않는다. Web open/restart는 자동 등록하지 않는다. 명시 Add만 재등록한다.
6. native Trash, 잔존 확인, 폴더 삭제/복구 구현과 CI를 제거했다. legacy removal_history는 opaque raw 자료로 보존하고 guard에서만 읽는다. 미지의 registry 필드와 unrelated paused 경계도 모든 writer 후 보존한다. 화면에는 legacy Trash 사실을 표시하지 않는다.
7. 기존 레이아웃·Korean 문구·Migration/Open을 유지하며 제한된 Hub/migration UI 영역만 변경했다. 실제 Chrome desktop/mobile/light/dark QA 2라운드 완료, 아래 증적과 한계 참조.
8. 구현·테스트·PR은 격리 worktree에서만 수행했다. Controller-owned merge/dev/canonical/archive/cleanup은 Worker 완료로 주장하지 않는다.

## 검증

- `go test ./...`: 최종 registry preservation 추가 후 전체 PASS.
- `go vet ./...`: PASS.
- `go test -race ./internal/install ./internal/maintenance ./internal/projectguard ./internal/webui ./cmd/task-mecca`: 최종 registry 추가 후 PASS.
- bundled Python 3.12.14, `PYTHONPATH=src python3 -m unittest discover -s tests`: 30 tests PASS.
- Go stale human-choice/legacy no-choice/symlink/read-error/backup failure, maintenance substituted ancestor/Archive-Restore-Forget/legacy raw preservation, Web archived migration/no-primary-autoregistration 회귀 PASS.
- `node --check` app.js, Go/Python app.js/style.css parity, `git diff --check`: PASS.
- main.go whole-file formatting churn 2669→70행으로 줄였다. base compact source에 semantic edits만 이식 후 gofmt-normalized bytes 동일 assert로 확인했다. maintenance unchanged functions도 원 formatting 복원했다.

## Impeccable 및 UI 증적

SKILL, harden, craft-floor를 읽고 incumbent design을 보존했다. context.mjs 1회 실행에 잘못된 `internal/webui/assets/app.js` target을 전달한 사실을 기록한다. 재실행하지 않았다. 실제 assets는 goassets/template/_task_mecca/framework/web다.

manual detect.mjs를 최종 app.js/style.css 대상으로 1회 실행했다. warning 4개(기존 Inter font, 기존 side accent line909, 기존 width/margin transition line250/256)만 있었고 새 Hub/migration 영역 경고는 없었다. scope 밖 incumbent styling을 재설계하지 않았다.

Round1: 실제 Chrome extension 1440×1000 desktop dark/light, 390×844 mobile light/dark 및 migration dialog/cancel을 확인했다. hidden feedback bar, 취소 후 disabled 버튼, 5초 refresh 중 입력 draft 손실, Add 중복 동작/좁은 archive row를 수정했다.

Round2: 이 Worker 재개 후 `mcp__cua_repl.js`의 아래 title별 inline JPEG와 DOM 결과가 정확한 시각 증적이다. 추가 rebuild/3차 polishing 없음.

- `데스크톱 다크 안정 화면 및 라이트 전환`: 1440×1000, incumbent sidebar와 Hub가 겹치지 않는 안정 다크 화면. 직전 resize animation screenshot은 판정에 사용하지 않았다.
- `데스크톱 라이트 및 모바일 확인`: 1440×1000 라이트 screenshot. 이름·전체 경로·활성/보관 영역 확인.
- `모바일 라이트 확인 및 다크 전환`: 390×844 라이트 screenshot. 긴 CJK 프로젝트명/경로 줄바꿈 확인.
- `모바일 다크 및 수정 선택 대화상자`: 모바일 다크 fullPage screenshot에서 보관 이름·전체 경로·다시 편입/목록 제거까지 확인.
- `세 가지 선택과 취소 후 활성 상태 확인`: 모바일 dialog screenshot, 세 선택 모두 표시, focused=`취소`, 취소 후 Migrate enabled=`true`. 실제 사용자 변경은 여전히 보존됨.
- `보관 프로젝트 다시 편입 검증`: 보관→활성 카드, 감시 해제 버튼, archive count0 확인.
- `보관 완료 및 목록 제거 확인 문구`: 보관 confirmation은 파일 유지·감시 중지, 완료 후 count1. Forget confirmation은 목록만 제거하고 suppression 유지 설명.
- `목록 제거 후 새로고침 자동 재등록 방지`: Forget 이후 reload count0, 해당 프로젝트 활성 카드도 없음.
- `상대 경로 차단과 명시적 경로 등록 확인`: 입력 draft가 유지됨; 명시 전체 경로 Add로 다시 활성 등록. 상대 경로 제출에는 활성 카드 추가 없음(오류 status는 auto-refresh 이후 없어져 안정적인 DOM 증적을 얻지 못했고, API 절대경로 거부는 자동 테스트로 확인).
- `합성 fixture 최종 보관 상태와 세션 보존`: 다시 보관 count1, active fixture는 감시 등록 버튼(중지). viewport override reset 및 Controller 요청에 따라 tab handoff 보존.

로컬 screenshot artifact 저장 API가 문서에 없어 JPEG inline emitImage만 제공했다. 비인가 CDP/별도 Playwright/파일 저장 우회는 하지 않았다. Controller가 이 한계를 수용했고 추가 시각 QA는 요구하지 않았다.

## 보존과 정리 인계

- Round1 fixture root: `/private/tmp/a26-kkobugi-ui-XB0hb4` (유지, 기존 listener PID66357만 종료).
- Round2 fixture root: `/private/tmp/a26-kkobugi-ui-Iw7kfM` (유지). 실행 session1675, port18926 listener `webui.test` PID84231. transport disabled, synthetic management home만 사용.
- Chrome agent-created tab1000214847: `http://127.0.0.1:18926/?view=hub`, handoff mark 및 viewport reset 완료. Controller가 잠시 보존 후 정리한다.
- ignored local 부산물: `src/task_mecca/__pycache__/`, `tests/__pycache__/`. 실제 사용자 데이터가 아닌 테스트 cache지만 삭제하지 않았다.
- tracked/untracked 구현 파일은 모두 이번 branch commit에 포함한다. native source/test 삭제는 Git으로 복구 가능하며 OS Trash/project folder는 건드리지 않았다.

## Delivery

Stable report ID: `A26-kkobugi-20261007-01`.

실제 PR: https://github.com/silverkhan/TaskMecca/pull/140 (base `dev`). 구현 head: `25c0113abce93b58e8eeafe818d798466e0727c8`. 해당 exact SHA의 CI run37596117171에서 9개 jobs(Go runtime, Python3.11/3.12/3.13, Go-Python smoke, macOS/Windows preserving management, macOS/Windows build) 모두 PASS. 상세: https://github.com/silverkhan/TaskMecca/actions/runs/37596117171 . standalone artifacts run37596117107도 SUCCESS: https://github.com/silverkhan/TaskMecca/actions/runs/37596117107 .

이 Delivery 갱신은 report-only commit이며 코드 변경은 없다. 자기 report commit SHA 본문 삽입 순환을 피하기 위해 final report commit SHA 및 해당 exact head CI는 native DONE에서 전달한다. original contract SHA는 위 작업 경계의 `cc6af5a8ef4797e2c8e636ade50da34725093ab7052ac4ea4105525e93aef18a`다. merge/dev integration 및 canonical done은 Controller 작업이다.
