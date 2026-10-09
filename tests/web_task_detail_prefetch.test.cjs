const {test}=require('node:test');
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function app(fetch, options={}){
 const ctx=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},history:{pushState(){}},navigator:{language:'ko'},queueMicrotask,document:{visibilityState:'visible'},localStorage:{getItem:()=>null,setItem(){},removeItem(){}},setTimeout:options.setTimeout||(()=>1),requestIdleCallback:options.requestIdleCallback});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+`
 render=()=>{};
 globalThis.api={state,openTask,loadTaskDetail,prefetchTaskDetail,requestTaskDetail,cachedTaskDetail,taskDetailPending,taskDetailCache,taskDetailKey,acceptContentRevision,invalidateTaskDetailCache,scheduleTaskDetailPrefetch};
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
