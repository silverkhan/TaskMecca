# Task Mecca 협업 규약

Task Mecca는 **Markdown backlog + Git lifecycle**을 durable source of truth로 사용한다. standalone `task-mecca` runtime은
원장을 읽고 검증하고 scheduling snapshot과 **local read-only Web UI**를 제공한다. agent 생성·메시지·대기는
Codex/Claude 등 현재 런타임의 협업 기능이 담당한다.

백로그 형식은 [`_template.md`](_template.md), 시작 절차는 [`SESSION_GUIDE.md`](SESSION_GUIDE.md), 역할은
[`roles/root.md`](roles/root.md), [`roles/registrar.md`](roles/registrar.md),
[`roles/controller.md`](roles/controller.md), [`roles/worker.md`](roles/worker.md)가 정의한다.

## 1. 역할과 canonical flow

```text
사용자
  ↕
/root                         Task Definition Owner / 사용자-facing
  ├─ /root/registrar         lossless registration
  └─ /root/controller        scheduling / orchestration
       ├─ /root/controller/kkobugi
       ├─ /root/controller/pairi
       └─ ...                bounded implementation workers
```

사용자 기원 작업은 두 lane을 사용한다.

```text
User request
→ Root classification
   ├─ Simple Task  → 작업 정의(목표 + 수용 기준) → Registrar
   └─ Defined Task → clarification → 요건 정의서 → User confirmation → Registrar
→ Controller가 worker 배정
→ Worker가 backlog 원문을 직접 읽고 구현
→ Controller가 수용 기준별 검증 후 done/archive
```

Simple Task는 작은 작업이라는 이유만으로 정하는 것이 아니라 **추가 해석 없이 바로 검증 가능한가**로 판단한다. 애매하면 Defined Task로 승격한다.

등록 이후 Root는 routine scheduling과 구현 완료를 기다리는 polling으로 사용자 대화를 막지 않는다.

## 2. Subagent 실행 전 effective Full Access gate

Task Mecca는 **권한이 부족한 상태에서 worker를 먼저 띄워 보고 실패를 관찰하는 방식**을 사용하지 않는다.
새 세션에서 실행형 작업 요청을 인지하면 Root는 긴 요구사항 정제나 subagent 생성보다 먼저 다음 gate를 실행한다.

```bash
task-mecca preflight --require-full-access --json
```

`access.orchestration_ready == true`일 때만 위임을 시작한다. `restricted` 또는 `unknown`이면:

- Registrar/Controller/worker를 새로 실행하지 않는다.
- todo를 doing으로 바꾸거나 Agent를 claim하지 않는다.
- 사용자에게 Codex의 Full Access 활성화를 권한다.
- 사용자가 권한을 변경했다고 알려오면 같은 effective preflight를 다시 실행한다.

preflight는 config 문자열만 신뢰하지 않는다. 알려진 `CODEX_SANDBOX` runtime marker와 workspace/Git metadata/subprocess/
outside-workspace write probe를 함께 사용한다. 알 수 없는 marker 값은 자동으로 restricted로 단정하지 않고 effective probe 결과와
함께 판단한다. 성공 결과는 `.runtime/access_preflight.json`에 ephemeral cache로 저장하며 Git 원장에 넣지 않는다. 이 cache는 Web UI에서
마지막 관측 상태와 확인 시각을 보여주는 용도일 뿐 dispatch 승인이 아니다. 오래된 성공값은 경고/`unknown`으로 강등하지 않고
중립적으로 표시하며, **실제 subagent dispatch 직전에 active preflight를 자동 재실행**한다.

Codex에서 network access는 filesystem sandbox mode와 별개일 수 있다. `CODEX_SANDBOX_NETWORK_DISABLED`가
관측되면 별도로 표시하고, 네트워크가 필요한 작업에서 추가 확인한다.

단순 질의나 설계 논의처럼 subagent 실행이 전혀 필요 없는 대화를 이 gate 때문에 막지는 않는다.

## 3. 작업 계약 — Simple / Defined

신규 작업은 두 종류의 canonical contract 중 하나를 사용한다.

### Simple Task

- 추가 해석 없이 결과·범위·완료 기준이 명확한 bounded 작업에 사용한다.
- `## 작업 정의` 아래 `### 목표`, `### 수용 기준`만 기록한다.
- 사용자 원 요청 자체가 충분한 계약이면 별도 요건정의서와 사용자 재확인을 요구하지 않는다.
- Registrar는 Simple을 임의로 Defined로 부풀리지 않는다.

### Defined Task

- 의미·범위·설계·사용자 선택·작업 분해·별도 수용기준 합의가 필요하면 사용한다.
- Root가 사용자와 필요한 만큼 명확히 만들고 `## 요건 정의서`를 작성한다.
- 사용자 확인 후에만 등록한다.
- Registrar는 확정 요건을 요약·재작성·축소·확대하지 않는다.

### 공통

- `## 작업 정의`와 `## 요건 정의서`는 각각 해당 task의 canonical contract다.
- 작업 분해가 필요하면 Registrar가 아니라 Root가 결정한다.
- Controller의 dispatch 설명은 backlog contract를 대체하지 않는다.
- Worker는 backlog 파일을 직접 읽고 `### 수용 기준`을 완료 기준으로 삼는다.
- Simple Task 수행 중 사용자 판단이 필요한 숨겨진 요구가 발견되면 Root에 올려 Defined Task 수준으로 재정의한다.
- 구현 중 새로 발견된 독립 문제는 기존 완료 조건에 누적하지 않고 별도 후속 후보로 다룬다.
- 새 사용자 선택이 필요한 경우 Controller는 Root에 `USER_DECISION_REQUIRED`를 올린다.

내부 발견 작업도 같은 분류 규칙을 사용한다. 명확하고 bounded하면 Simple, 제품 의사결정이 필요하면 Defined다.

## Project taxonomy / Tags

작업 태그는 `_task_mecca/data/tags/registry.json`의 project-owned taxonomy와 backlog의 `Tags` metadata를 사용한다.
Agent는 신규 태그 정의 전에 `task-mecca tags resolve/search`로 기존 canonical/alias를 먼저 탐색한다.
기존 active 태그 assign/remove는 자율 처리할 수 있지만 rename/merge/retire 같은 taxonomy 구조 변경은 영향 범위를 사용자에게 알리고 승인 후 실행한다.
상태·dependency·Agent assignment를 태그로 중복 표현하지 않는다. 전체 명령과 정책은 `TAGS.md`를 따른다.

## 사람용 backlog projection

신규 backlog는 canonical contract와 별도로 `## 핵심 요약`을 가진다. 이 section은 Web UI의 첫 화면 projection이며
계약·수용 기준·검증 근거를 대체하지 않는다. 제목/요약/상세의 작성 규칙과 상태별 갱신 예시는
`HUMAN_READABLE_BACKLOG.md`를 따른다.

- 등록: 목적·예정 변경·완료 조건의 의미
- 진행: 현재 단계·남은 일·차단 요인
- 완료: 실제 변경·검증 판정·남은 후속
- 사용자 판단/실패/중요 미검증은 요약에 숨기지 않음

## 4. 백로그 schema v2

신규 프로젝트의 canonical ledger는 `_task_mecca/data/backlog/`다. Installer는 project data를 만들지 않으며, 첫 작업을 등록하는 Registrar가 필요할 때 생성한다.

```text
_task_mecca/
  data/
    backlog/
      000143.A-143.local-web-ui.todo.md
      archive/2026-09/000142.A-142.previous.done.md
```

기존 프로젝트의 `backlog`, `backlog_b`, `backlog_*` 등 **backlog로 시작하는 ledger 이름은 읽기/운영 호환**한다. 새 ledger를 만들 때는 변형 이름을 새로 만들지 않고 canonical `data/backlog`를 사용한다.

`data/` 아래는 project-owned durable data 영역이다. Agent가 작업 과정에서 별도 감사·측정·테스트 증적을 보존할 필요가 있으면 `data/` 아래에 생성할 수 있지만, Task Mecca는 `audits`, `measurements`, `tests` 같은 하위 이름 자체를 표준으로 강제하지 않는다.

파일명:

```text
<6자리 정렬키>.<ID>.<slug>.<todo|doing|hold|done>.md
```

상태의 durable source는 파일명이다. legacy 4자리 파일과 기존 flat 본문은 읽기 호환한다.

신규 본문은 공통 section에 더해 contract section 하나를 선택한다.

Simple Task:

```text
# ID 제목
## 작업 개요
## 작업 정의
### 목표
### 수용 기준
## 실행 정보
## 작업 노트
## 결과
## 검증
```

Defined Task:

```text
# ID 제목
## 작업 개요
## 요건 정의서
### 배경 및 문제
### 목표
### 요구사항
### 범위
#### 포함
#### 제외
### 수용 기준
### 제약 및 보존 조건
## 실행 정보
## 작업 노트
## 결과
## 검증
```

두 contract를 한 항목에 동시에 넣지 않는다. 비교·매핑·상태별 동작은 필요할 때 Markdown table을 사용한다. 표 사용 자체를 강제하지 않는다.

### 작업 개요

- `등록자`: `user` 또는 발견 agent canonical path
- `Agent`: active doing의 현재 담당자. todo/hold는 `-`
- `변경범위`: active doing의 쓰기 범위. 읽기 전용이면 `읽기 전용`
- `선행`: 이 항목의 원래 수용을 직접 막는 ID만
- `연관`: 비차단 후속·맥락·continuity용 ID

### 실행 정보

dual-lane schema v2는 실제로 얻기 어려운 model/effort 추적을 제거한다.

- `RuntimeProvider`: 관측 가능한 provider, 아니면 `unknown`
- `Dispatch상태`: 관측 가능한 dispatch 상태, 아니면 `unknown`
- `실행근거`: 실행 상태를 뒷받침하는 근거
- `Fallback근거`: 실제 fallback이 관측된 경우만 기록

추측으로 채우지 않는다. legacy의 과거 model/effort 필드는 읽기만 하고 신규 항목에는 만들지 않는다.

### Hold

- `대기`: 직접 사유
- `대기유형`: `external` / `user` / `dependency` / `internal`
- `재개조건`: 해제 사건
- `대기근거`: 현재 판단 근거

내부에서 실행 가능한 구현·통합·검증이 남으면 doing이다. stale 경고 자체는 hold 전환 이유가 아니다.

## 5. 상태와 lifecycle

| 상태 | 의미 |
| --- | --- |
| `todo` | 등록됐지만 현재 미배정 |
| `doing` | 특정 Agent와 변경범위로 실행 중 |
| `hold` | 선점 해제 후 실제 외부/사용자/명시 dependency 등을 기다림 |
| `done` | 원래 수용 기준과 결과·검증을 확인했고 archive됨 |

Git의 상태 전환 commit이 lifecycle 시점의 durable 원장이다. Markdown 본문에 등록/착수/완료 시각을 중복
기록하지 않는다.

Web UI와 JSON snapshot은 Git history로 다음을 계산한다.

- Registered / Started / Hold / Resumed / Completed timeline
- Queue Time: 등록 → 최초 doing
- Active Time: 모든 doing 구간의 누적
- Wait Time: 모든 hold 구간의 누적
- Lead Time: 등록 → 완료, 미완료면 현재까지

진행 중 Active/Lead 표시는 브라우저가 기준 시각에서 실시간 증가시키므로 timer 때문에 backlog/Git을
주기적으로 수정하지 않는다.

## 6. Liveness와 Needs Attention

Lifecycle과 worker liveness는 별개다.

```text
Lifecycle: 이 작업이 얼마나 오래 걸렸는가
Activity : worker가 최근 관측되었는가
```

Web UI health:

- `Active`: 최근 runtime heartbeat가 있음
- `Runtime unknown`: runtime registry가 없거나 생존 여부를 확정할 수 없음
- `Quiet`: 설정된 경고 시간보다 오래 observable activity가 없음
- `Stale`: 더 긴 임계값 동안 activity 없음
- `Worker missing`: runtime registry가 존재하는데 doing의 assigned worker가 registry에 없음

`stale != dead`. UI 경고는 advisory이며 자동 todo/hold/done 전환이나 자동 재배정 근거가 아니다.
Controller가 실제 runtime 상태와 작업 특성을 확인한다.

기본 임계값:

```text
TASK_MECCA_STALE_WARN_SECONDS=1800
TASK_MECCA_STALE_CRITICAL_SECONDS=3600
```

### 선택적 runtime registry

`.runtime/agents/*.json`을 ephemeral heartbeat registry로 사용할 수 있다.

```json
{
  "agent": "/root/controller/kkobugi",
  "task_id": "A-143",
  "heartbeat_at": "2026-09-17T13:00:00+09:00",
  "state": "working"
}
```

`.runtime/`은 Git 원장이 아니며 version control에 넣지 않는다. 가능하면 orchestration/runtime layer가
heartbeat를 갱신하고 LLM worker가 heartbeat 유지 작업을 반복하지 않는다.

## 7. Registrar

기본 절차:

```text
preflight
→ ensure-backlog
→ search
→ 필요한 inspect
→ next-id --allow-empty
→ todo 생성
→ inspect
→ doctor
→ ID 반환
```

`ensure-backlog`는 기존 ledger를 우선 사용하고, 아무 ledger도 없을 때만 `_task_mecca/data/backlog/`를 생성한다.

중복/병합 판단이 필요하면 임의 병합하지 않고 `REGISTRATION_CONFLICT`로 Root에 돌린다. todo는 미배정으로
시작한다.

## 8. Controller scheduling

Scheduling 결론 전에는 fresh `coordinate --json`과 실제 live agent 상태를 확인한다. worker를 실제 dispatch하기 직전에는
과거 cache를 신뢰하지 않고 active Full Access preflight를 자동 재실행한다. 권한 확인 전에 `doing + Agent` claim을 만들지 않는다.

- 강한 continuity가 있고 idle/reusable worker가 있으면 우선 재사용한다.
- 서로 독립인 ready와 여유 슬롯이 있으면 첫 worker 배정 후 기다리지 말고 같은 pass에서 가능한 슬롯을 채운다.
- 변경범위가 겹치거나 선행이 미완료면 병렬화하지 않는다.
- dispatch 직전 exact task를 다시 inspect한다.
- Worker DONE/BLOCKED마다 fresh state로 재조율한다.
- `doing + runtime completed`, `doing + worker missing`, `doing + user wait` 조합은 **continuity gap**이다. `coordinate --json`의 `recovery_queue`를 일반 ready보다 먼저 확인하고 finalize, confirmed fresh turn/reassignment, 또는 `hold(user)` + Root escalation 중 하나로 해소한다.
- continuity gap의 worker는 live implementation slot을 점유한 것으로 계산하지 않는다. 남은 slot은 실제 live worker 기준으로 계산한다.
- recovery dispatch는 fresh Full Access preflight와 live agent 상태 재확인 후에만 수행한다.
- `recovery.uncommitted_changes`가 있으면 기존 변경을 보존·검토한 뒤 finalize/reassignment한다.
- Worker의 자기 자신 대상 follow-up은 새 turn 생성이 확인되지 않으면 continuity로 인정하지 않는다.

Controller가 바꿀 수 있는 것은 worker, 병렬화, 구현 순서, 변경범위, 재배정 같은 실행 방법이다. 확정된
요구사항·수용기준·비범위는 바꿀 수 없다.

## 9. Worker identity

Worker 이름은 현재 task가 아니라 재사용되는 identity다. 신규 worker는 `task-mecca worker-name`이 제공하는
정본 포켓몬 pool을 사용한다.

```text
/root/controller/kkobugi
/root/controller/pairi
/root/controller/isanghaessi
```

- 신규: `agent <new-path> --new --json`
- 기존 재사용: `agent <existing-path> --json`
- allocator: `worker-name --used <live path> --json`

기존 old alias는 재사용/충돌검사용 호환으로 보존하고 과거 archive를 rename/backfill하지 않는다.

## 10. 공유 checkout 안전성

여러 worker가 같은 checkout을 공유할 수 있다.

- 병렬 쓰기는 겹치지 않는 파일/디렉터리 범위만 허용한다.
- 같은 파일을 만질 가능성이 있으면 한 worker만 쓴다.
- Worker는 다른 agent나 사용자의 기존 변경을 되돌리지 않는다.
- `변경범위`는 현재 doing claim에만 속한다.

## 11. 완료와 검증

Worker의 DONE 선언만으로 완료하지 않는다. Controller가 해당 task의 원래 수용 기준 각각과 실제 결과·테스트 근거를
대조한다. 여러 기준이면 `## 검증`에 다음처럼 Markdown table을 사용할 수 있다.

```markdown
| 수용 기준 | 결과 | 검증 근거 |
| --- | --- | --- |
| AC-1 | PASS | `pytest ...` |
| AC-2 | PASS | 실제 UI 확인 |
```

기준을 충족한 항목은 전체 queue가 남아 있어도 즉시 done/archive한다.

## 12. Local Web UI

사람용 기본 인터페이스는 terminal TUI가 아니라 local read-only Web UI다.

```bash
task-mecca
# 또는
task-mecca web
```

기본 주소는 `http://127.0.0.1:8765`다. 포트가 사용 중이면 인접 포트를 선택한다.

주요 기능:

- 상태별 목록과 full-text 검색
- Simple Task 작업 정의 / Defined Task 요건 정의를 구분한 contract-first 상세 화면
- Markdown heading/table/checklist/code 렌더링
- 선행/연관/Agent/변경범위
- lifecycle timeline과 live duration
- Needs Attention / doctor issues
- effective Full Access / Restricted / Unknown 전역 표시
- Raw Markdown 확인
- URL `/tasks/<ID>` 직접 접근
- Light/Dark system theme

Web UI는 projection/view다. backlog를 직접 수정하지 않는다. `monitor`는 호환 alias이며 interactive 사용 시
Web UI를 열고 `--json`/`--once`는 기존 automation-friendly snapshot 출력을 유지한다.

## 13. 주요 CLI

```text
preflight      backlog + effective Full Access/dispatch gate
search         후보 검색
inspect        한 항목 상세 구조
next-id        다음 ID
ready          착수 가능한 todo
workload       Agent별 durable workload
coordinate     controller용 fresh scheduling snapshot
audit          상태별 필수 필드
check          문서 링크/dependency
doctor         종합 진단
status         hot set
web            local read-only Web UI
monitor        Web UI 호환 alias / JSON snapshot
```
