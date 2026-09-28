# Changelog

All notable public releases will be documented here.

## 0.2.1 - 2026-09-28

- make Python 3.11+ and Git the core requirements; `uv` / `uvx` are optional conveniences only
- document standard `python -m pip` + `python -m task_mecca` bootstrap/update paths
- make project-local `python _task_mecca/framework/collab_tools.py ...` the canonical runtime invocation
- remove `uvx ... web` from normal Web UI guidance so installed projects run fully locally without package download or network access
- update Root/session documentation and CI to validate the Python-first workflow

## 0.2.0 - 2026-09-28

- physically separate Task Mecca framework files under `_task_mecca/framework/`
- reserve `_task_mecca/data/**` for durable project/agent-owned data
- stop creating backlog data during installation
- make Registrar create canonical `_task_mecca/data/backlog/` on first registration
- keep discovery compatibility for existing ledger names beginning with `backlog`
- preserve pre-0.2 Git lifecycle history when a ledger moves from `_task_mecca/backlog_name` to `_task_mecca/data/backlog_name`
- add `task-mecca web` as a launcher for the project-contained runtime
- keep `.runtime/**` and `backups/**` outside durable project data

## 0.1.1 - 2026-09-28

- stop creating, modifying, or prompting to append Task Mecca instructions to project-level `AGENTS.md`
- make Root activation explicit and session-scoped through `_task_mecca/ROOT_PROMPT.md`
- document the invariant: installing Task Mecca does not make every project session Root
- safely retire the 0.1.0 `AGENTS_TASK_MECCA_SNIPPET.md` managed file during update

## 0.1.0 - 2026-09-28

Initial public alpha.

- project-contained Task Mecca runtime based on the internal v2.17 line
- `task-mecca init` bootstrap command
- interactive, backup-first `task-mecca update`
- version/manifest baseline tracking
- explicit framework/customizable/project-owned/runtime file boundaries
- local Web UI with backlog discovery, archive history, lifecycle timing, workload, i18n, Mermaid, themes, TOC, adaptive paging, and keyboard navigation
- Simple / Defined task contracts with legacy backlog compatibility
- CI and regression-test foundation
