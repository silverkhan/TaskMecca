# A-65 core prototype review boundary

This is the first working core prototype of the user-approved Smoke redesign. It is not completion of all A-65 acceptance criteria. After a new Draft PR, exact HEAD CI, isolated Preview and review evidence are ready, work pauses for the user's design review.

## Core delivered for review

- A multi-project control home ordered by attention, work, projects and recent notification-journal activity. Desktop uses two columns to keep managed projects in the first viewport; mobile preserves the same section order and controls.
- Independent server-side section pagination, opaque cursors, partial coverage, stale/pending/loading evidence and unknown values distinct from zero. Registry changes reset expired cursors once with an explicit recovery notice.
- A dedicated project backlog with debounced search, status, multiple tags, sorting, page size and URL context. Keyboard selection, search focus, list reading and back navigation are preserved.
- Dedicated task screens for overview, requirements, lifecycle and execution/validation. Original source, source links, related tasks, execution facts and measured/unknown timings stay available.
- The existing connected lifecycle interval renderer runs as a native ES module. Fit-all entry, wheel/pinch, directional navigation, individual interval evidence, nonmodal pinned readers, focus, source text selection and refresh behavior have native browser regression coverage.
- Smoke supports Light, Dark and System with Korean/English. The original name and logo are retained. Shared surfaces use thin neutral asymmetric reflections and offset soft shadows. There is no per-row blur, permanent canvas engine, render loop or old/new UI toggle.

## Actual source-change path

One visible-screen EventSource consumes existing project attention events or global overview invalidation hints. Fresh idle screens do not poll. A changed signal updates canonical lifecycle evidence without replacing the task source. Changed task text shows an explicit update notice. A safe first backlog page can refresh; filtered or later pages retain their reading and show a manual update action.

Overview cold loading uses a single aggregate request and at most 12 bounded follow-up requests. Completion, errors, hidden state and route departure stop this path. Live connection retries stop after three reconnect attempts. Stale/pending lifecycle snapshots use a finite retry budget. Navigation disposes timeline observers and nonmodal readers.

## Validation evidence and limits

The active browser tests load the native modules and real routes. Migrated workflow tests do not load the old application, inject old globals or disable old CI assertions. Native data tests cover shared pending prefetch, cache isolation, stale response invalidation, bounded speculation and safe document links. The unchanged legacy prefetch VM suite remains supplementary historical coverage and is not evidence that the new UI works.

`docs/evidence/a65-prototype/inspection-round1.json` records the first batched mechanical inspection; it is not visual acceptance. `design-detector.json` is the single final static detector run, with no findings. The actual-binary intermediate failure is retained in `integrated-smoke-intermediate.json`; it identified the missing `/ui/` static route, which required a guarded server route before final integration could pass.

Final screenshots, exact source/binary provenance, native test reports, paired performance medians, Draft PR and exact HEAD CI must be read together. Synthetic fixtures are public test records. No physical phone, OS trackpad, screen reader, battery/energy or native browser zoom claims are made. The core browser suite separately labels its 200% CSS zoom check as a layout equivalent.

## Remaining areas and resume conditions

Full operations/workload/runtime views, project/service/channel management, notification delivery/configuration and extended settings remain for the next stage. Their shell paths describe the review boundary; they are not claimed complete. Backlog CRUD and Agent execution/approval are excluded from the approved redesign scope.

Resume expansion only after the user reviews the core PC/mobile Light/Dark design and provides direction. Resolve any open finish-review or performance gate findings before claiming readiness. No Dev/Main/Stable merge, production deployment, service installation/restart, migration or user Preview switch is authorized by this prototype.
