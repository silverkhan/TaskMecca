const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');

function page({beforePost=async()=>{}}={}){
 const store=new Map(),projects=['/demo','/second'],updates=[];
 const status={configured:false,connected:false,project_enabled:true,recipient_mode:'individual',kinds:Object.fromEntries(['registered','started','intervention','approval','stalled','interrupted','runtime_unknown','finalize','completed'].map(k=>[k,true]))};
 const statuses=Object.fromEntries(projects.map(project=>[project,{...status,kinds:{...status.kinds}}]));
 const fetch=async (url,options={})=>{
  const path=String(url),project=new URL('https://fake.example'+path).searchParams.get('project')||'/demo';
  if(path.startsWith('/api/notifications/projects'))return {ok:true,json:async()=>({projects:projects.map(p=>({path:p,name:p.slice(1),status:{...statuses[p],kinds:{...statuses[p].kinds}}}))})};
  if(path.startsWith('/api/notifications/telegram')){
   if(options.method!=='POST')return {ok:true,json:async()=>({...statuses[project],kinds:{...statuses[project].kinds}})};
   const body=JSON.parse(options.body);updates.push({project,...body});
   await beforePost({project,...body});
   if(body.action==='test')return {ok:true,json:async()=>({ok:true})};
   if(body.action==='discover'||body.action==='discover_shared'){
    if(body.action==='discover_shared'){for(const p of projects)statuses[p]={...statuses[p],connected:true,enabled:true};}
    else statuses[project]={...statuses[project],connected:true,enabled:true};
   }
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
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko',userAgent:'Chrome'},Notification,window:{isSecureContext:true},alert:()=>{},localStorage:{
  getItem:k=>store.get(k)||null,setItem:(k,v)=>store.set(k,String(v)),removeItem:k=>store.delete(k)
 },document:{querySelector:()=>null,querySelectorAll:()=>[]},setTimeout,clearTimeout});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+
 '\nglobalThis.app={state,telegramAction,projectTelegramAction,configureSharedTelegram,loadProjectNotificationSettings,loadTelegramStatus,projectChannelSettingsMarkup,notificationOverviewMarkup,webNotificationEnabled,changeWebNotificationSetting,currentPushKinds,setAllTelegramChannelsEnabled,setNotificationSwitch,notificationSwitchPending,pollTelegramConnectionStatus,telegramSetupSummary,telegramSetupGuideMarkup,telegramSetupMayPrompt,setTelegramSetupPreference,telegramSetupNeed,telegramOnboardingChoice:telegramSetupPreference};',context);
 context.app.state.project='/demo';return {...context.app,updates,statuses,store};
}
test('individual token setup refreshes configured status immediately without browser reload',async()=>{
 const app=page();
 await app.loadProjectNotificationSettings();
 await app.telegramAction('configure',{token:'testing-only'});
 assert.equal(app.state.telegramStatus.configured,true);
 assert.equal(app.state.projectNotificationSettings.find(x=>x.path==='/demo').status.configured,true);
 const html=app.projectChannelSettingsMarkup();
 assert.match(html,/봇 토큰: <\/b>등록됨/);
 assert.match(html,/채팅 수신처: <\/b>연결 대기/);
 assert.equal((html.match(/data-telegram-discover-project="\/demo"/g)||[]).length,1,'only one discovery action per project');
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

test('mobile notification switches are atomic and never leak accessibility text',async()=>{
 const app=page();
 await app.loadProjectNotificationSettings();
 const markup=app.projectChannelSettingsMarkup()+app.notificationOverviewMarkup();
 assert.match(markup,/role="switch"/);
 assert.match(markup,/aria-checked="(?:true|false)"/);
 assert.match(markup,/aria-label="/);
 assert.doesNotMatch(markup,/notice-switch-track|class="sr-only"|>Off<\/span>|>On<\/span>/);
 // Nine event types across two channels and two projects plus masters.
 assert.equal((markup.match(/role="switch"/g)||[]).length,2*(9*2+2)+2);
 const css=fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8');
 assert.match(css,/\.notice-switch::before\s*\{/);
 assert.match(css,/\.notice-switch::after\s*\{/);
 assert.match(css,/\.notice-switch\.is-on::after\s*\{/);
 assert.doesNotMatch(css,/\.notice-switch-track/);
});

function switchTag(markup,attribute,value,project){
 const tags=markup.match(/<button\b[^>]*role="switch"[^>]*>/g)||[];
 return tags.find(tag=>tag.includes(attribute+'="'+value+'"')&&(!project||tag.includes('data-center-project="'+project+'"')));
}
test('turning Telegram channel off disables and visually marks all its child kinds, retaining stored selections',async()=>{
 const app=page();
 app.statuses['/demo'].configured=true;
 app.statuses['/demo'].kinds.registered=false;
 await app.loadProjectNotificationSettings();
 let html=app.projectChannelSettingsMarkup();
 assert.doesNotMatch(switchTag(html,'data-center-telegram-kind','completed','/demo'),/\sdisabled(?:\s|>)/);
 await app.setAllTelegramChannelsEnabled(false);
 html=app.projectChannelSettingsMarkup();
 const onType=switchTag(html,'data-center-telegram-kind','completed','/demo');
 const offType=switchTag(html,'data-center-telegram-kind','registered','/demo');
 assert.match(onType,/\sdisabled(?:\s|>)/);
 assert.match(onType,/aria-checked="true"/);
 assert.match(offType,/\sdisabled(?:\s|>)/);
 assert.match(offType,/aria-checked="false"/);
 assert.match(html,/data-telegram-locked="true"/);
 assert.match(html,/Telegram types are locked|Telegram 채널이 꺼져 있어/);
 const callCount=app.updates.length;
 await app.setNotificationSwitch({
  disabled:false,dataset:{centerProject:'/demo',centerTelegramKind:'completed'},
  getAttribute:()=> 'true',setAttribute:()=>{throw Error('mutated disabled child');}
 });
 assert.equal(app.updates.length,callCount,'programmatic events must not toggle kinds while channel is off');
 await app.setAllTelegramChannelsEnabled(true);
 html=app.projectChannelSettingsMarkup();
 assert.doesNotMatch(switchTag(html,'data-center-telegram-kind','completed','/demo'),/\sdisabled(?:\s|>)/);
 assert.match(switchTag(html,'data-center-telegram-kind','registered','/demo'),/aria-checked="false"/);
});
test('project Web OFF and global Web OFF lock dependent settings without clearing saved event kinds',async()=>{
 const app=page();await app.loadProjectNotificationSettings();
 app.changeWebNotificationSetting('/demo','started',false);
 app.changeWebNotificationSetting('/demo','enabled',false);
 let html=app.projectChannelSettingsMarkup();
 assert.match(switchTag(html,'data-center-web-kind','completed','/demo'),/\sdisabled(?:\s|>)/);
 assert.match(switchTag(html,'data-center-web-kind','started','/demo'),/aria-checked="false"/);
 assert.doesNotMatch(switchTag(html,'data-center-channel','web','/demo'),/\sdisabled(?:\s|>)/);
 const before=app.store.get('task-mecca-web-notification-settings-v1');
 await app.setNotificationSwitch({
  disabled:false,dataset:{centerProject:'/demo',centerWebKind:'completed'},
  getAttribute:()=> 'true',setAttribute:()=>{throw Error('mutated disabled child');}
 });
 assert.equal(app.store.get('task-mecca-web-notification-settings-v1'),before);
 app.changeWebNotificationSetting('/demo','enabled',true);
 html=app.projectChannelSettingsMarkup();
 assert.doesNotMatch(switchTag(html,'data-center-web-kind','completed','/demo'),/\sdisabled(?:\s|>)/);
 assert.match(switchTag(html,'data-center-web-kind','started','/demo'),/aria-checked="false"/);
 app.store.set('task-mecca-web-notification-settings-v1',JSON.stringify({enabled:false}));
 html=app.projectChannelSettingsMarkup();
 assert.match(switchTag(html,'data-center-channel','web','/demo'),/\sdisabled(?:\s|>)/);
 assert.match(switchTag(html,'data-center-web-kind','completed','/demo'),/\sdisabled(?:\s|>)/);
 assert.match(html,/data-web-locked="true"/);
});
test('OFF switch has distinct dark-theme border and child lock visuals',()=>{
 const css=fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8');
 assert.match(css,/:root\[data-theme="dark"\]\s+\.notice-switch:not\(\.is-on\)::before/);
 assert.match(css,/border-color:#9d92af/);
 assert.match(css,/background:#e7dfed/);
 assert.match(css,/\.notice-matrix\[data-web-locked="true"\]/);
 assert.match(css,/\.notice-matrix\[data-telegram-locked="true"\]/);
 assert.match(css,/\.notice-matrix \.notice-switch:disabled::before/);
});

function toggleFixture({id='',project='/demo',channel='',telegramKind='',webKind='',checked=true}={}){
 const attrs={ 'aria-checked':String(checked)};
 return {
  id,disabled:false,
  dataset:{centerProject:project,centerChannel:channel,centerTelegramKind:telegramKind,centerWebKind:webKind},
  getAttribute:name=>attrs[name],
  setAttribute:(name,value)=>{attrs[name]=value},
  classList:{toggle(){}}
 };
}
test('Telegram toggle visibly updates without waiting for a slow POST or full projects reload',async()=>{
 let complete;const slow=new Promise(resolve=>complete=resolve);
 const app=page({beforePost:async body=>{if(body.action==='project_enabled')await slow}});
 app.statuses['/demo'].configured=true;
 await app.loadProjectNotificationSettings();
 const toggle=toggleFixture({channel:'telegram'});
 let finished=false;const action=app.setNotificationSwitch(toggle).then(()=>{finished=true});
 assert.equal(finished,false,'the slow POST is still pending');
 assert.equal(app.state.projectNotificationSettings[0].status.project_enabled,false,'immediate optimistic status');
 assert.match(app.projectChannelSettingsMarkup(),/data-telegram-locked="true"/);
 assert.match(app.projectChannelSettingsMarkup(),/data-center-channel="telegram"/);
 assert.ok(app.notificationSwitchPending.size>0,'pending control guarded against duplicate interactions');
 assert.equal(app.updates.filter(x=>x.action==='project_enabled').length,1);
 complete();await action;
 assert.equal(finished,true);
 assert.equal(app.notificationSwitchPending.size,0);
 assert.equal(app.state.projectNotificationSettings[0].status.project_enabled,false);
 assert.equal(app.updates.length,1,'no redundant settings POST');
});
test('Telegram optimistic toggle rolls back on network failure',async()=>{
 let rejectPost;const blocked=new Promise((_,reject)=>rejectPost=reject);
 const app=page({beforePost:async body=>{if(body.action==='project_enabled')await blocked}});
 app.statuses['/demo'].configured=true;await app.loadProjectNotificationSettings();
 const original=app.state.projectNotificationSettings[0].status.project_enabled;
 const job=app.setNotificationSwitch(toggleFixture({channel:'telegram'}));
 assert.equal(app.state.projectNotificationSettings[0].status.project_enabled,false);
 rejectPost(new Error('network failure'));await job;
 assert.equal(app.state.projectNotificationSettings[0].status.project_enabled,original,'failed save restores previous server-confirmed value');
 assert.equal(app.notificationSwitchPending.size,0,'pending state always clears');
});

test('global Telegram toggle starts requests for all projects before their responses resolve',async()=>{
 let resolve;const slow=new Promise(done=>resolve=done);
 const app=page({beforePost:async body=>{if(body.action==='project_enabled')await slow}});
 await app.loadProjectNotificationSettings();
 const action=app.setAllTelegramChannelsEnabled(false);
 assert.equal(app.updates.filter(x=>x.action==='project_enabled').length,2,'project updates should be concurrent');
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.project_enabled===false));
 resolve();await action;
 assert.ok(app.state.projectNotificationSettings.every(x=>x.status.project_enabled===false));
});

test('pending Telegram connection has an obvious confirmation action outside recipient details',async()=>{
 const app=page();app.statuses['/demo'].configured=true;await app.loadProjectNotificationSettings();
 const list=app.projectChannelSettingsMarkup(),overview=app.notificationOverviewMarkup();
 assert.match(list,/data-telegram-discover-project="\/demo"/);
 assert.match(overview,/data-telegram-discover-project="\/demo"/);
 assert.match(list,/\/start/);
 assert.doesNotMatch(overview,/data-telegram-test-project/);
});
test('connected Telegram recipient has test-message action on overview and project detail',async()=>{
 const app=page();app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=true;await app.loadProjectNotificationSettings();
 const list=app.projectChannelSettingsMarkup(),overview=app.notificationOverviewMarkup();
 assert.match(list,/data-telegram-test-project="\/demo"/);
 assert.match(overview,/data-telegram-test-project="\/demo"/);
 assert.match(overview,/테스트 메시지 발송/);
});
test('test action never overwrites persisted connection with its acknowledgement payload',async()=>{
 const app=page();app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=true;await app.loadProjectNotificationSettings();
 const before=JSON.stringify(app.state.projectNotificationSettings),rev=app.state.projectNotificationSettingsRevision||0;
 const result=await app.projectTelegramAction('/demo','test');
 assert.equal(result.ok,true);
 assert.deepEqual(app.updates.filter(x=>x.action==='test').map(x=>x.project),['/demo']);
 assert.equal(JSON.stringify(app.state.projectNotificationSettings),before);
 assert.equal(app.state.projectNotificationSettingsRevision||0,rev);
});
test('read-only watch refreshes live connection without reloading page and stops off settings view',async()=>{
 const app=page();app.statuses['/demo'].configured=true;await app.loadProjectNotificationSettings();
 app.state.view='notifications';app.state.notificationCenterTab='settings';
 app.statuses['/demo'].connected=true;
 await app.pollTelegramConnectionStatus();
 assert.equal(app.state.projectNotificationSettings[0].status.connected,true);
 assert.equal(app.state.telegramStatus.connected,true);
 app.state.view='backlog';app.statuses['/demo'].connected=false;
 await app.pollTelegramConnectionStatus();
 assert.equal(app.state.projectNotificationSettings[0].status.connected,true);
});

test('Telegram test action matches the main notification UI button pattern',async()=>{
 const app=page();app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=true;
 await app.loadProjectNotificationSettings();
 const html=app.notificationOverviewMarkup()+app.projectChannelSettingsMarkup();
 assert.match(html,/class="action-btn telegram-message-action" data-telegram-test-project="\/demo"/);
 assert.match(html,/봇 토큰/);
 assert.match(html,/채팅 수신처/);
 assert.match(html,/등록 프로젝트 1\/2/);
 assert.match(html,/연결 프로젝트 1\/2/);
});
test('Registered Telegram bot shows name even when recipient chat is still pending',async()=>{
 const app=page();app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=false;
 app.statuses['/demo'].bot_username='company_bot';
 await app.loadProjectNotificationSettings();
 const html=app.projectChannelSettingsMarkup();
 assert.match(html,/@company_bot/);
 assert.match(html,/data-telegram-discover-project="\/demo"/);
 assert.match(html,/연결 대기/);
 assert.doesNotMatch(html,/data-telegram-test-project="\/demo"/);
});

test('optional Telegram guide appears only after server configuration is fetched',async()=>{
 const app=page();
 assert.equal(app.telegramSetupSummary(),null);
 assert.equal(app.telegramSetupGuideMarkup(),'' ,'no false initial unconfigured warning');
 await app.loadProjectNotificationSettings();
 const s=app.telegramSetupSummary();
 assert.equal(s.kind,'missing');assert.equal(s.total,2);
 const html=app.telegramSetupGuideMarkup();
 assert.match(html,/Telegram 알림을 설정해 보세요/);
 assert.match(html,/전체 프로젝트 알림용 봇 토큰 등록/);
 for(const step of ['봇 토큰 등록','\/start','수신처 연결 확인','테스트 메시지 발송'])assert.match(html,new RegExp(step));
 assert.match(html,/data-telegram-onboard="later"/);
 assert.match(html,/data-telegram-onboard="never"/);
});
test('recipient connection waiting is distinct from missing token and no step is prematurely completed',async()=>{
 const app=page();app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=false;
 app.statuses['/second'].project_enabled=false;
 await app.loadProjectNotificationSettings();
 const state=app.telegramSetupSummary();
 assert.equal(state.kind,'pending');assert.equal(state.total,1);assert.equal(state.ready,0);
 const html=app.telegramSetupGuideMarkup();
 assert.match(html,/채팅 수신처 연결 대기/);
 assert.match(html,/연결 설정 계속하기/);
 assert.match(html,/연결 전 확인 필요/);
 assert.equal((html.match(/is-done/g)||[]).length,1,'only token registration is confirmed');
});
test('partially connected projects need attention, fully connected ones do not',async()=>{
 const app=page();
 app.statuses['/demo'].configured=true;app.statuses['/demo'].connected=true;
 await app.loadProjectNotificationSettings();
 assert.equal(app.telegramSetupSummary().kind,'partial');
 assert.match(app.telegramSetupGuideMarkup(),/일부 프로젝트 미연결/);
 app.statuses['/second'].configured=true;app.statuses['/second'].connected=true;
 await app.loadProjectNotificationSettings();
 assert.equal(app.telegramSetupSummary().kind,'ready');
 assert.equal(app.telegramSetupGuideMarkup(),'');
});
test('explicitly disabled Telegram projects are not treated as setup failures',async()=>{
 const app=page();app.statuses['/demo'].project_enabled=false;app.statuses['/second'].project_enabled=false;
 await app.loadProjectNotificationSettings();
 assert.equal(app.telegramSetupSummary(),null);
 assert.equal(app.telegramSetupGuideMarkup(),'');
 assert.equal(app.telegramSetupNeed(),null);
});
test('Later snoozes Telegram onboarding and never suppresses reminders persistently',async()=>{
 const app=page();await app.loadProjectNotificationSettings();
 app.state.view='backlog';
 app.setTelegramSetupPreference('later');
 assert.equal(app.telegramSetupGuideMarkup(),'');
 assert.equal(app.telegramOnboardingChoice().mode,'later');
 assert.ok(app.telegramOnboardingChoice().until>Date.now());
 app.store.set('task-mecca-telegram-onboarding-v1',JSON.stringify({mode:'later',until:Date.now()-1000}));
 assert.ok(app.telegramSetupGuideMarkup());
 app.setTelegramSetupPreference('never');
 assert.equal(app.telegramSetupGuideMarkup(),'');
 assert.equal(app.telegramOnboardingChoice().mode,'never');
});
test('sidebar indicator uses a separate marker rather than overriding browser notification permission',()=>{
 assert.match(source,/function updateTelegramSetupNavIndicator/);
 assert.match(source,/projectNotificationsNav/);
 assert.match(source,/telegram-setup-nav-indicator/);
 assert.match(source,/function updateNotificationIndicator/);
 assert.match(source,/telegramSetupGuideMarkup\(true\)\+hubView/);
 assert.match(source,/\$\{telegramSetupGuideMarkup\(\)\}/);
});
