# Contributing

Thanks for contributing to Task Mecca.

## Principles

- Keep the installed runtime project-contained and Git-friendly.
- Do not move project-owned backlog data into a central database.
- Preserve backward compatibility for legacy backlog formats when practical.
- Keep the Web UI localhost-only and read-only unless a future design explicitly changes that boundary.
- Prefer standard-library Python for the installed runtime.

## Development setup

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
```

## Changing managed template files

Files under `src/task_mecca/template/_task_mecca/` become the runtime installed into user projects. The updater records baseline hashes for these files. When adding a new file, decide whether it is framework-managed, customizable-managed, or project-owned and update `src/task_mecca/installer.py` when necessary.

## Pull requests

Task Mecca 저장소의 PR 제목과 본문은 **한국어를 기본 언어**로 작성합니다. 코드 식별자, API, 파일 경로,
제품명처럼 정확성을 위해 필요한 고유명사는 원문 표기를 유지할 수 있습니다.

제목은 한글 대괄호 접두어 대신 널리 통용되는 영문 유형을 사용하고, 본문 제목 부분은 한국어로 작성합니다.

```text
TYPE: 한국어 제목
```

권장 유형:

- `DOC:` 문서 변경
- `FEAT:` 기능 추가 또는 사용자-visible 동작/UX 개선
- `FIX:` 오류 수정
- `REFACTOR:` 외부 동작을 바꾸지 않는 구조 개선
- `TEST:` 테스트 추가·정비
- `PERF:` 성능 개선
- `CI:` 빌드·배포·자동화 변경
- `CHORE:` 위 분류에 속하지 않는 유지보수 작업
- `WIP:` 아직 병합 대상이 아닌 진행 중 작업

예:

```text
DOC: README 빠른 시작 가이드 개선
FEAT: 백로그 상태 변화 안내 추가
FIX: Web 업그레이드 후 재시작 실패 수정
```

PR 본문에는 필요한 범위에서 다음 내용을 포함합니다.

- 해결하려는 문제와 배경
- 주요 변경 사항
- 호환성 영향
- 관련 검증과 테스트 결과
- CLI/UI 변경이 있는 경우 사용자-facing 문서 변경 사항

사용자가 별도로 다른 언어를 요청하지 않는 한 PR 전체를 영어로 작성하지 않습니다.


## Installed ownership boundary

The installed template has a physical ownership boundary:

- `_task_mecca/framework/**`: framework-managed or customizable-managed distribution files.
- `_task_mecca/data/**`: durable project/agent-owned data; never add these files to the package template or updater inventory.
- `_task_mecca/.runtime/**`: ephemeral runtime state.
- `_task_mecca/backups/**`: local updater safety copies.

The installer must not create project data. Canonical `data/backlog/` is created by Registrar on first task registration.
