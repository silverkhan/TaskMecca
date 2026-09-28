# Task Mecca

**멀티 에이전트 코딩 워크플로를 위한 local-first orchestration & observability 도구입니다.**

Task Mecca는 요건, 백로그 상태, worker 배분, lifecycle 시간, 로컬 Web UI를 프로젝트 안에 함께 둡니다. 중앙 서비스보다 Git clone만으로 운영 규약과 상태를 재현하는 것을 우선합니다.

> Public alpha: `0.1.0`. 프로젝트 내장 runtime은 현재 Task Mecca v2.17을 기준으로 합니다.

[English README](README.md)

## 빠른 시작

### 0. 프로젝트에 설치

PyPI 공개 전에는 GitHub 저장소에서 직접 실행합니다.

```bash
uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca init
```

PyPI 공개 이후 목표 사용법은 더 짧습니다.

```bash
uvx task-mecca init
```

`uvx`는 설치/업데이트 계층일 뿐이며, 실제 Task Mecca runtime은 계속 프로젝트의 `_task_mecca/`에 존재합니다.

### 1. 대시보드 실행

```bash
uv run _task_mecca/collab_tools.py web
```

### 2. Root 설정

`task-mecca init` 과정에서 프로젝트 `AGENTS.md`에 Task Mecca bootstrap 블록을 추가할 수 있습니다. 건너뛴 경우 `_task_mecca/AGENTS_TASK_MECCA_SNIPPET.md` 내용을 프로젝트 `AGENTS.md`에 반영합니다.

### 3. 작업 부여

별도 명령 문법 없이 자연어로 작업을 지시합니다.

```text
README의 잘못된 실행 명령을 수정해줘.
```

즉시 검증 가능한 작업은 **Simple Task**, 범위/설계/사용자 선택이 필요한 작업은 **Defined Task**로 처리합니다. Defined Task는 요건정의와 사용자 확인 후 등록됩니다.

## 업데이트

사용자가 기억할 업데이트 명령은 하나입니다.

```bash
uvx --from git+https://github.com/silverkhan/TaskMecca.git task-mecca update
```

PyPI 공개 이후:

```bash
uvx task-mecca update
```

업데이트가 사용자가 수정한 managed file을 덮어쓰게 되는 경우 Task Mecca가 자동으로:

1. 수정 감지 파일 목록을 보여주고,
2. 업데이트 전 백업을 권장하고,
3. 동의 시 `_task_mecca/backups/<timestamp>/`에 백업을 만든 뒤,
4. 해당 프로젝트 수정본이 공식 새 버전으로 덮어써진다는 점을 명확히 알리고,
5. 최종 진행 여부를 다시 확인합니다.

사용자가 `--force`, `--backup` 같은 옵션을 외울 필요가 없도록 설계합니다.

## 파일 관리 경계

| 영역 | 예시 | 업데이트 정책 |
|---|---|---|
| **Framework managed** | `collab_tools.py`, `runtime_metadata.py`, `web/*` | upstream 변경 시 자동 업데이트 |
| **Customizable managed** | `roles/*`, `SESSION_GUIDE*.md`, `collab.md`, `_template.md`, 매뉴얼 | 로컬/업스트림이 모두 변경되면 백업 + 확인 후 덮어쓰기 |
| **Project owned** | `backlog_*`, archive 내용, 프로젝트 설정 | updater가 절대 덮어쓰지 않음 |
| **Runtime/backup** | `.runtime/*`, `backups/*` | 로컬 임시/안전 데이터, 기본 Git 제외 |

사용자가 customizable file을 수정했더라도 새 버전에서 그 파일이 upstream 변경되지 않았다면 로컬 수정본을 그대로 유지합니다.

설치된 `_task_mecca/manifest.json`에는 버전과 baseline hash가 기록되어 이 판단에 사용됩니다.

## 주요 기능

- 백로그 폴더 자동 탐색 및 수동 전환
- `archive/YYYY-MM/` 재귀 조회
- 상태 다중 필터와 키보드 탐색
- ID/Updated 정렬, 화면 높이 기반 자동 페이지 크기
- Simple / Defined / legacy 백로그 호환
- Lifecycle, Queue / Active / Wait / Lead Time
- Subagent Workload 및 stale/worker-missing 관제
- dispatch 직전 Full Access preflight
- Light / Dark / System 테마
- 한국어 / 영어 UI 및 매뉴얼
- Markdown table / 코드 복사 / Mermaid
- localhost-only read-only Web UI

## 요구 환경

- Python 3.11+
- Git
- `uv` 권장

프로젝트에 설치되는 runtime 자체에는 별도 Python third-party runtime dependency가 없습니다.

## 라이선스

MIT. [LICENSE](LICENSE)를 참고하세요.
