# A-65 Worker 구현 완료 보고

- Report ID: a65-transmissive-kkobugi-20261109-13501085
- Assignment: assignment-058bf69bc4df84b3edde713f323b2db3
- Semantic role: /root/controller_recovery/kkobugi
- 승인 effective SHA256: e77203d25d3fe199eb668c4ae3658c0259e36c9c79b4b8bd9dc407bb0ebc8f82
- 실제 CODEX_THREAD_ID: 01a1bdd0-6553-7741-966f-ad1bd4059ebc
- checkout: /Users/hanati/.t3/worktrees/TaskMecca/t3-bd5f88bd
- 유지 branch: t3/transmissive-glass-materials
- 구현 commit: 13501085060cde1d4d2c9c88717d21f60f76920d

## 기반과 변경 범위

시작 HEAD d315e2f1은 지정 origin과 분기되어 있었다. fetch 후 origin은 정확히 e7c2ee974984f44441c78cdda5b303a69a12a81a였다. FF 불가능을 확인하고 정상 merge를 수행했다. 구 primary/core와 최신 PR 간 충돌은 최신 origin 트리를 선택해 정리했다. merge commit f79fd7efc21db2a18fb48a9cfafbae6e2cfe4e4b의 tree 8983ad146eccb96b008f58bfa12b8f8691b36381은 e7의 tree와 완전히 동일하다. e7 ancestry 검증 성공. 전체 dev merge, branch 생성/checkout 전환 없음. Controller는 구 primary ancestry 유입을 피하려면 구현 commit 13501085를 e7에 cherry-pick할 수 있다.

구현 파일은 material/background-comparison.css, background-comparison.js, 새 transmissive.js, README.md 4개다. canonical backlog/runtime/lifecycle/계약, 외부 PR, 기존 e7/a41 evidence는 수정하지 않았다. commit/push/서비스설치/restart 중 push·설치·restart는 0, 기존 소유 57069 접근/교체 0, preview_* 0, subagent 0.

## 수용 기준과 실제 구현

- 주요 세 표면 내부: 로컬 실내 사진을 원래 fixed 배경과 동일한 cover/중앙 좌표로 재샘플링한다. 사진 색/명암/경계가 각 interior에서 이어진다. 실제 뒤 DOM 전체를 투과하는 backdrop-filter가 아닌, 현재 단일 사진 backdrop의 좌표 샘플 방식이다. 임의 다른 DOM에 대한 굴절을 주장하지 않는다.
- Blur: 장식 사진 레이어 하나에 7px Gaussian blur. SVG enhancement는 R/G spatial lens map의 가장자리 좌표를 displacement scale 22로 변위한다. turbulence, CPU optics, canvas, 행별 blur, continuous rAF 없음. SVG blur와 displacement는 한 filter chain이다.
- 곡률/rim/두께/기존 근접광 유지: material.css/proximity.js byte-identical. 반사광은 굴절이라 부르지 않는다. 색유리 면은 중립 white/black tint이며 보라/파란 면을 추가하지 않았다.
- 텍스트·아이콘은 filter가 걸린 장식 레이어의 자손이 아니다. 콘텐츠에는 shadow/stroke/blur/displacement를 추가하지 않았다. h2 computed filter none/text-shadow none/stroke 0px 확인.
- fixed 좌표: scroll/resize 및 ResizeObserver에서 현재 이미지/표면 사각형에 맞춰 동기화한다. 배경의 100lvh minimum을 제거해 이미지 실제 fixed 박스와 좌표 기준을 일치시켰다. 모바일 scroll 점프/가로 overflow 관측 없음.
- image/solid, Light/Dark/System, PC rail 3종, mobile open/Escape, keyboard Enter 선택, pointer 이동, reduced-motion 환경 확인. 기존 UX handler는 변경하지 않았다.
- fallback: Chromium SVG path 실제 실행/픽셀 차이 검증. 다른 엔진은 CSS blur+실제 사진 샘플+rim으로 제한하고 edge displacement는 제공하지 않는다. Chrome에서 fallback 강제 경로 screenshot도 저장했다. Safari/Firefox 실엔진 실행은 하지 않았다. CSS.supports syntax만으로 SVG 실지원을 주장하지 않는다.

## 검증과 evidence

실제 로컬 소스 + 기존 tests/helpers/a65_fixture.cjs 합성 API를 사용하는 owned ephemeral fixture만 실행했고 종료했다. 운영 API/57069가 아니다. browser.cjs 재현 스크립트, browser-report.json, browser.log, *-optics.png/*-blur-only.png/*-no-optics.png/*-fallback.png 및 desktop/mobile Light/Dark 초기 캡처를 이 경로에 보존했다.

- 명령: node handoff-evidence/a65-transmissive/browser.cjs
- PC 1440×1000 / 모바일 390×844 × Light/Dark 4개: 모두 통과. 1280×900 /430×900 resize도 포함. 동일 위치의 필터 레이어 숨김 대비 interior RGB 평균 변화 4.92–12.12, blur-only 대비 실제 SVG 출력 변화 0.76–1.05, 무필터 대비 blur+displacement 변화 5.77–8.17. 이는 screenshot 차이 근거이며 물리적으로 정확한 유리 시뮬레이션이라는 뜻은 아니다.
- scroll/resize 좌표 오차 최대 0.004375px. 모든 케이스 가로 overflow false, pageErrors 0.
- 최초 묶음에서 System media 변경 직후 assertion이 비동기 반영 전에 읽어 실패했다. UI 결함이 아닌 harness 대기를 수정하여 묶음을 완료했다. 사진 대비 tint를 1회 수정하고 확인 묶음 1회로 종료했다. 추가 시각 polishing 반복 없음.
- node --test tests/a65_preferences.test.cjs tests/a65_return_dev.test.cjs tests/a65_task_data.test.cjs : 49/49 통과 (unit.log).
- WORKFLOW_PLAYWRIGHT_MODULE=/tmp/a65-transmissive-harness/node_modules/playwright WORKFLOW_REQUIRE_BROWSER=1 CHROME_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' RETURN_DEV_EVIDENCE_DIR=<this>/return-dev node --test tests/a65_core_browser.test.cjs tests/a65_return_dev_browser.test.cjs : 2/2 통과, skipped 0 (regression.log, core/, return-dev/). 실제 service return 실행은 하지 않는 기존 synthetic consent test다.
- go test ./internal/webui -run '^TestUpdateIndicatorsDoNotAbbreviateVersions$' -count=1 : 통과 (go-safe.log). 위험한 unprotected go test ./... 실행 0.
- Impeccable context.mjs --target material-review 이번 세션 1회. PRODUCT/DESIGN/관련 source, overdrive, UI 직전 craft-floor 적용. 이미 사용자가 구현 방향을 승인했으므로 playbook의 방향 재질문은 생략했다. detect.mjs 변경 UI 대상 1회 결과 [] (impeccable.json). 과거 DESIGN/opaque 시안을 정답으로 삼지 않았다.

## 계측 범위와 미검증

CDP Performance 및 LayerTree의 작은 로컬 샘플이다. 유휴 0.701–0.703초 동안 TaskDuration 증가 0.155–0.589ms, JS heap 변화 0. 초기 heap 2.65–2.74MB → 입력·캡처 후 3.93–3.98MB. 입력+캡처 구간 task 67–87ms / wall 1.11–2.87초. 사진 캡처와 CDP 호출 비용도 포함하며 이를 순수 제품 CPU 수치라 부르지 않는다. 관측 layer PC 9/mobile 7, 입력 전후 layer 차이와 GPU/native/process 메모리는 별도 계측하지 않았다. 전체 48개 시나리오/8개 counterbalanced pair/CPU 10% gate는 면제하거나 충족했다고 주장하지 않는다. Controller의 통합 HEAD gate가 남는다. Samsung physical, Safari/Firefox 실기, 접근성 전체 수용, energy/battery, 실제 운영 Go 데이터 시각 검증은 미실행이다.

## 보호 hash와 상태

protected-before.json의 415개 core/tests/다른 theme 파일은 모두 동일(변경 0), protected-result.json에 aggregate SHA256 보존. 추가 protected-material.json에 material.css/components.js/proximity.js/review.js/photo/material-review.html/logo.svg의 개별 SHA256 및 e7 동일 여부를 저장했다. 기존 큰 곡률과 UX, 로고 및 사진 원본 보호 확인.

## identity / transport 한계

canonical에서 읽기 전용 runtime query --agent-id 01a1bdd0-6553-7741-966f-ad1bd4059ebc 결과 attempts [], count 0, status not_found (2026-11-09T01:04:46Z). 실제 hook/attempt를 관측하지 못했으며 과거 completed identity를 재사용하거나 원장을 만들어 채우지 않았다. Controller native CODEX_THREAD_ID는 전달받은 01a1bdce-8fa4-7a62-987e-129ff53a6d9f, 현재 T3 조회는 지정 Controller thread completed를 반환했다. native collaboration 도구에는 그 imported Controller가 등록되어 있지 않으므로 T3 direct send(auto)로 durable 보고서/commit을 전달한다. 실제 transport 결과는 transport.json에 보존하며 전달/재개 성공과 원장 finalization 성공을 구분한다. Controller가 canonical/외부/PR361 통합/normal push 및 정확 owner 57069 갱신을 담당한다.
