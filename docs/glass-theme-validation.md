# 실사 배경과 Glass 소재 검증


## CI recovery for scroll rendering (2026-11-05)

Remote `ed307998` failed when the search style observation returned an empty
border shorthand and when a mobile resize reference frame had no reader canvas.
Chrome 156.0.8078.4 passes the original tests locally on macOS; the Linux timing
has not been reproduced. Deterministic controls show that a detached input has
empty computed border longhands and that `render()` can remove the existing
reader scene between animation callbacks without changing its owner or sampler
generation.

Search checks now resolve the current element and read all four border sides,
connectedness, outer frame and interior layout in one browser task. Detached
observations and an injected real 1px side border must both fail. The optional
Glass render hook retains only existing decorative reader planes on surviving
owners and synchronizes their geometry before paint. It preserves queued
structural work and the established missing-plane CSS fallback test. Motion
checks require both Cozy canvases and compare first-frame pixels before origin;
a render between refresh and reference must retain owner/plane/canvas identity,
while an explicitly removed reference canvas must fail. New recovery evidence
is kept separately from report01 and its frozen manifest.

## Fixed scene during scroll (A-56 follow-up, 2026-11-05)

The user observed the scene moving with a panel, then returning. In568ed18f the
body photograph remained fixed. The material bitmap was sampled using the old
panel origin and rode with the moving owner until the100ms document-scroll
timer repainted it. Actual mobile moving/settled PNG crops at the same pose had
reader mean channel differences22.7–28.7, while exposed body crop differences
were0. Desktop screenshot serialization sometimes crossed the delayed repaint;
those intervals are explicitly ambiguous. Scroll-event rAF-synchronous raw
canvas captures disambiguated them: same-pose fresh differences30.7–34.4 on
desktop and26.2–32.5 on mobile; steady controls were0 at the same generation.

A second defect affected overflow ports: the absolute decorative canvas moved
with scrollTop120 and left a persistent121px gap in a260px mobile sidebar.
Geometry repaint alone could not repair coverage. Intermediate direct-sticky
attempts exposed padding and horizontal-overflow errors and are retained as
failed development evidence. The final scene uses a zero-height/content-width
sticky anchor, with its absolute canvas extending across port padding. It
changes no text/foreground flow. In the same baseline fixture, sidebar11,
attention9 and nested-list72 controls had identical contentY/width/height;
scrollHeight/scrollWidth/client dimensions matched exactly. Earlier659-versus-
672 sidebar height observations used different fixture state; the matched
baseline/revision both measured672. Visible controls retain exact hit/focus.

The renderer now separates structural/scene work from geometry work. Scroll and
visualViewport movement coalesce into a pre-paint animation-frame callback,
which batch-reads actual canvas viewport rectangles and remaps only changed
visible surfaces. Actual canvas bounds account for the owner's border/padding.
The fixed sampler/decoded image are reused; a queued structural update absorbs
geometry work rather than scheduling duplicate passes. There is no recurring
frame loop, opacity cover, material-off phase, fallback flash or text treatment.
Offscreen connected materials are retained for reentry and skipped while hidden
outside the viewport; this costs some retained canvas storage.

`tests/web_glass_scroll.test.cjs` adds two real browser regressions. The first
checks72 first motion callbacks across desktop/mobile, Light/Dark and
Cozy/ambient/None, rapid35→70→25→0→80→10 reversals, six viewport changes and idle
paint/adaptation0. Cozy first-frame bitmap pixels must match a fresh reference
at the same pose within the existing one-step readback quantization bound, with
actual painted origins aligned. The second checks three nested ports, full
coverage, unchanged foreground coordinates/scroll metrics, exact visible control
hit/focus and geometry-invariant nested-list paint0. The unchanged568 baseline
fails the pixel assertion first (maximum216, mean33.287), independently of its
one-pixel border-origin discrepancy. The negative command selects only the
first motion test with `GLASS_SCROLL_ASSET_ROOT`; it is an expected failed test,
not a suppressed regression.

The final local suite runs existing14 plus new2 tests, all16 passing and0
skipped. Glass CI explicitly includes the new file and preserves its motion and
port JSON artifacts. Six workflow job definitions retain their existing matrix
expansions, for eight executed jobs, including both Glass/workflow browser jobs.
Named embedded photo/Glass asset Go tests run locally; the unsafe local full Go
uninstall-fixture suite is not run.

A serial Chrome156/DPR1/same fixture1.5-second scroll compared568 and revision:

| Metric | Desktop baseline → revision | Mobile viewport baseline → revision |
| --- | --- | --- |
| TaskDuration delta | 78.481 → 102.935ms | 83.962 → 137.876ms |
| ScriptDuration delta | 17.140 → 36.076ms | 18.015 → 49.092ms |
| main-thread paint trace | 41/6.639 → 24/4.370 events/ms | 66/6.884 → 80/8.256 events/ms |
| mean frame gap | 16.373 → 16.514ms | 16.480 → 16.512ms |
| explicit sampler/readback/canvas bytes | 13,786,648 → 14,123,480 | 7,336,728 → 7,429,088 |
| retained material maps at sample end | 3 → 4 | 3 → 4 |
| idle paint/adaptation | 0/0 → 0/0 | 0/0 → 0/0 |

Pre-paint correctness costs more script/main-thread time. The maximum frame gap
was about16.8ms in this single sample, not a universal performance guarantee.
Later actual viewport PNG pairs showed body/reader differences0 and no paint
change during capture, but their85–348ms screenshot intervals are only
corroboration. First-frame evidence comes from the synchronized raw bitmap
regression. It does not certify compositor-only/off-main-thread behavior on
Samsung hardware. Mobile height/orientation changes emulate a browser viewport,
not a real address-bar/keyboard or pinch/pan sequence. Actual Samsung Internet,
physical GPU/CPU energy, battery and prolonged use remain unverified. No static
Lumen pass is extended into a dynamic or final user-acceptance claim.

The following no-text-fog/material tables describe the earlier568 baseline and
its prior design iteration; the scroll measurements above are the current
follow-up evidence. Final remote exact HEAD CI/Preview/provenance/checksums and
PR/canonical completion belong to Controller.


## Current contract: no text-specific fog (A-56, 2026-11-05)

The user explicitly withdrew the text-by-text readability treatment after
reviewing PR348/b82b7098. This section supersedes older descriptions of local
ink support and photographic per-text AA enforcement below. Those measurements
remain historical evidence, not current implementation or acceptance claims.

Cozy optics no longer walks text nodes, creates character Ranges, reads input or
SVG ink rectangles, searches a support alpha, or paints fog/scrim/halo behind
text. The material cache key uses only the sampled scene generation, panel
geometry and panel tone. No CSS text-shadow, DOM text boxes or row backing
replaces the removed effect. Basic DOM text, theme/panel foreground color,
selection, inputs, focus and keyboard/ARIA behavior remain ordinary app UI.

The scene-preserving .14 broad material, continuous background displacement,
curvature, direction reflection and narrow spectral lip are retained. It does
not revert to a broad opaque readability blanket. None/ambient and unsupported
features/decode/reduced-transparency fallback retain their established fills.

Photographic contrast is an audit: the actual final raster is still measured
against the same 4.5 normal / 3 large-text reference values. The report contains
every sampled result, below-reference count and minimum/p05/median/maximum
distribution. Those reference values were not reduced; the withdrawn criterion
was forcing every photographic sample to pass through local ink treatment.
The suite explicitly detects low-contrast samples on the split desktop photo,
and separately uses a real white/black probe to verify that deliberate low
contrast and a changed background change the measured ratio. The frozen mobile
stage crop can show only one half of the split photo and is not falsely required
to contain both bright and dark regions.

Functional/fallback gates remain: None/ambient text raster AA, controlled
keyboard-focus visibility, palette colors, settings/System OS, native controls,
selector/backdrop/Canvas/decode fallback, reduced preferences, DOM restoration,
positive/negative optical comparisons and screenshot text/color/opacity/geometry
change detection. Foreground-only edits also verify that material pixels and
paint count remain unchanged when scene/panel geometry is unchanged. These
checks do not claim universal photographic contrast compliance.

The implementation is based on8af36459, which includes Dev source6c6c7563 and
retains the workflow browser job as well as the Glass browser job. Local
validation uses isolated synthetic fixture APIs and dynamic ports; it never
changes the operating PR/Dev session or18765 service. The independent Lumen
review and final user Preview acceptance are distinct from numeric testing.

PR #348의 Cozy 기본 배경을 AI로 생성한 실사풍 Cozy Beige Room으로 교체했습니다. 원래 `cozy` 저장값과 Cozy / Atmospheric / None, Dark / Light / System을 유지합니다. 이전 fb4fe7cf의 alpha 패널은 사용자가 거절한 과거 revision이며, 당시 CI·Preview 성공을 새 디자인의 수용 근거로 사용하지 않습니다. 최종 디자인 판단은 사용자가 최신 Preview에서 합니다.

새 소재는 화면 뒤의 viewport 한정 읽기 바탕과 부유 메뉴·팝업으로 구성합니다. 사진의 형태와 빛은 유리 아래로 투과하며, 가장자리 반사·두께·접촉 그림자로 표면을 구분합니다. CSS blur·반사와 같은 사진의 국소 재표본화를 사용하며, 아래 A-55 항목에 광학 구현과 한계를 설명합니다.


Actual unmodified Cozy photo audit (local Chrome156, DPR1, fixture list, normal
and large-text reference values4.5/3 unchanged):

| Viewport/theme | Enabled text/input samples | Below reference | Minimum / p05 / median |
| --- | --- | --- | --- |
| Desktop Light | 138 | 62 | 1.313 / 1.699 / 4.891 |
| Desktop Dark | 138 | 88 | 1.250 / 1.631 / 3.407 |
| Mobile Light | 50 | 15 | 2.404 / 2.749 / 5.596 |
| Mobile Dark | 50 | 36 | 1.358 / 1.619 / 3.422 |

The distribution does not establish WCAG compliance. The independent contrast
probe measured12.717 on white,1.044 after making only the text nearly white,
and20.119 after changing only the background to black. Foreground-only edits
left material paint count unchanged and material bitmap difference within one
8-bit readback quantization step. The split desktop photo exposed46/44 below-
reference Light/Dark samples; mobile stage cropping is reported separately.

A serial same-browser1.5-second scroll sample compared b82, merged baseline8af
and the no-text-fog revision. Each scene ran separately after the browser suite:

| Scene | b82 script ms | 8af script ms | No-text-fog script ms | No-text-fog drawing / adaptation ms |
| --- | --- | --- | --- | --- |
| 36-row list, desktop | 68.586 | 71.438 | 83.278 | 8.9 / 31.9 |
| 500-row detail, desktop | 65.105 | 68.333 | 41.417 | 8.9 / 13.0 |
| 500-row detail, mobile viewport | 62.982 | 66.421 | 33.401 | 8.3 / 16.2 |

The no-text-fog list had720 elements/478 text nodes; long detail had2,203/1,612.
These counts come from test-only observation; production optics no longer
collects text nodes or ranges at all. Maximum frame gap was about16.8ms in every
scene, with mean16.5–16.6ms. After the single Lumen batch, longer detail used less script time, while the
dense list used more. TaskDuration was98.923/105.458/170.520ms for the list,
124.169/120.046/103.675ms for desktop long detail and128.082/136.747/85.361ms
for mobile long detail (b82/8af/revision). This is an exploratory single sample,
not a hardware/GPU/energy guarantee or an across-the-board speedup. The initial
removal-only sample is retained separately from this final batch measurement.

At the same Dark Cozy PR view, explicit sampler/readback and material/rim canvas
RGBA estimates were14,348,072/9,146,248B for desktop/mobile b82 and8af.
The final Lumen batch extends scene material to header/banner/attention, so
revision estimates are16,533,744/10,315,608B, an increase of2,185,672/1,169,360B.
Ink previously occupied existing canvases, so its removal does not itself save
canvas memory. Material canvas count remains separate from the two-rim limit;
the observed final PR view had6 material canvases on each viewport, versus3/2
previously. No independent material-count hard cap is claimed.

The first review bundle captured67 unique PNGs, including b82/8af comparison and
the new desktop/mobile431 Light/Dark/System × Cozy/ambient/None matrix, actual
PR environment long/short copy, banner, attention, detail, menu and navigation.
Every bundle has source/capture hashes and an isolated reproducible fixture.

Lumen's first independent review requested one batch: repair the mobile menu
stacking/scrim relationship, connect header/PR/attention to the same material,
and give the broad reader a gradual boundary-to-plane transition. The batch
removes the app-shell stacking trap only while mobile navigation is open, keeps
the front dialog above the scrim, and uses a whole-background6px scrim blur.
Sidebar/expanded alert use8px scene diffusion and a uniform .22 tint; ordinary
material remains .14. A returning shallow bevel field is continuous with the
existing wave, with a thin directional reader reflection/opposite rim. No
foreground probing or ink treatment was reintroduced.

A51-PNG confirmation matrix records that single batch. Mobile navigation also
passes a real elementFromPoint hit-test above the visible scrim. An independent
identity-source mapping control changes74.984% of426,580 interior reader pixels
by more than two average channel steps, mean difference15.370, while foreground
and dimensions stay equal. This establishes actual broad optical change, not
physical accuracy or design acceptance. The independent Lumen confirmation and
final user Preview acceptance remain separate from numeric test success.


## 사진의 생성·변환과 제공

생성 원본은 1672×941입니다. 공식 Real-ESRGAN-ncnn-vulkan v0.2.0의 `realesrgan-x4plus` 모델로 4배 처리한 6688×3764 출력에서 Lanczos 축소와 미세 crop으로 3840×2160 마스터를 만들었습니다. **네이티브 4K 생성이 아닙니다.** 일반 보간만으로 확대한 결과를 신경망 출력으로 부르지 않습니다. 생성 원본·신경망 출력·마스터·모델은 project-owned 계약 자료에 보존하며 PR에는 WebP 두 개와 작은 [출처 메타데이터](assets/cozy-beige-room-photo.json)만 포함합니다. 원래 SVG와 제공 경로도 보존합니다.

| 웹 자산 | 실제 크기 | 파일 크기 | 전송 선택 | RGBA decode 크기 추정 |
| --- | --- | --- | --- | --- |
| `cozy-beige-room-photo-1920.webp` | 1920×1080 | 451,256 B | 모바일/coarse, 일반 desktop | 8,294,400 B, 약 8.3 MB |
| `cozy-beige-room-photo-3840.webp` | 3840×2160 | 1,158,428 B | 1024px 이상 고DPR desktop, 1921px 이상 화면 | 33,177,600 B, 약 33.2 MB |

WebP quality 88이며 파일별 SHA-256은 메타데이터와 자동 검사로 확인합니다. CSS는 hash query가 있는 URL을 사용합니다. 실제 embedded 경로는 `image/webp`, `nosniff`, SHA ETag와 version 일치 시 1년 immutable cache를 제공합니다. unversioned 요청은 재검증합니다. HEAD/304는 embedded metadata·ETag를 먼저 확인해 이미지 payload 복사를 생략합니다. GET/HEAD/304/405와 MIME·길이·cache는 Go handler 검사로 검증합니다.

브라우저에서는 mobile DPR 3이 실제 1920 자산 451,256 B를, desktop DPR 2가 3840 자산 1,158,428 B를 요청했음을 CSS Resource Timing과 자연 치수로 확인합니다. Resource Timing의 `decodedBodySize`는 HTTP content decoding 이후의 WebP 파일 크기이며 bitmap/GPU 메모리가 아닙니다. 위 RGBA 값도 픽셀 수×4의 추정치이고 JS heap·실제 GPU 할당량과 구분합니다. warm `Image.decode()` 시간은 cold decode 성능으로 해석하지 않습니다.

## 고정 배경과 필터 면적

`theme-boot.js`는 첫 paint 전에 screen 크기로 배경 canvas의 pixel 크기를 정합니다. coarse 화면은 긴 변의 정방형을 사용하고 left/top 0과 이미지 scale을 유지합니다. 주소창·키보드에 해당하는 viewport 높이 변경, 문서/독립 메뉴 scroll과 회전 때 재확대·재중심화하지 않습니다. 회전하면 같은 사진의 가시 범위가 달라집니다. 배경과 reader pane은 별개이므로 pane 높이는 viewport에 맞춰 바뀔 수 있습니다.

읽기 pane의 filter는 `.content` 자체가 아닌 별도 장식 바탕에 적용합니다. Cozy의 광학 표현이 활성화되면 aria-hidden `.glass-reader-material` 요소를 쓰고, 그 외에는 `::before`로 대체합니다. sticky와 같은 크기의 음수 margin으로 flow 높이를 상쇄해 긴 원장 전체를 필터 면으로 만들지 않습니다. reader/chrome blur 10px이며 row/card/code/Gantt에는 반복 backdrop filter가 없습니다. filter/transform/contain을 content 부모에 걸지 않으므로 fixed TOC의 containing block을 바꾸지 않습니다. Terminal의 별도 reader backing은 viewport 안의 terminal main에 한정됩니다.

실제 요소 또는 CDP pseudo box model과 가시 교차 영역을 기록하고, 내용이 약 3,000–3,800px이어도 backing 높이가 viewport보다 작은지 검사합니다. `.querySelectorAll('*')` 개수만으로 pseudo 비용을 누락하지 않습니다. reader 하나 외에 고정 메뉴와 활성 팝업의 제한된 filter가 있으며, **부모 하나라는 이유만으로 GPU 비용이 작다고 단정하지 않습니다.**

## 격리 재현과 자동 gate

실제 embedded Web 자산과 모든 `/api/` 요청을 합성 응답으로 처리하는 fixture입니다. 프로젝트 `/fixtures/atlas`, 백로그 `/fixtures/atlas/_task_mecca/data/backlog`, DEMO-100부터 36개 작업, timed segment 2개와 가상 Root/attempt/상태 이력을 사용합니다. shell/실제 PTY·운영 서버·사용자 원장을 사용하지 않으며 브라우저 외부 네트워크도 차단합니다.

```sh
npm install --prefix /tmp/taskmecca-glass-runtime --no-save --no-audit --no-fund playwright@1.64.1
/tmp/taskmecca-glass-runtime/node_modules/.bin/playwright install chromium
export GLASS_PLAYWRIGHT_MODULE=/tmp/taskmecca-glass-runtime/node_modules/playwright
export GLASS_EVIDENCE_DIR=/tmp/taskmecca-glass-evidence
node --test tests/web_glass_theme.test.cjs tests/web_glass_browser.test.cjs
```

macOS 설치 Chrome 사용 시 `CHROME_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'`을 추가합니다. CI `glass-browser` job은 Node 22/Playwright 1.64.1/설치 Chromium을 격리 prefix에서 실행하고 JSON만 artifact로 보존합니다. 사용자 참조와 로컬 PNG는 외부 업로드하지 않습니다. Node 목록의 Glass gate도 유지합니다.

직접 확인하려면 별도 foreground 터미널에서 다음을 실행하고 출력 JSON의 동적 `origin`을 엽니다. 운영 port 18765는 거부합니다. 종료는 해당 foreground 프로세스의 Ctrl+C만 사용합니다.

```sh
GLASS_FIXTURE_SHA="$(git rev-parse HEAD)" node tests/helpers/glass_fixture.cjs
```

SHA 미지정 기본 `0000000000000000000000000000000000000348`은 합성 식별자입니다. 실제 Preview artifact의 정확한 SHA 검증을 대체하지 않습니다.

브라우저 gate는 실제 custom palette 선택기 클릭으로 Glass → Mecca → Slate → Glass를 전환하고 layer·선택기 노출·색·reader surface·저장값을 확인합니다. 나머지 matrix는 실제 app state/render와 appearance handler로 Dark/Light/System, 세 배경의 reload, scroll·회전·높이 변경, 독립 메뉴, 목록·상세/raw 표·실제 Activity cache의 timed Gantt·실행 상태 이력·Hub·저장소·Dark/Light Terminal 가상 fallback 출력을 확인합니다. fixed TOC 검사는 TOC를 의도적으로 렌더하지 않는 raw 상태와 구분해 rendered detail에서 수행합니다.

focus, reduced motion/transparency도 검사합니다. blur 미지원은 Chromium에서 해당 capability 분기만 강제 활성화하고 filter 선언을 제거한 emulation이며 실제 미지원 브라우저 통과를 주장하지 않습니다. 큰 글꼴은 text node의 computed 크기를 실제 1.5배 올려 증가·overflow를 검사하는 fixture로, OS 글꼴/실제 browser zoom 검증과 구분합니다.

## 같은 사진 위의 비교와 비용 관측

fb4 alpha 스타일에 **동일한 WebP와 같은 선택 조건만** 연결한 baseline을 사용합니다. 새 기본 사진 위의 기존 소재와 새 frost를 비교하므로 SVG→사진 차이를 frost 효과로 혼동하지 않습니다. 실제 앱 viewport PNG와 같은 위치의 글자 없는 crop에서 바깥 사진의 선명도와 pane 안의 디테일 흐림·경계를 확인합니다. 텍스트를 숨길 때 `-webkit-text-fill-color`를 써 `currentColor` 기반 배경색을 바꾸지 않습니다.

고정 chrome을 왜곡했던 과거 fullPage 이미지는 제외합니다. 현재 근거는 1440×960 desktop, 390×844 touch viewport와 별도 scroll 위치, 실제 pseudo box/clip JSON입니다. 합성 pixel 대비는 제한된 표본 검사이며 전체 WCAG/AA 인증이 아닙니다.

```sh
GLASS_EVIDENCE_DIR=/tmp/taskmecca-glass-measure node tests/helpers/glass_measure.cjs
```

기준 자산 폴더는 `GLASS_ASSET_ROOT`로 지정합니다. `GLASS_CAPTURE_ONLY=1`이면 시각 비교와 geometry만 수집합니다. 비용은 동일 fixture·사진·DPR 1에서 1.5초 실제 원장 scroll의 rAF, CDP Task/Layout/RecalcStyle, main-thread Paint trace, JS heap, 8회 테마 변경을 관측합니다. 신경망 upscale GPU 작업이 끝난 뒤 순차 수집합니다. 단일 headless 표본으로 GPU raster/compositing·배터리·실기기 성능 향상을 입증하지 않습니다.

2026-11-04, Chrome 156.0.8078.80에서 upscale·변환 작업 종료 후 얻은 한 차례의 순차 headless 표본입니다. 기준은 fb4 alpha 소재+같은 사진이며 두 조건 모두 card/row filter는 없습니다. 새 조건에만 viewport에 한정된 reader frost가 있습니다.

| 조건 | 평균 frame 간격: 기준 → 변경 후 | Task 시간 | main-thread Paint: 횟수/시간 | JS heap | 테마 전환 평균 |
| --- | --- | --- | --- | --- | --- |
| Desktop DPR 1 | 16.46 → 16.35 ms | 23.64 → 26.05 ms | 3/0.717 → 12/2.598 ms | 4.58 → 4.61 MiB | 32.77 → 32.82 ms |
| Touch DPR 1 | 16.47 → 16.50 ms | 31.93 → 24.99 ms | 11/3.894 → 18/2.794 ms | 4.75 → 4.75 MiB | 32.84 → 32.94 ms |

Desktop Paint/Task의 증가도 그대로 기록합니다. rAF 최대 간격은 약 16.8ms였지만 이를 GPU 성능 보장이나 실기기 개선으로 표현하지 않습니다. GPU 비용·고DPR frame 비용·cold decode·배터리는 별도 미실행입니다.

CDP의 실제 reader border box는 desktop 1186×866px, mobile 384×674px였습니다. 원장 content 높이는 각각 약 3081px/3765px이며 원장 전체 높이의 filter texture로 만들지 않았습니다. 실제 scroll 후 가시 교차 영역도 각각 같은 1186×866, 384×674로 기록했습니다. 이는 geometry 근거이고 GPU 할당량 실측이 아닙니다.

합성 pixel 표본은 desktop Dark 129개에서 최소 4.88, mobile Dark 51개에서 최소 5.07이었습니다. 확인된 Light의 작은 글자·Preview 종료 버튼만 지역적으로 보완한 뒤 desktop Light 129개에서 최소 4.51, mobile Light 51개에서 최소 5.57을 측정했습니다. 일부 CSS 보완 뒤 해당 Light 표본만 다시 검사했으며, 초기 실패 값도 evidence에서 구분해 보존합니다. 제한된 screenshot pixel 표본으로 전 화면 AA 인증을 주장하지 않습니다.


## 미실행과 사용자 판단

Samsung Internet 실제 기기, Android 주소창·가상 키보드, OS 글꼴 확대, GPU·배터리, 특수한 다중 모니터 창 확대는 실행하지 못했습니다. 실기기에서 Cozy Dark/Light를 연 뒤 목록·팝업 scroll, 주소창 접기, 검색 키보드 열기/닫기, 세로/가로 회전, reload를 수행하고 창틀·소파의 기준점/scale과 글자·컨트롤 겹침을 확인합니다. 비용 비교는 같은 사진·데이터·밝기·스크롤 시간으로 수행해야 합니다.

자동 검사와 Root/Controller의 시각 검토는 사용자 디자인 승인이 아닙니다. 새 최종 SHA의 CI·Preview와 사용자 수용 판단은 Controller가 별도로 기록합니다. Draft 유지, Dev/main/Stable 미병합, 개인 배경 업로드·조정 UI 미추가 조건을 유지합니다.


## d237 구현 이력 — 사용자 Preview 거부로 교체

이 절과 다음 비용 표는 첫 구현의 역사입니다. 사용자가 과한 띠와 중첩 액자를 거부했으며 현재 설계는 아래의 Preview 피드백 수정 절을 따릅니다.

`glass.css`가 Glass의 토큰·형태·반사·상태를 소유합니다. Mecca/Slate는 기존 공통 스타일을 유지하며, 공유 선택기의 역할·검색·키보드 선택·포커스 복귀 로직을 재사용합니다. Glass 팝업만 visualViewport를 기준으로 너비·높이·위치를 제한합니다. 선택기가 처리한 Escape·이동·선택 키는 상위 앱 단축키로 전달하지 않아, 닫힌 뒤 포커스가 다시 사라지는 문제를 방지합니다.

작은 부유 메뉴·툴바·사이드바에는 볼록한 외곽과 방향성 반사·접촉 그림자를, 검색창에는 오목한 홈을 적용했습니다. 선택 상태는 색 외에도 체크·안쪽 경계·누른 깊이로 표시합니다. 밀집된 표·코드·원장·Gantt·카드에는 행별 필터를 추가하지 않습니다. 본문과 컨트롤은 원래 DOM에 남으며 장식층은 `aria-hidden` 및 `pointer-events:none`입니다. 안내 문구·표 머리글·아이콘에는 국소 읽기 바탕을 두어 사진의 밝은 위치에서도 대비를 보호합니다.

`glass-optics.js`는 화면 배경과 같은 사진·cover·중심·고정 stage 좌표를 공유합니다. 최대 2048px 공통 사진 sampler를 한 장 만들고, 대표 영역 최대 7개의 가장자리만 12개의 둥근 띠로 나눠 재표본화합니다. 띠마다 x/y 배율이 달라 가장자리로 갈수록 비선형 확대·축소가 나타납니다. 볼록 면은 양수, 검색 홈은 음수 변위를 씁니다. 캔버스 중앙은 투명하며 텍스트·아이콘·포커스를 복사하거나 휘지 않습니다. 고정 사진을 이용한 국소 굴절 근사이며 전체 화면 3D 엔진이나 DOM 굴절이 아닙니다.

캔버스별 DPR은 최대 1.5, 픽셀 수는 약 120만으로 제한합니다. 위치·크기·사진 세대가 바뀔 때만 그리고 같은 크기에서는 버퍼를 재사용합니다. 상시 RAF·애니메이션·행별 캔버스·반복 필터·will-change는 없습니다. `TaskMeccaGlass.diagnostics()`로 현재 소재, sampler, 영역·픽셀·draw 수와 소요 시간을 확인할 수 있습니다. Atmospheric/None·Mecca/Slate에서는 광학 캔버스를 회수합니다. Canvas2D/Path2D/backdrop-filter 미지원은 소재 바탕으로, reduced-transparency는 단색 바탕으로 대체하며 reduced-motion에서는 전환 효과를 제거합니다.

실제 선택 컨트롤의 위치는 Activity 시간 분석의 `Agent 그룹` 체크박스와 알림 설정의 웹/Telegram/Push `role=switch`입니다. Space로 체크박스를 바꾸고 실제 알림 handler로 스위치를 전환했습니다. 이 revision의 운영 화면에는 네이티브 radio가 없어 실제 사용처 검증은 해당 없음입니다. 원형 radio의 표현은 제공하되 새 제품 선택 흐름을 만들지 않았습니다. 브라우저 알림 공급자만 fixture로 대체했으므로 실제 Push 구독 성공을 주장하지 않습니다.

브라우저 gate는 대표 Dark/Light 메뉴의 중앙 alpha=0·가장자리 픽셀과 동일 사진의 무변형 투영 대비 차이를 검사합니다. 검색·비활성 항목 건너뛰기·선택·Escape 복귀·리사이즈 후 팝업 경계·aria-invalid 표현도 확인합니다. 가상 입력/선택기는 상호작용 하니스이며 제품의 새 설정이 아닙니다. 기존 배경/테마 저장·최초 로드·독립 스크롤·회전·큰 글꼴·TOC·터미널 및 Mecca/Slate 검사도 유지합니다.

2026-11-05 로컬 Chrome 156의 최종 표본에서 텍스트 숨김 화면의 실제 배경을 이용해 desktop/mobile × Dark/Light 총 350개 텍스트의 여러 위치를 비교했습니다. 장면별 최저 대비는 5.67 / 5.11 / 4.84 / 5.67:1이며 4.5:1 미달 표본은 0개였습니다. 이는 대표 위치 점검이며 전체 화면·모든 상태에 대한 WCAG 인증이 아닙니다. 시각 검토는 desktop/mobile 한 묶음과 대비 보완 후 확인 한 묶음으로 제한했습니다. 최종 디자인은 사용자가 Preview에서 수용 판단합니다.

### d237 구현 당시 비용

기준은 `689fa0f2`의 동일 사진·가상 데이터, 결과는 최종 Glass 소재입니다. macOS/설치 Chrome/headless/DPR 1/1.5초 원장 scroll을 다른 검증과 겹치지 않게 순차 측정한 단일 표본입니다.

| 조건 | 평균 frame 간격 | Task 시간 | main-thread Paint 횟수/시간 | JS heap | 테마 전환 평균 | 캔버스 RGBA 추정 |
| --- | --- | --- | --- | --- | --- | --- |
| Desktop | 16.37 → 16.51ms | 27.99 → 47.98ms | 6/1.57 → 19/3.55ms | 5.11 → 4.17MiB | 31.45 → 32.72ms | +9.60MiB |
| Mobile viewport | 16.60 → 16.48ms | 29.28 → 105.97ms | 21/2.01 → 52/9.86ms | 5.17 → 3.75MiB | 32.15 → 31.60ms | +3.89MiB |

최대 frame 간격은 두 조건 모두 약 16.8ms 이내였습니다. 정지 250ms 동안 추가 광학 paint는 0회였습니다. 캔버스 추정은 sampler와 각 버퍼의 픽셀×4이며 JS heap과 별도입니다. 사진 decode·합성·GPU 실제 메모리 총량을 의미하지 않습니다. 낮은 JS heap은 GC 시점의 영향을 받으므로 메모리 개선으로 해석하지 않습니다.

국소 재표본화 때문에 CPU 작업과 paint는 늘었습니다. 추가 Task 시간은 이 표본에서 desktop 약 20ms, mobile viewport 약 77ms/1.5초이며, 직전 동일 구현의 모바일 표본은 약 52ms였습니다. 최대 7영역·버퍼 재사용·세대/좌표 캐시로 범위를 제한했지만 실기기 비용이 낮다고 단정하지 않습니다. 관측된 광학 draw는 메뉴/스크롤/테마 변경에 한정됐고, reader는 viewport 높이를 넘지 않았으며 행·카드 필터는 none이었습니다. 실제 Samsung Internet·주소창/키보드·GPU·CPU 계측·장시간 배터리는 미측정입니다. remote CI·Preview 검증과 운영 Dev67 변경은 Worker가 수행하지 않았습니다.


## Preview 피드백 수정: 소재 면을 정돈하고 확산광을 복구

사용자는 d237 Preview를 “엄청 지저분하고 난잡해졌다”고 평가하고 확산광이 연갈색 단색으로 보인다고 보고했습니다. 앞선 자동 검증과 CI 성공은 사용자 디자인 수용을 뜻하지 않습니다. 이 revision은 새 장식이나 엔진을 추가하지 않고 사진 띠와 반복 경계를 줄였습니다.

- 사진 굴절은 팝업·부유 툴바 최대 2곳의 2px 경계에만 남겼습니다. 기존 최대 7곳·10~12px에서 축소했으며 검색창·사이드바·본문의 캔버스는 제거했습니다. 둥근 띠 12→4개, 변위 7~8→1.6px, 광학 alpha .82→.42, sampler 한도 2048→1536px, 캔버스 한도 약120만→35만 픽셀, DPR 상한1.5→1.25입니다. 중앙 텍스트·아이콘·포커스는 계속 원래 DOM입니다.
- 큰 읽기 면은 한 겹으로 유지하고 행·헤더·내부 툴바는 같은 바탕을 공유합니다. 반복 그림자·밝은 외곽선·모든 컨트롤의 캡슐 광택을 줄였습니다. 사이드바 머리의 안쪽 액자를 없애고, 검색창의 generic input 선택자 명시도를 `:where`로 낮춰 실제 입력이 두 번째 테두리·그림자를 만들던 문제를 해결했습니다. 검색창은 바깥 홈 한 겹만 가집니다.
- Dark 안내 영역과 읽기 면은 같은 차분한 소재로 맞추고 Light의 과한 비침은 읽기 면에서 줄였습니다. 모든 요소를 같은 캡슐로 만들지 않고 큰 면18px·팝업16px·일반 입력9px·내부 선택7~9px로 역할을 구분합니다. 본문 위 여백도 줄였습니다.
- 확산광은 로드 오류가 아니라 기존 warm gradient의 실제 표현이었습니다. 사진을 추가하지 않고 넓은 radial light field와 부드러운 중성·따뜻한 명암 분포로 교체했습니다. None은 단색으로 유지하며 boot/browser chrome의 Glass 기본색도 같은 중성색으로 맞춰 초기 색 전환을 줄였습니다. Cozy 사진·WebP·설정 키·System·고정 stage는 보존합니다.

`tests/helpers/glass_background_review.cjs`는 실제 앱 자산과 가상 API를 사용해 desktop1550×1080/펼친 사이드바 및 mobile390×844에서 Dark/Light × Cozy/ambient/None을 캡처합니다. body 배경만 보이는 실제 PNG의 픽셀 분포도 읽습니다. 확산광의 채널 밝기 범위는 장면에 따라33~99, None은0으로 구분됐습니다. 평균값만으로 밝은 영역과 어두운 영역을 상쇄하지 않고 분산까지 비교합니다. 검색 입력의 border0·shadow none 및 검색/사이드바/본문의 optical canvas0도 검사합니다. 열린 언어·테마 메뉴와 PR/알림·긴 목록을 함께 확인했습니다.

브라우저4건, 관련 Node16건, 안전 named Go 검사, 기존 Mecca/Slate 8조합×대표7요소 전후 외형 비교를 통과했습니다. 12장면의 대표 텍스트1344개에서 4.5:1 미달 표본은0개, 최저4.63:1이었습니다. 이는 대표 위치 표본이며 전체 WCAG 인증이 아닙니다. 최초 desktop/mobile/6조건 묶음 확인 뒤 검색 프레임과 여백을 일괄 보완했고, 확인은 한 번만 추가했습니다. 최종 디자인은 사용자가 새 Preview에서 판단합니다.

사용자가 제공한 Liquid Glass Studio 참고의 SDF 법선·경계 두께에 따른 변위·Fresnel/방향성 반사 개념 검토를 읽었습니다. 그 저장소의 코드·사진·영상을 재사용하거나 데모를 실행하지 않았고, WebGL/WebGPU 엔진을 도입하지 않았습니다. 독립 DOM toolkit이나 삼성 지원·성능이 검증된 것으로 취급하지 않습니다.

### 수정 전후 비용

기준은 사용자에게 거부된 d237 자산이며 사진·가상 API·DPR1·Chrome156·macOS·1.5초 스크롤 조건을 맞춰 순차 관측했습니다.

| 조건 | 평균 frame 간격 | Task | main-thread Paint 횟수/시간 | JS heap | 캔버스 RGBA 추정 | 광학 draw 횟수 |
| --- | --- | --- | --- | --- | --- | --- |
| Desktop | 16.32→16.38ms | 43.64→40.25ms | 20/3.28→12/2.69ms | 4.32→4.44MiB | 9.60→5.55MiB | 114→18 |
| Mobile viewport | 16.63→16.60ms | 87.26→71.52ms | 50/9.78→26/5.93ms | 4.42→4.41MiB | 3.89→2.93MiB | 154→34 |

최대 frame 간격은 전후 모두 약16.8ms 이내, 정지250ms의 추가 광학 paint는 모두0회였습니다. 광학 draw 총 소요 시간은 desktop24.8→21.6ms, mobile26.2→21.6ms였으며 이 합계에는 메뉴 열기·테마 변경·스크롤이 포함됩니다. 8회 테마 전환 평균은 desktop32.71→31.81ms, mobile31.76→31.74ms였습니다. 캔버스 추정은 sampler와 각 버퍼의 픽셀×4이며 JS heap·사진 decode·GPU 총량과 구분합니다. 단일 headless 표본과 GC 시점에 따른 heap 차이를 실기기 성능 개선 배수로 해석하지 않습니다.
 새 revision의 main-thread·paint·광학 draw·heap 및 별도 캔버스 추정은 Worker 보고서에 보존합니다. blur 반경은10px로 정돈했으므로 국소 캔버스 비용 감소만으로 물리 GPU·배터리 개선을 주장하지 않습니다. 삼성 실기기·주소창/키보드·물리 GPU/CPU·장시간 배터리는 미측정입니다. 운영 Web을 변경하지 않고 fixture/browser만 종료합니다. 최종 HEAD의 원격 CI·Preview와 PR 결과 반영은 Controller 담당입니다.

### Chromium headless shell의 광학 비교 교정

수정 Preview의 원격 Linux/Chromium156.0.8078.4 CI에서 광학 비교가 실패했습니다. 기존 검사는 `.42` alpha/2px 림에서 alpha>100인 픽셀만 선택했는데, 같은156.0.8078.4의 macOS headless shell에서도 최대 alpha가100이어서 표본0·평균NaN으로 실패가 재현됐습니다. 설치 Chrome156.0.8078.80은 최대107로4308개 채널을 선택했습니다. 플랫폼의 경계 raster 차이를 효과 유무로 오판한 것이며, 제품 효과·사진·소재 수치는 바꾸지 않았습니다. Linux 자체의 로컬 재현은 하지 못했으므로 최종 원격 CI와 구분합니다.

`tests/helpers/glass_optical_oracle.cjs`는 테스트에서만 Canvas 호출을 관측합니다. 실제 sampler·좌표·DPR·shade·클립·alpha·그리기 호출을 그대로 재생한 대조와, 중앙 기준 배율만 제거한 대조를 만듭니다. 동일 재생은 관측된1단계 8-bit 색 양자화 오차 이내인지 확인하고, 무변형 대조와 alpha mask가 같은지 검사합니다. alpha>100의 절단 대신 alpha 가중치로 충분한 표본과 유한한 차이·재생 잡음 대비 신호·비영의 제한된 좌표 변위를 확인합니다. 통계는 assertion 전에 JSON에 기록합니다.

가상 고대비 격자를 같은 생산 코드의 배경 입력으로만 공급한 기준 패턴에서도 유의한 림 변화가 있어야 합니다. 이어 같은 테스트 route에서 변위만0으로 만든 음성 대조가 광학 계약에 실패해야 합니다. 이는 배포 이미지나 제품 기능이 아니며 CI JSON에는 숫자만 남습니다. 임계값 완화·skip·효과 강도 증가로 통과시키지 않습니다.

156.0.8078.4 headless shell의 실제 사진은 desktop/mobile 모두1602개 유효 픽셀과561.2의 alpha 가중 표본을 얻었습니다. 동일한 무변형 참조와의 가중 평균 차이는6.31/4.89, 고대비 기준은45.80/30.76이었고 실제 변위는 약1.71px였습니다. 변위0 대조는 차이0·변위0으로 의도대로 거부됐습니다. 156.0.8078.80에서도 같은 검사와 음성 대조를 통과했습니다. 관련 키보드·검색·선택·포커스·checkbox/switch를 포함한 브라우저 검사와 Node12건을 확인했으며, 전체 시각 점검·성능 계측·Go 검사는 제품 변경이 없어 반복하지 않았습니다.

## A-56: 패널 배경 적응과 얇은 색 경계

Light/Dark는 기본 재질과 전체 밝기 선호이며 System은 OS를 따릅니다. 저장 키와 첫 paint 이전 복원은 그대로입니다. Glass의 상단, sidebar, 읽기 면, 열린 메뉴/팝업과 안내 패널은 실제 고정 배경에서 패널 뒤 영역을 11×7점으로 표본화합니다. 밝기는 선형 sRGB 상대 휘도의 평균으로, 혼합도는 범위로 판단합니다. 밝기 .28/.48 사이에서는 같은 테마의 이전 극성을 유지하고, 명시적 테마 변경에는 새 선호를 기준으로 다시 판단합니다. 혼합도에도 .24/.38 완충 구간이 있습니다. 행·셀 단위 색 전환은 없습니다.

Cozy는 기존 한 번 디코드한 사진과 cover/center crop, 고정 stage, 전역 shade를 그대로 사용합니다. Atmospheric는 실제 computed CSS의 선형/타원형 radial gradient 층과 색·위치를 같은 제한 샘플러에 재현합니다. None은 실제 배경 단색을 표본화합니다. 샘플 색은 중립 경계 반사에 18%만 섞습니다. 패널의 안정된 읽기 alpha는 .78, 밝고 어두운 영역이 섞인 면은 .86입니다. 작은 투명도 변화도 완충 구간을 거칩니다.

광학 면은 기존 최대 두 개이며 rim 2px, 변위 1.6px 제한을 유지합니다. 그 곡선 위 0.75px stroke에만 청록/청색과 금빛을 방향성으로 배치하고 샘플 밝기로 강도를 조절합니다. 입력·검색·sidebar·읽기 면에는 새 canvas나 반복 광택을 붙이지 않습니다. 텍스트·포커스는 원래 DOM입니다. DOM 전체의 물리적으로 정확한 광학 시뮬레이션은 아닙니다. 외부 코드나 데모 자산을 복사하지 않았습니다.

재평가는 배경/테마 변경, DOM 면 열림, resize, focus 및 scroll 이벤트에 한정합니다. Scroll은 100ms로 묶고, 같은 세대/12px 격자 영역은 재계산하지 않습니다. 정지 RAF나 타이머 반복은 없습니다. 사진을 테마 변경마다 다시 디코드하지 않습니다. 샘플러 픽셀 배열 하나를 추가로 유지하므로 메모리 증가는 아래 비용과 함께 봐야 합니다. 이미지 미준비/실패, 지원하지 않는 gradient/읽기 실패, blur 미지원, reduced-transparency에서는 선택 테마의 기존 읽기 기본값으로 돌아갑니다. Reduced-motion은 기존 정적 표현을 유지합니다.

추가 실행:

```sh
node --test tests/web_glass_theme.test.cjs tests/web_glass_browser.test.cjs tests/web_glass_adaptive.test.cjs
```

새 검사는 실제 배포 사진의 노출을 fixture에서 조정해 같은 패널의 밝음/어두움/혼합 조건을 만듭니다. 실제 screenshot에서 텍스트를 숨긴 배경 픽셀과 computed foreground를 비교하며, 대표 텍스트 4.5:1/큰 텍스트 3:1을 검사합니다. 키보드 포커스도 실제 outline와 인접 screenshot 픽셀로 3:1을 검사합니다. 모든 임의 콘텐츠와 전 브라우저의 AA 픽셀을 보장하는 감사는 아닙니다.

스펙트럼 대조는 같은 DOM/좌표/시각에서 실제 draw call을 재생하고 색 stroke만 제외합니다. 양성/음성 screenshot 차이가 2px 이내 곡선 경계에만 있고 중앙 차이가 0인지 검사합니다. 기존 굴절 검사는 동일 alpha의 실제 draw call 재생과 변위 0 음성 대조를 유지하며 stroke도 양쪽에 똑같이 재생합니다. 재로드 전후를 비교하던 초기 시험은 화면 상태가 달라 부적절했고, 곡선 모서리를 직사각형 중앙으로 잘못 분류하던 시험도 수정했습니다. threshold를 완화하거나 skip으로 통과시키지 않았습니다.

CI에는 정량 JSON만 보존합니다. PNG와 사용자 reference는 업로드하지 않습니다. 로컬에서는 동적 포트 fixture와 그 browser만 finally에서 종료합니다. 운영 18765 서비스, 설치, Preview 전환, 업데이트, 재시작은 조작하지 않습니다. 전체 Go uninstall fixture를 실행하지 않고 관련 named Go와 Node/browser 검사만 사용합니다. 실제 Samsung Internet, 주소창/키보드 실기기, 물리 GPU/CPU·배터리·장시간 사용은 미검증이며 사용자 최종 디자인 수용은 별도 Preview 검토로 남습니다.

A-56 로컬 Chrome 156.0.8078.80, DPR 1, desktop 1440×960 / touch-emulated 390×844, 동일 1.5초 scroll에서 4995c6f0와 비교한 탐색적 관측입니다. 원본 정량 JSON은 Worker evidence에 보존합니다.

| 조건 | script ms 4995 → A-56 | task ms | paint 횟수/ms | canvas + 샘플 배열 추정 MiB |
| --- | --- | --- | --- | --- |
| Desktop | 22.898 → 12.668 | 68.442 → 64.832 | 11/4.991 → 12/3.044 | 5.55 → 10.82 |
| Touch emulation | 30.918 → 17.726 | 91.092 → 122.761 | 29/7.045 → 57/13.021 | 2.93 → 5.64 |

정지 250ms에서 두 조건 모두 추가 optical paint와 적응 계산은 0회였습니다. 추가 픽셀 배열은 desktop 5,529,600B, touch 2,849,344B입니다. 이 추정에는 기존 decode bitmap·브라우저/GPU 복사본을 더하지 않았습니다. Touch의 task/paint 증가는 그대로 보고하며, run 간 변동이 있는 headless main-thread 관측을 실기기 성능 향상으로 해석하지 않습니다. 샘플링이 전체 흐름에 더하는 비용과 메모리 증가가 있습니다.

최종 screenshot 표본은 비활성 컨트롤 24개를 구분한 일반/큰 텍스트 1,116개에서 최소 5.386:1, 실제 키보드 focus outline 네 표본에서 최소 7.886:1이었습니다. 본문뿐 아니라 상단·안내 면의 foreground opacity도 합성해서 계산합니다. 색 stroke 제외 대조는 desktop 2,052 / mobile 650개 경계 픽셀에서 차이가 있었고, 2px보다 안쪽 중앙 변화는 0이었습니다. 이는 대표 표본과 실제 렌더 차이의 근거이며 전체 화면·모든 기기의 최종 디자인 수용은 아닙니다.

### A-56 확산광 경로 수정과 독립 raster 검증

첫 A-56 구현 `0f507259`는 shipped CSS의 `ellipse at …`가 브라우저에서 `radial-gradient(at …)`로 직렬화되는 경로를 놓쳐 Light/Dark 확산광 모두 `material` fallback/적응 패널 0개로 표시했습니다. 설정·OS 검사만으로 이 경로를 검증했다고 보았던 초기 보고는 이 재검증으로 정정합니다. 당시 보고/실패와 증거는 삭제하지 않습니다.

수정은 실제 `at` 좌표를 사용하고, 기본 farthest-corner 타원의 farthest-side 종횡비를 유지하면서 가장 먼 모서리를 지나는 반경을 계산합니다. 명시적 %/px 타원 반경도 처리합니다. CSS stop의 생략 위치와 단조 증가를 맞추고, CSS의 premultiplied sRGB와 Canvas의 straight RGB 보간 차이로 생기던 어두운 투명 fringe를 제거했습니다. 현재 색→투명 stop은 같은 색으로 정확히 fade하며, 일반 varying-alpha segment는 최대 32개 stop으로 근사합니다. 전체 CSS gradient 문법을 구현한 엔진은 아니며 미지원 문법/색 공간은 읽기 가능한 선택 테마 기본값으로 돌아갑니다.

새 검사는 실제 production sampler의 readback을 관측하고 **독립된 브라우저 CSS screenshot**과 비교합니다. desktop/mobile, Light/Dark 확산광/None, scroll와 viewport 높이 변경에 따른 fixed-stage crop, radial 위치와 색 변경 대조를 포함합니다. 명시적으로 active mode와 실제 패널 적응을 요구하므로 fallback은 통과할 수 없습니다. 위치만 옮길 때 같은 sidebar의 글자와 밝기 판단이 바뀌고, 색만 바꿀 때 같은 topbar가 반응합니다. 미지원 conic은 음성 대조로 안전 fallback을 확인합니다. CI는 `ambient/ambient.json` 정량 기록만 추가 보존합니다.

`GLASS_MEASURE_BACKGROUND=ambient`로 동일 Chrome 156.0.8078.80/DPR1/fixture/1.5초 scroll에서 초기 material fallback과 수정 후 active를 비교했습니다. Desktop script 6.277→11.641ms, task 45.381→53.475ms, paint 39/5.598→8/3.032ms; touch emulation script 6.094→18.253ms, task 45.642→119.516ms, paint 69/6.488→55/12.104ms입니다. 처음에는 기능이 비활성이었으므로 비용 증가를 정상 기능 활성화의 비용으로 구분합니다. Ambient sampler canvas+readback 추정은 0→desktop 11,059,200B / mobile 5,698,688B이며 물리 GPU/CPU/배터리 실측이 아닙니다. 정지 추가 paint/적응 계산은 두 조건 0회입니다.

스펙트럼 stroke와 굴절 canvas는 **Cozy의 최대 두 면에만 적용**합니다. Ambient/None은 패널 대비 적응만 수행하며 광학 canvas는 0개입니다. 모든 배경에 같은 스펙트럼 광학 효과를 제공한다고 해석하지 않습니다. 실기기·사용자 최종 디자인 수용·운영 변경 제한은 위와 같습니다.

수정 후 20개 scene, 독립 raster 표본 980점에서 최대 채널 오차 3/255, scene 평균 채널 오차 최대 0.599/255, 상대 휘도 오차 최대 0.00795였습니다. None 단색은 채널 오차 0입니다. 활성 상단·본문·안내 텍스트 1,830개 표본의 최저 대비는 5.121:1입니다. 실제 ambient/None 활성화를 추가로 요구하며 원래 사진 노출·hysteresis·키보드·복원·실패·미지원/reduced 검사를 유지합니다. 비Glass palette로 이동할 때 다음 frame의 실제 장식 제거를 기다리는 검사 조건도 보완했습니다.

### A-56 headless / 영어 locale 전환 프레임 수정

Linux CI41971930004는 첫 bright 사진의 필터 글자 대비와 Glass 전환 직후 backing 검사를 실패했습니다. macOS에서도 같은 Chromium headless156.0.8078.4/en-US의 프레임 probe에서 `tone=light`/밝은 reader인데 상태 필터 글자는 흰색인 첫 프레임을 확인했습니다. 기존 `.12s color` 전환이 재질보다 늦게 끝났고, 고정180ms 측정은 decode/재샘플/DOM 교체가 늦으면 앞 시점 글자와 뒤 시점 배경을 짝지을 수도 있었습니다. 실제 `render()` 직후 reader와 fallback이 모두 없는 경우도 확인했습니다.

Glass의 상태 필터·copy 버튼·TOC 링크는 색 전환만 제거해 글자와 재질 극성을 원자적으로 바꿉니다. 기존 hover fill/경계/transform 전환과 Mecca/Slate는 유지합니다. Pseudo reader는 전역 active flag가 아니라 **그 owner의 실제 backing이 연결되어 있을 때만** 숨깁니다. `:has` 미지원에서는 JS backing을 만들지 않고 CSS pseudo를 유지합니다. 광학 면 수·사진·스펙트럼 크기·설정·레이아웃을 재설계하지 않았습니다.

Read-only diagnostics에는 실제 sampler의 generation/theme/background/source와 pending/decode 상태를 추가했습니다. 새 fixture observer는 현재 CSS URL과 실제 decoded/sample URL, 같은 generation, 연결된 host/plane, computed foreground, 폰트/진행 중 transition을 확인합니다. 고정 지연 대신 같은 fingerprint의 안정 프레임을 기다리고 screenshot 전후도 대조합니다. 중간 DOM/색/세대가 달라지면 그 capture를 근거와 함께 버리고 최대3회까지만 재취득하며, 대비 assertion 자체를 재시도하지 않습니다. 영어 표본은 제외하지 않습니다. OS 변경도 요청한 OS 테마 event의 실제 반영을 기다립니다. Ambient readback 관측은 cache가 유효한 데이터를 지우지 않고 실제 CSS source/theme/background와 맞는지 검사합니다.

새 회귀는 양방향 밝기 전환 56프레임의 실제 재질/글자 짝, render 직후 reader 또는 fallback 존재, optical callback을 지연시킨 실제 화면의 fallback 양성/없음 음성 pixel 차이, selector 미지원 대체 표현을 검사합니다. 광학 음성 대조는 8-bit get/put 및 canvas copy를 거치지 않고 **동일 context에 실제 draw call을 다시 재생하면서 stroke만 제외**합니다. 양성 재생의 기존8-bit oracle 오차 한계1을 유지하고, foreground/중앙 변화는0을 요구합니다. Rim은 CSS 좌표를 screenshot pixel로 가정한 거리식 대신 브라우저가 그린 독립2 CSS px 경계 raster와 대조합니다. 대비4.5/3, focus3, rim2px, 중앙0 및 기존 alpha가중 광학 signal 기준은 완화하지 않습니다.

Node22.23.4를 격리 runtime에서 사용하고 headless156.0.8078.4 및 설치 Chrome156.0.8078.80, en-US, 세 test 파일 동시 실행을 비교합니다. macOS ARM의 동일 엔진 확인이며 Ubuntu26.04/x64 실행 성공과 동일시하지 않습니다. Linux exact-head CI는 Controller가 확인합니다. 이번 수정에서는 성능/Go를 반복하지 않았고 새 전면 canvas·sampling loop·상시 animation은 추가하지 않았습니다. 원격 CI·Preview·최종 사용자 수용과 운영 변경 제한은 그대로입니다.

최종 macOS 동일-client 확인은 Playwright1.64.1/Node22.23.4/headless156.0.8078.4/en-US/세 파일 동시 실행13/13(skip0)입니다. 설치 Chrome.80의 영향 회귀13/13은 같은 제품 소스와 Playwright1.62.1에서 별도 확인했으며 두 client 환경을 구분합니다. 원본d7의 동일-client macOS suite는10/12로 headless rim 좌표 대조와 readback 관측 cache race를 실패했고, Linux CI의 두 assertion 조합을 그대로 재현했다고 주장하지 않습니다. 실제 제품의 전환 불일치/reader 공백은 별도 frame probe로 전후 재현했습니다.

양성 광학 대조는 해당 canvas의 실제 가시 영역을 검사하고, 다른 영역의 변화 좌표는 별도 기록합니다. 일부 full-page 재생 screenshot의 20개 변화는 y179~209의 backlog 제어 경계로, 측정 대상 상단 canvas(y9~63) 밖이었습니다. 검사 대상 foreground/중앙0과 기존 1-byte oracle 오차 한계는 유지합니다. 최종 actual2CSSpx reference와 양성 재생을 포함한 PNG는 로컬 증거이며 CI에는 JSON만 남습니다.


## Superseded material rework with ink support (A-56, 2026-11-05)

The user rejected the broad neutral .86/.78 overlay after reviewing e6fa63bb. The
approved revision separates the broad material from ink contrast. In Cozy, the
sidebar, floating chrome/menu and bounded reader transmit the shared background
sampler with a .14 neutral tint. A smooth 8 CSS px lateral and 4 px vertical
displacement plus mild magnification bends background contours continuously; the
foreground DOM and focus outlines remain outside all filters/transforms. The
existing narrow spectral lip remains a secondary cue, not a photographic frame.
A one-pixel directional reader reflection defines thickness without nesting frames.

The shared photo is decoded once for optics, as before. Each visible material
canvas is capped at approximately 480,000 pixels (rounding each dimension can add
less than one row/column). One event-driven pass draws the broad material; no
per-row lens or frame loop is installed. Text geometry is grouped into small
semantic blocks and receives a soft neutral support only where the sampled scene
and authored child fills cannot support the computed ink color. `color(srgb ...)`
from computed color-mix is normalized correctly. Visible decorative text, SVG
navigation icons and input values/placeholders are protected too. This is a
bounded representative contrast check, not an exhaustive accessibility audit.

Reader DOM replacement reconnects both the material plane and its canvas. A CSS
solid fallback protects the short plane-absent interval; missing selector support,
missing Canvas/backdrop support, decode failure and reduced transparency retain
readable theme defaults. Ambient and None retain the original adaptive CSS
material path. Light/Dark remain user preferences and System follows OS. Mecca and
Slate do not receive any of these layers.

Validation keeps the existing 14 browser/theme/adaptive cases and AA/focus
thresholds. It additionally measures actual search placeholder raster contrast and
asserts that DOM replacement reconnects the continuous material canvas. Screenshot
readiness rejects a material-marked host whose canvas is missing. The measurement
helper now includes material canvas RGBA bytes as well as the existing sampler and
readback in memory estimates. Only the named embedded photographic/Glass delivery
Go tests run locally; the unprotected full Go suite is reserved for remote CI.

Bounded visual review covered a real detail renderer with sidebar expanded,
topbar, long reading surface and open menu in Light/Dark, plus mobile counterparts.
Warm/green scene regions, an inside/outside blur difference and continuous
background bending are visible compared with the rejected baseline. Dark still
shows some distinct local fog blocks around the title/Lifecycle. This limitation
remains an explicit Preview review item. Neither code properties, numeric contrast
nor this bounded review constitutes final user design acceptance. Physical Samsung
Internet, GPU/CPU energy, battery and extended device use remain unverified.


A serial local measurement used Chrome 156.0.8078.80, DPR 1, the same fixture,
Cozy photograph and 1.5-second scripted scroll. The full browser suite and Go
tests had finished before this measurement. This is one bounded sample; the
first concurrently measured run is retained as provisional and is not used below.

| Metric | Desktop e6fa63bb → revision | Mobile e6fa63bb → revision |
| --- | --- | --- |
| main-thread TaskDuration delta | 56.628 → 125.124 ms | 95.500 → 108.529 ms |
| ScriptDuration delta | 9.360 → 83.652 ms | 15.130 → 56.567 ms |
| paint events / total trace duration | 17 / 3.351 → 19 / 4.318 ms | 48 / 10.400 → 48 / 6.208 ms |
| mean animation-frame gap | 16.372 → 16.300 ms | 16.590 → 16.609 ms |
| sampler + readback + material RGBA estimate | 11,347,560 → 13,786,648 B | 5,919,072 → 7,125,088 B |
| idle material paints / adaptations | 0 / 0 → 0 / 0 | 0 / 0 → 0 / 0 |

The new material costs more script time and approximately 2.44 MB desktop /
1.21 MB mobile explicit RGBA storage. Mean/max frame gaps did not show a sustained
scroll slowdown in this sample (maximum about 16.8 ms in both revisions). Lower
mobile paint time in a single sample is not a general performance improvement.
These counters and byte estimates exclude GPU textures, energy, physical device
performance and total browser process memory.


The `MAX_LENSES=2` limit belongs only to narrow spectral rim canvases. Continuous
material canvases are a separate map: one per visible matching material host,
with a per-canvas pixel cap and no independent count cap. Their count depends on
the visible chrome, reader, sidebar and open popup/drawer topology. Diagnostics
report both maps separately. Each material reports actual text nodes visited,
visible ink rectangles, protected groups, last/cumulative geometry measurement
time and last/cumulative drawing time. The shared `totalPaintMs` measures drawing
only; `totalInkMeasureMs` must be considered separately for the DOM walk cost.

A local independent material witness replays the actual drawImage calls with an
identity screen-to-sampler source rectangle, preserving tint, ink support,
reflection, blur, dimensions and foreground. In 426,580 interior reader pixels,
72.802% differed by more than two average channel steps, with a 13.707 mean
channel difference. The foreground color and canvas dimensions stayed equal.
This verifies a broad actual source-mapping effect; it does not establish user
acceptance or a physically accurate optical simulation.


A second serial density sample used the actual 36-row list and a fixture-only
500-row markdown detail table, in the desktop browser and a mobile viewport.
The content subtree contained 719/478 element/text nodes for the baseline list,
and 2,201/1,613 for the long detail; the revision adds one decorative material
canvas. At the sampled final scroll position, the revised reader visited all
478/1,613 text nodes but retained 121 visible ink rectangles for the list and
27 desktop / 26 mobile rectangles for the long detail. Last geometry collection
was about 0.9 / 1.7 / 2.6 ms respectively; cumulative reader geometry collection
was 15.1 / 24.5 / 26.2 ms over the scripted sample.

| Density sample | TaskDuration baseline → revision | ScriptDuration baseline → revision | Drawing delta baseline → revision | Adaptation delta baseline → revision |
| --- | --- | --- | --- | --- |
| 36-row list, desktop | 72.077 → 109.462 ms | 34.765 → 74.012 ms | 0 → 27.8 ms | 11.7 → 11.8 ms |
| 500-row detail, desktop | 74.873 → 107.703 ms | 26.298 → 58.764 ms | 0 → 10.2 ms | 7.3 → 7.3 ms |
| 500-row detail, mobile viewport | 102.515 → 145.604 ms | 33.622 → 75.167 ms | 0.5 → 8.2 ms | 16.3 → 18.0 ms |

Each sample had 91 animation frames, a mean gap around 16.5–16.6 ms and a maximum
about 16.8 ms. The revision had three material canvases in desktop scenes and two
in the mobile scene, with one separate rim canvas. The DOM walk remains linear
in total content text-node count; the viewport test avoids color/geometry work
for offscreen text but is not a subtree traversal cap. Documents much longer
than this 500-row fixture and physical mobile performance remain unverified.
