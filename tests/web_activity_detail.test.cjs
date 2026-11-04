const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const startup=source.indexOf('\ntranslateChrome();');
assert.ok(startup>0);
function page(){
  const context=vm.createContext({
    URLSearchParams,AbortController,fetch:async()=>({ok:true,json:async()=>({})}),
    location:{search:'?project=/demo'},navigator:{language:'ko'},history:{pushState(){}},
    localStorage:{getItem:()=>null,setItem(){},removeItem(){}},
    document:{visibilityState:'visible',querySelector:()=>null,querySelectorAll:()=>[],getElementById:()=>null},
    queueMicrotask,setTimeout:()=>1,clearTimeout
  });
  vm.runInContext(source.slice(0,startup)+`
    state.project='/demo';state.detail='B-589';state.language='ko';
    globalThis.activityTest={state,activityMetricsBody,activityDetailPrefs,activityDetailCache,activityDetailKey,activityRoundBreakdown,activityRollingPeriod,activityFilterControls};
  `,context);
  return context.activityTest;
}
const origin='2026-11-04T00:00:00.000Z';
const shift=min=>new Date(Date.parse(origin)+min*60000).toISOString();
function payload(){
 return {
  summary:{
   agent_time_ms:300000,observed_wall_ms:300000,
   initial_worker_ms:120000,rework_worker_ms:180000,
   controller_first_review_ms:0,controller_rereview_ms:0,
   by_type_ms:{requirements:120000,implementation:180000},
   segments:[
    {type:'requirements',role:'worker',round:0,started_at:shift(0),ended_at:shift(2),duration_ms:120000},
    {type:'implementation',role:'worker',round:1,started_at:shift(5),ended_at:shift(8),duration_ms:180000}
   ]
  },
  lifecycle_coverage:{registered_at:shift(0),total_elapsed_ms:600000,registration_queue_ms:0,dispatch_wait_ms:0,explicit_wait_ms:0,unattributed_elapsed_ms:300000,wait_intervals:[]},
  events:[
   {event_id:'start-1',action:'start',type:'requirements',role:'worker',agent_id:'worker',round:0,occurred_at:shift(0),evidence:'Started review',source:'agent_report',confidence:'inferred'},
   {event_id:'start-2',action:'switch',type:'implementation',role:'worker',agent_id:'worker',round:1,occurred_at:shift(5),evidence:'Implementation',source:'agent_report',confidence:'inferred'}
  ],
  test_report:{runs:[]}
 };
}
test('Activity detail shows measured durations and truthful elapsed basis',()=>{
 const a=page();const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/작업시간|누적 Agent 시간/);
 assert.match(html,/Lifecycle 기반/);
 assert.match(html,/실제 시간축 Gantt/);
 assert.match(html,/Activity 원장 상세/);
 assert.match(html,/start-1/);
});
test('round filter retains original timestamps instead of rebasing',()=>{
 const a=page();a.activityDetailPrefs('B-589').round='1';
 const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/구현 · worker/);
 assert.doesNotMatch(html,/요구사항 분석 · worker · 회차/);
 assert.match(html,/11[.\/ ]+04/);
 assert.match(html,/활동 유형별 누적 \(전체 회차\)/);
});
test('stacked mode and zoom are controlled independently',()=>{
 const a=page();const p=a.activityDetailPrefs('B-589');p.mode='rounds';p.zoom=4;
 const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/activity-stacked-rounds/);
 assert.match(html,/activity-stacked-track/);
 assert.match(html,/value="4"/);
 assert.match(html,/disabled/);
 assert.doesNotMatch(html,/activity-timeline-canvas/);
});
test('evidence and provenance are HTML-escaped in drilldown',()=>{
 const a=page();const data=payload();
 data.events[0].evidence='<img src=x onerror=alert(1)>';
 const html=a.activityMetricsBody(data,'B-589');
 assert.doesNotMatch(html,/<img src=x/);
 assert.match(html,/&lt;img/);
});
test('no evidence does not invent duration or trigger additional reads',()=>{
 const a=page();
 const html=a.activityMetricsBody({summary:{segments:[]},events:[]},'B-589');
 assert.match(html,/관측된 Activity·Lifecycle 시간 근거가 없습니다/);
 assert.doesNotMatch(html,/activity-stacked-rounds/);
});

test('localized taxonomy and wait labels retain original event provenance',()=>{
 const a=page();const data=payload();
 data.summary.by_type_ms={controller_review:31000,other:26000,targeted_test:1000};
 data.summary.segments=[{type:'controller_review',role:'controller',round:0,started_at:shift(0),ended_at:shift(1),duration_ms:31000}];
 data.lifecycle_coverage.wait_intervals=[{kind:'registration_queue',started_at:shift(1),ended_at:shift(5),duration_ms:240000}];
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/Controller 검토/);
 assert.match(html,/기타 활동/);
 assert.match(html,/선별 테스트/);
 assert.match(html,/작업 배정 대기/);
 assert.match(html,/31초/);
 assert.match(html,/title="controller_review"/);
});
test('timeline title contains measured duration and original endpoints',()=>{
 const a=page();const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/2분/);
 assert.match(html,/aria-label="[^"]*2분/);
 assert.match(html,/작업시간|요구사항 분석/);
});

test('missing worker evidence is distinct from confirmed zero',()=>{
 const a=page();const data=payload();
 data.summary.segments=[{type:'controller_review',role:'controller',round:0,started_at:shift(0),ended_at:shift(1),duration_ms:60000}];
 data.summary.initial_worker_ms=0;data.summary.rework_worker_ms=0;
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/기록 없음/);
 assert.match(html,/최초 Worker 작업/);
});
test('lifecycle window and activity coverage explain their distinct timing bases',()=>{
 const a=page();const data=payload();
 data.summary.unattributed_gap_ms=120000;
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/등록~완료 범위를 기준/);
 assert.match(html,/기록 공백/);
 assert.match(html,/합산하지 않습니다/);
});
test('unclassified suggested types are grouped with escaped evidence',()=>{
 const a=page();const data=payload();data.events=[
 {type:'other',action:'start',proposed_type:'registration_authorization',agent_id:'agent-1',confidence:'inferred',explanation:'<svg/onload=alert(1)>',evidence:'first'},
 {type:'other',action:'stop',proposed_type:'registration_authorization',agent_id:'agent-1',confidence:'inferred',explanation:'second',evidence:'second'}
 ];
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/작업 실행 승인/);
 assert.match(html,/2건/);
 assert.match(html,/Agent 추정/);
 assert.doesNotMatch(html,/<svg\/onload/);
});

test('Gantt interval labels are measured by rendered pixels rather than percentage',()=>{
 const a=page();const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/activity-timeline-duration-label/);
 assert.match(html,/tabindex="0"/);
 assert.match(html,/2분/);
});

test('Gantt offers touch and keyboard detail output without relying on native title',()=>{
 const a=page();const html=a.activityMetricsBody(payload(),'B-589');
 assert.match(html,/id="activityTimelineSelection"/);
 assert.match(html,/role="status"/);
 assert.match(html,/aria-label="[^"]*2분/);
});

test('observation quality distinguishes ledger coverage from native runtime inventory',()=>{
 const a=page();const data=payload();
 data.summary.segments=[{type:'controller_review',role:'controller',round:0,started_at:shift(0),ended_at:shift(1),duration_ms:30000}];
 data.events=[{type:'controller_review',role:'controller',agent_id:'controller-1',confidence:'inferred',action:'start'}];
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/활동 기록의 관측 품질/);
 assert.match(html,/원장 이벤트 1건/);
 assert.match(html,/Worker의 종료까지 확인된 작업시간 구간이 없습니다/);
 assert.match(html,/Runtime 실행 목록/);
});

test('runtime reconciliation is opt-in and clearly distinguishes unmeasured work',()=>{
 const a=page();const data=payload();
 let html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/id="activityRuntimeDiagnose"/);
 assert.doesNotMatch(html,/Runtime·Activity 실행 근거 대조/);
 data.runtime_diagnostic={attempts:[{attempt_id:'attempt-1',agent_id:'worker-1',agent_type:'worker',diagnostic:'no_matching_activity',matched_activity_events:0}],matched_attempts:0,without_activity:1,identity_unknown:0,identity_mismatch:0};
 html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/Runtime·Activity 실행 근거 대조/);
 assert.match(html,/Activity 기록 없음/);
 assert.match(html,/작업이 없었다는 증거가 아닙니다/);
});

test('Gantt can group by exact agent attempt without changing absolute-time positions',()=>{
 const a=page();const data=payload();
 data.summary.segments=[
  {type:'implementation',role:'worker',agent_id:'worker-a',attempt_id:'attempt-1',round:0,started_at:shift(0),ended_at:shift(2),duration_ms:120000},
  {type:'targeted_test',role:'worker',agent_id:'worker-a',attempt_id:'attempt-1',round:0,started_at:shift(3),ended_at:shift(5),duration_ms:120000},
  {type:'implementation',role:'worker',agent_id:'worker-a',attempt_id:'attempt-2',round:0,started_at:shift(5),ended_at:shift(8),duration_ms:180000}
 ];
 const before=a.activityMetricsBody(data,'B-589');
 assert.match(before,/activityDetailGroupAgents/);
 assert.doesNotMatch(before,/class="activity-agent-group"/);
 a.activityDetailPrefs('B-589').groupAgents=true;
 const html=a.activityMetricsBody(data,'B-589');
 assert.equal((html.match(/class="activity-agent-group"/g)||[]).length,2);
 assert.match(html,/실행 attempt-1/);
 assert.match(html,/실행 attempt-2/);
 assert.match(html,/left:37.5%/);
 assert.match(html,/left:62.5%/);
});
test('Gantt groups waiting separately and never assumes an unbound agent',()=>{
 const a=page();const data=payload();
 data.summary.segments=[{type:'implementation',role:'worker',round:0,started_at:shift(0),ended_at:shift(2),duration_ms:120000}];
 data.lifecycle_coverage.wait_intervals=[{kind:'dispatch_wait',started_at:shift(2),ended_at:shift(4),duration_ms:120000}];
 a.activityDetailPrefs('B-589').groupAgents=true;
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/Agent 미확인/);
 assert.match(html,/대기 구간/);
 assert.match(html,/작업 착수 대기/);
});

test('Activity refresh timestamp is explicitly API-read time, not source freshness',()=>{
 const a=page();const data=payload();data._client_fetched_at='2026-11-04T07:00:00Z';
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/마지막 조회:/);
 assert.match(html,/원천 데이터의 최종 변경 시각을 의미하지 않습니다/);
});
test('exact attempt groups translate role but preserve actor identity',()=>{
 const a=page();const data=payload();
 data.summary.segments=[{type:'implementation',role:'registrar',agent_id:'registrar-1',attempt_id:'attempt-1',round:0,started_at:shift(0),ended_at:shift(2),duration_ms:120000}];
 a.activityDetailPrefs('B-589').groupAgents=true;
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/등록 담당 · registrar-1/);
 assert.match(html,/실행 attempt-1/);
});

test('on-demand Runtime diagnosis exposes configured hooks separately from actual observations',()=>{
 const a=page();const data=payload();
 data.runtime_diagnostic={
   attempts:[{attempt_id:'attempt-1',agent_id:'worker-1',agent_type:'worker',diagnostic:'no_matching_activity',matched_activity_events:0,recent_hook_transitions:2,latest_recent_hook_at:'2026-11-04T07:08:00Z'}],
   matched_attempts:0,without_activity:1,identity_unknown:0,identity_mismatch:0,
   hook_statuses:[{provider:'codex',status:'configured'},{provider:'claude',status:'not_configured'}]
 };
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/codex · 설정 확인/);
 assert.match(html,/claude · 설정 미확인/);
 assert.match(html,/최근 Hook 근거/);
 assert.match(html,/최근 2건/);
 assert.match(html,/실제 수신 성공을 보증하지 않습니다/);
});

test('all Activity timing views present explicit units in Korean',()=>{
 const a=page(),data=payload();
 data.summary.by_type_ms={requirements:120000,implementation:180000};
 data.lifecycle_coverage.wait_activity_overlap_ms=26000;
 data.test_report={runs:[{}],full_regression_ms:65000,full_regression_count:1,repetition_candidates:0};
 const html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/요구사항 분석/);
 assert.match(html,/2분/);
 assert.match(html,/활동·대기 근거 겹침: 26초/);
 assert.match(html,/1분 5초/);
 assert.doesNotMatch(html,/00:26/);
});

test('Activity rolling 7/30/90 day shortcuts use exact duration boundaries',()=>{
 const a=page();const now=new Date('2026-11-04T07:00:00.000Z');
 assert.equal(a.activityRollingPeriod('7',now).from,'2026-10-28T07:00:00.000Z');
 assert.equal(a.activityRollingPeriod('30',now).from,'2026-10-05T07:00:00.000Z');
 assert.equal(a.activityRollingPeriod('90',now).until,now.toISOString());
 assert.equal(a.activityRollingPeriod('custom',now),null);
 const html=a.activityFilterControls();
 assert.match(html,/activityFilterPeriod/);
 assert.match(html,/최근 7일/);
 assert.match(html,/최근 30일/);
 assert.match(html,/최근 90일/);
 assert.match(html,/선별 테스트/);
});

test('recent half and quarter Gantt shortcuts retain full measured evidence',()=>{
 const a=page(),data=payload();
 const base=a.activityMetricsBody(data,'B-589');
 assert.match(base,/id="activityDetailFocus"/);
 assert.match(base,/직접 탐색/);
 const pref=a.activityDetailPrefs('B-589');
 pref.focus='last50';pref.zoom=2;
 let html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/value="last50" selected/);
 assert.match(html,/min-width:200%/);
 assert.match(html,/누적 Agent 시간/);
 assert.match(html,/5분/);
 // A different viewport cannot rewrite original event start/end or elapsed.
 assert.match(html,/2026/);
 pref.focus='last25';pref.zoom=4;
 html=a.activityMetricsBody(data,'B-589');
 assert.match(html,/value="last25" selected/);
 assert.match(html,/min-width:400%/);
 assert.match(html,/5분/);
 assert.match(html,/작업 회차/);
});
