// Offline synthetic evidence through the actual shipped renderer.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/app.js'),'utf8');
function load(language='ko'){
 const context=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden')},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a97={state,activityMetricsBody,workflowActorLabel,activityDetailPrefs,workflowFloatingContent,workflowRowSummary,workflowCurrentTypeLegend};',context);
 context.a97.state.language=language;return context.a97;
}
function row(native){return {id:native,role:'controller',agent_id:native,display_name:'Controller',display_name_source:'bound_runtime_agent_path',round:0,intervals:[{id:'activity',kind:'activity',type:'coordination',start:'2026-11-05T00:10:00.000Z',end:'2026-11-05T00:20:00.000Z',measured:true,duration_ms:600000,attempt_id:'attempt-1',assignment_id:'assignment'}],session_identity:{status:'verified',provider:'codex',native_id:native,aliases:['/root/controller',native],attempts:[{attempt_id:'attempt-1',thread_id:'exact-t3',assignment_id:'assignment'}]}};}
test('same role/name sessions have distinct visible and accessible labels in KO/EN',()=>{
 for(const lang of ['ko','en']){const a=load(lang),one=row('01234567-same-prefix-native-first'),two=row('01234567-same-prefix-native-second');
 assert.notEqual(a.workflowActorLabel(one,lang==='ko'),a.workflowActorLabel(two,lang==='ko'));
 const p=structuredClone(require('./fixtures/workflow/fact-gap-details-a89.json').cases.mixed);p.workflow.rows=[one,two];
 const before=JSON.stringify(p),html=a.activityMetricsBody(p,'A97-FIXTURE');
 assert.match(html,/data-workflow-row-select="01234567-same-prefix-native-first"/);assert.match(html,/aria-label="[^"]*01234567…-first/);assert.match(html,/01234567…second/);assert.equal(JSON.stringify(p),before);
 }
});
test('unverified alias never displays a verified native label and identity is escaped',()=>{
 const a=load(),r=row('native<script>');r.session_identity.status='unverified';
 assert.match(a.workflowActorLabel(r,true),/세션 연결 미확인/);assert.doesNotMatch(a.workflowActorLabel(r,true),/<script>/);
 r.session_identity.status='verified';const p=structuredClone(require('./fixtures/workflow/fact-gap-details-a89.json').cases.mixed);p.workflow.rows=[r];const html=a.activityMetricsBody(p,'A97-XSS');assert.doesNotMatch(html,/<script>/);assert.match(html,/native&lt;script&gt;/);
});

test('verified session remains one row across rounds while filters/details preserve each source round',()=>{
 for(const language of ['ko','en']){
  const a=load(language),ko=language==='ko',r=row('native-cross-round'),base=r.intervals[0];r.round=null;r.round_status='multiple';r.rounds=[{round:0,status:'initial'},{round:1,status:'rework'}];
  r.intervals=[{...base,id:'before',round:0,round_status:'initial',explanation:'Before source phase',explanation_origin:'written'},{...base,id:'after',round:1,round_status:'rework',start:'2026-11-05T00:30:00.000Z',end:'2026-11-05T00:40:00.000Z',type:'controller_review',explanation:'After source phase',explanation_origin:'written'}];
  r.role_facts=[{id:'fact-before',kind:'started',at:'2026-11-05T00:10:00Z',round:0,source:'fixture',reason:'before fact'},{id:'fact-after',kind:'resumed',at:'2026-11-05T00:30:00Z',round:1,source:'fixture',reason:'after fact'}];r.gaps=[];r.session_identity.attempts[0].rounds=r.rounds;
  const p=structuredClone(require('./fixtures/workflow/fact-gap-details-a89.json').cases.mixed);p.workflow.rows=[r];const before=JSON.stringify(p),pref=a.activityDetailPrefs('A97-ROUNDS');
  const all=a.activityMetricsBody(p,'A97-ROUNDS');assert.equal((all.match(/data-workflow-row=/g)||[]).length,1);assert.equal((all.match(/data-workflow-interval=/g)||[]).length,2);assert.match(all,/<option value="1"/);assert.doesNotMatch(all,/<option value="unknown"/);
  const summary=a.workflowRowSummary(r,ko);assert.match(summary,/Before source phase/);assert.match(summary,/After source phase/);assert.match(summary,ko?/최초 작업/:/Initial work/);assert.match(summary,ko?/보완1차/:/Correction 1/);
  assert.match(a.workflowFloatingContent(r,r.intervals[1],null,ko),ko?/보완1차/:/Correction 1/);
  const factDetail=a.workflowFloatingContent(r,null,{kind:'fact',value:r.role_facts[1]},ko);assert.match(factDetail,ko?/보완1차/:/Correction 1/);assert.doesNotMatch(factDetail,ko?/최초 작업/:/Initial work/);
  pref.round='1';const filtered=a.activityMetricsBody(p,'A97-ROUNDS');const buttons=filtered.match(/<button[^>]*data-workflow-interval="[^>]*>/g)||[];assert.equal(buttons.length,1);assert.match(buttons[0],/after/);assert.doesNotMatch(buttons[0],/before/);assert.doesNotMatch(filtered,/data-workflow-fact="fact-before"/);assert.match(filtered,/data-workflow-fact="fact-after"/);
  const legend=a.workflowCurrentTypeLegend(p.workflow,pref,ko);assert.match(legend,/data-workflow-legend-type="controller_review"/);assert.doesNotMatch(legend,/data-workflow-legend-type="coordination"/);
  assert.equal(JSON.stringify(p),before,'filtering preserves original source bytes');
 }
});
