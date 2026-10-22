# Changelog

All notable public releases will be documented here.

## 0.3.3 - 2026-10-22 — Remote Web Terminal Rendering and Recovery

- **Remote terminal fix:** prevent the PTY surface from collapsing when optional status rows are hidden; restore the shell prompt and cursor over Tailscale HTTPS.
- **Diagnostics:** distinguish PTY output from browser streaming and display errors visibly.
- **Recovery:** add a basic-input switch and bounded, authenticated output replay when long-lived streams are buffered.
- **Tests:** cover explicit Grid placement, output replay offsets and terminal recovery controls.
- **After upgrading:** restart the Web service with the updated binary, verify Tailscale shell input/output, and confirm the fallback control if needed. No framework-instruction refresh is required for this patch.

Detailed bilingual release notes: [0.3.3](release-notes/0.3.3.json).

## 0.3.2 - 2026-10-20 — Agent CLI Operations, Handoff and Recovery Diagnostics

- Standardize contract validation, planning, registration and approval tracking.
- Add runtime query/diagnose, Codex native evidence counters and focused Controller handoff summaries.
- Provide safe worktree preservation and project file integrity verification.
- Guide per-project framework migration and role-specific operations after upgrading.

Detailed bilingual release notes: [0.3.2](release-notes/0.3.2.json).

## 0.3.1 - 2026-10-16 — Backlog, Lifecycle and Web Experience

- **Faster backlog:** prioritize initial list/detail rendering and reduce repeated Git-history lookups.
- **Stable layout:** prevent page-size recalculation loops and first-load screen jitter.
- **Lifecycle reliability:** discard stale refresh responses and improve retry and recovery.
- **Attention & interface:** avoid lingering resolved warnings and repeated notices; preserve floating attention styling.
- **Update safety:** reduce duplicate update checks and strengthen asset validation and OS prepublish checks.
- **After upgrading:** verify real desktop/mobile backlog first load, lifecycle history, and attention behavior.

### Contributors & development (Dev-only)

- Docker-free native PR Preview with pinned PR/SHA, isolated test data, lifecycle controls, and reliable return to the original Dev build.

Detailed bilingual release notes: [0.3.1](release-notes/0.3.1.json).

## 0.3.0 - 2026-10-10 — Operations and Monitoring Overhaul

- **Unified agent observation:** Root Session history, canonical task lifecycle ledger with visible Worker/event IDs, Controller/Worker state monitoring, Hook onboarding.
- **Notifications:** integrated notification center, per-project Telegram settings and sending-computer identity, browser push, durable replay/duplicate protection.
- **Project and storage management:** archive/restore, cross-project usage diagnostics, protected retention and safe cleanup.
- **Performance and design:** 400+ backlog Markdown content-first navigation, reduced project monitoring lock contention, steadier refresh, Mecca Dark, responsive mobile controls.
- **Update resilience and tools:** update progress phases, safer rollback and Windows restart, Web Terminal and remote operations.
- **Runtime:** Go standalone application is the only supported runtime. Python legacy support has been retired.
- **After upgrading:** migrate project framework instructions with explicit backup/consent for user modifications; confirm active provider Hooks and restart Root Sessions when enabling Hooks.

Detailed bilingual release notes: [0.3.0](release-notes/0.3.0.json). Verification evidence is documented with the Stable promotion review. Release from the tested latest Dev code with Stable 0.2.52 Windows hotfix and release history retained.

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
