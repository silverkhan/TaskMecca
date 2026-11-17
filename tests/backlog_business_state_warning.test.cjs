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
