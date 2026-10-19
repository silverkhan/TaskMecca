# Task Mecca CLI — Dev 실사용 인수 체크리스트 (AID-211)

> **실행 시점:** 최신 Dev CLI가 실제 장비에 설치된 이후, **Stable/main 승격 전**. 이 문서는 릴리즈 자체의 증명이 아닌 **사용자가 Dev를 사용하며 직접 관찰할 검증 항목**이다. 시험용 데이터를 쓰는 작업은 별도 프로젝트/별도 Git 테스트 저장소에서 수행한다. `PASS`, `FAIL`, `NOT_RUN`, `UNKNOWN`을 구분하고 근거를 남긴다.

## 환경 기록 (필수)

- [ ] 사용 장비/OS(Windows 또는 macOS), Dev 버전/채널, Web 실행판 버전 및 검사 날짜 기록
- [ ] `task-mecca --version`, `task-mecca --help`, `task-mecca help contract`, `task-mecca help runtime`, `task-mecca help worktree`, `task-mecca help protect`에 신규 명령·읽기/쓰기 범위가 설명됨

## 1. 백로그 일괄 등록·계약 검증 — AID-202/207

- [ ] **테스트 프로젝트에서만** 외부 이슈 6건(내부 외부키·출처·원문·태그·선행관계 포함) JSON manifest 작성, `contract validate --input FILE --json` 실행. 누락된 목표/수용 기준 입력은 적용 전 필드별 오류 반환
- [ ] `contract preview`의 표준 `### 목표`, `### 수용 기준` 및 원문·출처 링크가 정확하며 실제 백로그 파일은 생성되지 않음
- [ ] `contract plan --project TEST --input FILE --request-id STABLE_ID --json`의 ID/B-ID·외부키·선행관계·`plan_digest`가 적절하고 **파일 생성 없음**
- [ ] `contract apply --project TEST --input FILE --request-id STABLE_ID --expect-plan PLAN_DIGEST --approve --json` 이후 6개 등록 및 선행 B-ID·출처·태그·lifecycle 확인. `contract status`는 실제 완성된 파일/이벤트만 성공으로 표시
- [ ] **같은 요청 키·같은 입력으로 재실행**해도 중복 파일·등록 사건이 0개. 입력 변경/plan stale 등 충돌 시 자동 덮어쓰지 않고 JSON 오류 코드와 재개 안내 제시

## 2. 실행 승인·계약 보충 — AID-208

- [ ] 테스트 등록만 한 B-ID의 `contract decision show --project TEST --task ID --json`은 실행 미승인 표시
- [ ] 실제 테스트 승인 근거 없이 `contract decision record` 시도하면 거부되고, 명시적 테스트 승인 참조 + `--user-confirmed` 이후에만 유효 승인 표시
- [ ] `policy_pending` 중 실행 보류, `policy_confirmed` 뒤 유효 승인 판정, 계약 `supplement` 뒤 재승인 필요, 철회 후 실행 불가 확인
- [ ] 원본 계약·과거 snapshot 손상 없음. 실행 승인 기록만으로 **실제 Controller/Worker 자동 기동이 발생하지 않음**

## 3. 현재 실행 조회·Hook 관측 — AID-203/209

- [ ] 실제 실행 중인 Controller/Worker 세션에서 `runtime query --role controller --state open --json`, `--agent-id ID --limit 2 --fields attempt_id,binding_state,current_state,runtime_agent_id,session_id,turn_id --json`이 Python 후처리 없이 최신 N건 반환
- [ ] 실행 종료 후 과거 completed 이력은 active로 잘못 선택되지 않음. `not_found`와 `unknown`을 실행하지 않았음/사망으로 오표기하지 않음
- [ ] `runtime diagnose --provider codex --agent-id ID --json`에서 Hook **설정 여부**와 **실제 관측된 이벤트**가 분리. 복구 안내에 합성 이벤트/종료 attempt 재사용 요구가 없음
- [ ] Hook을 사용할 수 없는 provider 기능은 지원 불가/외부 수동 조치로 표현되고, native transport 성공을 추정하지 않음

## 4. Worktree 자료 보존·정리 — AID-204/210

- [ ] **정리 전 읽기 전용:** `worktree plan --project REPO --into REF --json`에서 primary/locked/detached/미병합/dirty/untracked/ignored/nested/symlink/사용 여부 미확인 사유와 파일 수·용량·digest 확인
- [ ] **별도의 격리 Git 저장소**에서 `worktree preserve --project REPO --into REF --target EXACT --archive OUTSIDE --expect-plan DIGEST --approve --json` 실행. archive의 `manifest.json`과 자료 SHA-256 검증 성공
- [ ] 같은 preserve 반복해도 보존 자료가 중복되거나 손상되지 않고, 복사본 해시 불일치/대상 변경 시 제거 차단
- [ ] **격리된 clean·merged·미사용 대상만**, 실제 미사용 확인 후 `worktree remove ... --confirm-unused --approve` 실행. 정해진 exact worktree만 제거되고 `removal.json` 및 Git branch/commit 보존 확인. 재실행 시 `already_removed`
- [ ] 일부러 untracked/ignored/dirty를 만든 대상은 **백업 후에도 제거되지 않음**. 별도 강제 삭제/`rm -rf` 사용 금지

## 5. 업데이트 전후 보호·버전 — AID-205/206

- [ ] 업데이트 **이전** `protect snapshot --project TEST --output EXTERNAL_MANIFEST.json --json`으로 사용자 소유 보호 범위와 파일 수·해시 기록(보호 데이터 내부 저장 금지)
- [ ] 테스트 프로젝트의 Framework `migrate --json`에서 `modified_files`와 `protected_files.status`가 분리되고, 정상 시 `verified`; 실제 변경/누락/읽기 실패면 `changed/unknown` 표시
- [ ] 업데이트 **이후** `protect verify --project TEST --snapshot EXTERNAL_MANIFEST.json --json` 결과에 `missing/changed/added` 구분, 실제 비교가 없으면 보존 성공 주장 없음
- [ ] `protect versions --project TEST --json`에 CLI 버전·Framework VERSION·Web 실행 중 버전/적용 여부가 별개로 출력
- [ ] 지원하지 않는 `task-mecca update` 등은 올바른 `upgrade`/`migrate` 대체 안내; `--help`는 실제 CLI 옵션과 일치

## 6. 멀티 OS·Agent 사용성 — AID-213

- [ ] macOS와 Windows 각 1회 이상 Dev 실행, 기본 CLI 도움말·JSON 응답·오류 코드·경로 처리 확인
- [ ] 새 Root/Registrar/Controller 세션이 배포된 `_task_mecca/framework/CLI_OPERATIONS_PLAYBOOK.md` 및 역할 지침만 보고 신규 CLI를 선택. 의존관계 등록·승인·handoff에 Python·JS 임시 후처리/원장 수동 수정 불필요
- [ ] CLI 지원되지 않는 native 작업은 근거와 사용자 동작이 명시됨. 실패/부분 성공/미확인/미실시를 구분

## 결과 회신 형식

검증마다 **이슈 ID / 장비·OS / 설치 Dev 버전 / 테스트 결과 PASS·FAIL·NOT_RUN·UNKNOWN / 사용한 명령·화면 / 실제 JSON 오류 코드 또는 증거 경로 / 재현 순서**를 기록한다. 인수 기간에는 작업 중인 운영 프로젝트·Worktree에 destructive 테스트를 실행하지 않는다. 전체가 PASS이거나 미지원 항목이 합의된 대체 절차로 확인된 이후에만 AID-211 사용자 인수 완료를 고려한다.

**주의:** 소스 PR 병합·GitHub CI 통과는 사용자 장비의 Dev 바이너리/Framework 갱신을 증명하지 않는다. 최신 Dev 기능을 설치한 뒤 실제로 확인해야 한다. **Stable 배포는 별도 사용자 승인 사항**이다.
