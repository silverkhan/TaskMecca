const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');

function fixture({failRuntime=false}={}){
 const requests=[],store=new Map(),historyPaths=[];
 const project='/demo';
 const fetch=async(input,options={})=>{
  const url=String(input);requests.push({url,method:options.method||'GET'});
  if(url.startsWith('/api/hub'))return {ok:true,json:async()=>({projects:[{path:project,name:'Demo'}]})};
  if(url.startsWith('/api/storage/logs'))return {ok:true,json:async()=>({
   total_bytes:100,reclaimable_bytes:20,items:[
    {id:'web-service',label:'Web service',size_bytes:20,can_clear:true,scope:'global'},
    {id:'notification-events',label:'Notification events',size_bytes:80,can_clear:false,scope:'project'}]})};
  if(url.startsWith('/api/runtime/storage'))return {
   ok:!failRuntime,status:failRuntime?500:200,
   json:async()=>failRuntime?{error:'Runtime unavailable'}:{
    total_bytes:200,retention:{raw_days:7,history_days:90,history_max_attempts:2000},
    raw:{bytes:120,files:3},history:{bytes:80,files:1},legacy:{bytes:0,files:0},protected_raw:{bytes:100,files:2},
    cleanup:{reclaimable_bytes:30,candidate_files:1,candidate_attempts:3},
    raw_protection:{safe_now_bytes:20,conditional_bytes:80,unverifiable_bytes:12,conditional_files:2,
     generated_at:'2026-10-20T12:00:00Z',
     files:[
      {file:'2026-10-18.jsonl',bytes:40,event_count:12,reason_codes:['retention'],
       last_observed_at:'2026-10-18T11:00:00Z',retention_eligible_after:'2026-10-25T11:00:00Z',conditional_bytes:40},
      {file:'2026-10-01.jsonl',bytes:40,event_count:15,reason_codes:['unfinished','missing_terminal'],
       unfinished_attempts:1,missing_evidence_attempts:1,blocking_attempt_ids:['run-unfinished'],conditional_bytes:40},
      {file:'broken.jsonl',bytes:12,event_count:0,reason_codes:['unverifiable'],conditional_bytes:0},
      {file:'2026-09-29.jsonl',bytes:20,event_count:2,reason_codes:[],safe_now:true,conditional_bytes:0}
     ]}}};
  throw Error('Unexpected URL: '+url);
 };
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'',pathname:'/'},navigator:{language:'ko'},window:{isSecureContext:true,matchMedia:()=>({matches:false})},document:{querySelector:()=>null,querySelectorAll:()=>[],activeElement:null},history:{pushState:(_a,_b,url)=>historyPaths.push(url)},localStorage:{
  getItem:k=>store.get(k)||null,setItem:(k,v)=>store.set(k,String(v)),removeItem:k=>store.delete(k)
 },setTimeout,clearTimeout,queueMicrotask,alert(){},console});
 const code=source.slice(0,source.indexOf('\ntranslateChrome();'))+
  '\nrender=()=>{}; globalThis.api={state,navigateView,storageSelectedProject,storageManagementView,loadStorageManagement,logStorageSection};';
 vm.runInContext(code,context);
 context.api.state.hub={projects:[{path:project,name:'Demo'}]};
 return {api:context.api,requests,historyPaths};
}
test('storage management has its own navigation and isolates project data',async()=>{
 const a=fixture();
 a.api.navigateView('storage');
 assert.equal(a.api.state.view,'storage');
 assert.equal(a.historyPaths[0],'/?view=storage');
 await a.api.loadStorageManagement(false);
 assert.equal(a.api.state.storageProject,'/demo');
 assert.equal(a.api.state.logStorageProject,'/demo');
 for(const call of a.requests.filter(x=>x.url.startsWith('/api/storage/logs')||x.url.startsWith('/api/runtime/storage'))){
  assert.match(call.url,/\?project=%2Fdemo/);
 }
 assert.equal(a.api.state.logStorage.total_bytes,100);
 assert.equal(a.api.state.storageRuntime.total_bytes,200);
});
test('dashboard differentiates measured categories and never offers protected-ledger deletion',async()=>{
 const a=fixture();a.api.state.view='storage';
 await a.api.loadStorageManagement(false);
 const html=a.api.storageManagementView();
 assert.match(html,/저장소 관리/);
 assert.match(html,/진단 로그와 보호 원장/);
 assert.match(html,/에이전트 실행 기록 관리/);
 assert.match(html,/안전 정리 가능/);
 assert.match(html,/50 B/); // 20B clearable logs + 30B eligible runtime records
 assert.match(html,/data-clear-log="web-service"/);
 assert.match(html,/알림 이력/);
 assert.match(html,/보호됨/);
 assert.doesNotMatch(html,/data-clear-log="notification-events"/);
 assert.doesNotMatch(html,/id="logStorageProject"/); // one project selector
 assert.match(html,/범주별 측정치가 일부 중복될 수 있으므로/);
});
test('failed usage query stays an error, never a fake zero-byte success',async()=>{
 const a=fixture({failRuntime:true});a.api.state.view='storage';
 await a.api.loadStorageManagement(false);
 assert.match(a.api.state.storageError,/Runtime unavailable/);
 assert.match(a.api.storageManagementView(),/role="alert"/);
});
test('unregistered project paths cannot be submitted by the global storage selector',()=>{
 const a=fixture();a.api.state.storageProject='/not-registered';
 assert.equal(a.api.storageSelectedProject(),'/demo');
});

test('agent execution record labels explain each bucket and protected-subset double counting',async()=>{
 const a=fixture();a.api.state.view='storage';
 await a.api.loadStorageManagement(false);
 const html=a.api.storageManagementView();
 assert.match(html,/에이전트 실행 기록 관리/);
 assert.match(html,/에이전트 실행 기록 보존 기준/);
 assert.match(html,/상세 실행 이벤트/);
 assert.match(html,/종료된 실행 요약/);
 assert.match(html,/정리 보호 대상 Raw 파일/);
 assert.match(html,/이전 형식 실행 기록/);
 assert.match(html,/보호 대상 Raw 파일은 상세 실행 이벤트에 이미 포함됩니다/);
 assert.match(html,/하나라도 들어 있으면 파일 전체를 보호합니다/);
 assert.match(html,/파일 2개/);
 assert.doesNotMatch(html,/Runtime 안전 정리|Raw 이벤트|Legacy 기록/);
 assert.match(html,/<details class="storage-runtime-explain">/);
 assert.match(html,/마지막 활동이 보존 기간보다 오래되고/);
});
test('zero cleanup candidates do not claim that nonprotected records are safe to erase',async()=>{
 const a=fixture();a.api.state.view='storage';
 await a.api.loadStorageManagement(false);
 a.api.state.storageRuntime.cleanup.reclaimable_bytes=0;
 const html=a.api.storageManagementView();
 assert.match(html,/안전하게 정리 가능한 실행 기록/);
 assert.match(html,/현재 기준에 맞는 정리 대상이 없습니다/);
 assert.match(html,/이 수치만으로 정확한 원인을 단정할 수는 없습니다/);
 assert.match(html,/최근 7일 이내의 기록/);
 assert.match(html,/id="storageRuntimeCleanup"[^>]*disabled/);
});
test('storage language uses accurate English agent record labels and inclusion note',async()=>{
 const a=fixture();a.api.state.language='en';a.api.state.view='storage';
 await a.api.loadStorageManagement(false);
 const html=a.api.storageManagementView();
 assert.match(html,/Agent execution record management/);
 assert.match(html,/Detailed execution events/);
 assert.match(html,/Completed execution summaries/);
 assert.match(html,/Raw files protected from cleanup/);
 assert.match(html,/already included in detailed execution events/i);
 assert.match(html,/No records meet all cleanup conditions|Why records are protected/);
});

test('storage diagnostics separates immediate cleanup from conditional estimates and unsafe files',async()=>{
 const a=fixture();a.api.state.view='storage';await a.api.loadStorageManagement(false);
 const html=a.api.storageManagementView();
 assert.match(html,/보호 해제 조건과 확보 가능 용량/);
 assert.match(html,/현재 정리 가능한 Raw/);
 assert.match(html,/조건 충족 시 추가 가능/);
 assert.match(html,/80 B/);
 assert.match(html,/분석 불가/);
 assert.match(html,/12 B/);
 assert.match(html,/가정상 상한/);
 assert.match(html,/실제 확보량·시점은 보장되지 않습니다/);
 assert.match(html,/서로 중복되지 않습니다/);
 assert.match(html,/보존 기간 미도래/);
 assert.match(html,/종료되지 않은 실행 1/);
 assert.match(html,/종료 증거 부족 1/);
 assert.match(html,/run-unfinished/);
 assert.match(html,/broken\.jsonl/);
 assert.match(html,/실행 이벤트 원장, 알림 전송 원장, 라이프사이클 원장/);
 assert.doesNotMatch(html,/data-delete-raw|id="forceDelete"/);
});
test('unavailable file diagnostics show missing support rather than invented zero results',async()=>{
 const a=fixture();a.api.state.view='storage';await a.api.loadStorageManagement(false);
 delete a.api.state.storageRuntime.raw_protection;
 const html=a.api.storageManagementView();
 assert.match(html,/보호 조건별 용량 분석은 서버가 제공하는 버전에서 표시됩니다/);
 assert.doesNotMatch(html,/조건 충족 시 추가 가능/);
});
test('English explanations distinguish future potential from guaranteed savings',async()=>{
 const a=fixture();a.api.state.view='storage';await a.api.loadStorageManagement(false);
 a.api.state.language='en';
 const html=a.api.storageManagementView();
 assert.match(html,/Release conditions and reclaim potential/);
 assert.match(html,/Additional if conditions are met/);
 assert.match(html,/hypothetical upper bound/);
 assert.match(html,/no approved deletion\/restoration contract/);
});
