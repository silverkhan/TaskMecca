# A-25 Worker delivery evidence

Stable report ID: `A25-raichyu-20261007-01`

Assignment: `assignment-5b7f2cce94c8b3765a99439e8fcba5ec`  
Attempt: `run-0b90f8ee9f1a2857`  
Contract SHA-256: `355ef1f7398db99c51000d63c8d2f2d23623fc59f323193adc8a26cd1553d271`  
Branch: `codex/a25-runtime-removal-suppression`  
Base: `9a4d40781f6248672275a12145fc791f3ced39c0`

## Delivered

- Cross-process shared/exclusive management-home leases; fail-closed low-level runtime/settings/cache/journal/network producers and missing-source checks before mkdir.
- Canonical + lexical persisted removal boundaries, alias race recheck, explicit resume compatibility, case-insensitive same-file fixture, and substituted-symlink nonexpansion fixture.
- Honest historical move outcome versus current presence, preserved OS restore holders, bounded same-owned-empty failure cleanup, and exposed cleanup errors.
- Bilingual Hub boundary/current-state display with no new destructive action. Go/Python assets remain identical. A-23/A-24 behavior is preserved.

## Validation

- Targeted producer/removal/nested delivery tests PASS. Initial explicit resume regression failed, was corrected by canonical paused-entry filtering, and passed without changing incumbent test expectations.
- Full `go test ./...` PASS; `go vet ./...` PASS.
- Race suites: projectguard, maintenance, notify, webui, runtimeobs, backlog PASS.
- Windows amd64 affected-package cross-compilation PASS (`-exec=true`, compilation only; no Windows native Trash execution claim).
- Python 3.12 full unittest suite and JavaScript syntax checks PASS. New renderer contract checks ko/en copy, escaping, and asset parity.
- Actual subprocess cached-permission/restart fixture; concurrent local HTTP delivery + removal fixture; real production monitor → notify chain; move success + recreated source before history persistence fixture all covered.
- `TestCaseVariantSameFileSuppression` PASS on this case-insensitive fixture volume (not skipped).
- `git diff --check` PASS.

## UI evidence and skill influence

Impeccable hardening/craft-floor guided explicit path boundaries, separated history/current observation, preservation copy, and reuse of existing responsive layout rather than a new destructive control. One initial inspection batch and one final four-view confirmation were used. Manual detector ran exactly once and returned `[]` (saved in `detector.json`); it is static evidence, not a visual-quality claim.

Final synthetic fixture views:

| View | Client / scroll width | Trash retry |
| --- | --- | --- |
| desktop-ko-light | 1425 / 1425 | disabled |
| desktop-en-dark | 1425 / 1425 | disabled |
| mobile-ko-dark | 375 / 375 | disabled |
| mobile-en-light | 375 / 375 | disabled |

Screenshots: `desktop-ko-light.png`, `desktop-en-dark.png`, `mobile-ko-dark.png`, `mobile-en-light.png`. The fixture used a paused synthetic project and removal records under a unique `/private/tmp/a25-raichyu-ui-*` boundary; no real Trash or Telegram action. Temporary foreground fixture server port 18925 stopped, tab closed, viewport reset. Opt-in server tests were intentionally interrupted when finished and are skipped in ordinary suites; those interruption exits are not suite failures.

Desktop screenshot limitation: Controller noticed the final desktop full-page images have left content partly underneath the fixed sidebar. The initial default-viewport screenshot was separated normally. Final captures immediately followed viewport overrides; incumbent desktop `margin-left` has a `.18s` transition, so a transient resize/full-page fixed-element capture is plausible but not proven. A25 changes neither CSS nor index. The width measurements prove no horizontal document overflow, not unobscured desktop content. No extra browser QA round was taken; Controller stable-state deployment smoke remains required.

## Controller operational evidence (not Worker actions)

Post-merge correction: changing the current LaunchAgent service PID during Worker/Controller full Go and race tests was not established external activity. The macOS rollback execution fixtures invoked the absolute `/bin/launchctl kickstart -k gui/UID/com.taskmecca.web` command, so an isolated `TASK_MECCA_HOME` did not isolate that operational job. This test side effect is addressed in the separate A25 fixture follow-up. It does not reattribute the independently identified obsolete A13 foreground writers or invalidate the read-only source stability observations. Worker did not intentionally request an operational restart, but the earlier executed fixtures did cause real service kickstart attempts; the original no-operational-action statement must be read with this correction.

Controller attributed the A13 recreation to obsolete 0.2.51 foreground test servers PID 46334 (port 18769) and PID 48823 (port 18888), executable `/tmp/task-mecca-a13`, targeting the exact A13 removed project. Current dev111 excluded A13 and was not identified as its writer. Controller narrowly stopped the two obsolete writers; Worker performed no operational stop, install, restart, source deletion, holder deletion, or Trash inventory.

Controller observed over 7m39s / approximately 30 scans with four source files' SHA/mtime/size unchanged after stopping those writers. Example session-monitor last scan: `2026-10-07T06:30:29.642227958Z`, mtime `1791354629`, size `203`, SHA-256 `9f07349d802263950a575247982e86762d8c84f98d681ffe25839113960ce2cf`. This is attributed evidence, not a Worker fresh operational validation or proof that the original source is absent.

## Limitations and handoff boundary

Old binaries do not honor the new protocol. Independently configured management homes are distinct registries, not an enforcement security boundary. Legacy custom alias tombstones without a canonical paused boundary require explicit review; no guessed symlink-target expansion. Updated producers protect runtime writes, not every explicit task edit or external program.

No actual A13 residual, containing worktree holder, restore holder, or user Trash content was deleted. No real native restoration was exercised. Historical `moved_to_trash` stays intact if source reappears; the UI warns rather than inferring recreated versus restored. Successful staging holders remain retained for OS restore.

Controller owns canonical lifecycle, merge/deployment, current service identity check, repeated read-only real-source validation, cleanup/archive, and completion recording. Worker PR CI and exact head are supplied in the stable DONE message separately from this committed report.
