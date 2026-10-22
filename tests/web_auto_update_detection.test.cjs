const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const fragment=(start,end)=>{
 const first=source.indexOf(start),last=source.indexOf(end,first);
 if(first<0||last<first)throw Error('missing version-check source: '+start);
 return source.slice(first,last);
};
const code=[
  fragment('function renderGlobalUpdateIndicator() {','\nfunction revisionStateMap('),
  fragment('async function refreshVersionInfo(force=false) {','\n// The current task\'s raw Markdown'),
  fragment('async function fetchWithDeadline(','\nlet revisionCheckInFlight=null;'),
  fragment('let lastAutomaticVersionCheckAt=0;','\nsetInterval(automaticallyCheckLatestVersion,30000);'),
].join('\n')+'\nsetInterval(automaticallyCheckLatestVersion,30000);\n'+'globalThis.api={refreshVersionInfo,automaticallyCheckLatestVersion,renderGlobalUpdateIndicator};';
const tick=()=>new Promise(setImmediate);
const data=(current,latest,channel='dev')=>({cli:{current,latest,channel,update_available:Boolean(latest&&latest!==current)},project:{}});
function setup(fetchImpl){
 const listeners={},windowEvents={},scheduled=[];
 const indicator={innerHTML:''};
 const state={project:'',view:'hub',language:'ko',hub:null,versionInfo:null,versionCheckError:''};
 let now=10_000_000;
 let fetches=[],timers=[],buttonHandler;
 const context=vm.createContext({
   state,URLSearchParams,AbortController,Promise,
   Date:{now:()=>now},
   setTimeout:(fn,ms)=>{const id=timers.length+1;timers.push({fn,ms,id});return id},
   clearTimeout:()=>{},
   setInterval:(fn,ms)=>{scheduled.push({fn,ms});return scheduled.length},
   document:{visibilityState:'visible',addEventListener:(name,fn)=>{(listeners[name]??=[]).push(fn)}},
   window:{addEventListener:(name,fn)=>{(windowEvents[name]??=[]).push(fn)}},
   fetch:(url,opts)=>{fetches.push({url,opts});return fetchImpl(url,opts)},
   $:selector=>selector==='#globalUpdateIndicator'?indicator:selector==='#globalVersionRetryBtn'?{addEventListener:(n,cb)=>{if(n==='click')buttonHandler=cb}}:null,
   esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;'),
   t:s=>s,devPreviewAvailable:()=>false,refreshExecutionTarget:async()=>{},
   nav:()=>{},maybeConfirmPreviewStop:()=>{},maybeApplyStableBrandIntro:()=>{},
   renderReleaseNotesBadge:()=>{},maybeShowCurrentReleaseNote:()=>{},
   maybeShowPendingFrameworkSync:()=>{},maybeShowUpgradeRecovery:()=>{},
   queueMicrotask,
 });
 vm.runInContext("let upgradeFlowMode='idle';\n"+code,context);
 return {state,indicator,fetches,timers,scheduled,listeners,windowEvents,document:context.document,
   api:context.api,now:(n)=>{now=n},buttonClick:()=>buttonHandler?.()};
}
const ok=value=>({ok:true,json:async()=>value});
test('automatic version check uses a forced Dev release lookup and renders the complete update',async()=>{
 const p=setup(()=>Promise.resolve(ok(data('0.3.6-dev.2','0.3.6-dev.3'))));
 await p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,1);
 assert.match(p.fetches[0].url,/\/api\/version\?refresh=1/);
 assert.equal(p.state.versionInfo.cli.latest,'0.3.6-dev.3');
 assert.match(p.indicator.innerHTML,/0\.3\.6-dev\.2/);
 assert.match(p.indicator.innerHTML,/0\.3\.6-dev\.3/);
 assert.match(p.indicator.innerHTML,/globalVersionUpdateBtn/);
});
test('return from locked or backgrounded browser checks immediately without waiting for 60-second tick',async()=>{
 const p=setup(()=>Promise.resolve(ok(data('0.3.6-dev.2','0.3.6-dev.3'))));
 assert.ok(p.scheduled.some(x=>x.ms===30000));
 p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,1);
 p.now(10_010_000);p.windowEvents.focus[0]();await tick();
 assert.equal(p.fetches.length,1,'rapid focus events must coalesce');
 p.state.versionCheckError='';
 // Visibility can be hidden while a computer is locked.
 p.now(10_100_000);p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,2);
 p.now(10_170_000);p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,3);
 const hidden=p.listeners.visibilitychange[0];
 p.now(10_250_000);p.indicator.innerHTML='';
 p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,4);
 // The event listener must be registered for visibility, focus and online.
 assert.ok(p.windowEvents.focus?.length && p.windowEvents.online?.length && p.windowEvents.pageshow?.length);
});
test('hidden tab avoids checks and resumes on visibility, including online recovery',async()=>{
 const p=setup(()=>Promise.resolve(ok(data('0.3.6-dev.2','0.3.6-dev.3'))));
 p.api.automaticallyCheckLatestVersion();await tick();
 p.now(10_100_000);
 p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,2);
 p.now(10_200_000);
 p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,3);
 // Visibility change simulates the tab becoming foreground after lock.
 const docs=p.listeners.visibilitychange;
 assert.ok(docs?.length);
 p.now(10_300_000);
 p.state.project='';
 // Hidden tabs are skipped. When visible again, the event forces rechecking.
 p.document.visibilityState='hidden';
 p.api.automaticallyCheckLatestVersion();await tick();
 assert.equal(p.fetches.length,3);
 p.document.visibilityState='visible';
 p.listeners.visibilitychange[0]();await tick();
 assert.equal(p.fetches.length,4);
 p.now(10_400_000);p.windowEvents.online[0]();await tick();
 assert.equal(p.fetches.length,5);
});
test('a cached initial request cannot defeat a later forced update check',async()=>{
 let releaseCached;
 const p=setup(url=>url.includes('refresh=1')
   ?Promise.resolve(ok(data('0.3.6-dev.2','0.3.6-dev.3')))
   :new Promise(resolve=>{releaseCached=resolve}));
 const cached=p.api.refreshVersionInfo(false);
 const forced=p.api.refreshVersionInfo(true);
 assert.equal(p.fetches.length,1);
 releaseCached(ok(data('0.3.6-dev.2','0.3.6-dev.2')));
 await Promise.all([cached,forced]);
 assert.equal(p.fetches.length,2);
 assert.equal(p.state.versionInfo.cli.latest,'0.3.6-dev.3');
 assert.equal(p.state.versionCheckError,'');
});
test('failed release lookup is visible, preserves known release and provides retry',async()=>{
 let fail=true;
 const p=setup(()=>Promise.resolve(ok(fail
  ?{cli:{current:'0.3.6-dev.2',channel:'dev',error:'GitHub unreachable'}}
  :data('0.3.6-dev.2','0.3.6-dev.3'))));
 await p.api.refreshVersionInfo(false);
 await p.api.refreshVersionInfo(true);
 assert.match(p.indicator.innerHTML,/Update|업데이트/);
 assert.match(p.indicator.innerHTML,/globalVersionRetryBtn/);
 assert.match(p.indicator.innerHTML,/GitHub unreachable/);
 assert.match(p.indicator.innerHTML,/DEV · v0\.3\.6-dev\.2/);
 fail=false;
 p.buttonClick();await tick();
 assert.equal(p.state.versionCheckError,'');
 assert.equal(p.state.versionInfo.cli.latest,'0.3.6-dev.3');
 assert.match(p.indicator.innerHTML,/globalVersionUpdateBtn/);
});
test('explicit channel label prevents Stable users from mistaking its update stream for Dev',async()=>{
 const p=setup(()=>Promise.resolve(ok(data('0.3.5','0.3.5','stable'))));
 await p.api.refreshVersionInfo(false);
 assert.match(p.indicator.innerHTML,/STABLE · v0\.3\.5/);
 assert.doesNotMatch(p.indicator.innerHTML,/globalVersionUpdateBtn/);
});
