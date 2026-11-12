# A-65 first material slice

This is the small working material/navigation sample authorized by the user revision. It is not a new accepted design system or completion of restored core UX. The prior `aa590433` candidate, its SHIP/CI/performance evidence and port57068 remain preserved. DESIGN.md remains the prior candidate record.

Working review URL: http://127.0.0.1:57069/ui/material-review.html . `?theme=dark` / `?theme=light` choose the starting appearance. The dedicated HTML uses the real reusable `ui/material` component, CSS and proximity modules. The active application index is unchanged; no image is pasted into the UI.

## Visual packet

| View | Default | Other states |
| --- | --- | --- |
| 1440 ×1000 Dark | [Default](fix01/desktop-dark-default.png) | [Near](fix01/desktop-dark-near.png), [outside24px](fix01/desktop-dark-outside24.png), [leave](fix01/desktop-dark-leave.png), [selected](fix01/desktop-dark-selected.png), [keyboard](fix01/desktop-dark-keyboard.png) |
| 1440 ×1000 Light | [Default](fix01/desktop-light-default.png) | [Near](fix01/desktop-light-near.png), [outside24px](fix01/desktop-light-outside24.png), [leave](fix01/desktop-light-leave.png), [selected](fix01/desktop-light-selected.png), [keyboard](fix01/desktop-light-keyboard.png) |
| 390 ×844 Dark | [Default](fix01/mobile-dark-default.png) | [Menu](fix01/mobile-dark-menu.png), [selected](fix01/mobile-dark-selected.png), [keyboard](fix01/mobile-dark-keyboard.png) |
| 390 ×844 Light | [Default](fix01/mobile-light-default.png) | [Menu](fix01/mobile-light-menu.png), [selected](fix01/mobile-light-selected.png), [keyboard](fix01/mobile-light-keyboard.png) |

[Reduced-motion capture](fix01/desktop-dark-reduced-motion.png) and the [capture report](fix01/capture-report.json) belong to the same frozen visual source. The first batched inspection is preserved under `round1/`; its cold provisional-data failure is not overwritten.

The Pane comparison originals and actual pointer observations remain in the canonical `material-recovery-20261108/` contract evidence. Actual Pane showed a260px local face glint following the pointer and returning to the static surface on leave. Its observed rim was fixed; neighboring illumination outside the card was not established by observation. The new quadratic outside-distance attenuation is the separately authorized implementation requirement. The global GSAP ticker, pixel optics and blur engine are not copied.

## Working behavior and limits

The slice implements SYSTEM/BACKLOGS/OPERATIONS/HELP grouping, Global Hub selection, directly opened actual synthetic projects, local close/reopen, desktop Auto/Expanded/Compact, a mobile full-width menu, Light/Dark/System, selected card and source reading. All data reads use the isolated actual Go server; API responses are not mocked. Closing a local sidebar item does not mutate project registration.

Full backlog/detail destinations, storage/Preview management and operations/help remain unconnected. Their controls have `aria-disabled` plus visible labels; activation announces the limitation and does not change the URL. This sample does not claim restored UX merely because selection changes. Touch/coarse-pointer and reduced-motion use static reflection. Escape closes the mobile menu and returns focus.

Initial overview retry is driven by actual loading/pending evidence, capped at8 requests. A complete empty coverage result and unknown/incomplete evidence render different availability copy. Missing counts/timing are not manufactured. The observed cold API shape was `projects.items=[]`, `status=partial`, `coverage.complete=false`, `loading=true`, `pending=false`; the old status-string check missed the loading boolean and was corrected in the one fix batch.

## Validation record

The final capture runner's original top-level verdict is **false**. Its last assertion read the System mode immediately after media emulation, before the real change event had run. This label remains untouched. [Separate targeted verification](technical-fix01/verification.json) waits for the actual media-query event/result: all four1440/390 × starting Light/Dark cases change to Light and Dark successfully, with original Smoke/legacy preference keys unchanged. This is a separate proof, not a rewrite of the failed report.

The capture cases before that final assertion record navigation grouping, three actual projects, direct project context, close/reopen, raw source, labeled unavailable destinations, selected state, visible keyboard focus, geometry/no document overflow, auto/compact widths, touch-static behavior and idle-zero updates. Reduced-motion is independently recorded. Runtime/source/actual HTTP asset hashes agree across all five loaded slice assets; see [technical summary](technical-summary.json) and [working provenance](working-preview-provenance.json).

[Short pointer traces](technical-fix01/verification.json) use the identical actual Go synthetic fixture, warm data/fonts,1440Dark Expanded rail and two counterbalanced on/off pairs. Each run performs80 trusted pointer moves with raw CDP counters/frame timestamps retained. In2200ms target windows, TaskDuration medians are48.282ms on versus19.9805ms off; the absolute difference is28.3015ms. Frame-gap medians are16.7ms for both. These small samples describe absolute local input cost; they establish neither a relative-percent gate nor whole-system CPU/GPU/energy behavior. No near-zero denominator percentage is reported. After the explicit measurement rAF stops, each1100ms idle check observes zero module rAF, style writes, pending frames, DOM mutations and API requests.

The static detector ran once in this revision cycle, with the superseded DESIGN context excluded. [One warning](design-detector-slice01.json) remains: a240ms CSS width transition for the fixed overlay Auto rail. It does not move the main-content column and is disabled for reduced motion, but is not described as a clean detector pass. The short pointer traces use Expanded mode and do not measure that expansion animation. No second detector or whole48-group/eight-pair suite was run for this first slice.

No physical-device, OS pointer/trackpad, assistive-technology or energy validation is claimed. The fresh [independent material review](independent-material-review/finish-review.md) has disposition **SHIP — FIRST SMALL MATERIAL SAMPLE**, for user review only, with zero material fixes/rebuilds. Its source-commit-null limitation is resolved by the following source/evidence freeze and separately built exact-HEAD binary, whose final provenance is post-commit intake. The review does not accept a final full-core design system.

User acceptance controls expansion. The user sees this small live sample before any full-core or operations/settings expansion. No merge, deployment, installation, operating Preview switch or canonical ledger mutation is included. Raw Go build information retains its original trailing tab; it is intentionally not whitespace-normalized.
