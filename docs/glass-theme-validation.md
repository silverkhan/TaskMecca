# Glass 테마 검증과 재현

PR #348은 기존 Cozy Beige SVG와 Cozy / Atmospheric / None 선택을 유지하면서 Glass를 데이터·고정 메뉴·팝업의 세 소재로 정리합니다. 데이터 카드·표·코드·Gantt에는 backdrop blur를 적용하지 않습니다. 긴 목록과 본문에는 따뜻한 반투명 읽기 보호층을 두고, 여백과 소재 가장자리에서는 방 배경이 드러납니다. Mecca와 Slate 규칙은 수정하지 않습니다.

`theme-boot.js`는 첫 paint 전에 screen 크기로 배경 canvas의 pixel 크기를 정합니다. 터치 화면에서는 긴 변을 한 변으로 하는 정방형을 사용하며, left/top 0과 이미지 scale을 유지합니다. 주소창·키보드·문서/독립 메뉴 스크롤·회전 때 배경을 다시 확대하거나 중앙에 재배치하지 않습니다. 회전하면 같은 이미지의 보이는 범위가 달라집니다. 창을 물리 화면보다 크게 확장하는 특수한 다중 모니터 조건과 Samsung Internet 실기기는 추가 확인 대상입니다.

blur 미지원 환경에서는 읽기 소재를 불투명하게 하고 방 배경은 남깁니다. `prefers-reduced-transparency: reduce`는 소재를 불투명하게 하고 배경 이미지도 제거합니다. 두 경로를 구분합니다. `prefers-reduced-motion`에서는 Glass의 animation과 transition을 끕니다.

## 안전한 로컬 재현

운영 서버나 실제 업무 원장을 사용하지 않습니다. `glass_fixture.cjs`는 실제 embedded Web 자산을 제공하고 모든 `/api/` 요청을 메모리의 합성 응답으로 처리합니다. 파일 수정, shell 실행, Web 서비스 조작은 하지 않습니다. 프로젝트 `/fixtures/atlas`, 백로그 `/fixtures/atlas/_task_mecca/data/backlog`, DEMO-100부터 36개 작업, 시간 구간 2개와 가상 Root/attempt/상태 이력을 사용합니다. 브라우저는 별도 anonymous context이며 외부 네트워크 요청은 차단합니다.

```sh
npm install --prefix /tmp/taskmecca-glass-runtime --no-save --no-audit --no-fund playwright@1.64.1
/tmp/taskmecca-glass-runtime/node_modules/.bin/playwright install chromium
export GLASS_PLAYWRIGHT_MODULE=/tmp/taskmecca-glass-runtime/node_modules/playwright
export GLASS_EVIDENCE_DIR=/tmp/taskmecca-glass-evidence
node --test tests/web_glass_theme.test.cjs tests/web_glass_browser.test.cjs
```

macOS에서 설치된 Chrome을 사용하려면 다음을 추가합니다.

```sh
export CHROME_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
```

fixture를 직접 열려면 별도 foreground 터미널에서 실행합니다. 실제 SHA를 확인할 경우 검사하려는 자산 checkout의 정확한 SHA를 명시합니다. 기본 Preview SHA `0000000000000000000000000000000000000348`은 합성 식별자입니다. 실제 Preview artifact의 SHA를 검증했다는 뜻이 아닙니다.

```sh
GLASS_FIXTURE_SHA="$(git rev-parse HEAD)" node tests/helpers/glass_fixture.cjs
```

출력 JSON의 `origin`을 브라우저에서 엽니다. 기본 port는 동적으로 배정되며, 필요할 때 `GLASS_FIXTURE_PORT`로 별도 port를 지정합니다. 운영 port 18765는 거부합니다. 종료는 해당 foreground 프로세스의 Ctrl+C만 사용합니다. mock API 기반 시각 검증이며 운영 Web의 통합 동작 검증으로 확대 해석하지 않습니다.

## 자동 회귀 범위

CI의 `glass-browser` job은 Node 22와 Playwright 1.64.1, Chromium을 별도 임시 prefix에 설치하고 Node/브라우저 검사를 모두 실행합니다. 실패 시에도 작은 JSON과 대표 viewport PNG를 artifact로 남깁니다. 기존 Node 회귀 목록에도 `web_glass_theme.test.cjs`가 연결됩니다.

브라우저 검사는 다음을 확인합니다.

- 실제 custom palette 선택기를 클릭해 Glass → Mecca → Slate → Glass로 전환하고 배경 layer·선택기 노출·주요 색·읽기 surface·저장값을 확인합니다. 나머지 matrix는 실제 application state/render 및 appearance handler를 호출합니다.
- Dark/Light, System 변경, 세 배경의 저장·reload, 초기 bootstrap의 잘못된 값과 접근 불가 storage 복구를 검사합니다.
- 문서 스크롤, 높이 변경, 회전, 독립 메뉴 스크롤 전후 배경 canvas 크기·anchor·image/scale이 같은지 검사합니다.
- 1440×960 desktop과 390×844 touch viewport에서 합성 목록·상세/raw 표·실제 Activity cache의 timed Gantt·workload Root/attempt/상태 이력·Hub·저장소·Terminal을 렌더합니다. 원격 API/실제 PTY는 실행하지 않으며 Terminal fallback 출력만 가상 로그로 채웁니다.
- focus outline, reduced motion/transparency와 blur 미지원 fallback을 검사합니다. Chromium은 blur를 지원하므로 미지원 검사는 해당 CSS capability 분기만 강제로 활성화하고 filter 선언을 제거한 emulation입니다. 실제 미지원 브라우저 통과를 주장하지 않습니다.
- 큰 글꼴 검사는 실제 text node의 computed 크기를 1.5배로 키워 증가 여부와 viewport overflow를 확인하는 명시적 fixture입니다. OS 글꼴 설정/브라우저 zoom/Samsung 검증을 대체하지 않습니다.

고정 chrome이 있는 긴 문서의 `fullPage` 이미지에서 잘못된 크기의 sidebar가 촬영되어 해당 이미지는 근거에서 제외했습니다. 최종 증거는 viewport 캡처와 별도 scroll 위치입니다. JSON에는 화면 크기·scroll·sidebar/main/topbar 경계도 함께 남깁니다. 실제 desktop sidebar는 72px, DPR 1, 기본 글꼴 16px입니다.

성능 관찰은 다음으로 재현합니다. 기준 checkout을 보존한 자산 디렉터리는 `GLASS_ASSET_ROOT`로 지정하고, 기준/변경 후에 같은 fixture·screen·browser를 사용합니다.

```sh
GLASS_EVIDENCE_DIR=/tmp/taskmecca-glass-measure node tests/helpers/glass_measure.cjs
```

이 관찰은 1.5초 스크롤의 rAF 간격, CDP Task/Layout/RecalcStyle 시간, heap, 8회 테마 변경의 두 frame 대기 시간을 기록합니다. 단일 headless 표본이며 GPU·paint 하드웨어 비용·배터리·성능 향상을 입증하는 benchmark가 아닙니다. 캐시/GC/장비·측정 순서에 영향을 받으므로 수치로 성능 우위를 단정하지 않습니다.

## 실기기 확인과 남은 판단

Samsung Internet, 실제 Android 주소창/가상 키보드, OS 글꼴 확대, GPU·배터리는 실행하지 못했습니다. 실제 기기에서는 foreground fixture 또는 별도 격리 Preview를 열고 Cozy Dark/Light에서 목록·팝업을 스크롤한 뒤 주소창 접기, 검색 키보드 열기/닫기, 세로/가로 회전, reload를 수행합니다. 방의 창틀·그림·소파 같은 기준점이 재확대/재중심화되지 않는지, 조작 버튼과 큰 글꼴이 겹치지 않는지 확인합니다. GPU/배터리는 같은 데이터·밝기·스크롤 시간으로 기준/변경을 비교해야 합니다.

자동 검사는 기술적 회귀 근거입니다. 투과감·반사·깊이의 최종 디자인 수용은 사용자가 최신 Preview에서 판단합니다. Cozy SVG와 Glass 효과는 승인된 요구이므로 일반 디자인 규칙의 해당 금지와 충돌할 때 이 작업 계약을 따릅니다.

## 2026-11-04 격리 관찰

Chrome 156.0.8078.80, DPR 1, 같은 합성 workload에서 얻은 한 차례의 tracing 표본입니다. CPU 열은 1.5초 구간의 CDP TaskDuration이고 Paint는 DevTools main-thread Paint event 합계입니다. GPU raster/compositing 비용은 포함하지 않습니다.

| 조건 | 기준 → 변경 후 평균 frame 간격 | Task 시간 | Paint event / 합계 | JS heap | 테마 변경 평균 |
| --- | --- | --- | --- | --- | --- |
| Desktop | 16.36 → 16.48 ms | 21.94 → 26.32 ms | 9 / 0.417 → 2 / 0.287 ms | 3.88 → 4.37 MiB | 32.69 → 32.75 ms |
| Touch emulation | 16.58 → 16.46 ms | 21.54 → 27.38 ms | 10 / 0.465 → 6 / 0.983 ms | 4.02 → 4.42 MiB | 32.85 → 31.30 ms |

각 Worker card의 computed backdrop-filter는 `blur(10px) saturate(1.32)`에서 `none`으로 바뀌었습니다. 프레임 간격은 비슷했고 CPU·heap·모바일 Paint는 이 표본에서 증가했습니다. filter 제거만으로 실기기 성능 향상을 주장하지 않습니다.

viewport의 실제 합성 배경 pixel과 CSS foreground를 대조한 표본은 desktop Dark/Light 각 129개, touch Light 52개에서 4.5 미만이 없었습니다. touch Dark의 작은 ID 한 표본은 고정 footer에 일부 가린 위치(y811–822, footer y816)에서 3.66으로 집계됐습니다. 이를 삭제하거나 AA 전체 통과로 표현하지 않고, 실제 스크롤로 ID를 y415.83–426.83으로 옮겨 가리지 않음을 확인한 별도 검사에서 7.14를 측정했습니다. 텍스트를 숨길 때 `-webkit-text-fill-color`를 사용해 `currentColor` 기반 배경색을 보존했습니다. 이 검사는 제한된 pixel 표본이며 전 화면 WCAG 적합성 인증이 아닙니다.
