# Frozen-source frontend performance reproduction

Run from this isolated worktree. This suite serves actual static frontend assets over owned loopback HTTP with one shared synthetic dataset, using each actual Go API projection shape. It excludes backend timing, physical phones/OS visibility/gestures, active-task ticking, and total CPU/GPU/memory/energy. The new overview initial display has no comparable baseline; only its scoped idle behavior is checked.

Dependencies are the installed Google Chrome binary at `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` and Playwright under `/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright`. Their actual versions/hashes are recorded. The baseline static archive is `/private/tmp/task-mecca-a65-baseline-f0a732e3`; tracked `tests/helpers/workflow_fixture.cjs` and `tests/fixtures/workflow/` supply the shared completed workflow fixture. No operating ledger is read.

Before capture, coordinate cessation of other agents' browser/build/race work, agree on the exact frozen HEAD, and independently save `binary-provenance.json` in this directory with `candidate_head`, `candidate_binary`, and `candidate_binary_sha256`. The final runner verifies those values. Never reuse an existing run directory. Preserve all raw failed/incomplete samples. Static harness/protocol files must remain unchanged once the final source is frozen.

```sh
export A65_PERF_HEAD='<exact-frozen-sha>'
export A65_PERF_RUN='gate-final-unique-name'
node docs/evidence/a65-performance/adapter-validation.cjs
node docs/evidence/a65-performance/paired.cjs
node docs/evidence/a65-performance/idle.cjs
node docs/evidence/a65-performance/idle-verdict.cjs
node docs/evidence/a65-performance/report.cjs
```

`--smoke` on paired.cjs runs excluded instrument checks only. Final timing uses eight counterbalanced paired repetitions plus one excluded warmup at each viewport/theme. `protocol.json` fixes all metrics, windows, readiness, same 20 IDs/page, actual 400px movement, and the strict median >10% deterioration gate. Service Workers and HTTP cache are disabled equally. Readiness uses passive MutationObserver/scroll events rather than active rAF polling.

Each run retains protocol/harness/provenance snapshots, full raw counters/frame timestamps/request logs, source and Chrome before/after hashes, idle V8 calibration/coverage/real SSE observations, and observed owned-resource cleanup. Synthetic document.hidden handling is explicitly labeled. A numeric PASS cannot override invalid source/binary provenance, failed readiness/scroll, page errors, insufficient pairs or failed idle checks. Above-threshold cases require source-owner correction and a new frozen-source run; no noise waiver is allowed.

Final artifacts are `report.md`, `final-verdict.json`, `summary.json`, `idle-verdict.json`, `manifest.json`, `raw.json`, cleanup records and `evidence-sha256.json` under the chosen run folder. A scoped frontend PASS is not whole A-65 completion. Final results are handed to Controller intake without source commits during capture.
