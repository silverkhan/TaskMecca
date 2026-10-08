const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function app({fetch=async()=>({ok:true,json:async()=>({projects:[]})}),registration=null,construct=()=>{}}={}){
 const storage=new Map(),elements=new Map(),document={querySelector:selector=>elements.get(selector)||null,querySelectorAll:()=>[],documentElement:{},activeElement:null};
 const Notification=function(...args){construct(...args);this.close=()=>{}};Notification.permission='granted';
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko',...(registration?{serviceWorker:{register:async()=>{},ready:registration}}:{})},Notification,window:{isSecureContext:true,focus(){},location:{}},document,localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,String(value)),removeItem:key=>storage.delete(key)},queueMicrotask,alert(){},setTimeout,clearTimeout});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nrender=()=>{};globalThis.app={state,sendBrowserNotification,browserDeliveryHistory,mergedNotificationHistory,notificationCenterView,changeWebNotificationSetting,webNotificationEnabled,projectChannelSettingsMarkup,loadNotificationHistory,refreshDiagnostics,diagnosticEntries,diagnosticRecoveryRequest,updateDiagnosticNavigation,renderOperationBanner,processTaskNotifications,refreshOperations};',context);
 context.app.state.project='/demo';return {...context.app,context,storage,elements};
}
const task={id:'A-44',title:'알림 내용 <script>'};

const tick=()=>new Promise(resolve=>setTimeout(resolve,0));
const snapshot=(project,backlog,items,events=[],at=new Date().toISOString())=>({project_path:project,backlog_selection:{selected:backlog,candidates:[]},snapshot_at:at,all_items:items,attention:[],notification_events:events});
test('center and hub polling dispatch all projects and folders without selected project',async()=>{
 const calls=[],at=new Date().toISOString();let a;
 const response=(value)=>({ok:true,json:async()=>value});
 a=app({construct:(title,options)=>calls.push({title,options}),fetch:async url=>{
  if(url==='/api/operations')return response({snapshot_at:at,active:[],projects:[{path:'/one'},{path:'/two'}]});
  const params=new URL('http://fixture'+url).searchParams,p=params.get('project'),folder=params.get('backlog')||'first';
  const data=snapshot(p,folder,{'A-1':{id:'A-1',title:'Actual task',file_state:'doing'}},[{id:p+folder,task_id:'A-1',kind:'started',at}]);
  if(folder==='first')data.backlog_selection.candidates=[{path:'first'},{path:'second'}];return response(data);
 }});a.state.project='';a.state.view='notifications';await a.refreshOperations();await tick();
 assert.equal(calls.length,4);assert.deepEqual(new Set(a.browserDeliveryHistory().map(x=>x.project)),new Set(['/one','/two']));
 assert.ok(calls.every(x=>x.options.data===undefined));
 assert.ok(calls.some(x=>x.options.tag.includes('/one:second:A-1')));
 await a.refreshOperations();await tick();assert.equal(calls.length,4);
});
test('explicit source controls title, tag, settings and click target across project transition',async()=>{
 const calls=[];const a=app({construct:(title,options)=>calls.push({title,options})});a.state.project='/selected';
 const at=new Date().toISOString();a.processTaskNotifications(snapshot('/source','folder',{'A-2':{id:'A-2',file_state:'doing',title:'Work'}},[{id:'e2',task_id:'A-2',kind:'registered',at}]),{project:'/source',backlog:'folder',updateCurrent:false});
 a.state.project='/changed';await tick();assert.equal(calls.length,1);assert.match(calls[0].title,/source/);assert.match(calls[0].options.tag,/source:folder/);assert.equal(a.browserDeliveryHistory()[0].project,'/source');
 a.changeWebNotificationSetting('/source','started',false);a.processTaskNotifications(snapshot('/source','folder',{'A-2':{id:'A-2',file_state:'doing'}},[{id:'e3',task_id:'A-2',kind:'started',at}]),{project:'/source',backlog:'folder',updateCurrent:false});await tick();assert.equal(calls.length,1);
});
test('folder-scoped previous rows do not fabricate completion of another same-ID task',async()=>{
 const calls=[];const a=app({construct:(...args)=>calls.push(args)});const at=new Date().toISOString();
 a.processTaskNotifications(snapshot('/demo','one',{'A-1':{id:'A-1',file_state:'doing'}}),{backlog:'one'});
 a.processTaskNotifications(snapshot('/demo','two',{'A-1':{id:'A-1',file_state:'done'}}),{backlog:'two'});await tick();assert.equal(calls.length,0);
 a.processTaskNotifications(snapshot('/demo','one',{'A-1':{id:'A-1',file_state:'done'}}),{backlog:'one'});await tick();assert.equal(calls.length,1);
});
test('denied unsupported insecure and display failure never create successful evidence',async()=>{
 for(const mode of ['denied','unsupported','insecure','failure']){
  const a=app({construct:()=>{if(mode==='failure')throw Error('display failed')}});
  if(mode==='denied')a.context.Notification.permission='denied';if(mode==='unsupported')a.context.Notification=undefined;if(mode==='insecure')a.context.window.isSecureContext=false;
  await a.sendBrowserNotification('completed',{id:'A-1'},null,'e','completed','e','/source','folder');assert.equal(a.browserDeliveryHistory().length,0);
 }
});
test('resolved reason, stale payload and old first-snapshot events are not redisplayed',async()=>{
 const calls=[],a=app({construct:(...args)=>calls.push(args)}),now=Date.now(),fresh=new Date(now).toISOString(),old=new Date(now-1000).toISOString();
 const task={id:'A-1',file_state:'doing',attention_reason:{type:'user_intervention',message:'Decide'}};
 a.processTaskNotifications(snapshot('/demo','folder',{'A-1':{id:'A-1',file_state:'doing'}},[],fresh),{backlog:'folder'});
 a.processTaskNotifications(snapshot('/demo','folder',{'A-1':task},[{id:'late',task_id:'A-1',kind:'intervention',reason_type:'user_intervention',at:fresh}],old),{backlog:'folder'});
 a.processTaskNotifications(snapshot('/demo','folder',{'A-1':{id:'A-1',file_state:'doing'}},[{id:'recovered',task_id:'A-1',kind:'intervention',reason_type:'user_intervention',at:fresh}],new Date(now+1).toISOString()),{backlog:'folder'});
 a.processTaskNotifications(snapshot('/demo','folder',{'A-1':{id:'A-1',file_state:'doing'}},[{id:'history',task_id:'A-1',kind:'started',at:new Date(now-25*3600000).toISOString()}],new Date(now+2).toISOString()),{backlog:'folder'});
 await tick();assert.equal(calls.length,0);assert.equal(a.browserDeliveryHistory().length,0);
});
test('server reason and current row produce one delivery, and type-off is not bypassed by row fallback',async()=>{
 for(const enabled of [true,false]){
  const calls=[],a=app({construct:(...args)=>calls.push(args)}),at=new Date().toISOString();a.changeWebNotificationSetting('/demo','approval',enabled);
  const task={id:'A-1',file_state:'doing',attention_reason:{type:'approval_required',title:'Approval'}};
  const data=snapshot('/demo','folder',{'A-1':task},[{id:'approval',task_id:'A-1',kind:'approval',reason_type:'approval_required',at}]);
  a.processTaskNotifications(data,{backlog:'folder'});await tick();assert.equal(calls.length,enabled?1:0);a.processTaskNotifications(data,{backlog:'folder'});await tick();assert.equal(calls.length,enabled?1:0);
 }
});
test('service worker accepts explicit source URL and delivery evidence only after success',async()=>{
 let release;const accepted=new Promise(resolve=>release=resolve),calls=[],a=app({registration:accepted});
 const promise=a.sendBrowserNotification('completed',{id:'A-1'},null,'source','completed','e','/other','other-folder');a.state.project='/later';assert.equal(a.browserDeliveryHistory().length,0);
 release({showNotification:async(title,options)=>calls.push({title,options})});await promise;
 assert.equal(calls.length,1);assert.equal(calls[0].options.data.url,'/tasks/A-1?project=%2Fother&backlog=other-folder');assert.equal(a.browserDeliveryHistory()[0].project,'/other');
});
test('deferred SW ready cannot resurrect a resolved reason while source transition remains valid',async()=>{
 for(const resolved of [true,false]){
  let ready;const calls=[],a=app({registration:new Promise(resolve=>ready=resolve)}),now=Date.now();
  const task={id:'A-1',file_state:'doing',attention_reason:{type:'user_intervention',title:'Decide'}};
  a.processTaskNotifications(snapshot('/source','folder',{'A-1':task},[{id:'deferred',task_id:'A-1',kind:'intervention',reason_type:'user_intervention',at:new Date(now).toISOString()}],new Date(now).toISOString()),{project:'/source',backlog:'folder',updateCurrent:false});
  if(resolved)a.processTaskNotifications(snapshot('/source','folder',{'A-1':{id:'A-1',file_state:'doing'}},[],new Date(now+1).toISOString()),{project:'/source',backlog:'folder',updateCurrent:false});
  a.state.project='/different';ready({showNotification:async(...args)=>calls.push(args)});await tick();assert.equal(calls.length,resolved?0:1);assert.equal(a.browserDeliveryHistory().length,resolved?0:1);
 }
});
test('new auto-folder mapping and mismatched source payload invalidate old-source dispatch',async()=>{
 const calls=[],a=app({construct:(...args)=>calls.push(args)}),now=Date.now();a.state.attentionScopes['/demo|']='new';a.state.attentionScopeObserved['/demo|']=now;
 const event={id:'stale-scope',task_id:'A-1',kind:'started',at:new Date(now).toISOString()},item={'A-1':{id:'A-1',file_state:'doing'}};
 a.processTaskNotifications(snapshot('/demo','old',item,[event],new Date(now-1).toISOString()),{backlog:''});
 a.processTaskNotifications(snapshot('/foreign','new',item,[event]),{project:'/demo',backlog:'new'});await tick();assert.equal(calls.length,0);
});
test('pending display rechecks settings and service-worker failure records no success',async()=>{
 for(const mode of ['off','failure']){
  let ready;const calls=[],a=app({registration:new Promise(resolve=>ready=resolve)});
  const pending=a.sendBrowserNotification('completed',{id:'A-1'},null,mode,'completed',mode,'/source','folder');
  if(mode==='off')a.changeWebNotificationSetting('/source','enabled',false);
  ready({showNotification:async()=>{calls.push(true);if(mode==='failure')throw Error('OS unavailable')}});await pending;
  assert.equal(calls.length,mode==='off'?0:1);assert.equal(a.browserDeliveryHistory().length,0);
 }
});
test('legacy seen reason stays consumed after introducing explicit folder source',async()=>{
 const calls=[],a=app({construct:(...args)=>calls.push(args)});a.storage.set('task-mecca-notification-seen',JSON.stringify(['/demo:A-1:intervention:user_intervention:stamp']));
 a.processTaskNotifications(snapshot('/demo','folder',{'A-1':{id:'A-1',file_state:'doing',updated_at:'stamp',attention_reason:{type:'user_intervention'}}}),{backlog:'folder'});await tick();assert.equal(calls.length,0);
});
