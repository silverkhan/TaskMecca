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
    globalThis.activityTest={state,activityMetricsBody,activityDetailPrefs,activityDetailCache,activityDetailKey,activityRoundBreakdown};
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
 assert.match(html,/implementation · worker/);
 assert.doesNotMatch(html,/requirements · worker · 회차/);
 assert.match(html,/11\/04/);
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
