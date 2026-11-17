const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const warningCode=source.slice(source.indexOf('function backlogBusinessState('),source.indexOf('function allRowsForView('));
const extract=(start,end)=>source.slice(source.indexOf(start),source.indexOf(end,source.indexOf(start)));
const esc=v=>String(v??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const state={language:'ko',statusFilters:['all'],listPage:1,selectedIndex:0,listSort:'id_desc'};
const labels={ko:{todo:'대기',doing:'작업 중',hold:'보류',done:'완료',ready:'준비',blocked:'선행 대기'},en:{todo:'To do',doing:'Doing',hold:'On hold',done:'Done',ready:'Ready',blocked:'Blocked'}};
const context=vm.createContext({state,esc,stateLabel:key=>labels[state.language][key]||key});
vm.runInContext(warningCode,context);
const call=(name,value,...args)=>context[name](value,...args);
for(const file of ['todo','doing','hold','done'])for(const type of ['runtime_unknown','runtime_stalled','execution_interrupted','approval_required']){
 const row={id:'B-643',file_state:file,state:'stalled',attention_reason:{type}};
 assert.equal(call('backlogBusinessState',row),file);
 assert.equal(call('matchesStatusFilter',row,file),true);
 const html=call('backlogStatusCell',row);assert(html.indexOf('data-backlog-warning')<html.indexOf('class="status '));assert(html.includes(`status ${file}`));
}
assert.equal(call('backlogBusinessState',{state:'doing'}),'','missing file state must not guess');
assert.equal(call('backlogBusinessState',{file_state:'doing',business_state:'blocked'}),'doing');
for(const b of ['ready','blocked'])assert.equal(call('backlogBusinessState',{file_state:'todo',business_state:b,state:'stalled'}),b);
for(const language of ['ko','en']){
 state.language=language;
 const plain=call('backlogStatusCell',{id:'plain',file_state:'doing'});assert(!plain.includes('backlog-warning'));assert.equal((plain.match(/<span/g)||[]).length,1);
 const label=call('backlogWarningLabel',{type:'runtime_unknown'});assert.equal(label,language==='ko'?'추적 확인 필요':'Check tracking');assert(!/stalled|정체/.test(label));
 for(const resolved of [{resolved:true},{resolved_at:'2026-11-17T00:00:00Z'},{recovered_at:'2026-11-17T00:00:00Z'},{history:true},{active:false},{status:'closed'}])assert.equal(call('backlogWarning',{attention_reason:{type:'runtime_unknown',...resolved}}),null);
}
assert(!call('backlogStatusCell',{id:'<unsafe>',file_state:'doing',attention_reason:{type:'unknown'}}).includes('data-backlog-warning="<unsafe>'));
console.log('A82 pure business/warning projection, KO/EN, resolution and escaping PASS');

// Read the real projection/wrapper shape so these regressions cannot silently
// replace the server's outer info severity with a simplified warning fixture.
const completionSource=fs.readFileSync('internal/backlog/completion_review.go','utf8');
const controlSource=fs.readFileSync('internal/backlog/control_tower.go','utf8');
const projection=completionSource.slice(completionSource.indexOf('func completionReviewProjection('),completionSource.indexOf('\nfunc completionReview('));
const projectionFields=[...projection.matchAll(/"([^"]+)":\s*([^,\n}]+)/g)];
assert(projectionFields.length>=13,'inspect changed server completion-review projection');
assert(controlSource.includes('map[string]any{"type": "controller_completion_review", "audience": "controller", "severity": "info", "state": review["state"], "message": review["diagnostic"], "completion_review": review}'));
function serverRecoveryTask(overrides={}) {
 const values={state:'controller_recovery',since:'2026-11-17T00:20:00Z',graceUntil:'2026-11-17T00:25:00Z',diagnostic:'Controller execution failed',reason:'controller_execution_failed',quality:'confirmed',action:'investigate_controller_execution',severity:'error',failed:true,handoffID:'synthetic-handoff',contract:'synthetic-contract',...overrides};
 const review=Object.fromEntries(projectionFields.map(([,key,raw])=>{const expr=raw.trim();return [key,expr==='action != ""'?values.action!=='':expr.startsWith('"')?JSON.parse(expr):values[expr]]}));
 assert.equal(review.action_required,values.action!=='');assert.equal(review.confirmed_failure,values.failed);assert.equal(review.observation_quality,values.quality);
 return {id:'SYNTHETIC-A82-RECOVERY',file_state:'doing',business_state:'doing',state:review.state,attention_reason:{type:'controller_completion_review',audience:'controller',severity:'info',state:review.state,message:review.diagnostic,completion_review:review},completion_review:review,activity:{health:review.state,last_observed_at:'2026-11-17T01:00:00Z'},updated_at:'2026-11-17T02:00:00Z'};
}
for(const language of ['ko','en']) {
 state.language=language;
 for(const overrides of [{},{reason:'handoff_step_failed',action:'investigate_failed_handoff_step',diagnostic:'Explicit handoff step failure'},{severity:'warning',failed:false,quality:'insufficient',reason:'controller_claim_unverified',action:'verify_exact_controller_binding',diagnostic:'Controller claim unverified'}]) {
  const task=serverRecoveryTask(overrides),before=JSON.stringify(task),warning=call('backlogWarning',task);
  assert(warning,'server-shaped actionable controller recovery must remain visible');
  assert.equal(warning.completion_review,task.completion_review);assert.equal(warning.message,task.completion_review.diagnostic);assert.equal(warning.action,task.completion_review.action);
  const html=call('backlogStatusCell',task);assert(html.indexOf('data-backlog-warning')<html.indexOf('class="status doing"'));assert.equal(call('backlogBusinessState',task),'doing');assert(call('matchesStatusFilter',task,'doing'));
  const details=call('backlogWarningDetails',task,warning);assert.equal(details.message,task.completion_review.diagnostic);assert.equal(details.time,language==='ko'?'미확인':'Unknown','since, grace, worker observation, and file timestamps are not Controller observation time');assert(details.next);
  assert.equal(JSON.stringify(task),before,'do not mutate server warning/activity/business data');
 }
 for(const phase of ['controller_review_pending','controller_review','controller_finalizing'])assert.equal(call('backlogWarning',serverRecoveryTask({state:phase,severity:'info',failed:false,action:'review_worker_result'})),null);
 assert.equal(call('backlogWarning',serverRecoveryTask({severity:'info',failed:false,action:''})),null,'non-actionable controller info stays excluded');
 for(const inactive of [{resolved:true},{resolved_at:'2026-11-17T00:30:00Z'},{recovered_at:'2026-11-17T00:30:00Z'},{history:true},{active:false},{status:'closed'}]) {
  for(const nested of [false,true]) {const task=serverRecoveryTask();Object.assign(nested?task.completion_review:task.attention_reason,inactive);assert.equal(call('backlogWarning',task),null)}
 }
 const timed=serverRecoveryTask();timed.completion_review.last_observed_at='2026-11-17T00:29:00Z';
 const warning=call('backlogWarning',timed);assert.equal(warning.last_observed_at,timed.completion_review.last_observed_at);
 context.dateTimeLabel=at=>'observed:'+at;
 assert.equal(call('backlogWarningDetails',timed,warning).time,'observed:2026-11-17T00:29:00Z');
 timed.completion_review.last_observed_at='invalid';assert.equal(call('backlogWarningDetails',timed,call('backlogWarning',timed)).time,language==='ko'?'미확인':'Unknown');
 assert.equal(call('backlogWarningDetails',{}, {type:'user_intervention',action:'verify_exact_controller_binding'}).next,'verify_exact_controller_binding','ordinary warning action behavior stays unchanged');
}
console.log('A82 actual server-shaped Controller error/warning recovery, info/history exclusion, nested diagnostic/action/observation and unchanged business state PASS');

// Exercise the production row/warning disclosure handlers against a tiny DOM
// adapter. This is source/DOM regression evidence, not another browser render.
const domTask=serverRecoveryTask({diagnostic:'Controller <execution> failed'});
let panel=null,rowOpens=0,buttons=[];
const element=()=>({dataset:{},attributes:{},listeners:{},style:{},isConnected:true,offsetWidth:320,offsetHeight:240,setAttribute(k,v){this.attributes[k]=v},removeAttribute(k){delete this.attributes[k]},addEventListener(k,fn){this.listeners[k]=fn},focus(){document.activeElement=this},contains(target){return target===this},getBoundingClientRect(){return {left:200,bottom:100}},remove(){panel=null;this.isConnected=false}});
const button=element();button.dataset.backlogWarning=domTask.id;buttons=[button];
const row=element();row.dataset.id=domTask.id;row.dataset.rowIndex='0';
const document={activeElement:null,querySelectorAll(selector){return selector==='[data-backlog-warning]'?buttons:['[data-id]','.task-list .task-row[data-id]','.task-row[data-id]'].includes(selector)?[row]:[]},addEventListener(){},createElement(){const p=element();p.closeButton=element();p.querySelector=()=>p.closeButton;return p},body:{appendChild(p){panel=p}}};
Object.assign(context,{document,window:{addEventListener(){}},AbortController,innerWidth:1440,innerHeight:900,allRowsForView:()=>[domTask],bindBacklogListTools(){},prefetchTaskDetail(){},$:()=>null,openTask(){rowOpens++}});
vm.runInContext(extract('function bindRows()','function changeListPage('),context);
const click=()=>{const e={prevented:false,stopped:false,target:{closest:()=>button},preventDefault(){this.prevented=true},stopPropagation(){this.stopped=true}};button.listeners.click(e);if(!e.stopped)row.onclick(e);return e};
for(const language of ['ko','en']) {
 state.language=language;context.bindRows();const e=click();
 assert(e.prevented&&e.stopped);assert.equal(rowOpens,0);assert(panel);assert.equal(panel.attributes.role,'dialog');assert.equal(button.attributes['aria-expanded'],'true');
 assert(panel.innerHTML.includes('Controller &lt;execution&gt; failed'));assert(panel.innerHTML.includes(language==='ko'?'미확인':'Unknown'));assert(panel.innerHTML.includes(language==='ko'?'Controller의 실행 오류·중단 근거를 확인하세요.':'Check the evidence of the Controller execution error or interruption.'));
 const key={key:'Escape',preventDefault(){this.prevented=true},stopPropagation(){this.stopped=true}};panel.listeners.keydown(key);assert(key.prevented&&key.stopped);assert.equal(panel,null);assert.equal(button.attributes['aria-expanded'],'false');assert.equal(document.activeElement,button);
 context.bindRows();click();domTask.completion_review.history=true;context.bindRows();assert.equal(panel,null,'resolved/history refresh closes existing disclosure');delete domTask.completion_review.history;
}
console.log('A82 production DOM disclosure: nested diagnostic/action, honest Unknown, row-click isolation, Escape/focus/aria and history cleanup PASS');

// Optional sanitized standalone fixture uses the production row renderer and
// warning/row event handlers. No backend, network, personal audit, or server.
if(process.argv[2]){
 const reason={type:'runtime_unknown',message:'Synthetic: runtime identity has not been verified.',last_observed_at:'2026-11-17T00:20:00Z',resume_condition:'Verify the latest runtime identity and observation with the Controller.'};
 const rows=[{id:'B-643',file_state:'doing',state:'stalled',attention_reason:reason},...['todo','doing','hold','done'].map((s,i)=>({id:`B-${644+i}`,file_state:s,state:s})),{id:'B-648',file_state:'doing',state:'stalled',attention_reason:{type:'runtime_stalled',message:'Synthetic long evidence: '+('Long localized text 한글 확인 내용. '.repeat(35)),resume_condition:'Check the latest activity without changing the task status.'}},{id:'B-649',file_state:'hold',state:'stalled',attention_reason:{type:'execution_interrupted'}},{id:'B-650',file_state:'doing',state:'doing',attention_reason:{...reason,resolved:true}}].map(r=>({...r,title:`Synthetic offline ${r.id}`,fields:{},document:{},activity:{},tags:['area:frontend'],updated_at:'2026-11-17T00:20:00Z'}));
 const listView=extract('function listView()','function attentionView(');
 // Keep exact production functions, with fixture-only data/format adapters.
 const setup=`const state=${JSON.stringify({...state,language:'ko',view:'backlog'})};let fixtureRows=${JSON.stringify(rows)},opened=0;const labels=${JSON.stringify(labels)};const esc=${esc.toString()};const $=s=>document.querySelector(s);const stateLabel=k=>labels[state.language][k]||k;const dateTimeLabel=(v,compact=false)=>new Date(v).toLocaleString(state.language==='ko'?'ko-KR':'en-US',compact?{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',timeZone:'UTC'}:{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',timeZone:'UTC'});const allRowsForView=()=>fixtureRows;const pageInfo=()=>({pageItems:fixtureRows,total:fixtureRows.length});const humanSummary=()=>({});const summaryPreview=v=>v||'';const titleOf=t=>t.title;const sourceLink=()=>'';const tagChip=tag=>'<span class="tag-chip">'+esc(tag)+'</span>';const fmtSec=()=>'-';const runningSeconds=()=>0;const ago=()=>'-';const updatedAt=t=>t.updated_at;const healthLabel=v=>v;const t=k=>({id:'ID',taskColumn:'Task',statusColumn:state.language==='ko'?'상태':'Status',agentColumn:'Agent',activeColumn:'Active',updatedColumn:'Updated',items:'items',allStatuses:'All'})[k]||k;const listControls=()=>'';const bindBacklogListTools=()=>{};const prefetchTaskDetail=()=>{};const setStatusFilter=()=>{};const setTagFilter=()=>{};const clearTagFilters=()=>{};const refreshList=()=>{};const openTask=()=>{opened++};const currentProjectData=()=>({counts:{}});`;
 const bindRows=extract('function bindRows()','function changeListPage(');
 const script=setup+warningCode+listView+bindRows+`function draw(language='ko',theme='dark'){state.language=language;document.documentElement.dataset.theme=theme;document.querySelector('#fixtureRows').innerHTML=listView();bindRows();}draw();window.fixture={draw,resolve(){fixtureRows[0].attention_reason=null;draw(state.language,document.documentElement.dataset.theme)},get opened(){return opened}};`;
 const html='<!doctype html><html lang="ko" data-theme="dark"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>A82 synthetic offline fixture</title><style>'+fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8')+'</style><body><main style="padding:20px;max-width:1400px;margin:auto"><p>Synthetic offline fixture · A-82</p><div id="fixtureRows"></div></main><script>'+script.replace(/<\/script/gi,'<\\/script')+'</script></body></html>';
 fs.writeFileSync(process.argv[2],html);console.log('Fixture: '+path.resolve(process.argv[2]));
}
