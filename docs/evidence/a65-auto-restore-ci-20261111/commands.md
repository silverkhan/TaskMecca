gh run view 42791585728 --repo silverkhan/TaskMecca --log-failed
gh run download 42791585728 --repo silverkhan/TaskMecca -n workflow-browser-42791585728-1 -D docs/evidence/a65-auto-restore-ci-20261111/ci-artifact
WORKFLOW_REQUIRE_BROWSER=1 WORKFLOW_PLAYWRIGHT_MODULE=<existing pr348 runtime>/playwright CHROME_EXECUTABLE=<Chrome157> WORKFLOW_EVIDENCE_DIR=<local-diagnostic> node --test --test-name-pattern='actual dev entry' tests/a65_dev_floating_pilot.test.cjs
node <existing runtime>/playwright/cli.js install chromium-headless-shell (Playwright1.64.1, build1248)
Same named actual-entry command without CHROME_EXECUTABLE for matching-version headless shell diagnostic.
Final: WORKFLOW_REQUIRE_BROWSER=1 WORKFLOW_PLAYWRIGHT_MODULE=<existing runtime>/playwright WORKFLOW_EVIDENCE_DIR=<final-named> node --test tests/a65_dev_floating_pilot.test.cjs
node tests/backlog_list_controls.test.cjs
git diff --check; original restore predicate and current dev controller byte/hash comparison.
No screenshots/new visual rounds/full performance/Go/build/service actions.
