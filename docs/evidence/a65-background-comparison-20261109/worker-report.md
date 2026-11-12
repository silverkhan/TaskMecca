# A-65 static background comparison

The requested image/solid comparison is ready for Controller intake and user review. This is a narrow addition to the material sample, not acceptance of the material or completion of A-65. No full-core or operational UX expansion is included.

## Source scope and unchanged material

Worktree: `/Users/hanati/.codex/worktrees/a65-background-comparison-20261109/TaskMecca`, branch `codex/a65-background-comparison-20261109`, base `7455f579f00a2f71aad9470374d9cbf480e0afaa`. Only the material-review HTML entry is changed among39 pre-existing web assets: five additive lines load the decorative image and independent comparison CSS/JS. All other38 existing web assets retain their exact bytes, including `material.css`, `components.js`, `proximity.js`, `review.js`, P0 environment CSS/JS, main/shared/index/boot/smoke. [Before](protected-before.json) and [after](protected-after.json) hashes record this boundary.

The fixed decorative image plane has `pointer-events:none`, `aria-hidden=true`, an empty-alt image and viewport-size-independent document layout. It uses a fixed element with100lvh minimum height and object-fit cover, not `background-attachment:fixed`. Existing material gradients, alpha, opacity, blur, refraction, sheen, shadows, pointer handlers and animation remain unchanged. The image is visible around the surfaces; current interiors remain honestly opaque. New controls and copy state that explicitly.

Only the background plane has a uniform readability veil: **Dark rgba(12,13,17,.68); Light rgba(255,255,255,.66)**. There are no new gradients, blur, canvas, row effects, global rAF or optics. The AI-generated image retains visible furniture, plant, window, color and light boundaries. Controls precede the existing review notes and are reached by actual scrolling on both tested viewports. Image/solid selection is independent of the existing appearance preferences and survives reload.

## Asset origin and delivery

The project-owned AI-generated photorealistic room image is copied byte-identically from PR348 origin commit `5d9c85fca75274a6026e1beb42bd577a690e3143`. Runtime file: `ui/material/cozy-beige-room-photo-1920.webp`,1920×1080,451256 bytes, SHA-256 `48c7a6a5c4dbc1e6afe9b07ab0d026850e8516fbde21e621fa4bbabc9c072600`.

The [original generation/processing metadata](origin-cozy-beige-room-photo.json) and [origin worker report](origin-worker-report.md) are preserved verbatim. [Reuse provenance](asset-provenance.json) links their hashes. The UI labels the image AI-generated. No new public-domain, CC0, camera or licensing claim is made. No image generation, re-encoding, crop or asset alteration occurred. Actual loopback HTTP returned `image/webp`,451256 bytes and the identical SHA; CSS/JS/HTML also matched local bytes.

## Bounded actual-browser result

One batch, actual Go HTTP, no API-response replacement: PC1440×800 and mobile390×844, Light/Dark, **4/4 passed**. [Raw report](round1/report.json) retains computed styles, viewport/object-fit geometry, focus/pointer state, actual scroll offsets, local delivery and interior hashes. No source fix or second visual round was needed. Controller also reviewed the four image states read-only.

| View | Image | Solid | Actual scroll |
| --- | --- | --- | --- |
| PC Dark | [Image](round1/pc-dark-image.png) | [Solid](round1/pc-dark-solid.png) | [Scrolled](round1/pc-dark-image-scroll.png) |
| PC Light | [Image](round1/pc-light-image.png) | [Solid](round1/pc-light-solid.png) | [Scrolled](round1/pc-light-image-scroll.png) |
| Mobile Dark | [Image](round1/mobile-dark-image.png) | [Solid](round1/mobile-dark-solid.png) | [Scrolled](round1/mobile-dark-image-scroll.png) |
| Mobile Light | [Image](round1/mobile-light-image.png) | [Solid](round1/mobile-light-solid.png) | [Scrolled](round1/mobile-light-image-scroll.png) |

For every pair, the same canonical pointer path, blur/focus state, scroll-to-top and750ms settling precede comparison. All three material surfaces have identical computed backgrounds/shadows/opacity/filter/radius and pseudo-element light styles. Cropped opaque card-interior PNG SHA is exactly identical between image and solid in all four cases; rim/corner/outer-shadow pixels are deliberately outside that crop. This is direct evidence of the current opaque interior, not a transparency effect.

Actual UI button clicks and keyboard Enter switch state; reload retains it. Actual wheel input changes document scrollY by more than30px in each case while the fixed backdrop rectangle stays identical. Mobile proof is Chromium at a touch/mobile viewport with actual browser scrolling; it is not a physical phone, native swipe or browser-address-bar collapse claim. Page errors0, external requests0, document overflow0. Related Node syntax and scoped diff checks pass. No broad suites, local `go test ./...`, full48-group performance suite, installation, merge, deployment or user Preview switch was run.

An optional offline copy-contrast estimate could not run because PIL was unavailable; [the failure](optional-estimate-unavailable.log) is retained and no dependency was installed. Captures were visually checked for canvas-copy readability; no formal contrast/comprehensive accessibility or physical-device acceptance is claimed.

## Owned fixture and transport

Initial working fixture: [provenance](working-fixture.json), own PID79959, loopback port56929, public synthetic36/14/8-task projects. The binary is explicitly labeled base7455 plus uncommitted comparison assets; its build info and hash are preserved. Post-commit exact-head binary/fixture/asset checks are separate delivery intake.

Reproduction start command from this isolated worktree: `node scripts/a65-isolated-preview.cjs /path/to/built/task-mecca`. Omitting `A65_PREVIEW_PORT` selects an ephemeral port. `TASK_MECCA_HOME` is isolated by the launcher; Telegram is disabled. Fixture root for this run is `/var/folders/07/q_8xp6j90zn8r_gdxk6sj5nm0000gn/T/task-mecca-a65-preview-5ic8T7`. Controller can safely reuse the launcher and a fresh synthetic root for a separately owned57069 transition after verifying its actual live PID; Worker does not touch57069,18765, Dev or the integration worktree.

Commit is isolated only; no push or PR write. Controller owns serial integration, operational journals/handoffs, PR metadata and final hold(user). Current effective contract SHA `275e2732ff5d613edd919745675be1aa170c9cf2bec60273ccf4428f89a19db9` is distinct from base contract-section fingerprint `2c27553e4e280bbdf61813b461367668b2689567f562e1160225deca857149c3`. All A-65 full acceptance criteria stay open; further expansion requires later user direction.
