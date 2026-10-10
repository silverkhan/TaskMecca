const {test}=require('node:test');
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function app(fetch, options={}){
 const ctx=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},history:{pushState(){}},navigator:{language:'ko'},queueMicrotask,document:{visibilityState:'visible',querySelector:()=>null,querySelectorAll:()=>[]},localStorage:{getItem:()=>null,setItem(){},removeItem(){}},setTimeout:options.setTimeout||(()=>1),requestIdleCallback:options.requestIdleCallback});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+`
 render=()=>{};
 globalThis.api={state,openTask,loadTaskDetail,prefetchTaskDetail,requestTaskDetail,cachedTaskDetail,taskDetailPending,taskDetailCache,taskLiveCache,taskLivePending,taskDetailKey,acceptContentRevision,invalidateTaskDetailCache,scheduleTaskDetailPrefetch,scheduleTaskLiveEnrichment,lifecycleEvidenceLabel};
 `,ctx);
 const a=ctx.api;Object.assign(a.state,{project:'/demo',backlog:'/demo/backlog',view:'backlog',contentRevision:'r1',detail:null,detailTask:null,statusFilters:['all'],tagFilters:[],listPage:1,listSort:'id_desc',listPageMode:'manual',listPageSize:20,query:''});
 return {...a,ctx};
}
const task=id=>({id,title:'full '+id,raw_markdown:'# '+id,document:{requirements:{goal:'Complete'}}});
const reply=value=>({ok:true,json:async()=>value});
const tick=()=>new Promise(setImmediate);

test('adjacent list pages only contain summary, while hover prefetch shares click and paints body',async()=>{
 const pending=[],urls=[],a=app(url=>{urls.push(url);return new Promise(resolve=>pending.push(resolve))});
 a.prefetchTaskDetail('A-1');a.prefetchTaskDetail('A-2');assert.equal(urls.length,1,'bounded to one in-flight body prefetch');
 a.openTask('A-1');assert.equal(urls.length,1,'click must join already pending body fetch');
 pending.shift()(reply(task('A-1')));await tick();
 assert.equal(a.state.detailTask.raw_markdown,'# A-1');
 assert.equal(a.cachedTaskDetail('A-1').id,'A-1');
 a.state.detail=null;a.openTask('A-1');
 assert.equal(a.state.detailTask.raw_markdown,'# A-1','cached body paints synchronously');
 assert.equal(urls.length,1,'cached reentry skips network');
});
test('project changes cannot display previous project detail or reuse its cache',async()=>{
 const pending=[],a=app(url=>new Promise(resolve=>pending.push(resolve)));
 a.openTask('A-2');
 a.state.project='/other';a.state.backlog='/other/backlog';a.state.detail=null;a.state.detailTask=null;
 pending.shift()(reply(task('A-2')));await tick();
 assert.equal(a.state.detailTask,null);
 assert.equal(a.cachedTaskDetail('A-2'),null);
 a.openTask('A-2');assert.equal(pending.length,1);
 pending.shift()(reply({...task('A-2'),title:'from other'}));await tick();assert.equal(a.state.detailTask.title,'from other');
});
test('cached long detail is rendered only once on reentry',async()=>{
 let renders=0;
 const a=app(()=>Promise.resolve(reply(task('A-1'))));
 a.ctx.render=()=>{renders++;};
 await a.requestTaskDetail('A-1');
 a.openTask('A-1');await tick();
 assert.equal(renders,1,'a cache hit must not render the same Markdown twice');
});

test('content revision invalidation prevents stale prefetch reuse',async()=>{
 let n=0;const a=app(()=>{n++;return Promise.resolve(reply({...task('A-3'),title:'revision '+n}))});
 await a.requestTaskDetail('A-3');assert.equal(a.cachedTaskDetail('A-3').title,'revision 1');
 a.acceptContentRevision('r2');assert.equal(a.cachedTaskDetail('A-3'),null);
 await a.requestTaskDetail('A-3');assert.equal(a.cachedTaskDetail('A-3').title,'revision 2');
});
test('failed speculative fetch never locks out foreground retry',async()=>{
 let attempts=0;const a=app(()=>{attempts++;return attempts===1?Promise.resolve({ok:false,status:503,json:async()=>({error:'temporary'})}):Promise.resolve(reply(task('A-9')))});
 a.prefetchTaskDetail('A-9');await tick();
 a.openTask('A-9');await tick();
 assert.equal(attempts,2);assert.equal(a.state.detailTask.id,'A-9');assert.equal(a.state.loadError,'');
});
test('idle preload limits work to one current row and never crosses changed query context',async()=>{
 const callbacks=[],urls=[],a=app(url=>{urls.push(url);return Promise.resolve(reply(task('A-1')))}, {requestIdleCallback:fn=>callbacks.push(fn)});
 a.state.listData={items:[{id:'A-1'},{id:'A-2'}]};
 const query=vm.runInContext('listQueryString()',a.ctx);
 a.scheduleTaskDetailPrefetch(query);a.state.query='changed';callbacks.shift()();assert.equal(urls.length,0);
 a.state.query='';a.scheduleTaskDetailPrefetch(query);callbacks.shift()();await tick();
 assert.equal(urls.length,1);assert.match(urls[0],/A-1/);
 a.scheduleTaskDetailPrefetch(query);callbacks.shift()();assert.equal(urls.length,1);
});

test('content-first detail paints Markdown before a deliberately slow full reconciliation',async()=>{
 const timers=[],requests=[];
 let releaseContent,releaseFull;
 const a=app(url=>{
  requests.push(url);
  if(url.includes('projection=content'))return new Promise(resolve=>releaseContent=resolve);
  return new Promise(resolve=>releaseFull=resolve);
 },{setTimeout:fn=>{timers.push(fn);return timers.length;}});
 let renders=0;
 a.ctx.render=()=>{renders++};
 a.openTask('A-7');
 assert.equal(requests.length,1);
 assert.match(requests[0],/projection=content/);
 releaseContent(reply({...task('A-7'),content_only:true}));
 await tick();
 assert.equal(a.state.detailTask.raw_markdown,'# A-7');
 assert.equal(a.state.detailTask.content_only,true);
 assert.equal(renders,2,'click paints loading once, then actual Markdown');
 assert.equal(requests.length,1,'slow full request has not blocked the initial document');
 assert.equal(timers.length,1,'reconciliation is delayed until after body paint');
 timers.shift()();
 assert.equal(requests.length,2);
 assert.doesNotMatch(requests[1],/projection=content/);
 assert.equal(a.state.detailTask.content_only,true,'body remains available while control tower is slow');
 releaseFull(reply({...task('A-7'),content_only:false,activity:{health:'healthy'},lifecycle:{events:[{label:'Registered',at:'2026-10-01T00:00:00Z'}]}}));
 await tick();
 assert.equal(a.state.detailTask.content_only,false);
 assert.equal(a.state.detailTask.activity.health,'healthy');
 assert.equal(renders,3,'lifecycle info is enriched separately');
});
test('late full response cannot replace the body from a different task',async()=>{
 const timers=[],fullRequests=[];
 const a=app(url=>{
  if(url.includes('projection=content')){
   const id=url.includes('A-1')?'A-1':'A-2';
   return Promise.resolve(reply({...task(id),content_only:true}));
  }
  return new Promise(resolve=>fullRequests.push(resolve));
 },{setTimeout:fn=>{timers.push(fn);return timers.length;}});
 a.openTask('A-1');await tick();
 timers.shift()();
 assert.equal(fullRequests.length,1);
 a.openTask('A-2');await tick();
 timers.shift()();
 assert.equal(fullRequests.length,2);
 fullRequests[0](reply({...task('A-1'),content_only:false}));
 await tick();
 assert.equal(a.state.detail,'A-2');
 assert.equal(a.state.detailTask.raw_markdown,'# A-2');
 fullRequests[1](reply({...task('A-2'),content_only:false,activity:{health:'healthy'}}));
 await tick();
 assert.equal(a.state.detailTask.id,'A-2');
 assert.equal(a.state.detailTask.content_only,false);
});
test('outdated canonical result never silently swaps changed Markdown',async()=>{
 const timers=[];let fullResolve;
 const a=app(url=>url.includes('projection=content')
  ?Promise.resolve(reply({...task('A-4'),content_only:true}))
  :new Promise(resolve=>fullResolve=resolve),
  {setTimeout:fn=>{timers.push(fn);return timers.length;}});
 a.openTask('A-4');await tick();
 timers.shift()();
 fullResolve(reply({...task('A-4'),raw_markdown:'# A-4 newer',content_only:false}));
 await tick();
 assert.equal(a.state.detailTask.raw_markdown,'# A-4');
 assert.equal(a.state.pendingContentUpdate,true);
 assert.equal(a.cachedTaskDetail('A-4'),null);
});

test('confirmed lifecycle rows omit internal source, actor and ID while preserving evidence',()=>{
 const a=app(()=>Promise.resolve(reply(task('A-1'))));
 a.state.language='ko';
 const confirmed={source:'durable_lifecycle',actor:'worker/abc',event_id:'lifecycle-long-event-id',label:'Started',at:'2026-10-10T01:00:00Z'};
 const execution={source:'execution_ledger',actor:'runtime',event_id:'attempt-long-id',label:'Started'};
 assert.equal(a.lifecycleEvidenceLabel(confirmed),'','canonical durable record needs no provenance on the normal timeline');
 assert.equal(a.lifecycleEvidenceLabel(execution),'','verified execution ledger also needs no provenance label');
 assert.equal(confirmed.actor,'worker/abc');
 assert.equal(confirmed.event_id,'lifecycle-long-event-id','the model keeps diagnostic evidence intact');
 const inferred={source:'runtime_observed',provisional:true,event_id:'observation'};
 assert.equal(a.lifecycleEvidenceLabel(inferred),'','the phase label already marks provisional evidence');
 assert.equal(a.lifecycleEvidenceLabel({source:'git'}),'Git 이력','Git fallback remains identifiable');
 assert.equal(a.lifecycleEvidenceLabel({source:'runtime_observed'}),'파일 상태 관측','unflagged file observation remains distinguishable');
 a.state.language='en';
 assert.equal(a.lifecycleEvidenceLabel({source:'git'}),'Git history');
 assert.equal(a.lifecycleEvidenceLabel({source:'durable_lifecycle'}),'');
});
