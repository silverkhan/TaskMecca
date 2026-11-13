# AID-61 — Root Session 표시 이름 소스 계약

## 목적

Runtime Observability의 canonical identity는 계속 `provider + session_id`로 유지한다. 사람이 읽는 Root Session 이름은 표시용 metadata이며 identity, 정리 범위, 실행 결합 키로 사용하지 않는다.

## 표시 우선순위

1. `provider_name`: provider 공식 인터페이스의 명시적 이름
2. `t3_title`: native 이름이 비어 있고 정확한 native/T3/project 연결이 검증된 T3 제목
3. `provider_title`: Codex 외 provider가 공식 인터페이스로 제공하는 제목
4. `task_mecca_fallback`: `Root · YYYY-MM-DD HH:mm`

Provider metadata를 얻지 못한 경우 session id를 큰 제목으로 승격하지 않는다.

## Provider 검증 결과 (2026-10-02)

### Claude Code

공식 Hooks reference의 `SessionStart` payload는 선택적 `session_title`을 제공한다. 이 값은 `--name`, `/rename`, hook의 `sessionTitle` output, Agent SDK의 `renameSession()` 등으로 설정된 **custom title**이다.

공식 문서는 이름을 지정하지 않은 세션에도 generated title이 있을 수 있지만, generated title은 `session_title`에 나타나지 않는다고 명시한다.

따라서 Task Mecca는 Claude에 한해 `SessionStart`를 추가 관측하고 `session_title`을 `provider_name`으로 저장한다. generated title을 얻기 위해 Claude의 내부 transcript/DB 포맷을 파싱하지 않는다.

### Codex

공식 Hook common payload는 `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `model` 등을 제공하지만 session/thread name/title 필드는 제공하지 않는다. 공식 문서도 transcript format은 stable hook interface가 아니라고 명시한다.

Codex App Server의 공식 metadata-only `thread/read` 경로는 A69에서 도입한 reader를 재사용한다. exact native ID를 확인한 뒤 nonempty `thread.name`을 우선 사용한다. 이름이 비어 있으면 아래 A-75의 T3 연결 검증 규칙을 적용한다. native 내부 SQLite나 프롬프트로 표시 이름을 보충하지 않는다.

## 저장/집계 계약

- `SessionStart` metadata는 `session_metadata` 이벤트로 기존 append-only runtime journal에 기록한다.
- 이 이벤트는 execution attempt가 아니므로 `BuildLedger`의 Attempt 집계에서 제외한다.
- Root Session 집계가 `provider + session_id`로 metadata를 병합한다.
- Claude `source=startup`의 최초 관측 시각은 `created_at_source=provider_metadata`로 사용할 수 있다.
- resume/compact 관측은 이름 갱신에는 사용할 수 있지만 개설시각을 덮어쓰지 않는다.
- `display_name_source`는 `provider_name | t3_title | provider_title | task_mecca_fallback`을 유지한다.

## UI 계약

Root Session 카드의 시각적 우선순위는 다음과 같다.

1. 가장 큰 제목: `display_name`
2. 작은 metadata: Provider, provider session id(Root Session ID)
3. 개설일시 또는 최초 관측일시
4. 마지막 활동일시

내부 `root_session_id` 해시는 UI 제목/식별자처럼 강조하지 않는다. canonical grouping과 API 동작에는 계속 사용할 수 있다.

## 비목표

- provider의 비공식 SQLite/내부 DB 직접 조회
- 안정성이 보장되지 않은 transcript JSON 구조에서 title 추출
- 표시 이름을 canonical identity로 사용
- dev 검증 전 main 승격


## Hook 활성화·실사용 Provider 계약

Hook 설치 여부와 실제 Runtime 관측 가능 여부를 분리한다.

- `configured`: Task Mecca의 project Hook 정의가 파일에 존재한다.
- `in_use`: 현재 doing 백로그의 `RuntimeProvider` 또는 현재/최근 비종료 Root Session 근거로 해당 Provider가 실사용 중이다.
- `current_evidence`: 해당 Provider의 현재/최근 Root Runtime 신호가 실제로 관측되고 있다.
- `needs_attention`: `in_use=true`인데 Hook이 미설정이거나 현재 관측 신호가 없다.

다른 Agent의 Hook이 설정되어 있다는 이유만으로 경고를 해제하지 않는다. 예를 들어 Claude Hook만 설치되어 있고 현재 Codex가 실사용 중이면 Codex에 대해 경고한다.

Hook 설정/활성화 뒤에는 **새 Root Session 시작을 권장이 아니라 적용 계약으로 안내**한다. 이미 열려 있는 Root에서 놓친 `SessionStart` 및 기존 Agent의 Start 이벤트를 소급 복원할 수 없기 때문이다.

### Codex

공식 문서 기준 프로젝트 Hook은 `<repo>/.codex/hooks.json` 또는 `<repo>/.codex/config.toml`에서 로드되며, project-local Hook은 프로젝트가 trusted 상태여야 한다. 새/변경 command Hook은 exact definition에 대한 review/trust가 완료되기 전까지 실행되지 않는다. CLI에서는 `/hooks`에서 source 확인, review/trust, 개별 enable/disable을 수행할 수 있다. `[features] hooks = false`이면 Hooks 자체가 비활성화된다.

Codex Desktop에서는 현재 제품 UI 기준 프롬프트 입력창 좌측 하단의 Hooks 진입점을 안내한다. Task Mecca는 이 UI 위치를 Provider의 안정 API로 간주하지 않고 사용자 가이드 metadata로만 사용한다.

### Claude Code

Task Mecca는 프로젝트 `.claude/settings.json`에 Hook을 추가한다. Claude Code 공식 문서상 terminal, IDE extension, Desktop app은 같은 Hook events를 실행한다. 프로젝트 설정 Hook은 workspace trust가 필요한 실행 surface가 있으므로 설정 파일 존재와 실제 관측을 별도로 판정한다.

Claude Code의 `/hooks`는 configured Hook과 source를 확인하는 **read-only browser**이므로, Task Mecca UI에서는 이를 검증 방법으로 안내하고 설정 자체는 Task Mecca의 Hook 설정 버튼 또는 settings JSON을 통해 수행하도록 안내한다.

## T3 환경의 Root 표시 이름 수집 (A-75)

Root 표시 이름은 공식 native 이름을 먼저 사용하고, 이름이 비어 있으면 정확한 native 세션·T3 thread·프로젝트 연결을 검증한 T3 제목을 사용한다. 둘 다 확인되지 않으면 기존 기본 이름을 표시한다. T3 제목의 출처는 `t3_title`이며 native 이름으로 기록하거나 두 이름을 자동 동기화하지 않는다. 사용자 Root를 자동으로 rename하지 않는다.

Codex는 공식 `thread/read`를 exact native ID와 `includeTurns=false`로 조회한다. 설치된 공식 스키마의 실제 필드를 확인하며 `preview`, 첫 프롬프트, native SQLite의 title은 표시 이름 후보로 사용하지 않는다. Codex 0.168.0은 nullable `name`을 제공하고 `title`은 제공하지 않는다. native 조회 실패·미지원·ID 불일치를 빈 이름의 성공으로 취급하지 않는다.

T3 수집은 기존 읽기 전용 구조화 관측 경로를 사용한다. 프로젝트 workspace 경로와 thread 소유권, native ID, 최신 run/attempt의 연결을 검증하고 살아 있는 T3 owner를 확인한다. 연결이 없거나 모호하거나 다른 프로젝트이면 제목을 사용하지 않는다. 제목과 연결은 검증 전후 다시 읽으며 변경 중이면 다음 갱신까지 확인 필요로 처리한다. caller JSON이나 전달받은 제목만으로 검증 성공을 주장하지 않는다. 구조화 저장소가 미지원이면 우회 SQL이나 transcript 추출을 추가하지 않는다.

이름·제목의 변경, 삭제, 빈 값도 갱신 결과다. 성공 결과는 최대 30초, 조회 불가는 10초 간격으로 재조회하며 이전 이름을 계속 유지하지 않는다. `display_name_status`와 `display_name_checked_at`으로 확인 상태와 조회 시점을 구분한다. 표시 metadata는 attempt/assignment/claim/권한/liveness의 근거가 아니며 기존 판정을 변경하지 않는다. 관측은 SQLite/JSONL 수동 수정, native/T3 rename, 운영 설치와 별개다.
