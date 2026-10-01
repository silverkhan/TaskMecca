# Task Mecca

**사람이 자연어로 작업을 정의하고, 여러 AI Agent가 durable backlog를 중심으로 협업하도록 만드는 local-first orchestration & observability framework입니다.**

Task Mecca의 목표는 단순히 여러 Agent를 동시에 실행하는 것이 아닙니다.  
사용자는 내부 Agent 구조를 해석하는 데 인지 자원을 쓰지 않고, 자신이 평소 사용하는 언어와 업무 맥락 안에서 작업을 설명하고 진행 상황을 확인할 수 있어야 합니다. Task Mecca는 그 사용자-facing 대화를 **Root**에 집중시키고, 등록·관제·구현 역할을 분리하며, 작업 계약과 lifecycle을 프로젝트 안에 durable하게 남깁니다.

[English README](README.md)

---

## Task Mecca가 해결하려는 문제

Agent 기반 개발이 길어질수록 다음 문제가 반복됩니다.

- 대화 세션 안에만 작업 정의와 결정이 남아 시간이 지나면 맥락을 잃는다.
- 여러 Agent가 동시에 일할 때 누가 무엇을 맡았는지, 어떤 작업이 막혔는지 파악하기 어렵다.
- 구현 Agent가 원래 요구사항을 요약해서 전달받는 과정에서 의미가 축소되거나 변형된다.
- “작업 중”, “완료” 같은 상태가 대화상 표현에 머물러 실제 lifecycle 근거가 약하다.
- Linear, GitHub Issue 같은 외부 업무 원천에서 시작한 작업이 Agent 실행 과정과 분리된다.
- 사용자에게 내부 Agent 용어와 영문 구조가 그대로 노출되어 읽는 비용이 커진다.

Task Mecca는 이를 다음 구조로 해결합니다.

1. **Root**가 사용자와의 단일 대화 창구가 됩니다.
2. 작업을 **Simple / Defined Task** 계약으로 명확히 합니다.
3. **Registrar**가 의미를 바꾸지 않고 durable backlog에 등록합니다.
4. **Controller**가 dependency, continuity, worker 상태를 보고 실행을 조율합니다.
5. **Worker**는 전달 요약이 아니라 backlog 원문 계약을 직접 읽고 구현합니다.
6. 상태 전환과 결과·검증을 backlog와 Git lifecycle에 남깁니다.
7. Web UI에서 backlog, lifecycle, workload, attention signal을 관찰합니다.
8. Linear / GitHub Issue 등 외부 원천이 있으면 출처와 결과를 양방향으로 연결합니다.

---

## 핵심 구조

```text
사용자
  ↕
/root                         사용자-facing · 작업 정의 소유자
  ├─ /root/registrar         lossless registration
  └─ /root/controller        scheduling / orchestration
       ├─ /root/controller/<worker>
       ├─ /root/controller/<worker>
       └─ ...                bounded implementation workers

                    ↓
       Markdown backlog + Git lifecycle
                    ↓
            Task Mecca Web UI
```

Task Mecca 자체가 특정 Agent runtime을 대체하지는 않습니다. Agent 생성·메시지·대기는 Codex, Claude 등 현재 사용하는 runtime의 협업 기능이 담당하며, Task Mecca는 **작업 계약, durable state, scheduling context, 관제 surface**를 제공합니다.

### 역할

| 역할 | 책임 |
|---|---|
| **Root** | 사용자와 대화, 작업 정의, Simple/Defined 분류, 사용자 판단 요청, 외부 이슈 동기화 |
| **Registrar** | 확정된 계약을 의미 변경 없이 backlog에 등록 |
| **Controller** | dependency/readiness 확인, worker 배정, 병렬 실행, continuity/recovery, 완료 검증 |
| **Worker** | backlog의 canonical contract를 직접 읽고 bounded implementation 수행 |

Worker는 별도 alias 계층을 두지 않으며 Task Mecca의 worker identity 자체를 사용합니다. 프로젝트의 현재 운영 규약은 설치된 `_task_mecca/framework/roles/` 문서가 정의합니다.

---

## 작업 계약: Simple Task와 Defined Task

Task Mecca는 작업 크기가 아니라 **추가 해석 없이 바로 검증 가능한가**를 기준으로 두 lane을 구분합니다.

### Simple Task

목표와 완료 조건이 이미 명확한 bounded 작업입니다.

예:

```text
README의 잘못된 명령어 표기 하나를 바로잡아 주세요.
```

Backlog에는 최소 계약만 둡니다.

```markdown
## 작업 정의
### 목표
### 수용 기준
```

불필요한 요건 정의나 재확인 round-trip을 만들지 않습니다.

### Defined Task

범위, 설계, 제품 동작, 사용자 선택, 복수 수용 기준의 합의가 필요한 작업입니다.

예:

```text
백로그 상세 UX를 개선하고 싶은데
요약, 접기/펼치기, 기존 backlog 호환성까지 같이 검토하고 싶습니다.
```

Root가 필요한 만큼 논의한 뒤 확정된 내용을 다음 계약으로 남깁니다.

```markdown
## 요건 정의서
### 배경 및 문제
### 목표
### 요구사항
### 범위
#### 포함
#### 제외
### 수용 기준
### 제약 및 보존 조건
```

사용자 확인이 끝난 뒤 Registrar가 등록합니다.

---

## Durable backlog

신규 프로젝트의 canonical backlog 위치는 다음과 같습니다.

```text
_task_mecca/data/backlog/
```

Installer는 빈 `data/`나 backlog를 미리 만들지 않습니다. 첫 작업을 등록할 때 Registrar가 필요에 따라 생성합니다.

예:

```text
_task_mecca/
├── VERSION
├── manifest.json
├── ROOT_PROMPT.md
├── framework/
│   ├── SESSION_GUIDE.md
│   ├── collab.md
│   ├── HUMAN_READABLE_BACKLOG.md
│   ├── TAGS.md
│   ├── roles/
│   └── web/
├── data/
│   ├── backlog/
│   │   ├── 000143.A-143.example.todo.md
│   │   └── archive/
│   │       └── 2026-09/
│   └── tags/
│       └── registry.json
├── .runtime/
└── backups/
```

기존 프로젝트의 `backlog`, `backlog_b`, `backlog_*` 같은 ledger도 호환을 위해 탐색합니다.

### Framework와 project data의 경계

| 영역 | 소유권 | 업데이트 정책 |
|---|---|---|
| `_task_mecca/framework/**` | Task Mecca framework | migrator 관리 |
| `_task_mecca/ROOT_PROMPT.md` | managed/customizable | upstream/local 충돌 시 보호 |
| `_task_mecca/data/**` | 프로젝트/Agent durable data | migrator가 덮어쓰지 않음 |
| legacy `_task_mecca/backlog*/**` | 프로젝트 data | 호환용, framework update 대상 아님 |
| `_task_mecca/.runtime/**` | ephemeral runtime state | Git durable source가 아님 |
| `_task_mecca/backups/**` | migration safety copy | framework 관리 대상 아님 |

이 경계를 통해 repository clone만으로 Task Mecca 운영 규약과 프로젝트의 durable 작업 상태를 함께 보존할 수 있습니다.

---

## 사람이 읽기 편한 backlog

Backlog는 Agent용 execution record이면서 동시에 사람이 빠르게 판단할 수 있는 문서여야 합니다.

Task Mecca는 canonical contract와 별도로 `## 핵심 요약`을 사용합니다.

- **목적**: 왜 필요한 작업인지
- **핵심 변경**: 무엇을 바꾸는지 / 실제 무엇이 바뀌었는지
- **상태·결과**: 현재 단계 또는 완료 결과
- **확인·후속**: 차단, 승인, 중요한 미검증, 후속 작업이 있을 때만

긴 상세 section에는 필요할 때만 의미 요약을 둘 수 있습니다.

```markdown
> 요약: 이 섹션 전체를 읽지 않아도 핵심 판단이 가능하도록 의미를 요약한다.
```

- 의미 요약이 있으면 Web UI가 요약을 유지한 채 상세 본문만 접고 펼칩니다.
- 요약이 없으면 접기 UI를 만들지 않고 본문을 바로 보여줍니다.
- 짧거나 명확한 section에 요약을 억지로 만들지 않습니다.

또한 사용자-facing 자연어는 사용자가 실제로 쓰는 언어와 용어 감각을 우선합니다. 정확한 코드, 경로, 식별자, 전문용어는 유지할 수 있지만 내부 영문 어순이나 Agent 축약 표현을 그대로 사용자 문장에 섞는 것을 피합니다.

---

## 외부 이슈와 연결된 작업

Linear, GitHub Issue 등 외부 업무 항목에서 시작된 작업은 **Source-linked Task**로 다룹니다.

```text
Linear / GitHub Issue
        ↓
Root가 연결된 MCP/도구로 원문과 현재 상태 확인
        ↓
사용자와 범위·설계·수용 기준 논의
        ↓
Task Mecca backlog에 출처 + 실행 계약 등록
        ↓
Controller / Worker 실행
        ↓
중요한 결정·상태 전환·완료 결과를 외부 원천에도 반영
```

Backlog에는 가능한 경우 클릭 가능한 출처를 남깁니다.

```markdown
- 출처: [Linear · ENG-123](https://linear.app/...)
- 출처: [GitHub · owner/repo#84](https://github.com/owner/repo/issues/84)
```

Task Mecca는 외부 issue tracker를 복제하지 않습니다.

- **외부 이슈**: 사람/조직이 공유하는 협업 원천
- **Task Mecca backlog**: Agent가 실제 실행하는 현재의 확정 계약과 lifecycle
- **Root**: 중요한 합의, 의미 있는 상태 전환, 완료 결과를 양쪽에 맞춰 반영

외부 이슈가 나중에 수정됐다고 Task Mecca 계약을 자동 덮어쓰지 않습니다. 변경이 현재 범위나 수용 기준에 영향을 주면 Root가 차이를 해석하고 필요한 경우 사용자와 재합의합니다.

외부 status 이름도 `todo = Todo`처럼 하드코딩하지 않습니다. 연결된 시스템의 실제 workflow를 확인해 의미적으로 대응되는 상태를 선택합니다.

---

## Lifecycle과 관제

Backlog 상태의 durable source는 파일명입니다.

```text
<정렬키>.<ID>.<slug>.<todo|doing|hold|done>.md
```

| 상태 | 의미 |
|---|---|
| `todo` | 등록됐지만 현재 미배정 |
| `doing` | 특정 Worker가 실제 실행 중 |
| `hold` | 사용자/외부 dependency 등 실제 대기 상태 |
| `done` | 수용 기준 검증과 결과 기록이 끝난 상태 |

Git의 상태 전환 commit과 runtime observation을 이용해 다음 시간을 계산합니다.

- **Queue Time**: 등록 → 최초 doing
- **Active Time**: 모든 doing 구간의 누적
- **Wait Time**: 모든 hold 구간의 누적
- **Lead Time**: 등록 → 완료 또는 현재

Task Mecca는 lifecycle과 worker liveness를 분리합니다. `Quiet`, `Stale`, `Worker missing`은 관제 신호이지 자동 상태 전환 명령이 아닙니다.

Controller는 dependency, 변경범위 충돌, worker continuity를 함께 보고 가능한 ready 작업을 병렬로 채우며, completed/missing worker가 붙은 `doing` 같은 continuity gap도 복구 대상으로 다룹니다.

---

## Full Access preflight

실행형 작업에서 Task Mecca는 제한된 권한 상태로 Worker를 먼저 띄워 실패를 관찰하는 방식을 기본으로 하지 않습니다.

Worker dispatch 직전:

```bash
task-mecca preflight --require-full-access --json
```

을 통해 effective Full Access를 fresh probe로 확인합니다.

- 통과하면 dispatch
- restricted/unknown이면 Worker를 생성하거나 doing claim을 만들지 않음
- 사용자에게 필요한 권한 조치를 요청
- 권한 변경 후 다시 fresh preflight

Web에 표시되는 과거 Full Access 결과는 참고 정보일 뿐 dispatch 승인이 아닙니다.

---

## Project taxonomy / Tags

Task Mecca는 project-owned taxonomy를 지원합니다.

```text
_task_mecca/data/tags/registry.json
```

Backlog의 `Tags`는 상태·dependency·Agent assignment를 중복 표현하기 위한 것이 아니라 업무 의미를 분류하기 위한 것입니다.

주요 동작:

```bash
task-mecca tags list
task-mecca tags search <query>
task-mecca tags resolve <query>
task-mecca tags define <namespace:name> [description] [aliases]
task-mecca tags tasks <expression>
task-mecca tags stats
```

새로운 의미가 필요하면 신규 canonical tag를 정의할 수 있습니다. 기존 canonical tag만 강제하지 않으며, 중복과 표기 분산을 막는 방향으로 운영합니다.

---

## Web UI

프로젝트 루트에서:

```bash
task-mecca web
```

을 실행합니다.

Task Mecca Web은 사용자 단위 singleton background service로 동작하며 기본 포트는 `18765`입니다.

### 제공 기능

- backlog 자동 탐색 및 수동 전환
- active + `archive/YYYY-MM/` 통합 조회
- Simple / Defined / legacy backlog rendering
- 핵심 요약, 출처 링크, 관련 작업, 실행 근거, Lifecycle
- Queue / Active / Wait / Lead Time
- dependency/readiness와 Controller coordination 정보
- Subagent Workload
- Needs Attention / stale / worker-missing signal
- 프로젝트 taxonomy / tags 조회
- Global Hub에서 등록 프로젝트와 framework 상태 확인
- CLI update / project framework migration surface
- Light / Dark / System theme
- 한국어 / English UI와 사용자 매뉴얼
- Markdown table, code copy, Mermaid
- floating TOC
- 화면 높이 기반 자동 page size
- 키보드 탐색
- 내용 변경 시 읽던 위치를 방해하지 않고 refresh 안내
- HTTPS secure origin에서 browser notification 사용

Backlog 본문 자체를 Web 편집기로 다루기보다는 durable Markdown/Git 원장을 관찰하는 surface로 사용합니다. Upgrade/Migrate 같은 maintenance action은 명시적인 사용자 동작으로 수행됩니다.

### Web service 제어

```bash
task-mecca web status
task-mecca web restart
task-mecca web stop
task-mecca web logs
task-mecca web logs --follow
task-mecca web --foreground
```

### Tailscale HTTPS

기본 `--host auto`에서는 로컬 HTTP를 제공하고, Tailscale이 감지되며 MagicDNS와 HTTPS Certificates가 활성화되어 있으면 같은 포트에 직접 HTTPS endpoint를 제공합니다.

예:

```text
Local      http://127.0.0.1:18765/
Tailscale  https://<machine>.<tailnet>.ts.net:18765/
```

별도의 `tailscale serve` 없이 동작하도록 설계되어 있습니다.

---

## 설치

### 요구 환경

- Git
- Windows / macOS / Linux
- 최종 사용자에게 Python, `uv`, Go toolchain 불필요

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.sh | sh
```

프로젝트 루트에서:

```bash
task-mecca init
```

### Windows

rolling release의 Windows standalone binary(`task-mecca-windows-amd64.exe`)를 내려받아 `task-mecca.exe`로 사용할 수 있는 PATH 위치에 둡니다.

Go나 Python runtime을 별도로 설치할 필요가 없습니다.

---

## 첫 사용

### 1. 프로젝트 초기화

```bash
task-mecca init
```

설치 직후에는 framework와 metadata만 생성됩니다. `data/backlog/`가 없는 것이 정상입니다.

### 2. Web 실행

```bash
task-mecca web
```

첫 backlog가 없어도 Web을 먼저 실행할 수 있습니다.

### 3. Root 세션 활성화

Task Mecca 설치와 Root 역할 부여는 별개입니다.

```text
프로젝트에 Task Mecca가 설치되어 있음 ≠ 현재 세션이 Root임
```

Root로 사용할 **한 개의 사용자-facing AI 세션**을 직접 선택합니다.

Web의 **User Manual → 빠른 시작 → Root 세션 프롬프트**에서 실제 프롬프트를 바로 복사할 수 있습니다.

파일에서 확인하려면:

```text
_task_mecca/ROOT_PROMPT.md
```

해당 세션에 프롬프트를 전달한 뒤 자연어로 작업을 요청하면 됩니다.

---

## 업데이트와 마이그레이션

CLI 자체 업데이트:

```bash
task-mecca upgrade
```

Web에서도 업데이트 가능 상태를 감지하고 upgrade action을 제공합니다.

- macOS/Linux에서 CLI upgrade 시 실행 중인 Task Mecca Web이 있으면 새 binary로 자동 재시작합니다.
- Windows에서는 실행 중인 binary 교체 제약 때문에 Web을 안전하게 종료한 뒤 재실행 안내를 제공합니다.
- CLI 자체는 별도 “restart” 개념이 없습니다. 교체 후 다음 명령부터 새 binary를 사용합니다.

프로젝트 안의 managed framework를 새 버전으로 맞출 때는:

```bash
task-mecca migrate
```

또는 Web Global Hub의 migration action을 사용합니다.

`ROOT_PROMPT.md`, `SESSION_GUIDE.md`, `collab.md`, `roles/*.md`처럼 현재 세션 행동에 영향을 주는 지침이 바뀌면 Web이 **Root session resync** 필요 여부와 복사 가능한 재동기화 프롬프트를 제공합니다.

Project-owned `data/**`는 migrate가 덮어쓰지 않습니다.

---

## 자주 쓰는 CLI

| 명령 | 용도 |
|---|---|
| `task-mecca init` | 현재 프로젝트에 Task Mecca framework 설치 |
| `task-mecca upgrade` | standalone CLI 업데이트 |
| `task-mecca migrate` | 프로젝트 managed framework 업데이트 |
| `task-mecca web` | Web dashboard 실행 |
| `task-mecca web status` | Web 상태 확인 |
| `task-mecca status` | backlog 상태 요약 |
| `task-mecca inspect <ID>` | 특정 작업 상세 확인 |
| `task-mecca ready` | 실행 가능한 작업 확인 |
| `task-mecca coordinate` | scheduling snapshot 확인 |
| `task-mecca workload` | Agent workload 확인 |
| `task-mecca doctor` | 구조/계약/링크 등 종합 점검 |
| `task-mecca preflight --require-full-access` | dispatch 전 권한 점검 |
| `task-mecca tags ...` | taxonomy 조회/관리 |

대부분의 사용자는 CLI 명령을 직접 조합하기보다 Root와 자연어로 소통하고, Root/Agent가 deterministic primitive를 이용하도록 두는 방식이 기본입니다.

---

## 내부 매뉴얼

설치된 프로젝트 안에는 README보다 더 세밀한 운영 계약이 함께 배포됩니다.

| 문서 | 내용 |
|---|---|
| `_task_mecca/ROOT_PROMPT.md` | Root 세션 활성화 프롬프트 |
| `framework/SESSION_GUIDE.md` | 사용자 Quick Start + 운영 규칙 |
| `framework/collab.md` | 역할, 계약, lifecycle, source-linked task 등 협업 규약 |
| `framework/HUMAN_READABLE_BACKLOG.md` | 사람이 읽기 편한 backlog 작성 원칙 |
| `framework/TAGS.md` | taxonomy / tag 운영 규칙 |
| `framework/roles/*.md` | Root / Registrar / Controller / Worker 역할 계약 |
| `framework/_template.md` | canonical backlog template |

같은 내용은 Web의 **User Manual**에서도 확인할 수 있습니다.

---

## 프로젝트의 시작과 감사

Task Mecca는 제 동료인 [gyusu](https://github.com/gyusu)로부터 받은 아이디어와 영감에서 시작되었습니다.

초기에는 백로그를 등록하는 세션과 백로그를 수행하는 Worker 세션을 분리하고, Worker가 `/goal` 방식으로 남아 있는 백로그를 계속 선택·수행하여 백로그가 소진될 때까지 작업을 이어가는 AI 협업 구조에 대한 아이디어를 공유받았습니다. 또한 이러한 방식을 실제로 탐색하고 구현하는 데 참고할 수 있는 소스도 함께 제공받았습니다.

이 아이디어는 Task Mecca를 만들게 된 중요한 출발점이 되었습니다.

이후 Task Mecca는 이 초기 개념을 바탕으로 Root, Registrar, Controller, Worker의 역할 체계, 작업 관제와 병렬 실행, 백로그 생명주기 관리, Web UI, 외부 업무 원천 연결, 설치 및 배포 체계 등으로 지속적으로 확장되며 현재의 구조로 발전했습니다.

초기의 아이디어와 영감을 나누어 준 [gyusu](https://github.com/gyusu)에게 감사드립니다.

---

## 개발

현재 standalone Go runtime이 최종 사용자 배포의 중심이며 Python package는 호환 경로를 유지합니다.

Go:

```bash
go test ./...
go build ./cmd/task-mecca
```

Python compatibility:

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

기여 방법은 [CONTRIBUTING.md](CONTRIBUTING.md)를 참고하세요.

---

## 라이선스

MIT License. [LICENSE](LICENSE)를 참고하세요.
