# Task Mecca

**멀티 에이전트 코딩 워크플로를 위한 local-first orchestration & observability 도구입니다.**

Task Mecca는 orchestration framework와 프로젝트가 축적하는 durable data를 프로젝트 내부에서 **물리적으로 분리**합니다. Git clone만으로 해당 프로젝트가 사용한 Task Mecca 규약과 백로그 상태를 함께 재현하는 것을 목표로 합니다.

> Public alpha: `0.2.1`. 프로젝트 내장 runtime은 현재 Task Mecca v2.17 계열을 기반으로 하며 공개용 설치/업데이트 계층을 추가했습니다.

[English README](README.md)

## 빠른 시작

### 0. Framework 설치

Task Mecca의 필수 환경은 **Git**입니다. 최종 사용자는 Python, `uv`, Go toolchain을 설치할 필요가 없습니다.

macOS 또는 Linux에서는 저장소 주소를 이용해 한 줄로 standalone CLI를 설치할 수 있습니다.

```bash
curl -fsSL https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.sh | sh
```

설치 후 현재 프로젝트에서 다음을 실행합니다.

```bash
task-mecca init
```

이후 CLI 자체를 최신 main 빌드로 갱신할 때는 같은 설치 명령을 다시 실행하고, 현재 프로젝트 framework를 갱신합니다.

```bash
curl -fsSL https://raw.githubusercontent.com/silverkhan/TaskMecca/main/install.sh | sh
task-mecca update
```

설치 시에는 Task Mecca framework와 설치 메타데이터만 만듭니다. **프로젝트 data와 backlog는 만들지 않으며**, 프로젝트 최상위 `AGENTS.md`도 생성하거나 수정하지 않습니다.

설치 직후 구조:

```text
_task_mecca/
├── VERSION
├── manifest.json
├── ROOT_PROMPT.md
├── framework/
│   ├── README*.md
│   ├── SESSION_GUIDE*.md
│   ├── collab.md
│   ├── _template.md
│   ├── roles/
│   └── web/
├── .runtime/        # 필요해질 때 생성
└── backups/         # updater 백업이 필요할 때 생성
```

설치 직후에는 `data/`가 없는 것이 정상입니다.

### 1. Root 세션 명시적 활성화

```text
프로젝트에 Task Mecca가 설치되어 있음 ≠ 현재 세션이 Root임
```

Root로 사용할 한 개의 user-facing 세션을 직접 선택하고 다음 파일의 한국어 프롬프트를 붙여넣습니다.

```text
_task_mecca/ROOT_PROMPT.md
```

명시적으로 활성화한 해당 세션만 `/root` 역할을 수행합니다.

### 2. 작업 부여와 첫 backlog 생성

Root에게 자연어로 작업을 지시합니다. 즉시 검증 가능한 작업은 **Simple Task**, 범위·설계·사용자 선택이 필요한 작업은 **Defined Task**로 처리합니다.

처음으로 작업을 등록할 때 Registrar가 canonical project-owned ledger를 생성합니다.

```text
_task_mecca/data/backlog/
```

Installer가 미리 만들지 않습니다. 기존 `backlog_b`, `backlog-team`처럼 이름이 `backlog`로 시작하는 원장은 호환성을 위해 계속 탐색합니다.

### 3. Web UI 실행

standalone CLI를 실행합니다.

```bash
task-mecca web
```

이 명령은 embedded read-only Web UI를 localhost에서 직접 서비스하며 Python, `uv`, 네트워크 연결을 요구하지 않습니다.

Web UI는 read-only / localhost-only입니다. 아직 첫 backlog가 생성되지 않은 상태에서도 실행되며 uninitialized 상태를 표시할 수 있습니다.

## Framework와 Data의 물리적 경계

| 영역 | 소유권 | 업데이트 정책 |
|---|---|---|
| `_task_mecca/framework/**` | Task Mecca framework | updater가 관리 |
| `_task_mecca/ROOT_PROMPT.md` | managed/customizable | 로컬·upstream 모두 변경 시 백업+확인 |
| `_task_mecca/data/**` | 프로젝트/agent durable data | updater가 절대 덮어쓰지 않음 |
| legacy `_task_mecca/backlog*/**` | 기존 프로젝트 data | 호환용, updater가 건드리지 않음 |
| `_task_mecca/.runtime/**` | 임시 runtime state | durable data가 아님 |
| `_task_mecca/backups/**` | updater 안전 백업 | framework 관리 대상 아님 |

Agent가 특정 작업을 수행하면서 measurement, audit evidence 등 durable 부산물이 실제로 필요하면 `data/` 아래에 만들 수 있습니다. Task Mecca가 임의의 `audits/`, `measurements/`, `tests/` 폴더를 미리 생성하거나 표준으로 강제하지는 않습니다.

## Backlog 규칙

새 프로젝트의 canonical 위치는 하나입니다.

```text
_task_mecca/data/backlog/
```

첫 등록 시 Registrar가 생성합니다. 탐색은 기존 프로젝트 호환을 위해 `backlog`로 시작하는 다른 이름도 허용합니다.

기존 프로젝트는 basename을 보존한 채 다음처럼 이관할 수 있습니다.

```text
_task_mecca/backlog_b/
    ↓
_task_mecca/data/backlog_b/
```

0.2 runtime은 현재 ledger가 `data/` 아래에 있을 때 같은 basename의 pre-0.2 경로도 Git lifecycle history path로 함께 조회합니다. 따라서 과거 `todo → doing → hold → done` 이력을 이어서 계산할 수 있습니다.

## 업데이트

새 Task Mecca standalone 실행파일로 교체한 뒤 managed framework를 갱신합니다.

```bash
task-mecca update
```

새 binary를 가져오는 과정에는 사용자가 선택한 네트워크/파일 전송 수단이 필요하지만, Web UI, doctor, preflight, backlog 작업 등 평상시 runtime 명령은 standalone binary와 프로젝트 파일만으로 실행됩니다.

managed file의 로컬 수정이 새 버전과 충돌하면 Task Mecca가 수정 파일 목록을 보여주고, 백업을 권장하고, 동의 시 `_task_mecca/backups/<timestamp>/`에 보존한 뒤 덮어쓰기/퇴역 내용을 다시 알리고 최종 확인을 받습니다.

## 주요 기능

- backlog 자동 탐색 및 수동 전환
- `archive/YYYY-MM/` 이력 재귀 조회
- Simple / Defined / legacy backlog 호환
- Lifecycle, Queue / Active / Wait / Lead Time
- 0.2 data-layout migration 전후 Git lifecycle 이력 호환
- dependency/readiness 검사 및 Controller coordination snapshot
- Subagent Workload, stale/worker-missing 관제
- dispatch 직전 Full Access preflight
- Light / Dark / System 테마
- 한국어 / 영어 UI 및 매뉴얼
- Markdown table / 코드 복사 / Mermaid
- 화면 높이 기반 자동 페이지 크기와 키보드 탐색

## 요구 환경

- Git
- Windows, macOS 또는 Linux용 standalone `task-mecca` binary

프로젝트에 설치되는 runtime 자체에는 별도 Python third-party dependency가 없습니다.

## 라이선스

MIT. [LICENSE](LICENSE)를 참고하세요.
