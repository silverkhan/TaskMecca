const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function app({fetch=async()=>({ok:true,json:async()=>({projects:[]})}),registration=null,construct=()=>{}}={}){
 const storage=new Map(),elements=new Map(),document={querySelector:selector=>elements.get(selector)||null,querySelectorAll:()=>[],documentElement:{},activeElement:null};
 const Notification=function(...args){construct(...args);this.close=()=>{}};Notification.permission='granted';
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko',...(registration?{serviceWorker:{register:async()=>{},ready:registration}}:{})},Notification,window:{isSecureContext:true,focus(){},location:{}},document,localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,String(value)),removeItem:key=>storage.delete(key)},queueMicrotask,alert(){},setTimeout,clearTimeout});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nrender=()=>{};globalThis.app={state,sendBrowserNotification,browserDeliveryHistory,mergedNotificationHistory,notificationCenterView,changeWebNotificationSetting,webNotificationEnabled,projectChannelSettingsMarkup,loadNotificationHistory,refreshDiagnostics,diagnosticEntries,diagnosticRecoveryRequest,updateDiagnosticNavigation,renderOperationBanner,captureHistoryWindow,acceptHistoryUpdate,notificationHistoryMarkup,historyRowTime,loadProjectNotificationSettings,setProjectNotificationEnabled,bindNotificationHistoryActions,preserveNotificationSettings,restoreNotificationSettings,loadTelegramStatus};',context);
 context.app.state.project='/demo';return {...context.app,context,storage,elements};
}
const task={id:'A-44',title:'알림 내용 <script>'};

const records=count=>Array.from({length:count},(_,i)=>({project:'/demo',event_id:'e'+String(i).padStart(3,'0'),task_id:'A-'+i,kind:'completed',event_at:new Date(Date.UTC(2026,9,8,0,0,i)).toISOString(),title:'Task '+i}));
test('0/1/30/31 boundaries render 30 latest rows with real page navigation and no current body',()=>{
 for(const count of [0,1,30,31]){const a=app();a.state.notificationHistory=records(count);a.state.notificationHistoryLoaded=true;a.captureHistoryWindow();const html=a.notificationCenterView();assert.equal((html.match(/data-history-row=/g)||[]).length,Math.min(30,count));assert.match(html,/data-center-tab="settings"/);assert.doesNotMatch(html,/data-center-tab="current"|notification-center-current|현재 확인이 필요한 작업이 없습니다/);if(count===31){a.state.notificationHistoryPage=2;assert.equal((a.notificationHistoryMarkup().match(/data-history-row=/g)||[]).length,1);}}
});
test('past pages keep exact ordered window under new arrivals and changed send times until Latest',()=>{
 const a=app();a.state.notificationHistory=records(61);a.captureHistoryWindow();a.state.notificationHistoryPage=2;const ids=a.state.notificationHistoryWindow.map(x=>x.event_id).join(',');
 a.state.notificationHistory.push({...records(1)[0],event_id:'incoming',event_at:'2027-01-01T00:00:00Z'});a.state.notificationHistory[0]={...a.state.notificationHistory[0],telegram:{state:'sent',sent_at:'2027-01-02T00:00:00Z'}};a.acceptHistoryUpdate();
 assert.equal(a.state.notificationHistoryWindow.map(x=>x.event_id).join(','),ids);assert.equal(a.state.notificationHistoryNew,2);assert.match(a.notificationHistoryMarkup(),/새 알림 2건/);a.state.notificationHistoryPage=1;a.captureHistoryWindow();assert.equal(a.state.notificationHistoryNew,0);assert.equal(a.state.notificationHistoryWindow[0].event_id,'e000');
});
test('settings preserves snapshot and counts local web evidence; first page updates only in history',()=>{
 const a=app();a.state.notificationHistory=records(1);a.captureHistoryWindow();a.state.notificationCenterTab='settings';a.storage.set('task-mecca-browser-deliveries-v1',JSON.stringify([{project:'/other',event_id:'local',sent_at:'2027-01-01T00:00:00Z',state:'display_requested'}]));a.acceptHistoryUpdate();assert.equal(a.state.notificationHistoryNew,1);assert.equal(a.state.notificationHistoryWindow.length,1);a.state.notificationCenterTab='history';a.acceptHistoryUpdate();assert.equal(a.state.notificationHistoryWindow.length,2);assert.equal(a.state.notificationHistoryNew,0);
});
test('actual later send time orders merged history deterministically across project/time ties',()=>{
 const a=app();a.state.notificationHistory=[{project:'/b',event_id:'same',event_at:'2026-01-01',telegram:{state:'sent',sent_at:'2026-02-01'}},{project:'/a',event_id:'same',event_at:'2026-01-01',telegram:{state:'sent',sent_at:'2026-02-01'}}];a.storage.set('task-mecca-browser-deliveries-v1',JSON.stringify([{project:'/a',event_id:'same',sent_at:'2026-03-01',state:'display_requested'}]));const rows=a.mergedNotificationHistory();assert.equal(rows.length,2);assert.equal(rows[0].project,'/a');assert.equal(a.historyRowTime(rows[0]),'2026-03-01');
});
test('conditional revision request merges changed rows and deletions without full-history reload',async()=>{
 const requests=[];let call=0;const a=app({fetch:async url=>{requests.push(url);return {ok:true,json:async()=>++call===1?{revision:'r1',reset:true,history:records(2)}:call===2?{revision:'r1',unchanged:true}:{revision:'r2',history:[{...records(1)[0],title:'Updated'}],removed:['e001'],reset:false}};}});
 await a.loadNotificationHistory();await a.loadNotificationHistory();assert.match(requests[1],/since=r1/);assert.equal(a.state.notificationHistory.length,2);await a.loadNotificationHistory();assert.equal(a.state.notificationHistory.length,1);assert.equal(a.state.notificationHistory[0].title,'Updated');assert.equal(requests.length,3);
});
test('late history response cannot apply after project context switch',async()=>{
 let finish;const a=app({fetch:()=>new Promise(resolve=>finish=resolve)});const request=a.loadNotificationHistory();a.state.project='/else';finish({ok:true,json:async()=>({revision:'wrong',history:records(1),reset:true})});await request;assert.equal(a.state.notificationHistory.length,0);assert.equal(a.state.notificationHistoryVersions['/demo'],undefined);
});
test('old settings GET cannot undo a successful saved channel value',async()=>{
 let finish;const a=app({fetch:async(url,options)=>options?.method==='POST'?{ok:true,json:async()=>({project_enabled:false,kinds:{registered:false}})}:new Promise(resolve=>finish=resolve)});a.state.projectNotificationSettings=[{path:'/demo',status:{project_enabled:true,kinds:{registered:false}}}];const old=a.loadProjectNotificationSettings();await a.setProjectNotificationEnabled('/demo',false);finish({ok:true,json:async()=>({projects:[{path:'/demo',status:{project_enabled:true,kinds:{registered:true}}}]})});await old;assert.equal(a.state.projectNotificationSettings[0].status.project_enabled,false);assert.equal(a.state.projectNotificationSettings[0].status.kinds.registered,false);
});
test('previous/number 1 capture newest window immediately while past-page moves retain snapshot',()=>{
 const a=app();a.state.notificationHistory=records(31);a.captureHistoryWindow();a.state.notificationHistoryPage=2;a.state.notificationHistory.push({...records(1)[0],event_id:'newest',event_at:'2027-01-01'});a.acceptHistoryUpdate();let click;
 a.context.document.querySelectorAll=selector=>selector==='[data-history-page]'?[{dataset:{historyPage:'1'},addEventListener:(_,handler)=>click=handler}]:[];a.bindNotificationHistoryActions();click();assert.equal(a.state.notificationHistoryPage,1);assert.equal(a.state.notificationHistoryWindow[0].event_id,'newest');assert.equal(a.state.notificationHistoryNew,0);
});
test('settings render preservation retains typed draft, selection, disclosure and focus',()=>{
 const a=app();let focused=false;const old={type:'password',value:'synthetic-draft',selectionStart:5},input={type:'password',value:'',focus:()=>focused=true,setSelectionRange:(from,to)=>{input.selection=[from,to]}},oldDetail={open:true},newDetail={open:false};a.context.document.activeElement=old;
 const before={querySelectorAll:selector=>selector==='input,textarea'?[old]:[oldDetail]},after={querySelectorAll:selector=>selector==='input,textarea'?[input]:[newDetail]};const draft=a.preserveNotificationSettings(before);a.restoreNotificationSettings(after,draft);assert.equal(input.value,'synthetic-draft');assert.equal(focused,true);assert.deepEqual(input.selection,[5,5]);assert.equal(newDetail.open,true);
});
test('mixed fractional RFC3339 and offsets use actual epoch for channel maximum sort and new count',()=>{
 const a=app();a.state.notificationHistory=[{project:'/demo',event_id:'fraction',event_at:'2026-10-08T00:00:00Z',telegram:{state:'sent',sent_at:'2026-10-08T00:00:00Z'}},{project:'/other',event_id:'offset',event_at:'2026-10-08T09:00:00+09:00'}];a.captureHistoryWindow();a.state.notificationHistoryPage=2;
 a.storage.set('task-mecca-browser-deliveries-v1',JSON.stringify([{project:'/demo',event_id:'fraction',sent_at:'2026-10-08T00:00:00.001Z',state:'display_requested'}]));const rows=a.mergedNotificationHistory();assert.equal(rows[0].event_id,'fraction');assert.equal(a.historyRowTime(rows[0]),'2026-10-08T00:00:00.001Z');a.acceptHistoryUpdate();assert.equal(a.state.notificationHistoryNew,1);
});
test('late selected-project Telegram status cannot replace the new project settings',async()=>{
 const responses=[],a=app({fetch:()=>new Promise(resolve=>responses.push(resolve))});const old=a.loadTelegramStatus();a.state.project='/new';const latest=a.loadTelegramStatus();responses[1]({ok:true,json:async()=>({marker:'new',kinds:{registered:false}})});await latest;responses[0]({ok:true,json:async()=>({marker:'old',kinds:{registered:true}})});await old;assert.equal(a.state.telegramStatus.marker,'new');assert.equal(a.state.telegramStatus.kinds.registered,false);
});
