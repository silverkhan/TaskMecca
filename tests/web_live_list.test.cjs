const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function page(fetch,{quickFetch=null}={}){
 // Existing reconciliation tests model the authoritative endpoint. The
 // independent provisional first-paint request defaults to an unavailable
 // optional optimization; dedicated tests below exercise its successful path.
 const effectiveFetch=(url,options)=>{
  if(url.startsWith('/api/backlog/tasks?')&&new URLSearchParams(url.split('?')[1]).get('projection')==='initial'){
   return quickFetch?quickFetch(url,options):Promise.resolve({ok:false});
  }
  return fetch(url,options);
 };
 const prompt={innerHTML:''},dot={style:{}},active={id:'search',selectionStart:2,selectionEnd:4,focus(){this.focused=true},setSelectionRange(a,b){this.range=[a,b]}};
 const scroll=[],context=vm.createContext({fetch:effectiveFetch,URLSearchParams,AbortController,setTimeout,clearTimeout,location:{search:'?project=/demo'},navigator:{language:'ko'},queueMicrotask,requestAnimationFrame:fn=>fn(),window:{scrollX:3,scrollY:120,scrollTo:args=>scroll.push(args),addEventListener(){}},document:{visibilityState:'visible',addEventListener(){},activeElement:active,contains:()=>true,getElementById:()=>active,querySelector:selector=>selector==='#connectionDot'?dot:selector==='#contentUpdatePrompt'?prompt:null},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+source.slice(source.indexOf('function preserveViewportAndFocus('),source.indexOf('\nsetInterval(()=>{',source.indexOf('function preserveViewportAndFocus(')))+source.slice(source.indexOf('let lastWakeListRefreshAt=0;'),source.indexOf('\nsetInterval(()=>{',source.indexOf('let lastWakeListRefreshAt=0;')))+`\nrender=()=>{};processTaskNotifications=payload=>(globalThis.notifications||(globalThis.notifications=[])).push(payload);globalThis.app={state,refreshList,markContentUpdate,checkContentRevision,revalidateFirstPageOnWake,revalidateVisibleFirstPageByAge,ensureAttentionStream,updateLiveListRelativeTimes,runningSeconds,listPageCaches,invalidateListPages,listContextKey};`,context);
 context.app.state.view='backlog';context.app.state.listPageMode='manual';context.app.state.listPageSize=20;
 return {...context.app,prompt,scroll,active,context};
}
const tick=()=>new Promise(setImmediate),reply=(page,ids=['A-1'],revision='r1')=>({ok:true,json:async()=>({page,revision,items:ids.map(id=>({id})),backlog_selection:{candidates:[]}})});
test('first-page content and runtime events coalesce, preserve query, selection, scroll and focus',async()=>{
 const calls=[];const app=page(url=>{calls.push(url);return Promise.resolve(reply(1,['A-2','A-1']))});
 Object.assign(app.state,{listData:{items:[{id:'A-1'}]},statusFilters:['doing'],tagFilters:['area:web'],query:'한글',listSort:'updated_desc'});
 app.markContentUpdate('content');app.markContentUpdate('runtime');await tick();
 assert.equal(calls.length,1);assert.match(calls[0],/page=1/);assert.match(calls[0],/status=doing/);assert.match(calls[0],/tags=area%3Aweb/);assert.match(calls[0],/sort=updated_desc/);
 assert.equal(app.state.listData.items.length,2);assert.equal(app.state.selectedIndex,1);assert.equal(app.state.pendingContentUpdate,false);assert.equal(app.prompt.innerHTML,'');assert.equal(app.scroll.at(-1).top,120);assert.equal(app.active.focused,true);assert.deepEqual(app.active.range,[2,4]);
});
test('later pages keep current rows and ask for refresh; returning to first page resumes live updates',async()=>{
 let calls=0;const app=page(()=>{calls++;return Promise.resolve(reply(1,['A-3']))});app.state.listPage=2;app.state.listData={items:[{id:'A-1'}]};
 app.markContentUpdate('runtime');await tick();assert.equal(calls,1);assert.equal(app.state.listData.items[0].id,'A-1');assert.equal(app.state.pendingContentUpdate,true);assert.match(app.prompt.innerHTML,/contentUpdateRefreshBtn/);
 app.state.listPage=1;app.markContentUpdate();await tick();assert.equal(calls,2);assert.equal(app.state.pendingContentUpdate,false);
});
test('scheduled live refresh cannot change a page reached before its microtask starts',async()=>{
 let calls=0;const app=page(()=>{calls++;return Promise.resolve(reply(2))});app.markContentUpdate();app.state.listPage=2;await tick();assert.equal(calls,0);
});
test('late list responses cannot overwrite page, filters, project, folder, view or detail',async()=>{
 for(const change of [{listPage:2},{query:'changed'},{listSort:'updated_asc'},{statusFilters:['done']},{tagFilters:['tag']},{project:'/other'},{backlog:'other'},{view:'hub'},{detail:'A-1'}]){
  let resolve;const app=page(()=>new Promise(r=>resolve=r));app.state.listData={items:[{id:'current'}]};const refresh=app.refreshList();Object.assign(app.state,change);resolve(reply(1,['stale']));await refresh;assert.equal(app.state.listData.items[0].id,'current');assert.equal(app.state.listPage,change.listPage||1);
 }
});
test('refresh requested while first-page fetch is running queues only one latest-page fetch',async()=>{
 const pending=[],calls=[];const app=page(url=>{if(url.includes('projection=summary'))return Promise.resolve(reply(2));calls.push(url);return new Promise(resolve=>pending.push(resolve))});
 const first=app.refreshList();app.state.listPage=2;app.refreshList();app.refreshList();pending[0](reply(1,['old']));await first;await tick();assert.equal(calls.length,2);assert.match(calls[1],/page=2/);pending[1](reply(2,['new']));await tick();assert.equal(app.state.listData.items[0].id,'new');assert.equal(app.state.listPage,2);
});
test('revision polling routes first page live and later pages to existing confirmation',async()=>{
 for(const n of [1,2]){const calls=[];const app=page(url=>{calls.push(url);return Promise.resolve(url.startsWith('/api/revision')?{ok:true,json:async()=>({revision:'new',states:[]})}:reply(1,[],'new'))});app.state.listPage=n;app.state.contentRevision='old';await app.checkContentRevision();await tick();assert.equal(calls.length,2);assert.equal(app.state.pendingContentUpdate,n===2)}
});

test('actual SSE callback keeps notification processing and follows page-specific update policy',async()=>{
 for(const n of [1,2]){let calls=0;const app=page(()=>{calls++;return Promise.resolve(reply(1))});app.state.listPage=n;
 vm.runInContext(`globalThis.EventSource=class {static CLOSED=2;constructor(){this.readyState=1}addEventListener(name,handler){this.handler=handler}close(){}}`,app.context);
 app.state.backlog='team';app.ensureAttentionStream();const event=revision=>({data:JSON.stringify({revision,attention:[{id:'A-1',type:'user_intervention',message:revision}]})});app.state.eventSource.handler(event('old'));app.state.eventSource.handler(event('new'));await tick();assert.equal(app.context.notifications[0].attention[0].type,'user_intervention');assert.equal(calls,1);assert.equal(app.state.pendingContentUpdate,n===2);
 }
});

test('first-page relative times tick without fetching or replacing rows; later-page behavior stays unchanged',()=>{
 const app=page(()=>{throw Error('unexpected fetch')});const time={dataset:{updatedAt:new Date(Date.now()-120000).toISOString()},textContent:'old'};app.context.document.querySelectorAll=()=>[time];app.updateLiveListRelativeTimes();assert.notEqual(time.textContent,'old');time.textContent='preserved';app.state.listPage=2;app.updateLiveListRelativeTimes();assert.equal(time.textContent,'preserved');
 const task={file_state:'doing',active_seconds:60,lifecycle:{current_state:'doing',current_state_at:new Date(Date.now()-120000).toISOString(),current_segment_seconds:60}};assert.ok(app.runningSeconds(task,'active')>=120);
});

const packet=(n,revision='fresh')=>({page:n,pages:5,page_size:20,revision,snapshot_at:'2026-10-08T00:00:00.000Z',counts:{all:100,done:20},items:[{id:'A-'+n}],backlog_selection:{candidates:[]}});
const json=value=>({ok:true,json:async()=>value});
test('foreground renders before adjacent requests, three-page cache evicts and reentry paints before validation',async()=>{
 const pending=[],calls=[];const app=page(url=>{calls.push(url);if(url.includes('prefetch=1'))return Promise.resolve(json(packet(Number(new URLSearchParams(url.split('?')[1]).get('page')))));return new Promise(resolve=>pending.push(resolve));});
 const first=app.refreshList();assert.equal(calls.length,1);pending.shift()(json(packet(1)));await first;await tick();
 assert.equal(app.state.listData.items[0].id,'A-1');assert.equal(calls.length,2);
 app.state.listPage=2;const next=app.refreshList();assert.equal(app.state.listData.items[0].id,'A-2');assert.equal(app.state.listRevalidating,true);pending.shift()(json(packet(2)));await next;await tick();
 const cache=app.listPageCaches.get(app.listContextKey());assert.deepEqual([...cache.pages.keys()].sort(),[1,2,3]);
 app.state.listPage=3;const third=app.refreshList();assert.equal(app.state.listData.items[0].id,'A-3');pending.shift()(json(packet(3)));await third;await tick();assert.deepEqual([...cache.pages.keys()].sort(),[2,3,4]);
 app.state.view='hub';app.state.listData=null;app.state.view='backlog';const reenter=app.refreshList();assert.equal(app.state.listData.items[0].id,'A-3');const consumed=app.context.notifications.length;assert.equal(consumed,3);pending.shift()(json(packet(3)));await reenter;
 assert.equal(app.context.notifications.length,4);
});
test('later-page summary updates aggregate and current notification without replacing rows or acknowledging changes',async()=>{
 const app=page(url=>Promise.resolve(json(url.includes('projection=summary')?{...packet(2),items:[],counts:{all:101,done:21},attention:[]} :packet(2))));
 app.state.listPage=2;app.state.listData={...packet(2),items:[{id:'reading'}]};app.markContentUpdate('runtime');await tick();
 assert.equal(app.state.listData.items[0].id,'reading');assert.equal(app.state.listData.counts.all,101);assert.equal(app.state.pendingContentUpdate,true);assert.equal(app.context.notifications.length,1);
});
test('resolved invalidation rejects prefetch in flight and cached rows never consume attention',async()=>{
 let resolveAdjacent;const app=page(url=>url.includes('prefetch=1')?new Promise(resolve=>resolveAdjacent=resolve):Promise.resolve(json(packet(1))));
 await app.refreshList();await tick();assert.equal(app.listPageCaches.size,1);app.invalidateListPages();resolveAdjacent(json({...packet(2),attention:[{id:'resolved',type:'user_intervention'}]}));await tick();assert.equal(app.listPageCaches.size,0);assert.equal(app.context.notifications.length,1);
});
test('cache context isolates every query dimension and first/last pages never prefetch outside bounds',async()=>{
 const calls=[];const app=page(url=>{calls.push(url);const n=Number(new URLSearchParams(url.split('?')[1]).get('page'));return Promise.resolve(json({...packet(n),pages:2}));});
 const base=app.listContextKey();for(const change of [{project:'/else'},{backlog:'folder'},{query:'find'},{tagFilters:['tag']},{statusFilters:['done']},{listSort:'id_asc'},{listPageSize:10}]){const saved={...app.state};Object.assign(app.state,change);assert.notEqual(app.listContextKey(),base);Object.assign(app.state,saved);}
 await app.refreshList();await tick();assert.equal(calls.some(url=>url.includes('page=0')),false);app.state.listPage=2;await app.refreshList();await tick();assert.equal(calls.some(url=>url.includes('page=3')),false);
});

test('late summary cannot overwrite a resolved epoch or a project switch',async()=>{
 for(const change of ['resolution','project']){
  let resolve;const app=page(()=>new Promise(r=>resolve=r));app.state.listPage=2;app.state.listData={...packet(2),counts:{all:7},items:[{id:'reading'}]};app.markContentUpdate();
  if(change==='resolution')app.invalidateListPages();else app.state.project='/other';
  resolve(json({...packet(2),counts:{all:999}}));await tick();assert.equal(app.state.listData.counts.all,7);assert.equal(app.state.listData.items[0].id,'reading');assert.equal(app.context.notifications,undefined);
 }
});
test('last page deletion clamps current cache and still prefetches its previous valid neighbor',async()=>{
 const calls=[];const app=page(url=>{calls.push(url);if(url.includes('prefetch=1'))return Promise.resolve(json({...packet(3),pages:4}));return Promise.resolve(json({...packet(4),pages:4}));});app.state.listPage=5;await app.refreshList();await tick();assert.equal(app.state.listPage,4);assert.equal(calls.length,2);assert.match(calls[1],/page=3/);const cache=app.listPageCaches.get(app.listContextKey());assert.deepEqual([...cache.pages.keys()].sort(),[3,4]);
});

test('accepted background resolution invalidates cached rows while late history cannot restore a cache',async()=>{
 const app=page(url=>Promise.resolve(json(packet(Number(new URLSearchParams(url.split('?')[1]).get('page'))))));
 await app.refreshList();await tick();assert.equal(app.listPageCaches.size,1);
 vm.runInContext(`storeCommonUserAttention({snapshot_at:'2026-10-08T00:01:00Z',content_revision:'one',backlog_selection:{selected:'folder'},all_items:{},attention:[]},'/demo','');storeCommonUserAttention({snapshot_at:'2026-10-08T00:02:00Z',content_revision:'two',backlog_selection:{selected:'folder'},all_items:{},attention:[]},'/demo','');`,app.context);
 assert.equal(app.listPageCaches.size,0);
});

test('newer automatic folder selection invalidates a cached default source even when attention is empty',async()=>{
 const app=page(url=>Promise.resolve(json(packet(Number(new URLSearchParams(url.split('?')[1]).get('page'))))));await app.refreshList();await tick();
 vm.runInContext(`storeCommonUserAttention({snapshot_at:'2026-10-08T00:01:00Z',content_revision:'same',backlog_selection:{selected:'old'},attention:[]},'/demo','');storeCommonUserAttention({snapshot_at:'2026-10-08T00:02:00Z',content_revision:'same',backlog_selection:{selected:'new'},attention:[]},'/demo','');`,app.context);assert.equal(app.listPageCaches.size,0);
});

test('changing a displayed query context hides previous rows before a different-context response',async()=>{
 let resolve;let second=false;const app=page(()=>second?new Promise(r=>resolve=r):Promise.resolve(json({...packet(1),pages:1})));
 await app.refreshList();assert.equal(app.state.listData.items[0].id,'A-1');second=true;app.state.query='different';const next=app.refreshList();assert.equal(app.state.listData,null);resolve(json({...packet(1),pages:1,items:[{id:'different'}]}));await next;assert.equal(app.state.listData.items[0].id,'different');
});

test('new project starts immediately instead of waiting for a previous project response',async()=>{
 const requests=[];
 const app=page((url,{signal}={})=>new Promise((resolve,reject)=>{
  requests.push({url,resolve,signal});
  signal?.addEventListener('abort',()=>reject(new Error('aborted')),{once:true});
 }));
 const first=app.refreshList();
 app.state.project='/different-project';
 const next=app.refreshList();
 assert.equal(requests.length,2);
 assert.equal(requests[0].signal.aborted,true);
 assert.match(requests[1].url,/different-project/);
 requests[1].resolve(reply(1,['B-1']));
 await Promise.all([first,next]);
 assert.equal(app.state.listData.items[0].id,'B-1');
 assert.equal(app.state.loadError,'');
});

test('first page live-refreshes when only execution evidence changes',async()=>{
 const calls=[];
 const app=page(url=>{
  calls.push(url);
  return Promise.resolve(json(url.startsWith('/api/revision')
   ?{revision:'content-1',signal_revision:'runtime-2',states:[{id:'A-1',title:'Alpha',file_state:'doing'}]}
   :{...packet(1,'content-1'),signal_revision:'runtime-2',items:[{id:'A-1',runtime_state:'running'}]}));
 });
 Object.assign(app.state,{contentRevision:'content-1',signalRevision:'runtime-1',listData:{...packet(1,'content-1'),items:[{id:'A-1',runtime_state:'starting'}]}});
 await app.checkContentRevision();await tick();
 assert.equal(calls.filter(url=>url.startsWith('/api/revision')).length,1);
 assert.equal(calls.filter(url=>url.startsWith('/api/backlog/tasks')&&!url.includes('prefetch=1')).length,1);
 assert.equal(app.state.listData.items[0].runtime_state,'running');
 assert.equal(app.state.signalRevision,'runtime-2');
 assert.equal(app.state.pendingContentUpdate,false);
 assert.equal(app.prompt.innerHTML,'');
});

test('runtime-only evidence change on page two prompts once without moving rows',async()=>{
 const calls=[];
 const app=page(url=>{
  calls.push(url);
  return Promise.resolve(json(url.startsWith('/api/revision')
   ?{revision:'content-1',signal_revision:'runtime-2',states:[]}
   :{...packet(2,'content-1'),signal_revision:'runtime-2',items:[],counts:{all:101},attention:[]}));
 });
 Object.assign(app.state,{listPage:2,contentRevision:'content-1',signalRevision:'runtime-1',listData:{...packet(2,'content-1'),items:[{id:'reading'}]}});
 await app.checkContentRevision();await tick();
 assert.equal(app.state.listData.items[0].id,'reading');
 assert.equal(app.state.pendingContentUpdate,true);
 assert.match(app.prompt.innerHTML,/contentUpdateRefreshBtn/);
 await app.checkContentRevision();await tick();
 assert.equal(calls.filter(url=>url.includes('projection=summary')).length,1,'repeated checks must not repeatedly reload summary');
 assert.equal(calls.filter(url=>url.startsWith('/api/backlog/tasks')&&!url.includes('projection=summary')).length,0);
});

test('unchanged content and execution revision does not request a list refresh',async()=>{
 const calls=[],app=page(url=>{calls.push(url);return Promise.resolve(json({revision:'content-1',signal_revision:'runtime-1',states:[]}));});
 Object.assign(app.state,{contentRevision:'content-1',signalRevision:'runtime-1'});
 await app.checkContentRevision();await tick();
 assert.equal(calls.length,1);
 assert.equal(app.state.pendingContentUpdate,false);
});

test('initial project backlog fetch subscribes to realtime updates after its folder is resolved',async()=>{
 const app=page(()=>Promise.resolve(json({...packet(1,'content-1'),backlog_selection:{selected:'team',candidates:[]},signal_revision:'runtime-1'})));
 vm.runInContext(`globalThis.EventSource=class {static CLOSED=2;constructor(url){this.url=url;this.readyState=1}addEventListener(){}close(){}}`,app.context);
 // The production notification processing resolves the automatic folder.
 app.state.attentionScopes['/demo|']='team';
 await app.refreshList();
 assert.ok(app.state.eventSource,'the initial list fetch must establish SSE');
 assert.match(app.state.eventSource.url,/backlog=team/);
});

test('backlog Active column displays known completed and live durations but not fabricated history',()=>{
 const app=page(()=>{throw Error('render should not fetch')});
 app.state.listData={
  page:1,pages:1,page_size:20,total:4,counts:{working:1,ready:1,done:2,attention:0},
  backlog_selection:{selected:'demo'},items:[
   {id:'B-4',title:'New',file_state:'todo',state:'ready',active_seconds:null},
   {id:'B-3',title:'Running',file_state:'doing',state:'doing',
    active_seconds:80,lifecycle:{current_state:'doing',current_state_at:new Date(Date.now()-120000).toISOString(),current_segment_seconds:80}},
   {id:'B-2',title:'Old done',file_state:'done',state:'done',active_seconds:null},
   {id:'B-1',title:'Completed',file_state:'done',state:'done',active_seconds:125}
  ]
 };
 const html=vm.runInContext('listView()',app.context);
 assert.match(html,/<div class="task-active timer live-timer" data-id="B-4">-<\/div>/);
 assert.match(html,/<div class="task-active timer live-timer" data-id="B-3">\d\d:\d\d<\/div>/);
 assert.match(html,/<div class="task-active timer live-timer" data-id="B-2">-<\/div>/);
 assert.match(html,/<div class="task-active timer live-timer" data-id="B-1">02:05<\/div>/);
});


test('provisional list is visible before the authoritative reconciliation finishes',async()=>{
 let complete;
 const official=url=>new Promise(resolve=>{complete=resolve});
 const fast=()=>Promise.resolve({ok:true,json:async()=>({
  pending_reconciliation:true,page:1,pages:1,page_size:20,total:1,
  items:[{id:'B-1',file_state:'todo',state:'todo',state_provisional:true}],
  backlog_selection:{selected:'backlog',candidates:[]}
 })});
 const app=page(official,{quickFetch:fast});
 const running=app.refreshList();
 await tick();
 assert.equal(app.state.listData.pending_reconciliation,true);
 assert.equal(app.state.listRevalidating,true);
 assert.equal(app.state.listData.items[0].id,'B-1');
 assert.equal(app.context.notifications,undefined,'provisional file rows cannot deliver notifications');
 complete(json({...packet(1,'official'),items:[{id:'B-1',state:'blocked',file_state:'todo'}]}));
 await running;
 assert.equal(app.state.listData.pending_reconciliation,undefined);
 assert.equal(app.state.listData.items[0].state,'blocked');
 assert.equal(app.context.notifications.length,1);
});

// Sleep/wake regression: the authoritative list must be reloaded without
// waiting for a future SSE event, even if the revision fingerprint was missed.
test('waking a stale first page refreshes authoritative rows, not page two or task detail',async()=>{
 for(const pageNum of [1,2]){
  const calls=[];
  const app=page(url=>{calls.push(url);return Promise.resolve(reply(1,['A-9','A-1'],'rev-9'));});
  Object.assign(app.state,{listPage:pageNum,listData:{items:[{id:'A-1'}]},lastFetch:Date.now()-60000,contentRevision:'rev-1'});
  app.revalidateFirstPageOnWake();await tick();
  assert.equal(calls.length,pageNum===1?1:0);
  assert.equal(app.state.listData.items[0].id,pageNum===1?'A-9':'A-1');
 }
 const calls=[];
 const app=page(url=>{calls.push(url);return Promise.resolve(reply(1,['A-9']));});
 Object.assign(app.state,{detail:'A-1',listData:{items:[{id:'A-1'}]},lastFetch:Date.now()-60000});
 app.revalidateFirstPageOnWake();await tick();
 assert.equal(calls.length,0);
});

test('first page rejects late pre-sleep data after waking and keeps new response',async()=>{
 let oldResolve;
 const calls=[];
 const app=page(url=>{calls.push(url);if(calls.length===1)return new Promise(resolve=>{oldResolve=resolve});return Promise.resolve(reply(1,['NEW'],'rev-2'))});
 Object.assign(app.state,{listData:{items:[{id:'OLD'}]},lastFetch:Date.now()-60000});
 const old=app.refreshList();
 app.revalidateFirstPageOnWake();
 await tick();
 assert.equal(app.state.listData.items[0].id,'NEW');
 oldResolve(reply(1,['OLD'],'rev-1'));await old;await tick();
 assert.equal(app.state.listData.items[0].id,'NEW');
});

test('revision establishment does not acknowledge a newly added but unseen backlog',async()=>{
 const calls=[];
 const app=page(url=>{calls.push(url);return Promise.resolve(url.startsWith('/api/revision')?{ok:true,json:async()=>({revision:'rev-2',states:[{id:'A-2'}]})}:reply(1,['A-2','A-1'],'rev-2'));});
 Object.assign(app.state,{listData:{items:[{id:'A-1'}]},contentRevision:'rev-1',signalRevision:'signal-1'});
 await app.checkContentRevision(true);await tick();
 assert.equal(calls.length,2);
 assert.equal(app.state.listData.items[0].id,'A-2');
 assert.equal(app.state.contentRevision,'rev-2');
});

test('a hung revision poll releases its single-flight gate after deadline and detects new work',async()=>{
 let calls=0;
 const app=page(url=>{
  if(url.startsWith('/api/revision')) {
   calls++;
   if(calls===1)return new Promise(()=>{});
   return Promise.resolve({ok:true,json:async()=>({revision:'rev-2',states:[{id:'A-2'}]})});
  }
  return Promise.resolve(reply(1,['A-2'],'rev-2'));
 });
 Object.assign(app.state,{listData:{items:[{id:'A-1'}]},contentRevision:'rev-1'});
 const nativeTimeout=app.context.setTimeout;
 app.context.setTimeout=(cb,ms)=>ms===20000?nativeTimeout(cb,0):nativeTimeout(cb,ms);
 await app.checkContentRevision();
 await app.checkContentRevision();await tick();
 assert.equal(calls,2);
 assert.equal(app.state.listData.items[0].id,'A-2');
});

test('a hung list reload times out, then next first-page refresh can recover',async()=>{
 let requests=0;
 const app=page(url=>{
  requests++;
  if(requests===1)return new Promise(()=>{});
  return Promise.resolve(reply(1,['NEW'],'rev-2'));
 });
 const nativeTimeout=app.context.setTimeout;
 app.context.setTimeout=(cb,ms)=>ms===60000?nativeTimeout(cb,0):nativeTimeout(cb,ms);
 await app.refreshList();
 await app.refreshList();await tick();
 assert.equal(requests,2);
 assert.equal(app.state.listData.items[0].id,'NEW');
});

test('archived B-539 is recovered from revision inventory without needing B-540 registration',async()=>{
 const calls=[];
 const app=page(url=>{
  calls.push(url);
  if(url.startsWith('/api/revision')) {
   return Promise.resolve(json({revision:'rev-539',signal_revision:'s1',states:[{id:'B-539',file_state:'done'}]}));
  }
  return Promise.resolve(json({...packet(1,'rev-539'),total:1,items:[{id:'B-539',file_state:'done',location:'archive',archive_month:'2026-10'}]}));
 });
 Object.assign(app.state,{listData:{...packet(1,'rev-539'),total:0,items:[]},contentRevision:'rev-539',signalRevision:'s1'});
 await app.checkContentRevision(false);await tick();
 assert.equal(app.state.listData.items[0]?.id,'B-539');
 assert.equal(calls.filter(x=>x.startsWith('/api/backlog/tasks?')&&!x.includes('prefetch=1')).length,1);
 assert.equal(calls.filter(x=>x.startsWith('/api/revision?')).length,1);
});

test('newer summary with identical revision does not suppress authoritative archived rows',async()=>{
 const urls=[];
 const app=page(url=>{
  urls.push(url);
  if(url.includes('projection=summary'))return Promise.resolve(json({...packet(2,'rev-539'),items:[],total:1,snapshot_at:'2026-10-23T10:00:02Z',signal_revision:'s1'}));
  return Promise.resolve(json({...packet(1,'rev-539'),total:1,snapshot_at:'2026-10-23T10:00:01Z',signal_revision:'s1',items:[{id:'B-539',location:'archive'}]}));
 });
 Object.assign(app.state,{listPage:2,listData:{...packet(2,'rev-539'),items:[{id:'old'}]},contentRevision:'rev-539'});
 app.markContentUpdate('runtime');await tick();
 app.state.listPage=1;await app.refreshList();await tick();
 assert.equal(app.state.listData.items[0].id,'B-539');
 assert.equal(app.state.listData.total,1);
 assert.equal(app.state.listRevalidating,false);
});

test('stale visible first page reconciles even without any new revision or focus event',async()=>{
 const urls=[];
 const app=page(url=>{urls.push(url);return Promise.resolve(json({...packet(1,'rev-539'),total:1,items:[{id:'B-539',location:'archive'}]}));});
 Object.assign(app.state,{listData:{...packet(1,'rev-539'),total:0,items:[]},contentRevision:'rev-539',lastFetch:Date.now()-90000});
 await app.revalidateVisibleFirstPageByAge();await tick();
 assert.equal(app.state.listData.items[0]?.id,'B-539');
 const foreground=()=>urls.filter(x=>!x.includes('prefetch=1')).length;
 assert.equal(foreground(),1);
 await app.revalidateVisibleFirstPageByAge();await tick();
 assert.equal(foreground(),1,'do not reconcile again while the snapshot is fresh');
 app.context.document.visibilityState='hidden';
 app.state.lastFetch=Date.now()-90000;
 await app.revalidateVisibleFirstPageByAge();await tick();
 assert.equal(foreground(),1,'hidden pages must not perform expensive full list scans');
});
