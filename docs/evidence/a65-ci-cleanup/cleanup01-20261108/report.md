# A65 CI fixture cleanup

Report ID: `a65-ci-fixture-cleanup-20261108-01`.

Commit `6ba0fa548f4b344ee44cf4b15dde907291ebf21d` (actual parent `1c40cb27c888ae7f055f572a5593826ac60b8a41`) adds exactly one line to `internal/webui/activity_lifecycle_http_test.go`: `stopProjectAttentionFeedsForTest(t,project)` immediately after `registerWebFixture`. Removing only that line restores the original487 file bytes exactly. No assertions or production/API/UI files changed.

The preserved487 remote log failed in `testing.go` TempDir RemoveAll with `directory not empty`, rather than a lifecycle coverage assertion. A local unfixed race run passed once; this nondeterministic baseline does not negate the remote failure. The source log SHA and exact excerpt are in `remote-failure.json`.

The existing helper registers `t.Cleanup`, removes only the fixture project's feeds, releases the registry mutex, closes stopCh and waits for doneCh. Its registration follows the project/home TempDir and env cleanups, so LIFO waits for the writer to finish before registry restoration and directory deletion. Registration before Handler/assertions covers Fatal paths too. No skip, relaxed expectation, sleep or global service stop was added.

Validation passed:

- `go test -race ./internal/webui -run '^TestActivityHTTPIncludesReportedLifecycleCoverage$' -count=20 -v`:20/20, package3.731s.
- `go test -race ./internal/webui -run '^TestActivityWorkflowCachesUnchangedSourcesWithoutRuntimeReconcile$' -count=1 -v`:1/1, package1.318s.

No race or cleanup failure appeared. Raw baseline and corrected logs are preserved. A full local Go suite was not run. The bounded CPU window was coordinated with KK/Lumen and released immediately afterward.

Runtime inputs are separate from test commit metadata. All239 observed non-test Go/embed inputs retain matching before/after SHA256, and no runtime path differs from487. The19 UI/index group digest is `68ef8c7001287ca727661c52a1dc248aaa6a6e518f1838225694ad10b68f0419`. This does not claim rebuilt binary equality or inherit broad UI/performance acceptance. The independent gesture test commit1c40 precedes cleanup6ba; final exact-HEAD CI/binary/Preview gates remain with the lead and Controller.

Historical report01/02/03 evidence:27 files unchanged; original01/03 manifests still verify. No canonical operational ledger/copy, Activity/handoff, service, user Preview, environment/certificate, merge or deployment action was performed. This completes only the assigned fixture cleanup support scope.
