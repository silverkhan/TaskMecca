# A-65 Overview recovery03 verification

Report ID: `a65-overview-recovery03-20261108-01`.

The new corrective assignment verified complete committed source `767b0f42ceab5da2e8d01efdd1a9aaf87b33a00c` (tree `8d5f953894f3f1f785c57ccc1276bb3af295154b`). No source fix or new commit was needed. This report covers API/static/SSE verification; full A65 UI acceptance remains with the lead and Controller.

The exact command was:

```sh
go test -race ./internal/webui -run '^(TestOverview|TestOperationMonitorStartsCanonicalEventFeedsForEveryActiveProject)' -count=1 -v
```

All17 tests passed, with zero skips and no race finding. The package reports12.365s. The recorded verification window was22:56:50.710689–22:57:06.907981Z, including the post-run source inventory; this is not a UI performance benchmark. Lumen confirmed no benchmark was running, and the CPU-heavy window was released to the lead/performance worker immediately afterward.

All638 tracked files and the HEAD/tree stayed identical before/after. The19 committed UI/index inputs are recorded in `committed-ui-assets.json`, separately from the five API/test source hashes. `source-before.json` and `source-after.json` cover the whole tracked tree. The handler test compared actual HTTP response bytes against embedded JS, nested JS, CSS and font bytes, and checked MIME class/nosniff. Invalid directories, absent assets and traversal paths returned non-200 instead of successful catchall HTML. The test does not claim every raw traversal returns404 because ServeMux can redirect unclean paths. Preview POST403 is the existing outer guard; direct SSE method validation is405/Allow GET.

The isolated SSE HTTP fixture confirms current-revision bootstrap and reconnect recovery, comment-only heartbeats, queue1 coalescing, subscriber limit256, race-safe cancel/shutdown cleanup and refused fixture listener after shutdown. Stable semantic working/lifecycle changes invalidate the aggregate while elapsed/age/computed_at changes do not. Duplicate lifecycle calls and retired sources stay quiet. The generation test discards a stale in-flight result and excludes the previous five-second cache after a real change; new data then recovers. Unchanged repeated reads add no source calls. These are server protocol/handler assertions, not measurements of native browser rendering or UI CPU.

Read-only fixtures preserved their file digests. Source class, provisional working/time-null, unknown-null versus known0, and stale canonical attention semantics remain unchanged. Existing feed metadata can leave validated_at null; the API retains stale/computed_at and canonical reason/link instead of promoting it to fresh.

The historical lineage is68c35342 → f56ebcb3 → be9f9284 →767b0f42. In particular, be9's actual parent is f56ebcb3eff1cf27960a433e3c6c7c232e472c75, whose parent is68c. The old17PASS at be9 used untracked UI inputs without a stored UI tree hash; it is not retrospectively attested. This new run hashes the fully committed767 inputs. Report01's9-file manifest still has zero mismatches and its manifest SHA remains64c551bdc2b6cb2dd0dc3727dcbc0513c9619e8818815787a4aade027904cf78. Both unfinished report02 raw logs remain unchanged. No old claimed handoff is consumed by this report.

The lead is responsible for the real767 binary/three-synthetic-project browser integration probe, native-app/body gate, paired performance gate, PR/CI/isolatedPreview and user review hold. That live binary result is pending and is not replaced by these tests. No full local Go suite, operating service/Preview change, canonical operational ledger/handoff write, environment configuration change or Git merge/deployment action was performed.

A later observation at report finalization found `internal/webui/server_test.go` dirty after the completed validation window. The Controller confirmed this is the lead's migration of old theme/static markup expectations. All five API/test sources and19 UI/index inputs still match the frozen hashes. This later change is recorded in `post-validation-observation.json`; no claim is made that the present working tree remains globally clean, and support did not repeat CPU tests. The final integrated HEAD/CI remains a separate lead/Controller gate.
