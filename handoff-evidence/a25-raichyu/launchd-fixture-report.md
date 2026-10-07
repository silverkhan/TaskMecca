# A25 macOS rollback fixture isolation

Stable report ID: `A25-launchd-fixture-raichyu-20261007-01`

Assignment: `assignment-db99116870e69d9a1973eb265e470db2`  
Attempt: `run-c51cd3e1b71a6f91`  
Turn: `01a11530-900d-7d91-b4bb-cd07fcfb5435`  
Runtime: `01a11477-13aa-76e3-866a-a8d007002787`  
Contract SHA-256: `3918a32f37740c5841c808da4c3472a5973ed75ab450168bb625aa9c13513814`  
Base: merged dev `6ca5b206785a62c501074ae4ae6c5e16980e8390`  
Branch: `codex/a25-launchd-fixture-isolation`

## Cause and correction

Two macOS rollback tests executed the production shell renderer. Its absolute `/bin/launchctl kickstart -k gui/UID/com.taskmecca.web` command targets the real fixed LaunchAgent regardless of `TASK_MECCA_HOME`. Thus earlier Worker/Controller full Go/race runs could restart the actual service. Previously unexplained service PID changes must not be attributed to external activity without evidence. This finding is separate from the obsolete A13 foreground-writer attribution and unchanged-source read-only evidence.

Production wrappers still render exactly the same fixed kickstart suffix and health-probe meaning. A private renderer accepts an explicit kickstart command; only executed fixture tests supply their temporary local stub. No environment-based operational command override was added.

The fixture helper rejects scripts containing `/bin/launchctl`, requires the injected stub suffix, and appends every argument to a temporary log. Exact log equality proves one `kickstart`, `-k`, and fixed job target invocation, without launching that job. Failure and old-healthy-instance cases preserve binary restoration and recovery-notice assertions. Production renderer verification stays string-only and checks exactly one unchanged fixed restart command.

No new UI, browser QA, screenshot, or detector round was performed. No operational install/restart/stop, actual A13/holder/Trash mutation, or Telegram call was intentionally performed in this follow-up. All executed rollback scripts used the non-destructive stub.

## Known separate health-probe issue, deliberately not changed

A temporary positive fixture served ordinary JSON `{"ok":true,"instance_id":"new-instance"}` while requesting `new-instance`. The existing probe renders `grep -F` with literal backslash-quote characters (`\"instance_id\":\"new-instance\"`), so it rejected this response and entered rollback. The stub log existed and the temporary assertion failed with `healthy target invoked kickstart stub: <nil>`.

Controller explicitly directed that probe escape/production meaning remain outside this narrow fixture-isolation scope. The temporary positive test was removed, not weakened to make it pass. No health-probe fix is claimed here. Existing old-instance race assertions remain. A separately authorized change should correct the literal matching and restore a positive-instance regression before relying on this watchdog as evidence of normal upgrade acceptance.

## Verification and delivery boundary

- Targeted three `TestLaunchdRollback*` tests: PASS using local stub, including old-instance rejection, binary restoration, and recovery notice.
- Full Go tests, full race tests, Go vet, Python compatibility suite, and diff checks are verified for the delivery head and reported in the stable DONE message.
- No whole-file legacy formatting or unrelated service behavior change.
- New PR/head/exact-head CI supplied in the stable DONE message; Worker does not merge or deploy.
- Controller owns operational service identity checks, serial merge/release/application, real-source read-only validation, and canonical completion.
