All runs from integration checkout; every shell command uses a distinct mktemp TASK_MECCA_HOME.
Before: go test ./internal/webui -run '^TestPreviewBuildListServerDevOnlyAndNeverExecutes$' -count=40
Before lifetime proof: go test ./internal/webui -run '^TestRegisteredWebFixtureJoinsAttentionFeedBeforeTempCleanup$' -count=1
After: go test ./internal/webui -run '^(TestRegisteredWebFixtureJoinsAttentionFeedBeforeTempCleanup|TestPreviewBuildListServerDevOnlyAndNeverExecutes|TestPreviewBuildLookupFailureDoesNotReturnEmptySuccess)$' -count=40
Race: go test -race ./internal/webui -run '^(TestRegisteredWebFixtureJoinsAttentionFeedBeforeTempCleanup|TestPreviewBuildListServerDevOnlyAndNeverExecutes|TestPreviewBuildLookupFailureDoesNotReturnEmptySuccess|TestAttentionFeedOwnerLifecycle|TestAttentionFeedCanonicalRootSwitchKeepsOnlyNeededStreams|TestAttentionFeedPrunesUnmonitoredProjects|TestHandlerServesDashboardAPIsAndAssets|TestSignatureThemeBranding|TestOverviewUIAssetsNestedMIMEBytesAndTraversalReadOnly)$' -count=1
No full go test, uninstall fixture, UI/browser/visual/performance tests or operating service actions.
