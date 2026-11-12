All commands ran in the integration checkout.
- PHASE=before / PHASE=after [RESUME=1] node docs/evidence/a65-floating-containment-20261110/inspect.cjs
- node docs/evidence/a65-floating-containment-20261110/performance.cjs (single counterbalanced scoped batch)
- WORKFLOW_REQUIRE_BROWSER=1 node --test tests/a65_dev_floating_pilot.test.cjs tests/a65_pilot_review_entry.test.cjs tests/a65_pilot_neutral.test.cjs (final4/4)
- node --test tests/web_notification_center.test.cjs tests/web_live_list.test.cjs tests/web_select_ui.test.cjs tests/a65_preferences.test.cjs (64/64)
- go test ./internal/webui -run '^(TestHandlerServesDashboardAPIsAndAssets|TestSignatureThemeBranding|TestOverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly)$' -count=1
- node /Users/hanati/.agents/skills/impeccable/scripts/detect.mjs --json --scope layout goassets/template/_task_mecca/framework/web/ui/dev-floating/pilot.css
- git diff --check
Playwright: existing pr348/runtime/node_modules/playwright; Chrome /Applications/Google Chrome.app/Contents/MacOS/Google Chrome. Browser screenshots are synthetic test data. No service install/restart/full Go/uninstall fixtures.
