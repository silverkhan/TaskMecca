// Synthetic payloads and shipped renderer only. No service or operating journal.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const web=path.join(__dirname,'../goassets/template/_task_mecca/framework/web');
const source=fs.readFileSync(process.env.A89_SOURCE||path.join(web,'app.js'),'utf8');
const fixture=JSON.parse(fs.readFileSync(path.join(__dirname,'fixtures/workflow/fact-gap-details-a89.json'),'utf8'));
function load(language='ko'){
 const context=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden');},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a89={state,activityMetricsBody,activityDetailPrefs,workflowPlotHTML,workflowMarkers:typeof workflowMarkers==="function"?workflowMarkers:null,workflowMarkerAttrs:typeof workflowMarkerAttrs==="function"?workflowMarkerAttrs:null,workflowMarkerNote:typeof workflowMarkerNote==="function"?workflowMarkerNote:null,workflowMarkerTime:typeof workflowMarkerTime==="function"?workflowMarkerTime:null};',context);
 context.a89.state.language=language;return context.a89;
}
const payload=()=>structuredClone(fixture.cases.mixed);
test('facts and global/row gaps use common accessible selectable anchors, never title-only',()=>{
 for(const lang of ['ko','en']){const a=load(lang),p=payload(),before=JSON.stringify(p),html=a.activityMetricsBody(p,'A89-FIXTURE');
 assert.match(html,/<button[^>]*class="workflow-fact-point"[^>]*data-workflow-marker=/);
 assert.match(html,/<button[^>]*class="workflow-gap-paint[^>]*data-workflow-marker=/);
 assert.doesNotMatch(html,/class="workflow-(?:fact-point|gap-paint)[^>]*role="img"/);
 assert.match(html,/data-workflow-marker-options="global"/);assert.match(html,/data-workflow-marker-options="controller"/);
 assert.match(html,lang==='ko'?/표식 선택/:/Select marker/);assert.match(html,/future&lt;fact&gt;/);assert.doesNotMatch(html,/<script>fixture/);
 assert.equal(JSON.stringify(p),before,'rendering leaves original events, periods, and durations unchanged');
 }
});
test('incomplete times and missing evidence remain explicit, exact identity/fingerprint invalidate changed sources',()=>{
 const a=load(),p=payload(),row=p.workflow.rows.find(r=>r.role==='controller'),markers=a.workflowMarkers(p.workflow,row);
 assert.equal(markers.length,4);assert.equal(new Set(markers.map(m=>m.key)).size,4);
 assert.equal(a.workflowMarkerTime(null,true),'시각 미확인');assert.equal(a.workflowMarkerTime('0001-01-01T00:00:00Z',false),'Time unverified');assert.equal(a.workflowMarkerTime('bad',false),'Time unverified');
 const original=a.workflowMarkerAttrs(markers[0],true);row.role_facts[0].reason='Changed source';
 assert.notEqual(a.workflowMarkerAttrs(a.workflowMarkers(p.workflow,row)[0],true),original);
 row.role_facts.shift();assert.ok(!a.workflowMarkers(p.workflow,row).some(m=>m.key===markers[0].key));
});
test('KOEN semantics distinguish facts, bounds, wait, collection failure, unmeasured and open Activity',()=>{
 const a=load();for(const ko of [true,false]){
 const fact=a.workflowMarkerNote({kind:'fact',value:{}},ko);assert.match(fact,ko?/작업 시간이 아닙니다/:/not time spent working/);
 const unknown=a.workflowMarkerNote({kind:'gap',value:{status:'unmeasured'}},ko);assert.match(unknown,ko?/무활동이나 정체/:/inactivity or a stall/);
 const native=a.workflowMarkerNote({kind:'gap',value:{status:'native_observed'}},ko);assert.match(native,ko?/단계별 활동 시간/:/not measured phase/);
 const wait=a.workflowMarkerNote({kind:'gap',value:{status:'confirmed_wait'}},ko);assert.match(wait,ko?/유휴시간/:/idle time/);
 }
 assert.match(source,/The end is unverified; this does not prove it is running now/);
});
test('marker wiring participates in floating, input, refresh and focus reconciliation',()=>{
 assert.match(source,/const workflowAnchorSelector=.*data-workflow-marker/);
 assert.match(source,/data-workflow-marker-fingerprint|workflowFingerprint/);
 assert.match(source,/focusedMarker/);assert.match(source,/workflowMarkerOptions/);
 const css=fs.readFileSync(path.join(web,'style.css'),'utf8');assert.match(css,/workflow-marker-option[^}]*min-height:44px/);assert.match(css,/workflow-fact-point[^\n]*focus-visible/);
});
