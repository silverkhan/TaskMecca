# A-53 실사 배경·Glass 보충 구현 보고

- report_id: `pr348-photoreal-worker-done-20261104`
- assignment_id: `assignment-d6bb4cf02256069a6bcc9e18b3476403`
- effective_sha256: `1c5dd7a1a5ae3ca7442c9fb5fe1ca741e462883b09240fcf6b912b2f3ff79c94`
- original core hash: `b11108f3bf199f3d3254983ca6ee9dacd08c50c54812ae64aa4511328759886d`
- native Worker: `01a18157-0a5c-7ac2-9054-95f702ed30e3`, attempt `run-9dd568a46ff94c87`
- branch: `feature/glass-theme-dark-light`; 시작 HEAD `fb4fe7cfba456eef7eae8bfe261e14f028898bfd`
- 최종 commit: 5d9c85fca75274a6026e1beb42bd577a690e3143
- 상태: Worker 구현·로컬 검증 완료. push/최신 SHA의 CI·Preview·PR 반영은 Controller 후속. 디자인 사용자 수용은 대기.

## 과거 결과와 구분

사용자가 fb4의 alpha 파스텔 소재를 명시적으로 거절했다. fb4 CI 41826398514 및 Preview 41826417202는 과거 revision 증거이며 이번 보충의 최종 성공으로 재사용하지 않았다. 기존 handoff-evidence/pr348/은 덮어쓰지 않았다. 과거 fb4의 mobile ID3.66은 footer 부분가림 표본이었고 화면 중앙 국소 재측정7.14도 과거 revision 근거다. 이를 이번 실사 대비 결과로 재사용하거나 초기 실패를 삭제하지 않았다. Pinterest 참조 질문을 별도 WebGL/3D·렌즈 조절 UI 승인으로 확대하지 않고 확정된 실사·frost 보충만 구현했다.

## 구현과 수용 기준

| 기준 | 구현·근거 / 남은 확인 |
| --- | --- |
| S-1 생성·고화질 자산 | Root 생성 원본1672×941 → 공식 Real-ESRGAN-ncnn-vulkan v0.2.0 / realesrgan-x4plus 4배6688×3764 → Lanczos 축소·미세crop3840×2160의 실제 provenance를 확인했다. 네이티브4K 생성으로 부르지 않는다. Root 원본/모델/마스터는 canonical project-owned data에 보존, Worker는 웹2종과 작은 manifest만 추가. |
| S-2 실사 위 유리 소재 | 기존 개별 alpha 읽기섬을 제거하고 reader의 별도 sticky ::before에 backdrop frost(12px, mobile8px), 경계반사·내부광/하단음영·바깥 그림자를 적용. 바깥 사진은 선명하게 남고 pane 아래에서만 실제 디테일이 흐려짐을 같은사진 old/new 렌더 crop으로 확인. CSS 효과이며 물리 광학 굴절을 주장하지 않는다. 사용자 디자인 승인은 아님. |
| S-3 제공·상태·회귀·비용 | Go 실제 embedded MIME/cache/ETag/HEAD/304/405, 실제 CSS Network와 natural dimensions, 3배경/저장/초기색/고정anchor·scale/기존palette/접근성·fallback/기능 회귀를 검사. 같은사진 비용 관측을 upscale 종료 뒤 수집. 최종 원격CI·Preview·PR 갱신은 Controller 후속. |
| S-4 보존·한계 | Draft/base dev 유지, dev/main/Stable merge/force/reset/push 없음. 운영18765·launchctl·서비스변경/실제사용자 원장·외부이슈 Worker 수정 없음. Samsung/GPU/배터리 등 미실행을 명시. |

원래 AC-1의 격리 worktree/기존 commit·Controller Dev merge를 보존했다. AC-2~5의 소재·배경·읽기·기능·비용은 아래 로컬 근거를 추가했다. AC-6~7 중 push/새최종SHA CI·Preview·PR·원천/백로그 반영과 사용자 디자인 판단은 Controller/Root 후속이므로 전체 백로그 완료를 주장하지 않는다.

## 자산·MIME·cache

- `cozy-beige-room-photo-1920.webp`: 1920×1080 / 451,256B / SHA256 `48c7a6a5c4dbc1e6afe9b07ab0d026850e8516fbde21e621fa4bbabc9c072600`
- `cozy-beige-room-photo-3840.webp`: 3840×2160 / 1,158,428B / SHA256 `542346a844197fd8eb594cc414abd8ebcd9b726d9ef809ed14cd91c05bb8a4fe`
- byte budget500,000/1,250,000B와 manifest SHA 일치. 원래 SVG 파일·경로는 보존하고 cozy 기본 참조만 실사로 바꿈. 저장 identifier/Atmospheric/None은 그대로.
- 실제 mobile DPR3는1920, desktop DPR2는3840 요청. `confirmation/photo-delivery.json`에 encoded body451256/1158428, transfer451556/1158728, 자연치수, MIME/cache 기록.
- version query 일치 시 immutable1년, unversioned must-revalidate, SHA ETag. fs.Stat/ETag/HEAD/304 판단이 payload read 앞에 있어 재검증·HEAD에서 큰 []byte 복사를 생략.
- RGBA 약8.3MB/33.2MB는 픽셀×4 추정이다. HTTP decodedBodySize(WebP 파일 크기), JS heap, GPU 메모리와 구분한다. warm Image.decode 시간은 cold decode 성능으로 주장하지 않는다.

## 회귀 결과와 실패 이력

- Node 관련75개 PASS(Glass6 포함).
- 실제 Chromium2개 PASS: 전체 desktop/touch app matrix와 responsive photo delivery. 마지막 전체 기능 로그 `browser-functional-success.log` / `browser-tests-final.log`, 약21.26초. screenshot53개 geometry, 페이지 오류0.
- Go named3개 PASS: TestPhotographicRoomAssetDelivery(1920/3840 subcases), TestSignatureThemeBranding, TestHandlerServesDashboardAPIsAndAssets. 무보호 `go test ./...` 미실행.
- 실제 custom selector 클릭으로 Mecca/Slate/Glass 전환·layer·surface·색·저장값을 검사. 실제 state/render로 3배경 reload/System, 목록·상세/raw·timed Gantt·Root/attempt 이력·Hub·저장소·Dark/Light Terminal 가상 fallback 로그를 검사. 실제 PTY는 실행하지 않음.
- reduced motion/transparency 및 blur 미지원 분기 emulation. 큰글꼴은 computed 실제text1.5배 확대이며 OS글꼴/실제zoom으로 부르지 않음.
- TOC 실패는 assertion을 raw=true 상태에 잘못 둔 하니스 오류. app.js:6250은 raw에서 TOC를 의도적으로 생략한다. 앱 수정 없이 rendered detail로 검사를 옮겨 fixed button이 scroll 전후 같은 bbox임을 통과했다. 실패 전체 raw 로그는 같은 파일 재실행으로 덮어쓴 사실을 숨기지 않고 tool-output 관측 발췌를 `toc-raw-failure-excerpt.md`에 별도 보존했다.
- 합성 대비 최초 실패·보완값은 `contrast-first.log`, `contrast-final.log`, `contrast-light-local-fix.log`에 보존. 최신 Dark desktop129/mobile51개 최소4.88/5.07. Light는 작은 확인badge/header/실제 danger-action만 국소 보완 후 desktop129/mobile51개 최소4.51/5.57, 4.5미만0개. 제한된 pixel 표본이며 전체WCAG/AA 인증 아님.
- 마지막 Light 보완은 확인된 대비 요소만 재측정(`light-contrast-final/` desktop, `light-contrast/` mobile), 전체 미감 round를 재시작하지 않음. 최종 성능 캡처에도 그 source가 적용됨.
- 이번 revision detector 정확히1회. 기존 영역7건(side-tab4/layout-transition2/overused-font1), 새 Glass2257행 이후0건. 원래 revision의1회와 별도.

## 필터 크기·비용과 렌더 차이

reader는 content 자체filter가 none이며 pseudo 하나만 frost한다. card/row filter는 모두none이고 고정chrome/활성overlay의 별도 bounded filter가 있다. 부모 하나이므로 GPU가 작다고 단정하지 않는다.

CDP 실제 pseudo border box desktop1186×866, mobile384×674이며 실제 원장content 높이3081/3765px와 분리됐다. 실제 scroll 후 visible intersection도 각각같은1186×866/384×674이며 viewport-bound assertion 통과. 배경의 screen pixel canvas는 scroll/높이/회전/독립메뉴스크롤에서 anchor/scale을 보존한다. filter를 부모에 두지 않고 sticky/음수margin으로 flow높이를 상쇄하며 JS scroll reflow를 추가하지 않음.

같은사진 baseline은 fb4 스타일에 동일한 WebP/선택조건만 연결했다. `baseline-same-photo/` vs `final-same-photo/`의 실제 앱 viewport와 글자 없는 frost crop을 대조한다. Desktop crop은 각680×280, normalized horizontal gradient 비율이 Dark0.146/Light0.137로 디테일 저하를 보인다. 이는 보조 설명 지표이지 물리 광학·디자인 수용 benchmark가 아니다. mobile crop은72px/66px 높이가 달라 해당 정량비교를 수용근거로 사용하지 않는다. 실제 차이는 desktop crop 및 전체 mobile viewport에서 판단한다.

Chrome156.0.8078.80, 같은사진·DPR1·같은가상원장1.5초 scroll, GPU upscale 종료 후 baseline→final 순차 단일표본:

| 조건 | frame평균 | Task시간 | main-thread Paint횟수/합 | JS heap | 테마전환평균 |
| --- | --- | --- | --- | --- | --- |
| Desktop |16.46→16.35ms|23.64→26.05ms|3/0.717→12/2.598ms|4.58→4.61MiB|32.77→32.82ms|
| Touch |16.47→16.50ms|31.93→24.99ms|11/3.894→18/2.794ms|4.75→4.75MiB|32.84→32.94ms|

Desktop Paint/Task 증가를 그대로 남긴다. 최대rAF 간격 약16.8ms였지만 headless표본은 GPU raster/compositing·고DPR frame·배터리·실기기 성공이 아니다. 같은사진은 소재비용을 분리한 조건이며 이전SVG 대비 사진 추가 decode/전송비용이 없다고 주장하지 않는다.

## 근거 경로·재현

- worktree: `/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca`
- 문서: `docs/glass-theme-validation.md`, provenance: `docs/assets/cozy-beige-room-photo.json`
- fixture: `GLASS_FIXTURE_SHA="$(git rev-parse HEAD)" node tests/helpers/glass_fixture.cjs` (실제 build 검증 대신으로 사용하지 말 것).
- 기본합성SHA:0000000000000000000000000000000000000348. dynamic port 출력JSON origin 사용,18765거부. 최신 matrix origin=http://127.0.0.1:61241 (검증 후 서버 종료).
- 가상 project `/fixtures/atlas`, backlog `/fixtures/atlas/_task_mecca/data/backlog`. API모두합성·browser외부network차단.
- 최종광학/비용자료: `final-same-photo/observations.json`과 PNG, `baseline-same-photo/`, `frost-comparison.json`.
- 넓은 기능matrix: `confirmation/`; 국소최신Light대비: `light-contrast-final/` + `light-contrast/`. 최초 `matrix/`, `new-style/`, `old-style-same-photo/`는 최초검토/실패도 구분해 보존.
- 로그: `node-tests.log`, `go-tests.log`, `browser-functional-success.log`, `photo-delivery.json`(confirmation안), `design-detector.json`, `contrast-light-final.log`, `perf-baseline.log`, `perf-final.log`.
- 원본이미지/신경망출력/4KPNG/모델/도구는 canonical generated에 있고 PR에서 제외. runtime/node_modules/cache/binary/screenshot/evidence 및 기존Controller소유 pr348/controller-preview/main.go도 stage하지 않고 보존.

## 미실행과 Controller 후속

Samsung Internet 실기기, 실제 Android 주소창·키보드/OS글꼴, GPU·배터리, 고DPR frame비용·cold decode는 미실행. 재현절차는 docs에 기록. source/test/docs/WebP2종 commit 이후 remote push, 새정확한SHA CI·Preview/PR본문·원천/백로그 갱신은 Controller 직접 수행. Draft 유지, dev/main/Stable 미병합. 최종 사용자 디자인 수용 대기.
