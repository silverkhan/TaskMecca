# Changelog

All notable public releases will be documented here.

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
