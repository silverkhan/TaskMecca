const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function page(fetch){
 const prompt={innerHTML:''},dot={style:{}},active={id:'search',selectionStart:2,selectionEnd:4,focus(){this.focused=true},setSelectionRange(a,b){this.range=[a,b]}};
 const scroll=[],context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko'},queueMicrotask,requestAnimationFrame:fn=>fn(),window:{scrollX:3,scrollY:120,scrollTo:args=>scroll.push(args)},document:{activeElement:active,contains:()=>true,getElementById:()=>active,querySelector:selector=>selector==='#connectionDot'?dot:selector==='#contentUpdatePrompt'?prompt:null},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+source.slice(source.indexOf('function preserveViewportAndFocus('),source.indexOf('\nsetInterval(()=>{',source.indexOf('function preserveViewportAndFocus(')))+`\nrender=()=>{};processTaskNotifications=payload=>(globalThis.notifications||(globalThis.notifications=[])).push(payload);globalThis.app={state,refreshList,markContentUpdate,checkContentRevision,ensureAttentionStream,updateLiveListRelativeTimes,runningSeconds,listPageCaches,invalidateListPages,listContextKey};`,context);
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
