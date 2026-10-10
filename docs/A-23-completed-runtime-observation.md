# Completed tasks and unverified runtime observations

A canonical task completion is not a runtime terminal hook. The monitor may resolve a `runtime_unknown` identity/hook warning only when one done record with meaningful result/verification, one `controller_verified` completed lifecycle event, one assignment, and the exact bound attempt agree within the same project and time boundary. `-` placeholders, missing/ambiguous mappings, later assignments/activity, invalid evidence, and errored/interrupted/shutdown attempts remain unverified or active. No attempt start, completion, terminal state or task lifecycle event is manufactured.

The original incident ID, kind, evidence and observation/detection timestamps remain. A separate `resolution` records the canonical completion event, assignment, attempt, task path, completion time and reconciliation time. The Web distinguishes current warnings from **Past runtime observations**: “Task completion verified; runtime observation at the time remains unverified.” A later hook/new execution creates a separate current incident; an old resolved observation is not deleted. Resolved observations are listed separately from the bounded recent-incident list.

## Exact supported reconciliation (Go CLI)

First inspect without writing:

```sh
task-mecca operations reconcile --project /absolute/project --incident ops-EXACT --dry-run
```

Apply the verified mapping to only that incident:

```sh
task-mecca operations reconcile --project /absolute/project --incident ops-EXACT
```

Both paths bypass notification delivery. The preview is read-only; apply atomically updates only the operations journal and uses a cross-process lock shared with the new monitor. Runtime ledgers, task documents, lifecycle and notification settings/delivery ledgers are not changed. Retries are idempotent. Unsupported/missing mappings are errors, not broad “ignore unknown” instructions. Go/Python Web assets have the same history labels; the operation service and this CLI command are implemented in Go.

## Safe runtime maintenance

An old Web process will drop the new resolution field and can recreate warnings. Do not use CLI-only reconciliation while continuing an old monitor. Only the Controller may replace/restart the operational runtime after source/CI verification and service ownership checks. No Worker deployment is authorized.

Use a verified new binary with a source-specific version marker; default development builds may still report `0.2.51`, which is not proof of their commit. For example, build with `-ldflags '-X main.version=dev-A23-VERIFIED_SHA'` and preserve its binary hash. Start/restart the new service with the process-only Telegram transport gate:

```sh
env TASK_MECCA_TELEGRAM_TRANSPORT=disabled /absolute/verified/task-mecca web restart --no-open --project /absolute/project
```

The gate blocks automatic delivery before config/ledger writes and blocks all direct Telegram API transport before network requests. macOS managed-service plists explicitly retain it; detached Unix/Windows children inherit the environment. `/api/health` and `/api/operations` report `telegram_transport_disabled=true`, and Web shows “Telegram transport blocked · maintenance mode.” Verify the actual child flag before accepting deployment. Launchd plist extraction and a real child process are covered without loading/changing the user's LaunchAgent.

Saved bot token, recipient, enabled kinds, activation cutovers and delivery history remain intact. A disabled process does not consume queued notifications, so restoring normal transport later can deliver still-eligible pending events according to existing cutover/dedupe rules. Review those events before restoring transport. Maintenance mode survives managed-service restarts until the service is explicitly started again without the gate. Normal restoration is a separate authorized action, **not performed during A-23**:

```sh
env -u TASK_MECCA_TELEGRAM_TRANSPORT /absolute/verified/task-mecca web restart --no-open --project /absolute/project
```

A running service is not reconfigured merely by invoking `web`; use the supported `web restart` path to regenerate its service environment, preserving the actual primary project/host/port. Never interpret saved `enabled=true` as proof that this process can transmit while the transport gate is active.

## B-455 read-only evidence

Canonical archived done record, verified completion `completed-B-455-9cae4b62` at `2026-10-07T00:21:39.848789Z`, assignment `assignment-4670703cafdfafdec9d273cdbfeeed46`, attempt `run-d1252d6cd04b0a98`, analysis report and commit `9cae4b623f3afceda9467db82805b661e7a009c9` main ancestry were directly checked. The report explicitly retains missing original input/model/vintage/config fingerprints and does not prove full mathematical equivalence. No DB access, analysis rerun, EMPFUND code edit or Telegram action was performed for this diagnosis.

Task-level `ops-d16aba50a5e14ccc` was already recovered at `2026-10-06T23:58:59.892381Z`. Attempt-level `ops-40358712d0f68da9` remained active with observation `2026-10-06T23:58:50.093428Z` and “runtime identity or hook execution unverified.” The exact new CLI preview verifies its completion mapping, without changing the incident or claiming a runtime completion.
