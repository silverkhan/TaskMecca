# 작업 태그·Taxonomy 운영 가이드

Task Mecca 태그는 상태·의존성·Agent 배정을 대체하는 label이 아니라, 사람이 작업을 탐색하고 Agent가 과거 맥락을 재사용하기 위한 **project-owned taxonomy**다.

## 저장 구조

```text
_task_mecca/data/tags/registry.json     # canonical tag definitions
backlog Markdown - Tags: ...            # task assignments
_task_mecca/.runtime/tag_index.json     # rebuildable projection/cache
```

`data/**`는 프로젝트 소유 데이터다. framework migration은 기존 registry를 덮어쓰지 않는다.

## 기본 namespace

- `area`: backend, frontend, strategy, data, infra, docs 등 업무 영역
- `type`: feature, bug, improvement, refactor, research 등 작업 유형
- `concern`: performance, security, ux, compatibility, data-integrity 등 횡단 관심사
- `custom`: 프로젝트 고유 분류

태그는 선택사항이다. `todo`, `doing`, `hold`, `done`, dependency ID, Agent name처럼 이미 구조화된 metadata를 태그로 반복하지 않는다.

## 신규 태그 정의 전 순서

Agent는 새 태그를 만들기 전에 반드시 다음을 확인한다.

1. `task-mecca tags resolve <표현> --json`으로 exact/alias 확인
2. `task-mecca tags search <검색어> --json`으로 유사 태그 탐색
3. 기존 description과 의미 중복 확인
4. 재사용 가능한 태그가 없을 때만 `define`

예:

```bash
task-mecca tags resolve "프론트엔드" --json
task-mecca tags search frontend --json
task-mecca tags define custom:portfolio "포트폴리오 구성·최적화 작업" "포트폴리오,portfolio"
```

## 조회

```bash
task-mecca tags
task-mecca tags list
task-mecca tags search strategy
task-mecca tags show area:strategy
task-mecca tags resolve "전략" --json
task-mecca tags tasks 'area:strategy,type:bug|type:improvement'
task-mecca tags stats
```

`tags tasks` 표현식은 쉼표가 AND, `|`가 OR다. 위 예시는 `area:strategy AND (type:bug OR type:improvement)`다.

## Task 태그

```bash
task-mecca task tags A-143
task-mecca task tag-add A-143 area:backend
task-mecca task tag-remove A-143 concern:performance
task-mecca task tag-set A-143 area:backend type:bug concern:data-integrity
```

기존 active 태그의 assign/remove/set은 Agent가 자율 처리할 수 있다. 정의되지 않은 태그는 바로 assign하지 않고 먼저 taxonomy를 탐색/정의한다.

## Taxonomy 변경

```bash
task-mecca tags rename custom:server area:backend
task-mecca tags merge custom:web-ui area:frontend
task-mecca tags retire custom:old-category
task-mecca tags retire custom:old-category area:data
task-mecca tags rebuild
```

정책:

- `rename`: 새 canonical tag를 만들고 기존 canonical은 retired + `replaced_by`로 남긴다. active/archive task를 모두 새 이름으로 소급 변경한다.
- `merge`: 기존 태그를 active target으로 합치고 old canonical/aliases를 target alias로 보존한다. active/archive task를 모두 target으로 소급 변경한다.
- `retire`: 신규 assignment를 막는다. replacement를 함께 주면 merge와 동일하게 소급 변경한다.
- rename/merge/retire/namespace 변경은 대량 수정이므로 **대상 task 수와 의미를 사용자에게 알리고 승인 후 실행**한다.
- Git diff가 소급 변경의 감사 기록이다.

## Registry 모델

각 정의는 canonical name, namespace, description, aliases, status(active/retired), replaced_by, created_at, updated_at을 가진다.
alias 또는 retired canonical으로 resolve해도 `replaced_by` chain을 따라 현재 active canonical을 반환한다.

## Web 탐색

Backlog 화면에서 status filter와 tag multi-filter를 독립적으로 조합한다. 여러 선택 태그는 AND다.
Tag Explorer는 namespace별 사용량과 total/active/hold/done 통계를 보여주며 태그를 클릭하면 해당 task만 drill-down한다.
Task row와 detail에서도 tag chip을 직접 선택할 수 있다.

## Index

`.runtime/tag_index.json`은 원장이 아니다. 삭제돼도 다음 명령으로 전체 active/archive backlog에서 복원 가능하다.

```bash
task-mecca tags rebuild
```
