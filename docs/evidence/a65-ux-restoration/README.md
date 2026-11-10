# 현재 결과

최신8728 source의[구현·잔여·삼성대체검증](CURRENT.md)을먼저읽으세요. 전체timing44/48FAIL/idle26/27FAIL,삼성실기미검증·디자인미수용이며아래초기복원과기각실험설명은역사입니다.

<details>
<summary>초기 복원·이전 실험 이력</summary>

# d5ac0673 실사용 회귀에 따른 기존 GUI UX 복원

사용자 실행 SHA가 **d5ac06733e7271f74bdde0e96ad10965004194bc**로 확정됐다. 최신 원문:

> 여전히 스크롤 시에 랙이 심하다 / 기존 UX대비 개선된 체감이 없다 / UI도 기존이 훨씬 깔끔하다. / 좌측 메뉴 아이콘도 기존이 직관적이다. 지금은 중복이 많다. / 불투명도 또한 들죽날쭉이며, 백로그 본문이 분산되어 있어 오히려 탐색이 힘들다. 기존에는 한 번에 다 보여주면서도 앵커기반 네비게이터를 항상 제공해서 편리함도 확보됨. 또한 플로팅 인덱스도 지원.

Root 합의 https://github.com/silverkhan/TaskMecca/pull/361#issuecomment-6753067542 . d5ac의48/48 timing·8/8 CI 성공은 역사로 보존하며, 그 성공과 별개로 사용자의 실사용 랙 지속·디자인 미수용을 현재 상태로 기록한다.

## 복원 범위와 소스

기존 GUI THEME 실제 origin/dev f0a732e3의 `app.js`/`index.html`/`style.css`를 권위로 대조했다. 새 IA가 아니라 기존 UX 복원이다. 메뉴그룹·위치·실제 기능·정보를 유지한다.

- 서로 같은 폴더/활동 아이콘을 반복하던 항목을 저장소의 선반, Preview의 분기, 워크로드의 구획, 터미널의 `>_`, 도움말의 물음표, 업데이트의 위 화살표, 진단의 경고, 최근활동의 시계 등 기존 의미에 대응하는 고유 SVG로 복원. 라벨과 키보드/링크 동작은 유지.
- 배타적인4탭을 없애고 개요·결과·작업흐름·연결작업·운영근거·요건·메모·검증을 하나의 문서 DOM으로 렌더한다. 기존 `human-summary/background/requirements/scope/acceptance/lifecycle/progress-result/verification/operations` 앵커를 유지. 예전 `?tab=` deep URL도 해당 문서 위치로 연결한다. 본문별 내부 스크롤 제한을 제거한다. 원문은 기존처럼 별도 disclosure로 보존.
- 기존 GUI와 같은 우측 상시 sticky 목차(>1280px), 우측 상단 floating index(<=1280px), 모바일 우측하단 버튼(bottom74/right12)·목차(bottom126/right12)를 복원. 클릭/키보드/Escape/포커스복귀/refresh hash를 지원. 라이프사이클 갱신은 본문 전체를 갈아끼우지 않고 기존 분리 갱신을 유지한다.
- 개별 제목·요약·문서마다 겹친 광학면을 제거하고 workspace의 한 viewport 크기 reading plane으로 공유한다. 문서 길이가 필터 텍스처 길이가 되지 않는다. sidebar/읽기면은 동일 native tint, control/overlay는 그 역할의 shared token을 사용한다. 주요 곡률·rim·7px blur·edge22/사진·로고·Light/Dark/System는 유지. 텍스트/아이콘 필터0, 행별 blur0, CPU optics/canvas/상시rAF0.

첫 source **c7a697624618b589505d25ca4dd1d9184f620689**는 sticky reading plane이었다. 최종 scroll trace에서 모바일 TaskDuration 증가가 드러나 실패를 보존하고, 위치 처리만 fixed viewport로 바꾼 **42030bcd85220df81b0b9648fb36fb01eff96deb**로 교정했다. c7을 성능 통과 후보로 승격하지 않는다.

## 브라우저 기능·시각 근거의 경계

- `final-regression.tap`: core/expansion/P0returnDev/workflow browser **5/5 PASS** (본문 순서 최종 교정 전). 마지막 단순 위치 CSS는 이 배치 후의 변경이다.
- `confirm/results.json`: 1440/1280/320/390/430×Light/Dark **10조건**, 한 DOM의 모든 본문·목차·hash refresh·모바일 Escape·고유아이콘·문서 overflow0·viewport 효과 높이 제한 확인. 프로세스는 이후 GUI 비교 selector 오류로 종료1이었고 후보10조건은 모두 완료됐다. 실패 로그 그대로 보존.
- `reference/results.json`: 기존 GUI를 실제 row target으로 고쳐 같은 데이터의 PC/mobile Light/Dark **4/4** 비교를 완료했다. 이 수정은 측정 fixture 선택자를 고친 것이고 제품 assertion을 완화한 것이 아니다.
- `mechanics/results.json`: 최종 본문 순서를 기존 GUI 순서로 맞춘 뒤 **10/10** 비시각 기능 검사, pageerror0/root overflow0. background/requirements와메모·검증의 끝단 내용까지 한 페이지에 존재하고 workflow가 요건 앞에 위치한다. fixed CSS 교정 전이며 이후 위치처리와 내용은 구분한다.
- `units.tap`: **31PASS/6기존조건부SKIP/0FAIL**. 6skip을 통과로 세지 않는다.
- `first-regression.tap`의 readonly panel 자체광학층 assertion 실패는 현재 공유면이 실제 blur를 갖고 해당 내용 아래에 정렬·존재하는지를 검사하는 assertion으로 대체했다. '표면에 빛만 있고 투과가 없다'는 기존 기능 요구를 제거하지 않았다.

시각 검수는 후보PC초기batch→간격/정렬fix→PC/mobile Light/Dark confirm1회로 제한했다. `confirm/*.png`는 **c7의 viewport공유면/본문순서 최종교정 전**, `reference/*.png`는 동일 데이터의 기존 GUI다. 이후 실제 trace가 발견한 비용 회귀로 fixed 위치 CSS와 기존 본문 순서를 고쳤으나 추가 전체 시각라운드는 열지 않았다. 따라서 이를 최종4203 픽셀 완전검수나 사용자 수용 증거라고 부르지 않는다. 차후 실제 후보 사용자 확인이 남는다.

## 스크롤 진단과 측정 한계

`scroll-trace.cjs`는 같은20행/viewport/theme에서 CDP compositor의600px scroll gesture를 사용한다. 각2회 순서를 바꿔 실행하고 rAF 간격, trace `DroppedFrame` 이벤트수, 개별 Paint/Raster/Draw/compositor 이벤트의 count·X-duration 원시를 보존한다. 60Hz rAF에서 추정한 missed frame과 trace-event count는 실제 물리 디스플레이의 dropped presentation 수와 다르다. 이벤트 duration을 GPU메모리나 GPU물리시간으로 주장하지 않는다.

- `diagnose`: GUI / d5ac / SVG굴절 제외 blur-only 진단. d5ac의Paint는대략44~60회 vs GUI2회, SVG제외도24~26회로잔여. 단순히효과를전부삭제하는해법을채택하지않음.
- `plane-probe`: runtime에만fixed 공유면을주입한원인분리. 원본소스변경0.
- `scroll-confirm`: c7 sticky후보의모바일Task 증가실패. 원시삭제/노이즈면제0.
- `fixed-correction`: 최종4203 / d5ac / GUI의같은조건대조. 600px movement/오류여부포함. nativeChromium GPU는 AppleM5Pro/ANGLEMetal, GPUcompositing/raster enabled. **headless/약60Hz** 환경으로 실제120Hz·사용자브라우저성능을대신하지않는다. 여기서는심한스크롤랙이동일하게재현되지않았다. 사용자환경브라우저·GPU가속/느린화면·대략문서크기/주사율·확대율을Root에최소재현정보로보고했다. 답을기다리며독립수정을중단하지않았다.

원시trace는 gzipJSON이고source d5ac assets snapshot은canonical에보존한다. 로컬재실행시 `d5ac-web/`에d5ac의실제web자산을별도asset directory로추출한다(새repository checkout/worktree를만들지않는다). 모든fixture는소유ephemeral loopback이며종료증거를보존. 운영18765/57069/서비스install·restart·전환/merge/deploy0. 새Worker/리뷰delegate0.

원래48group/8pair/10% timing gate와immutable idle는 별도 frozen source에서 실행하며 작은 trace로 대체하지 않는다. 정책hold/8AC미체크/최종디자인미수용은 유지한다. Samsung물리/Safari/Firefox/네이티브OS200%/screenreader/GPUmemory·energy·실사용환경 재현은 미실행이다.


</details>
