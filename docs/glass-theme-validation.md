# 실사 배경과 Glass 소재 검증

PR #348의 Cozy 기본 배경을 AI로 생성한 실사풍 Cozy Beige Room으로 교체했습니다. 원래 `cozy` 저장값과 Cozy / Atmospheric / None, Dark / Light / System을 유지합니다. 이전 fb4fe7cf의 alpha 패널은 사용자가 거절한 과거 revision이며, 당시 CI·Preview 성공을 새 디자인의 수용 근거로 사용하지 않습니다. 최종 디자인 판단은 사용자가 최신 Preview에서 합니다.

새 소재는 화면 뒤의 viewport 한정 읽기 바탕과 부유 메뉴·팝업으로 구성합니다. 사진의 형태와 빛은 유리 아래로 투과하며, 가장자리 반사·두께·접촉 그림자로 표면을 구분합니다. CSS blur·반사와 같은 사진의 국소 재표본화를 사용하며, 아래 A-55 항목에 광학 구현과 한계를 설명합니다.

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

읽기 pane의 filter는 `.content` 자체가 아닌 별도 장식 바탕에 적용합니다. Cozy의 광학 표현이 활성화되면 aria-hidden `.glass-reader-material` 요소를 쓰고, 그 외에는 `::before`로 대체합니다. sticky와 같은 크기의 음수 margin으로 flow 높이를 상쇄해 긴 원장 전체를 필터 면으로 만들지 않습니다. reader blur 6px, chrome blur 5px이며 row/card/code/Gantt에는 반복 backdrop filter가 없습니다. filter/transform/contain을 content 부모에 걸지 않으므로 fixed TOC의 containing block을 바꾸지 않습니다. Terminal의 별도 reader backing은 viewport 안의 terminal main에 한정됩니다.

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


## A-55: Glass 전용 소재와 곡면 가장자리

`glass.css`가 Glass의 토큰·형태·반사·상태를 소유합니다. Mecca/Slate는 기존 공통 스타일을 유지하며, 공유 선택기의 역할·검색·키보드 선택·포커스 복귀 로직을 재사용합니다. Glass 팝업만 visualViewport를 기준으로 너비·높이·위치를 제한합니다. 선택기가 처리한 Escape·이동·선택 키는 상위 앱 단축키로 전달하지 않아, 닫힌 뒤 포커스가 다시 사라지는 문제를 방지합니다.

작은 부유 메뉴·툴바·사이드바에는 볼록한 외곽과 방향성 반사·접촉 그림자를, 검색창에는 오목한 홈을 적용했습니다. 선택 상태는 색 외에도 체크·안쪽 경계·누른 깊이로 표시합니다. 밀집된 표·코드·원장·Gantt·카드에는 행별 필터를 추가하지 않습니다. 본문과 컨트롤은 원래 DOM에 남으며 장식층은 `aria-hidden` 및 `pointer-events:none`입니다. 안내 문구·표 머리글·아이콘에는 국소 읽기 바탕을 두어 사진의 밝은 위치에서도 대비를 보호합니다.

`glass-optics.js`는 화면 배경과 같은 사진·cover·중심·고정 stage 좌표를 공유합니다. 최대 2048px 공통 사진 sampler를 한 장 만들고, 대표 영역 최대 7개의 가장자리만 12개의 둥근 띠로 나눠 재표본화합니다. 띠마다 x/y 배율이 달라 가장자리로 갈수록 비선형 확대·축소가 나타납니다. 볼록 면은 양수, 검색 홈은 음수 변위를 씁니다. 캔버스 중앙은 투명하며 텍스트·아이콘·포커스를 복사하거나 휘지 않습니다. 고정 사진을 이용한 국소 굴절 근사이며 전체 화면 3D 엔진이나 DOM 굴절이 아닙니다.

캔버스별 DPR은 최대 1.5, 픽셀 수는 약 120만으로 제한합니다. 위치·크기·사진 세대가 바뀔 때만 그리고 같은 크기에서는 버퍼를 재사용합니다. 상시 RAF·애니메이션·행별 캔버스·반복 필터·will-change는 없습니다. `TaskMeccaGlass.diagnostics()`로 현재 소재, sampler, 영역·픽셀·draw 수와 소요 시간을 확인할 수 있습니다. Atmospheric/None·Mecca/Slate에서는 광학 캔버스를 회수합니다. Canvas2D/Path2D/backdrop-filter 미지원은 소재 바탕으로, reduced-transparency는 단색 바탕으로 대체하며 reduced-motion에서는 전환 효과를 제거합니다.

실제 선택 컨트롤의 위치는 Activity 시간 분석의 `Agent 그룹` 체크박스와 알림 설정의 웹/Telegram/Push `role=switch`입니다. Space로 체크박스를 바꾸고 실제 알림 handler로 스위치를 전환했습니다. 이 revision의 운영 화면에는 네이티브 radio가 없어 실제 사용처 검증은 해당 없음입니다. 원형 radio의 표현은 제공하되 새 제품 선택 흐름을 만들지 않았습니다. 브라우저 알림 공급자만 fixture로 대체했으므로 실제 Push 구독 성공을 주장하지 않습니다.

브라우저 gate는 대표 Dark/Light 메뉴의 중앙 alpha=0·가장자리 픽셀과 동일 사진의 무변형 투영 대비 차이를 검사합니다. 검색·비활성 항목 건너뛰기·선택·Escape 복귀·리사이즈 후 팝업 경계·aria-invalid 표현도 확인합니다. 가상 입력/선택기는 상호작용 하니스이며 제품의 새 설정이 아닙니다. 기존 배경/테마 저장·최초 로드·독립 스크롤·회전·큰 글꼴·TOC·터미널 및 Mecca/Slate 검사도 유지합니다.

2026-11-05 로컬 Chrome 156의 최종 표본에서 텍스트 숨김 화면의 실제 배경을 이용해 desktop/mobile × Dark/Light 총 350개 텍스트의 여러 위치를 비교했습니다. 장면별 최저 대비는 5.67 / 5.11 / 4.84 / 5.67:1이며 4.5:1 미달 표본은 0개였습니다. 이는 대표 위치 점검이며 전체 화면·모든 상태에 대한 WCAG 인증이 아닙니다. 시각 검토는 desktop/mobile 한 묶음과 대비 보완 후 확인 한 묶음으로 제한했습니다. 최종 디자인은 사용자가 Preview에서 수용 판단합니다.

### A-55 변경 전후 비용

기준은 `689fa0f2`의 동일 사진·가상 데이터, 결과는 최종 Glass 소재입니다. macOS/설치 Chrome/headless/DPR 1/1.5초 원장 scroll을 다른 검증과 겹치지 않게 순차 측정한 단일 표본입니다.

| 조건 | 평균 frame 간격 | Task 시간 | main-thread Paint 횟수/시간 | JS heap | 테마 전환 평균 | 캔버스 RGBA 추정 |
| --- | --- | --- | --- | --- | --- | --- |
| Desktop | 16.37 → 16.51ms | 27.99 → 47.98ms | 6/1.57 → 19/3.55ms | 5.11 → 4.17MiB | 31.45 → 32.72ms | +9.60MiB |
| Mobile viewport | 16.60 → 16.48ms | 29.28 → 105.97ms | 21/2.01 → 52/9.86ms | 5.17 → 3.75MiB | 32.15 → 31.60ms | +3.89MiB |

최대 frame 간격은 두 조건 모두 약 16.8ms 이내였습니다. 정지 250ms 동안 추가 광학 paint는 0회였습니다. 캔버스 추정은 sampler와 각 버퍼의 픽셀×4이며 JS heap과 별도입니다. 사진 decode·합성·GPU 실제 메모리 총량을 의미하지 않습니다. 낮은 JS heap은 GC 시점의 영향을 받으므로 메모리 개선으로 해석하지 않습니다.

국소 재표본화 때문에 CPU 작업과 paint는 늘었습니다. 추가 Task 시간은 이 표본에서 desktop 약 20ms, mobile viewport 약 77ms/1.5초이며, 직전 동일 구현의 모바일 표본은 약 52ms였습니다. 최대 7영역·버퍼 재사용·세대/좌표 캐시로 범위를 제한했지만 실기기 비용이 낮다고 단정하지 않습니다. 관측된 광학 draw는 메뉴/스크롤/테마 변경에 한정됐고, reader는 viewport 높이를 넘지 않았으며 행·카드 필터는 none이었습니다. 실제 Samsung Internet·주소창/키보드·GPU·CPU 계측·장시간 배터리는 미측정입니다. remote CI·Preview 검증과 운영 Dev67 변경은 Worker가 수행하지 않았습니다.
