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
    raw:{bytes:20},history:{bytes:80},legacy:{bytes:0},protected_raw:{bytes:100},
    cleanup:{reclaimable_bytes:30,candidate_files:1,candidate_attempts:3}}};
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
 assert.match(html,/Runtime 안전 정리/);
 assert.match(html,/안전 정리 후보/);
 assert.match(html,/50 B/); // 20B clearable logs + 30B eligible runtime records
 assert.match(html,/data-clear-log="web-service"/);
 assert.match(html,/notification-events/);
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
