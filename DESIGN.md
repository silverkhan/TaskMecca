---
name: Task Mecca — Smoke
description: Neutral material and restrained blue for reading multi-project work and its evidence.
colors:
  light-bg: "#f0f1f2"
  light-surface: "#fafbfc"
  light-surface2: "#e9ebee"
  light-text: "#24272d"
  light-muted: "#626873"
  light-border: "#d3d7dc"
  light-accent: "#315ca9"
  light-accent-bg: "#e7edf8"
  light-ok: "#23765c"
  light-warn: "#865b16"
  dark-bg: "#202124"
  dark-surface: "#292b2f"
  dark-surface2: "#33363b"
  dark-text: "#eef0f3"
  dark-muted: "#b0b5bf"
  dark-border: "#454950"
  dark-accent: "#a2bfff"
  dark-accent-bg: "#35435d"
  dark-ok: "#87d5b6"
  dark-warn: "#e8c487"
  workflow-worker: "#595fb1"
  workflow-controller: "#006f65"
  workflow-registrar: "#87520e"
  workflow-sub-controller: "#5d6074"
  workflow-wait: "#59697f"
typography:
  headline:
    fontFamily: 'Manrope, "Apple SD Gothic Neo", "Noto Sans KR", sans-serif'
    fontSize: "30px"
    fontWeight: 750
    lineHeight: 1.3
    letterSpacing: "-0.03em"
  section:
    fontFamily: 'Manrope, "Apple SD Gothic Neo", "Noto Sans KR", sans-serif'
    fontSize: "19px"
    lineHeight: 1.4
    letterSpacing: "-0.02em"
  task-title:
    fontFamily: 'Manrope, "Apple SD Gothic Neo", "Noto Sans KR", sans-serif'
    fontSize: "14px"
    fontWeight: 650
  body:
    fontFamily: 'Manrope, "Apple SD Gothic Neo", "Noto Sans KR", sans-serif'
    fontSize: "14px"
    lineHeight: 1.65
  task-metadata:
    fontSize: "11px"
    lineHeight: 1.5
  source:
    fontFamily: "ui-monospace, SFMono-Regular, monospace"
    fontSize: "12px"
    lineHeight: 1.8
rounded:
  status: "5px"
  control: "8px"
  navigation: "9px"
  utility-surface: "12px"
  surface: "14px"
  rail: "16px"
spacing:
  compact: "8px"
  control-gap: "12px"
  row-inset: "16px"
  page-inset: "20px"
  panel-inset: "24px"
  desktop-gap: "28px"
  section-gap: "36px"
components:
  button-light:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.control}"
    padding: "8px 13px"
  button-dark:
    backgroundColor: "{colors.dark-surface}"
    textColor: "{colors.dark-text}"
    rounded: "{rounded.control}"
    padding: "8px 13px"
  search-action-light:
    backgroundColor: "{colors.light-accent-bg}"
    textColor: "{colors.light-accent}"
    rounded: "{rounded.control}"
    padding: "8px 13px"
  status-light:
    backgroundColor: "{colors.light-surface2}"
    textColor: "{colors.light-muted}"
    rounded: "{rounded.status}"
    padding: "3px 8px"
  summary-sheet-light:
    backgroundColor: "{colors.light-surface}"
    textColor: "{colors.light-text}"
    rounded: "{rounded.surface}"
    padding: "26px 28px"
---

# Design System: Task Mecca — Smoke

## Overview

**Creative North Star: "Smoke"**

Smoke makes multi-project evidence legible, from judgment to task history. The built world is a quiet floating navigation rail beside neutral reading surfaces, with an asymmetric reflection, soft offset depth, and restrained blue for interaction. Attention evidence carries the strongest emphasis. The original Task Mecca name and [logo](goassets/template/_task_mecca/framework/web/logo.svg) remain the identity; the purple mark does not turn the interface into a purple theme.

This is a record of the active core prototype, not a proposed redesign. [PRODUCT.md](PRODUCT.md) supplies the durable product constraints. The [surface brief](docs/a65-surface-brief.md), [prototype boundary](docs/a65-prototype-review.md), and [feature map](docs/a65-feature-map.md) describe composition and stage scope. Pane is a material/quality reference only; its calendar, focus tools, content, and metrics are not this product's design system.

**Key Characteristics:**

- Evidence text, identity, provenance, and uncertainty remain readable together.
- Desktop and mobile share the main jobs and dedicated task screens.
- Native links, form fields, disclosures, and a connected lifecycle reader carry the interaction.
- Surface depth is CSS material; no per-row blur, permanent canvas engine, or continuous optical render loop is part of Smoke.

**Implementation authority.** The active [index](goassets/template/_task_mecca/framework/web/index.html) loads `ui/boot.js`, `ui/smoke.css`, `ui/components/workflow.css`, and the native module `ui/main.js`. Screens, shared components, state, and data access are separate modules. Historical `app.js` and legacy styles remain compatibility/test material, not the current visual authority. Extend the existing semantic CSS properties, appearance-mode separation, modules, and original assets. There is no implemented theme registry or old/new UI switch.

**Recorded baseline and review limit.** Visual source is `487c23918449aaab6ab422c5d1f47b71a1669055`. Inspected HEAD is `6ba0fa548f4b344ee44cf4b15dde907291ebf21d`; its two later commits change tests only. The independent [finish verdict](docs/evidence/a65-finish-review/isanghaessi/verdict-fix01-487c2391.md) resolves M1–M4 and states **VISUAL CORE READY FOR USER DESIGN REVIEW**. It does not establish whole A-65 acceptance, final exact-HEAD CI/Preview, or the final eight-pair performance gate. This document does not rerun those checks.

## Colors

The primary accent is a muted interaction blue against cool neutral material. Light and Dark use the same semantic roles with separate values; the frontmatter records the source values, not generated tonal ramps.

### Primary

`--accent` marks links, focus outlines, the active navigation icon, and working-state emphasis. `--accent-bg` is the quiet selected/working backing, including keyboard-selected task rows and the search action. It is not a large decorative wash.

### Neutral

| Source property | Frontmatter pair | Use |
| --- | --- | --- |
| `--bg` | `light-bg` / `dark-bg` | Page field behind the floating shell |
| `--surface` | `light-surface` / `dark-surface` | Rail, lists, sheets, controls, evidence reader |
| `--surface2` | `light-surface2` / `dark-surface2` | Hover, active navigation, notices, neutral status |
| `--text` | `light-text` / `dark-text` | Primary information |
| `--muted` | `light-muted` / `dark-muted` | Metadata, secondary text, labels |
| `--border` | `light-border` / `dark-border` | Row divisions, controls, reading boundaries |

`--surface-2` aliases `--surface2` for the existing lifecycle component; it is not another material. The declared danger properties remain in source, but the core load-error surface is neutral with explicit text and Retry; do not infer a red error-card pattern from an unused palette declaration.

### Status and lifecycle color

Done text uses `--ok`; hold and attention reasons use `--warn`. Working/doing uses the accent pair. State text and provisional labels accompany color. The workflow's Worker, Controller, Registrar, Sub-controller, and Wait colors are role encodings scoped to `.workflow-view`, not additional global accents. Unknown workflow roles use muted text. Start-only and open/unmeasured evidence have distinct outlined/dotted treatments; filled activity alone must not imply that every interval was measured.

**The Evidence Rule.** A color never substitutes for a state label, reason, provenance, or measurement status. Missing, partial, stale, pending, and zero are different facts.

Representative [contrast measurements](docs/evidence/a65-prototype/final-fix01-487c2391/contrast-summary.md) composite sampled foregrounds through ancestor backgrounds and gradient endpoints. Those samples meet (4.5:1) for body/secondary/placeholder/control/status and (3:1) for large headings; the lowest sampled active ratio is approximately (4.606:1). Disabled controls were excluded. This is sampled evidence, not a claim about every pixel or assistive technology.

## Typography

The shell uses local variable [Manrope](goassets/template/_task_mecca/framework/web/ui/fonts/manrope.ttf), with Apple SD Gothic Neo, Noto Sans KR, and sans-serif fallbacks for Korean/system coverage. The bundled font uses `font-display: swap`, supports weights (200–800), disables synthetic font styles, and retains its [SIL OFL license](goassets/template/_task_mecca/framework/web/ui/fonts/OFL.txt). No separate display face or external font service is required.

The hierarchy is compact and reading-focused: page headline, section title, task title, body, then explicit secondary facts. The frontmatter captures desktop roles. On screens at or below (760px), page headlines become (26px, line-height 1.4), section headings commonly become (17px), and task titles become (13px). The desktop combined home also uses (13px) task titles. Metadata stays subordinate; the repaired mobile task owner/project/Updated facts use (11px) rather than inheriting the earlier narrower row copy treatment.

Task IDs use tabular numerals. Source text uses the frontmatter's monospace stack, preserved wrapping, and a maximum measure of (85ch). The timeline has its own compact labels; the floating evidence reader currently declares a system-ui fallback separately. That component-local fallback is not a new global typography rule.

Keep full titles and reasons wrap-capable. Updated/수정 is explicitly modification time from valid `updated_at` or `mtime`; it is never an activity duration. Invalid or missing values render Updated: Unknown/수정 시각 미확인.

## Layout

The desktop shell is a grid with a floating rail (224px), a flexible workspace, gap (28px), and outer padding (20px), capped at (1920px). The rail is sticky (20px from the top), with height `calc(100dvh - 40px)`. At or below (1100px), the shell uses a (192px) rail, gap (22px), and padding (16px). The workspace is capped at (1420px) in the wide layout, while task detail reading content is capped at (1100px).

At or above (1100px), the combined home uses columns (1.24fr / 1fr), with gaps (30px vertically, 28px horizontally). Attention spans both columns and its preview has two columns; working tasks sit left, projects then recent activity right. The underlying section order remains attention → working → projects → activity. Home fetches four records per independently paginated section. Its combined attention preview shows the first two current-page records, with the remainder in a native disclosure; independent cursors expose later pages. This is not a hard total of two tasks. Standalone section views do not inherit that compact preview rule.

Wide home project rows are compact grids (30px mark / flexible identity / counts / 14px arrow), inset (12px 14px). Identity, observed state, and Working/Attention counts appear together. The visual repair places actual project rows before the desktop fold in the supplied captures. Abbreviated project paths retain full-path titles; complete project/backlog locations remain in backlog tools.

At or below (760px), the shell becomes a single column. The brand stays above the workspace, main navigation becomes a fixed four-column bottom bar with safe-area padding, and Preferences stays near the top brand area. Workspace padding is (0 20px 100px), reserving reading space above bottom navigation. The bar's links have a minimum height of (49px). Main section spacing becomes (32px), compared with the default stacked (36px).

Mobile task rows place ID and status on the first row; the full title/reason/metadata body spans both columns below. Status can wrap within its bounded width; the decorative chevron is hidden. Project counts remain next to project identity. Backlog search stays visible; its auxiliary Filters & sort disclosure shows the active summary while closed. Mobile search uses Enter and the existing input debounce; the separate Search button is hidden. Open tools retain state, sort, page size, tags, and source locations.

Spacing entries in frontmatter are extracted recurring literals, not a separate runtime spacing framework. Preserve the implemented density rather than forcing every value onto a new universal scale. The lifecycle component also has local responsive rules at (720px) and historical auxiliary styles at (580px); the Smoke stylesheet loads first, so read both stylesheets and their cascade before extending those controls.

## Elevation & Depth

Smoke combines tonal separation, thin borders, a faint angled reflection, and a soft shadow offset toward the lower right. Shared rail/list/sheet/workflow surfaces use `linear-gradient(130deg, #ffffff04, transparent 48%), var(--surface)`. The final declarations below supersede earlier definitions in the same stylesheet:

| Mode | `--reflection` | `--shadow` |
| --- | --- | --- |
| Light | `inset 1px 1px 0 #ffffffed, inset -1px -1px 0 #ffffff22` | `4px 16px 34px -18px #29323d4d` |
| Dark | `inset 1px 1px 0 #ffffff24, inset -1px -1px 0 #00000016` | `4px 18px 36px -16px #000a` |

Filters use the reflection without the full ambient shadow. Hover changes the surface tone; it does not lift or scale every row. The fixed workflow evidence reader uses its own border and shadow (`0 8px 28px rgba(0,0,0,.18)`). The mobile bottom bar uses a soft upward shadow (`0 -8px 30px -20px #0004`).

**The Shared Surface Rule.** Apply material at the list or sheet boundary; row divisions and text establish internal structure. Do not turn each evidence row into a separate optical effect.

The rail has one bounded settle animation only under `prefers-reduced-motion: no-preference` (0.45s, `cubic-bezier(0.16, 1, 0.3, 1)`), changing its shadow. With reduced motion that entrance animation is absent. Data feedback is text/state driven, not a perpetual animation.

## Shapes

Soft corners distinguish containment levels: rail, shared sheets/lists, smaller utility surfaces, controls, navigation items, and compact statuses use the extracted radius roles. Lists clip their shared rounded boundary and divide rows with a thin border. Task tags are smaller source labels (4px corners), not oversized pills. Project marks are outlined letter tiles; the original logo remains the brand mark.

Workflow geometry expresses time. Connected activity runs have rounded endcaps based on run height; contiguous runs preserve visible event divisions, overlap occupies distinct lanes, and open ends can be square. Exact intervals are not widened to make short durations look longer. Transparent hit areas improve selection without altering time geometry.

## Components

### Navigation and appearance

The [shared shell](goassets/template/_task_mecca/framework/web/ui/components/shared.js) provides Control home, Projects, Recent activity, Notifications, Preferences, refresh, language, and appearance. Rail links are muted at rest, gain a neutral hover surface, and use neutral backing/reflection plus a blue icon when active. Mobile uses the same main destinations in the bottom bar. Icons are authored inline SVG paths with rounded strokes (1.7px), not a font-icon dependency; the brand uses the original SVG asset.

[Boot](goassets/template/_task_mecca/framework/web/ui/boot.js) sets language and theme before paint. [State](goassets/template/_task_mecca/framework/web/ui/state.js) separates saved mode (`light`, `dark`, `system`) from resolved `data-theme`; System follows the browser color-scheme preference. Valid legacy mode values migrate once into the Smoke key. Saved Korean/English is retained; an unsaved language follows navigator Korean or defaults to English. There is one Smoke world with two resolved palettes.

### Buttons, fields, and focus

Default buttons and fields are neutral, bordered, and softly curved. Buttons use the frontmatter padding and minimum height (40px); fields use padding (7px 10px) and minimum height (40px). Hover uses the second surface. Disabled buttons use opacity (0.45) and a default cursor. Quiet and refresh controls are compact (36px); pagination is denser (32px minimum). These are actual component differences, not a claim that every target is 44px.

Buttons, links, inputs, selects, and summaries share a visible accent outline (2px, offset 3px). The main reading region has programmatic focus; the index supplies a skip link and the shell a polite live region. Native disclosure and form semantics remain. These source affordances do not establish a completed screen-reader audit.

### Task rows and project rows

Each task is a real route link containing ID, full title, project when available, assignee, Updated fact, labeled state, and an attention reason when present. Unknown assignee and provisional state are explicit. Hover uses the second surface; keyboard selection uses accent backing. Rows route to dedicated task pages on every viewport. Project rows pair identity and source state with counts; unavailable counts remain unknown rather than becoming zero.

### Search, filters, and pagination

[Backlog](goassets/template/_task_mecca/framework/web/ui/screens/backlog.js) keeps search and the closed active-filter summary visible. A native `details` contains state, supported ID/modification-time sort directions, page sizes (10/20/50), comma-separated tags with the actual catalog, and full locations. Rich multi-status editing and a namespace explorer are not implemented. Tools-open state is retained; form-associated selects still submit through the primary filter form. Search debounces (400ms) and Enter submits immediately.

[Main](goassets/template/_task_mecca/framework/web/ui/main.js) reuses the shell and patches backlog results rather than replacing active inputs for same-context changes. Route/query state carries project, backlog, search, status, tags, sort, page size, and page; history carries reading position, focused row, and tools state. Arrow keys move list selection, Enter follows the link, `/` focuses search, and PageUp/PageDown paginate. Dedicated detail uses Back, Escape, or ArrowLeft outside chart/input controls to return to list context. These reading affordances are part of the component behavior, not decoration.

### Loading, empty, error, partial, and stale states

Loading is labeled text with status semantics. Same-context backlog refresh leaves controls in place, announces Updating results, and marks the reading region busy. Empty results describe the current coverage/filters. Failure is a neutral alert surface with an explicit Retry action. Partial overview includes a notice, section coverage, and source-error disclosure. Projects distinguish observed, stale, partial, paused, missing path, read error, loading, and unknown. Cursor expiry recovers once with a membership-change notice.

Lifecycle pending/stale/error notices preserve previously verified evidence. A changed task document prompts the reader rather than silently replacing source text; lifecycle evidence can update separately. One visible-screen event source supplies invalidation hints with bounded retries/follow-ups; this is not continuous polling of a fresh idle screen. See [prototype behavior and limits](docs/a65-prototype-review.md).

### Dedicated task and evidence reading

[Detail](goassets/template/_task_mecca/framework/web/ui/screens/detail.js) exposes Overview, Requirements, Lifecycle, and Execution & validation as dedicated screen sections. The tab row can scroll horizontally on narrow screens. Overview uses a shared summary sheet; absent summaries say Not recorded. Requirements fall back to original source when populated structured requirements are unavailable. Source identity, safe source links, tags, related/dependency task links, raw source, and execution facts remain available. The initial detail path renders `requestContent` before its separate lifecycle refresh, keeping source reading available while execution evidence is checked. Missing timing is unmeasured/unknown, never a fabricated zero; cumulative activity is not added to elapsed time.

The native [workflow module](goassets/template/_task_mecca/framework/web/ui/components/workflow.js) and [workflow styles](goassets/template/_task_mecca/framework/web/ui/components/workflow.css) preserve fit-all entry, round filtering, visible range, zoom/pan controls, keyboard navigation, wheel/pinch/range navigation, and interval selection. A hover reader can become a pinned nonmodal evidence surface with selectable source text, included-event selection, close/focus return, and retained reading context. Short, zero, open, historical, and overlapping intervals retain their distinct evidence geometry. Bounds that cannot be verified do not produce a fictitious timeline. Observer/readers are disposed on navigation.

The richer continuous/short/zero/history/overlap [CI fixture captures](docs/evidence/a65-prototype/ci-5b04-workflow/provenance.json) demonstrate the same native component with measured fixture evidence. They are not measured Go integration. The final actual-Go lifecycle captures remain honestly unmeasured.

### Review evidence and remaining stage

The [final capture set](docs/evidence/a65-prototype/final-fix01-487c2391/capture-contrast-report.json) covers home, backlog, detail, and lifecycle across five supplied viewport/theme cases: two desktop and three mobile widths (1440/1280 and 320/390/430), totaling 20 primary images, plus full-home/open-tools evidence. [Preview provenance](docs/evidence/a65-prototype/final-fix01-487c2391/preview-provenance.json) identifies the isolated actual Go binary and public synthetic projects with (36/14/8) tasks. No route-response mocks were used for that set. It is not live user data.

Native Chrome 200% is only a reported wide-window check, restored to 90%; it is not all-breakpoint zoom evidence. Physical phones, OS trackpad behavior, screen readers, battery/energy, and comprehensive accessibility acceptance remain unverified here. Final exact-delivery-HEAD CI/Preview and eight counterbalanced performance pairs remain separate delivery gates; no final pass is asserted by this sidecar.

The [feature map](docs/a65-feature-map.md) retains next-stage operations/workload/runtime, project/service/channel management, delivery/configuration, release/update/help, terminal, storage, and extended settings/Activity surfaces. Rich multi-status controls, namespace exploration, and automatic viewport-driven row count also remain residual. Existing APIs or compatibility assets do not establish completed replacement screens. User design review precedes that expansion; backlog CRUD and Agent execution/approval remain outside the approved redesign scope.

## Do's and Don'ts

- **Do** extend semantic surface/text/accent roles in both palettes and preserve System as a mode choice.
- **Do** keep Task Mecca's original identity and the same main reading jobs on desktop and mobile.
- **Do** preserve full task evidence, explicit uncertainty, source labels, dedicated detail routes, and list reading/focus restoration.
- **Do** read the final CSS cascade and active modules before extending a component; earlier declarations and legacy assets are not the final world.
- **Don't** invent activity durations, counts, progress, or observed certainty to fill a visually empty state.
- **Don't** replace compact native disclosures with missing controls or let a status badge constrain the mobile title/reason to a narrow column.
- **Don't** copy Pane's demo features/content, restore the old palette picker, or introduce a second UI world.
- **Don't** promote prototype stage labels, fixture content, or pending operational screens into reusable visual commitments.
- **Don't** treat the resolved visual verdict, older green CI, or measured component fixtures as whole A-65/user acceptance or final performance/CI approval.
