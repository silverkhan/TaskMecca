# 실사 배경과 Glass 소재 검증

PR #348의 Cozy 기본 배경을 AI로 생성한 실사풍 Cozy Beige Room으로 교체했습니다. 원래 `cozy` 저장값과 Cozy / Atmospheric / None, Dark / Light / System을 유지합니다. 이전 fb4fe7cf의 alpha 패널은 사용자가 거절한 과거 revision이며, 당시 CI·Preview 성공을 새 디자인의 수용 근거로 사용하지 않습니다. 최종 디자인 판단은 사용자가 최신 Preview에서 합니다.

새 소재는 읽기 화면 뒤의 한 장짜리 frost backing pane과 별도 고정 메뉴·팝업으로 구성합니다. 사진의 형태·빛은 유리 아래로 투과하되 디테일만 흐려지고, 가장자리 반사와 내부 경계·바깥 그림자로 표면을 구분합니다. Light의 크림 채움과 Dark의 전역 어두운 막을 줄였으며 고밀도 읽기는 지역적으로 보호합니다. CSS backdrop blur·밝기·채도·반사 표현이며 실제 물리 광학 굴절 시뮬레이션은 아닙니다.

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

읽기 pane의 filter는 `.content` 자체가 아닌 `::before`에 적용합니다. sticky와 같은 크기의 음수 margin으로 flow 높이를 상쇄해 긴 원장 전체를 필터 면으로 만들지 않습니다. desktop blur 12px, mobile 8px이며 row/card/code/Gantt에는 반복 backdrop filter가 없습니다. filter/transform/contain을 content 부모에 걸지 않으므로 fixed TOC의 containing block을 바꾸지 않습니다. Terminal의 별도 reader backing은 viewport 안의 terminal main에 한정됩니다.

CDP의 실제 pseudo box model과 가시 교차 영역을 기록하고, 내용이 약 3,000–3,800px이어도 backing 높이가 viewport보다 작은지 검사합니다. `.querySelectorAll('*')` 개수만으로 pseudo 비용을 누락하지 않습니다. reader 하나 외에 고정 메뉴와 활성 팝업의 제한된 filter가 있으며, **부모 하나라는 이유만으로 GPU 비용이 작다고 단정하지 않습니다.**

## 격리 재현과 자동 gate

실제 embedded Web 자산과 모든 `/api/` 요청을 합성 응답으로 처리하는 fixture입니다. 프로젝트 `/fixtures/atlas`, 백로그 `/fixtures/atlas/_task_mecca/data/backlog`, DEMO-100부터 36개 작업, timed segment 2개와 가상 Root/attempt/상태 이력을 사용합니다. shell/실제 PTY·운영 서버·사용자 원장을 사용하지 않으며 브라우저 외부 네트워크도 차단합니다.

```sh
npm install --prefix /tmp/taskmecca-glass-runtime --no-save --no-audit --no-fund playwright@1.64.1
/tmp/taskmecca-glass-runtime/node_modules/.bin/playwright install chromium
export GLASS_PLAYWRIGHT_MODULE=/tmp/taskmecca-glass-runtime/node_modules/playwright
export GLASS_EVIDENCE_DIR=/tmp/taskmecca-glass-evidence
node --test tests/web_glass_theme.test.cjs tests/web_glass_browser.test.cjs
```

macOS 설치 Chrome 사용 시 `CHROME_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'`을 추가합니다. CI `glass-browser` job은 Node 22/Playwright 1.64.1/설치 Chromium을 격리 prefix에서 실행하고 JSON·대표 PNG를 artifact로 보존합니다. Node 목록의 Glass gate도 유지합니다.

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

자동 검사와 Root/Controller의 시각 검토는 사용자 디자인 승인이 아닙니다. 새 최종 SHA의 CI·Preview와 사용자 수용 판단을 별도로 기록합니다. Draft 유지, Dev/main/Stable 미병합, 개인 배경 업로드·조정 UI 미추가 조건을 유지합니다.
