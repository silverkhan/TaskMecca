# A-65 core review evidence

This packet supports user review of the Smoke core prototype. It does not complete all A-65 acceptance criteria. [Review boundary](a65-prototype-review.md), [feature map](a65-feature-map.md), and [implemented design system](../DESIGN.md) define the scope.

## Visual review

- [Final corrected capture report](evidence/a65-prototype/final-fix01-487c2391/capture-contrast-report.json): actual isolated Go server, public synthetic projects with 36/14/8 tasks, no response mocks. Twenty primary captures cover home, backlog, detail and lifecycle at 1440/1280/390/320/430 widths across Light/Dark and Korean/English. Full-home and opened-mobile-tools evidence are separate files.
- [Corrected finish verdict](evidence/a65-finish-review/isanghaessi/verdict-fix01-487c2391.md): all four material findings resolved; visual core ready for user design review. The [original findings](evidence/a65-finish-review/isanghaessi/report.md) and [original capture set](evidence/a65-prototype/final/capture-contrast-report.json) remain unchanged.
- [Opened 320px controls](evidence/a65-prototype/final-fix01-487c2391/open-tools-proof.json): real state, sort, page size and tag interaction; full source paths; no document overflow.
- [Sampled contrast](evidence/a65-prototype/final-fix01-487c2391/contrast-summary.md): composited ancestor backgrounds and gradient endpoints; lowest sampled active ratio 4.60595:1. This is sampled evidence, not comprehensive accessibility acceptance.
- [Rich lifecycle fixture provenance](evidence/a65-prototype/ci-5b04-workflow/provenance.json): exact older green CI artifact demonstrates native component continuous, short, zero, historical and overlapping measured intervals. These are component fixtures. The actual Go synthetic lifecycle captures correctly show unmeasured evidence.
- [Native Chrome zoom record](evidence/a65-prototype/native-chrome-zoom-5b04.json): one wide-window 200% check, restored to the prior 90%. CSS zoom tests are separately labeled layout equivalents. Physical-device, OS-trackpad and screen-reader validation remains unclaimed.

## Source and integration

Visual source is `487c23918449aaab6ab422c5d1f47b71a1669055`. Through `6ba0fa548f4b344ee44cf4b15dde907291ebf21d`, later changes are limited to two test files; runtime and embedded UI source is unchanged. The final delivery commit adds documentation and evidence. Its new binary is built and hashed independently; no binary byte-identity assumption is made.

- [Captured source manifest](evidence/a65-prototype/final-fix01-487c2391/committed-source-manifest.json), [preview provenance](evidence/a65-prototype/final-fix01-487c2391/preview-provenance.json), and [Go build information](evidence/a65-prototype/final-fix01-487c2391/go-build-info.txt).
- [6ba integration smoke](evidence/a65-prototype/integrated-smoke-6ba0fa54.json) and [runtime source equivalence](evidence/a65-prototype/runtime-equivalence-6ba0fa54.json): three projects, real compact backlog, requirements fallback, lifecycle, exact back URL, no browser errors, embedded asset MIME/bytes and global SSE initial revision.
- [Native core browser contract](evidence/a65-prototype/finish-fix-regression/native-core-contract.json) and the other JSON contracts in that directory cover focus/caret and reading restoration, bounded loading/cursor recovery, responsive reading, connected lifecycle and evidence readers. Final material-fix native browser run passed 8/8 with no skips; native unit run passed 58/58.
- [CI cleanup report](evidence/a65-ci-cleanup/cleanup01-20261108/report.md) documents joining the lifecycle test fixture feed before temporary-directory cleanup. Failed historical CI logs under `ci-767b0f42` and `ci-487c2391` remain raw evidence, not current failures.

## Final delivery gates

Draft [PR #361](https://github.com/silverkhan/TaskMecca/pull/361) remains the review entry. Exact delivery-HEAD CI and Preview, final binary/provenance, eight counterbalanced performance pairs and idle results are recorded in the PR and Controller intake after this documentation commit freezes. Earlier green runs do not replace these gates. A confirmed median deterioration above 10% requires correction.

Final measurement runs in a quiet window without builds, commits or competing browser work. Performance reports describe the frontend-only synthetic fixture boundary and do not claim backend latency, physical-device or energy results. Performance output is preserved afterward without advancing the measured delivery HEAD.

When all gates pass, the stage pauses for user design review. Operations/settings expansion and the full residual feature map resume after user direction. No merge, deployment, service restart, migration or user Preview switch is included.
