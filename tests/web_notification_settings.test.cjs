const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');

function page(){
 const store=new Map(),projects=['/demo','/second'],updates=[];
 const status={configured:false,connected:false,project_enabled:true,recipient_mode:'individual',kinds:Object.fromEntries(['registered','started','intervention','approval','stalled','interrupted','runtime_unknown','finalize','completed'].map(k=>[k,true]))};
 const statuses=Object.fromEntries(projects.map(project=>[project,{...status,kinds:{...status.kinds}}]));
 const fetch=async (url,options={})=>{
  const path=String(url),project=new URL('https://fake.example'+path).searchParams.get('project')||'/demo';
  if(path.startsWith('/api/notifications/projects'))return {ok:true,json:async()=>({projects:projects.map(p=>({path:p,name:p.slice(1),status:{...statuses[p],kinds:{...statuses[p].kinds}}}))})};
  if(path.startsWith('/api/notifications/telegram')){
   if(options.method!=='POST')return {ok:true,json:async()=>({...statuses[project],kinds:{...statuses[project].kinds}})};
   const body=JSON.parse(options.body);updates.push({project,...body});
   if(body.action==='configure'){
    statuses[project]={...statuses[project],configured:true,enabled:false,bot_username:'robot'};
   }else if(body.action==='configure_shared'){
    for(const p of projects)statuses[p]={...statuses[p],configured:true,connected:false,recipient_mode:'shared',bot_username:'robot'};
   }else if(body.action==='project_enabled')statuses[project].project_enabled=body.project_enabled;
   else if(body.action==='kinds')statuses[project].kinds=body.kinds;
   return {ok:true,json:async()=>({...statuses[project],kinds:{...statuses[project].kinds}})};
  }
  return {ok:true,json:async()=>({})};
 };
 const Notification=function(){};Notification.permission='granted';
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko',userAgent:'Chrome'},Notification,window:{isSecureContext:true},localStorage:{
  getItem:k=>store.get(k)||null,setItem:(k,v)=>store.set(k,String(v)),removeItem:k=>store.delete(k)
 },document:{querySelector:()=>null,querySelectorAll:()=>[]},setTimeout,clearTimeout});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+
 '\nglobalThis.app={state,telegramAction,projectTelegramAction,configureSharedTelegram,loadProjectNotificationSettings,loadTelegramStatus,projectChannelSettingsMarkup,notificationOverviewMarkup,webNotificationEnabled,changeWebNotificationSetting,currentPushKinds,setAllTelegramChannelsEnabled};',context);
 context.app.state.project='/demo';return {...context.app,updates,statuses};
}
test('individual token setup refreshes configured status immediately without browser reload',async()=>{
 const app=page();
 await app.loadProjectNotificationSettings();
 await app.telegramAction('configure',{token:'testing-only'});
 assert.equal(app.state.telegramStatus.configured,true);
 assert.equal(app.state.projectNotificationSettings.find(x=>x.path==='/demo').status.configured,true);
 assert.match(app.projectChannelSettingsMarkup(),/봇 설정됨/);
});
test('shared token setup refreshes every project without reloading the browser',async()=>{
 const app=page();await app.loadProjectNotificationSettings();
 await app.configureSharedTelegram('/demo','testing-only');
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.configured));
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.recipient_mode==='shared'));
});
test('all nine event types have independent web and Telegram switches',async()=>{
 const app=page();await app.loadProjectNotificationSettings();
 const html=app.projectChannelSettingsMarkup();
 for(const kind of ['registered','started','intervention','approval','stalled','interrupted','runtime_unknown','finalize','completed']){
  assert.match(html,new RegExp('data-center-web-kind="'+kind+'"'));
  assert.match(html,new RegExp('data-center-telegram-kind="'+kind+'"'));
 }
 assert.equal((html.match(/role="switch"/g)||[]).length,2*(9*2+2));
 assert.doesNotMatch(html,/type="checkbox"/);
 assert.match(app.notificationOverviewMarkup(),/centerGlobalWebChannel/);
 assert.match(app.notificationOverviewMarkup(),/centerGlobalTelegramChannel/);
});
test('Telegram channel master toggles all projects while preserving their type filters',async()=>{
 const app=page();await app.loadProjectNotificationSettings();
 await app.setAllTelegramChannelsEnabled(false);
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.project_enabled===false));
 await app.setAllTelegramChannelsEnabled(true);
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.project_enabled===true));
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.kinds.finalize===true));
});
test('Web types are independent and support finalize in Push type synchronization',async()=>{
 const app=page();
 app.changeWebNotificationSetting('/demo','started',false);
 app.changeWebNotificationSetting('/demo','finalize',false);
 assert.equal(app.webNotificationEnabled('/demo','started'),false);
 assert.equal(app.webNotificationEnabled('/demo','finalize'),false);
 assert.equal(app.webNotificationEnabled('/demo','completed'),true);
 assert.equal(app.currentPushKinds('/demo').finalize,false);
 assert.equal(Object.keys(app.currentPushKinds('/demo')).length,9);
});
