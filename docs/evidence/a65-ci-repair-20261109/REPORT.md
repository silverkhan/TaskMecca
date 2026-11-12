# A-65 CI repair — a65-ci-race-20261109

Worker implementation complete; Controller integration and independent validation remain. A-65 overall material user-review hold remains in effect and its AC remains unchecked. No design expansion or product change.

## Identity, scope, and starting point

- Actual Worker role: `/root/controller/kkobugi`, sole implementation worker; no children.
- Native runtime: T3 Code, Codex harness, `gpt-6.1-sol`, Codex `0.160.0`.
- Native T3 thread: `thread:delegated-task:command%3Amcp%3A55f92140-6805-4dc3-b669-1cf03c7787b0%3Adelegate-task%3Ac228ae52-2fc8-45af-90e1-90258939c523`.
- Native Codex session/thread: `01a1bde4-5552-7da0-916f-073ef1beb1b8`.
- Identity evidence: environment plus read-only `t3_thread_configure`; actual start captured promptly in `start.json`. No historical runtime identity represented as live. Native hook emission, canonical assignment binding, Activity recording, and Controller receipt were not independently observed. No runtime registration, synthetic hook event, or canonical/copied ledger/runtime write was performed.
- Worktree: `/Users/hanati/.t3/worktrees/TaskMecca/t3-688d06d2` (native T3-prepared checkout, retained).
- Prepared HEAD: `d315e2f184c156cafe97d4d2c3129fa2a71733f9`, initially clean. Only this checkout was switched detached to assigned base `f3beec75f6b082a2852cb88ddea2c8d5d901e66e` before investigation. No checkout was created in the shell.
- Implementation and evidence are committed together; the native final handoff supplies the exact resulting commit SHA (`git rev-parse HEAD`).
- Approval scope supplied by Controller: `e77203d25d3fe199eb668c4ae3658c0259e36c9c79b4b8bd9dc407bb0ebc8f82`; remaining CI regression only.
- Read canonical framework `SESSION_GUIDE`, `collab`, `roles/controller`, `CLI_OPERATIONS_PLAYBOOK`, `EXECUTION_PROTOCOL`, and the specified transmissive-glass sample contract. Canonical files were read-only.

## Reproduction and cause

The raw CI failure is canvas WIDTH, not scroll position: attempt 2, run `42543905419`, job `128926504925`, exact base HEAD, `tests/web_workflow_gestures.test.cjs:55:157`, expected `1052`, actual `0`. `ci-source-reference.json` records the canonical raw-log SHA-256. Attempt 1's reported navigation failure was expected end `1793838660000`, actual `1793838600000`.

`tests/helpers/workflow_native.cjs` passed an **async callback** to Playwright `waitForFunction`, because it dynamically imported the native test bridge inside the callback. Playwright 1.64.1's polling implementation checks the immediate callback return for truthiness. An async callback immediately returns a truthy Promise; the outer result later adopts even its `false` resolution without polling again. Thus `fixture.waitFor(page,()=>!activityDetailPending.size)` was not a working wait. Native wheel-stream delivery predicates using this helper were equally ineffective.

Evidence gathered before repair:

- Exact-base five-file concurrent group passed (7/7), including under 4× renderer CPU throttling with test-only tracing (7/7). These passes did not establish correctness and were not used as the repair.
- The old helper accepted an always-false predicate in **1 ms**, returning `false` instead of timing out. See `false-predicate-before.log` and `playwright-polling-proof.json` (installed source location, SHA-256, polling excerpt).
- The controlled native response-gate reproducer (`reproduce.cjs`, `deterministic-reproduction.log`) uses the original helper locally, a real refresh-button click, the real fetch, and native rendering. With the refresh response withheld, the old wait returns `false`, pending count is `1`, and the visible end is **1793838600000 rather than 1793838660000**, with width **1052**. After release, native refresh detaches the previous canvas, which reads width **0**, while the current canvas reads the correct new end and width **1052**.
- The new regression run with only the exported helper restored to its old implementation fails with `Missing expected rejection (TimeoutError)` (`regression-negative-control.tap`). It passes with the fix (`regression-positive.tap`).

This deterministically reproduces the reported stale-end values and demonstrates the exact zero-width detached-element mechanism. The uninstrumented CI run contains no node-identity trace, so its precise locator-resolution interleaving is inferred, not claimed directly observed. The initial unchanged local groups and 200-refresh exploratory loop did not randomly reproduce the CI assertion. The response gate supplies an explicit adverse schedule rather than relying on repeated reruns.

## Minimal repair and observable completion

The only existing implementation changed is the test helper: import the native bridge once with `evaluateHandle` **before** starting `waitForFunction`; pass that handle into a **synchronous** polling callback; dispose it in `finally`. Native APIs, module exports, cache behavior, fetch, render, and screenshots are unchanged. An optional wait options argument permits a bounded negative-predicate regression.

Native `loadTaskActivity` sets the pending job before awaiting its fetch; it updates the cache and synchronously calls `renderActivityDetailControls` before deleting the pending entry in `finally`. The repaired helper now actually waits for that observable native completion. It also actually polls the existing delivered native-wheel sum predicate before the stream assertions. Pointer input uses CDP dispatch and native `queuePaint` schedules its paint with rAF; original frame waits remain unchanged. No new fixed-frame assumption, sleep, screenshot-based settle, assertion retry, or CI serialization was introduced.

The added browser regression deliberately holds a real activity response, proves that the pending wait stays open across multiple observed predicate polls, releases the response, and verifies completed rendering, old-node detachment, exact original time bound/width, and unchanged workflow JSON. Its 100 ms timeout is a negative test for a predicate that is always false, not an application settling delay. Existing assertions were neither deleted nor weakened nor skipped.

## Validation

Runtime: Node **22.23.2**, Playwright **1.64.1**, default Chromium **156.0.8078.4**, macOS arm64. Cached runtime `/private/tmp/taskmecca-a54-runtime/node_modules/playwright`, cached browsers `/private/tmp/taskmecca-scroll-ci-browsers`; no application installation. CI uses Ubuntu, so this is CI-like native local execution, not a claim of Linux/hosted CI execution.

| Check | Result | Evidence |
|---|---|---|
| Deterministic held-response reproduction, stale bound and detached width | pass | `deterministic-reproduction.log` |
| Regression with old exported helper | expected failure | `regression-negative-control.tap` |
| Regression with repaired helper | 1/1 pass | `regression-positive.tap` |
| Selective browser/navigation/gestures files | 5/5 pass | `selective.tap` |
| CI stage 1: `node --test tests/a65_core_browser.test.cjs` | 1/1 pass | `ci-stage1-a65-core.tap` |
| CI stage 2: browser/navigation/gestures/popover/connected in one default-concurrent `node --test` invocation | 8/8 pass, zero skips | `ci-stage2-group.tap` |
| CI stage 3: `WORKFLOW_CORE_ONLY=1 WORKFLOW_NO_CAPTURE=1` navigation | 1/1 pass | `ci-stage3-navigation-core.tap` |
| Repaired five-file group under CDP renderer CPU throttling ×4 | 8/8 pass, zero skips | `throttled-fixed-group.tap` |
| Production/sample/original assertion byte verification; `git diff --check` | pass | `source-byte-proof.json`, `verify-source.cjs` |

The three CI stages were executed sequentially in their real order, with captures enabled for the full group. Detailed successful native contract JSON is copied under `contracts/`; screenshot files remain in the external test output directory. The unchanged baseline and instrumented baseline logs are retained. Two early regression-authoring setup failures are also retained as `selective-setup-failure.tap` and `negative-control-setup-failure.tap`: the new test called `fixture.mount` from `about:blank`, causing localStorage SecurityError. Adding the required `fixture.open` corrected that test setup before either real negative/positive regression run. No existing assertion failed in the repaired validation sequence.

No Go tests, `go test ./...`, app install, remote CI rerun, push, PR change, watch, branch merge, deployment, service restart, or Linux run was performed. Remote checks/reviews were not queried; the supplied attempt-2 failure remains the last known hosted CI result.

## Byte preservation, cleanup, and handoff

`source-byte-proof.json` verifies **519 tracked files** against base Git blob IDs and records SHA-256 aggregates: all tracked `cmd`, `internal`, `goassets`, fixture JSON, CI workflows, and the **39 transmissive sample artifacts** are identical. Navigation, gestures, popover and connected tests are byte-identical; the entire original browser-test prefix is identical, with the regression appended. Production timestamps, gaps, state and width invariants are preserved. There is no production diff and no CI scheduling policy change.

All test Chromium instances and fixture servers were closed. The native fixture checks both ephemeral listener ports for refusal; the relevant contract reports also record cleanup. No other worktree, dirty/untracked file, the primary modified `internal/backlog/completion_review_test.go`, Dev18765, or sample57069 was modified. No force/reset/delete operation was performed. This detached worktree and evidence are retained for serial Controller integration.

Temporary outputs are retained, not required for reproducing the durable evidence: `/private/tmp/a65-ci-race-{baseline,traced,selective,selective-fixed,final,throttled-fixed}`; the exploratory preload `/private/tmp/a65-trace-preload.cjs` is also copied here. Shared cached runtime/browser directories were left untouched. These external scratch artifacts were not committed. Only the helper, the appended regression, and this `docs/evidence/a65-ci-repair-20261109/` directory belong to the repair commit.

Controller next action: independently validate and integrate this commit serially into `codex/a65-uiux-redesign`, then handle existing Draft PR361/canonical ledger per its own authorization. Overall material review hold and unchecked A-65 AC must remain. Worker completion does not constitute final product acceptance.
