const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const stamp='2026-10-14T12:00:00Z';
function session(fetch,options={}){
 const section={
   innerHTML:'Original lifecycle',
   height:50,
   rectBottom:100,
   getBoundingClientRect(){return {height:this.height,bottom:this.rectBottom}}
 };
 const scrolled=[];
 const context=vm.createContext({
   fetch,URLSearchParams,AbortController,setTimeout:(fn,delay)=>delay===1200?(options.setTimeout?options.setTimeout(fn,delay):1):setTimeout(fn,delay),clearTimeout,location:{search:'?project=/demo'},
   navigator:{language:'ko'},queueMicrotask,
   window:{scrollX:0,scrollY:600,scrollTo:args=>scrolled.push(args)},
   document:{visibilityState:'visible',querySelector:()=>null,getElementById:id=>id==='lifecycle'?section:null},
   localStorage:{getItem:()=>null,setItem(){},removeItem(){}}
 });
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+
 '\nglobalThis.harness={state,checkContentRevision,refreshViewedTaskLifecycle,lifecycleVisualFingerprint};',context);
 const harness=context.harness;
 Object.assign(harness.state,{
   view:'backlog',project:'/demo',backlog:'team',detail:'A-1',
   contentRevision:'r1',signalRevision:'s1',pendingContentUpdate:false,
   language:'ko',contentStateSnapshot:[{id:'A-1',title:'Original',file_state:'doing'}],
   detailTask:{id:'A-1',raw_markdown:'# Task A-1',content_only:false,
     activity:{health:'healthy'},
     lifecycle:{events:[{label:'Registered',at:stamp,event_id:'e1',actor:'worker'}]}}
 });
 return {harness,section,scrolled,context};
}
const json=data=>({ok:true,json:async()=>data});
const revision=(rev='r2',sig='s2')=>({revision:rev,signal_revision:sig,
 states:[{id:'A-1',title:'Original',file_state:'doing'},{id:'A-2',title:'Other',file_state:'done'}]});
const live=events=>({id:'A-1',lifecycle:{events},activity:{health:'healthy'}});
test('other backlog edits do not notify current reading; lifecycle updates only its DOM',async()=>{
 const urls=[];
 const a=session(url=>{
   urls.push(url);
   if(url.includes('/api/revision'))return Promise.resolve(json(revision()));
   if(url.includes('projection=content'))return Promise.resolve(json({raw_markdown:'# Task A-1'}));
   if(url.includes('projection=lifecycle'))return Promise.resolve(json(live([
      {label:'Registered',at:stamp,event_id:'e1',actor:'worker'},
      {label:'Started',at:'2026-10-14T12:10:00Z',event_id:'e2',actor:'worker'}
    ])));
   throw Error('unexpected URL: '+url);
 });
 const original=a.harness.state.detailTask;
 await a.harness.checkContentRevision();
 assert.equal(a.harness.state.pendingContentUpdate,false);
 assert.equal(a.harness.state.contentRevision,'r2');
 assert.equal(a.harness.state.signalRevision,'s2');
 assert.equal(a.harness.state.detailTask.raw_markdown,original.raw_markdown);
 assert.match(a.section.innerHTML,/Lifecycle|라이프사이클/);
 assert.match(a.section.innerHTML,/e2/);
 assert.equal(a.scrolled.length,0,'the document reading position must not be moved');
 assert.ok(urls.some(u=>u.includes('projection=content')));
 assert.ok(urls.some(u=>u.includes('projection=lifecycle')));
 assert.ok(!urls.some(u=>u.includes('projection=summary')));
});
test('actual current Markdown modification prompts but does not replace the open document',async()=>{
 const a=session(url=>{
   if(url.includes('/api/revision'))return Promise.resolve(json(revision('r2','s1')));
   if(url.includes('projection=content'))return Promise.resolve(json({raw_markdown:'# Edited A-1'}));
   throw Error(url);
 });
 await a.harness.checkContentRevision();
 assert.equal(a.harness.state.pendingContentUpdate,true);
 assert.equal(a.harness.state.pendingContentReason,'content');
 assert.equal(a.harness.state.detailTask.raw_markdown,'# Task A-1');
 assert.equal(a.section.innerHTML,'Original lifecycle');
 assert.equal(a.scrolled.length,0);
});
test('heartbeat-only signal change produces no alert and no lifecycle markup replacement',async()=>{
 const original=[{label:'Registered',at:stamp,event_id:'e1',actor:'worker'}];
 const a=session(url=>{
   if(url.includes('/api/revision'))return Promise.resolve(json(revision('r1','s2')));
   if(url.includes('projection=lifecycle'))return Promise.resolve(json(live(original)));
   throw Error(url);
 });
 await a.harness.checkContentRevision();
 assert.equal(a.harness.state.signalRevision,'s2');
 assert.equal(a.harness.state.pendingContentUpdate,false);
 assert.equal(a.section.innerHTML,'Original lifecycle');
});
test('task navigation during a pending lifecycle response must not rewrite a different document',async()=>{
 let release;
 const a=session(url=>{
   if(url.includes('/api/revision'))return Promise.resolve(json(revision('r1','s2')));
   if(url.includes('projection=lifecycle'))return new Promise(resolve=>release=resolve);
   throw Error(url);
 });
 const checking=a.harness.checkContentRevision();
 await new Promise(setImmediate);
 a.harness.state.detail='A-2';
 a.harness.state.detailTask={id:'A-2',raw_markdown:'# Task A-2',lifecycle:{}};
 release(json(live([{label:'Started',at:stamp}])));
 await checking;
 assert.equal(a.harness.state.detailTask.id,'A-2');
 assert.equal(a.section.innerHTML,'Original lifecycle');
 assert.equal(a.harness.state.signalRevision,'s1');
});

test('snapshot stale and error notices patch lifecycle without replacing body or open disclosures',async()=>{
 const oldEvents=[{label:'Registered',at:stamp,event_id:'e1',actor:'worker'}];
 const a=session(()=>Promise.resolve(json({...live(oldEvents),snapshot:{trust:'stale',version:'v2',computed_at:stamp}})));
 const original=a.harness.state.detailTask.raw_markdown;
 await a.harness.refreshViewedTaskLifecycle('/demo','team','A-1');
 assert.equal(a.harness.state.detailTask.raw_markdown,original);
 assert.match(a.section.innerHTML,/이전 확인 결과|earlier snapshot/);
 assert.match(a.section.innerHTML,/lifecycleRetryBtn/);
 assert.equal(a.scrolled.length,0);
 assert.equal(a.harness.state.taskLoadTimings[0].phase,'lifecycle:refresh');
});

test('failed independent lifecycle refresh keeps evidence and displays retry',async()=>{
 const a=session(()=>Promise.reject(new Error('offline')));
 await assert.rejects(a.harness.refreshViewedTaskLifecycle('/demo','team','A-1'),/offline/);
 assert.equal(a.harness.state.detailTask.raw_markdown,'# Task A-1');
 assert.match(a.section.innerHTML,/lifecycleRetryBtn/);
 assert.match(a.section.innerHTML,/이전 확인 결과|previous snapshot/);
 assert.equal(a.harness.state.detailTask.lifecycle.events[0].event_id,'e1');
});

test('late lifecycle response cannot replace a newer independent refresh',async()=>{
 const replies=[];const a=session(()=>new Promise(resolve=>replies.push(resolve)));
 const first=a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 const second=a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 replies[1](json(live([{label:'Started',at:stamp,event_id:'new-event'}])));await second;
 replies[0](json(live([{label:'Registered',at:stamp,event_id:'old-event'}])));await first;
 assert.equal(a.harness.state.detailTask.lifecycle.events[0].event_id,'new-event');
 assert.match(a.section.innerHTML,/new-event/);
 assert.doesNotMatch(a.section.innerHTML,/old-event/);
});

test('server restart pending snapshot keeps previous evidence and marks it stale',async()=>{
 const a=session(()=>Promise.resolve(json({id:'A-1',snapshot:{trust:'pending',version:''},lifecycle:{},activity:{health:'runtime_unknown'}})));
 await a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 assert.equal(a.harness.state.detailTask.lifecycle.events[0].event_id,'e1');
 assert.equal(a.harness.state.detailTask.snapshot.trust,'stale');
 assert.match(a.section.innerHTML,/lifecycleRetryBtn/);
 assert.equal(a.harness.state.detailTask.raw_markdown,'# Task A-1');
});

test('browser timings connect response Server-Timing and HTTP TTFB without inventing network values',async()=>{
 const a=session(()=>Promise.resolve({...json(live([{label:'Registered',at:stamp,event_id:'e1',actor:'worker'}])),headers:{get:()=> 'control;dur=0, projection;dur=0.1'}}));
 a.context.performance={now:()=>30,getEntriesByName:()=>[{requestStart:10,responseStart:15,responseEnd:20}],measure(){}};
 await a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 const timing=a.harness.state.taskLoadTimings[0];
 assert.equal(timing.phase,'lifecycle:refresh');assert.equal(timing.ttfb_ms,5);assert.equal(timing.transfer_ms,5);
 assert.equal(timing.server_timing,'control;dur=0, projection;dur=0.1');
 assert.match(timing.request_url,/projection=lifecycle/);
});

test('unchanged signal still rechecks lifecycle to recover Git changes and missed events',async()=>{
 const urls=[];const a=session(url=>{
  urls.push(url);
  if(url.includes('/api/revision'))return Promise.resolve(json(revision('r1','s1')));
  if(url.includes('projection=lifecycle'))return Promise.resolve(json(live([{label:'Started',at:stamp,event_id:'git-recovered'}])));
  throw Error(url);
 });
 await a.harness.checkContentRevision();
 assert.equal(a.harness.state.detailTask.lifecycle.events[0].event_id,'git-recovered');
 assert.ok(urls.some(url=>url.includes('projection=lifecycle')));
 assert.equal(a.harness.state.detailTask.raw_markdown,'# Task A-1');
});

test('stale snapshot automatically rechecks with a bounded retry budget',async()=>{
 const timers=[];let calls=0;
 const a=session(()=>{calls++;return Promise.resolve(json({...live([{label:'Registered',at:stamp,event_id:'e1'}]),snapshot:{trust:'stale',version:'v1'}}))},{setTimeout:fn=>{timers.push(fn);return timers.length;}});
 await a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 while(timers.length){timers.shift()();await new Promise(setImmediate);assert.ok(calls<=6,'recheck must not create an endless polling loop');}
 assert.equal(calls,6);
 assert.equal(a.harness.state.detailTask.raw_markdown,'# Task A-1');
 assert.match(a.section.innerHTML,/lifecycleRetryBtn/);
});

test('historical reassignment shows current assignee, former Worker and ambiguous Controller start separately',async()=>{
 const records=[
  {kind:'started',label:'Started',at:stamp,event_id:'digeuda-start',source:'durable_lifecycle',
   actor:'/root/controller/digeuda',assigned_worker:'/root/controller/digeuda',
   assignment_id:'assignment-digeuda'},
  {kind:'assigned',label:'Assigned',at:'2026-10-14T12:05:00Z',event_id:'new-assignment',source:'durable_lifecycle',
   actor:'/root/controller',assigned_worker:'/root/controller/nayong_1030_3m'},
  {kind:'started',label:'Started',at:'2026-10-14T12:10:00Z',event_id:'legacy-controller-start',source:'durable_lifecycle',
   actor:'/root/controller',assigned_worker:'/root/controller/nayong_1030_3m'}
 ];
 const a=session(()=>Promise.resolve(json(live(records))));
 a.harness.state.detailTask.agent='/root/controller/nayong_1030_3m';
 a.harness.state.detailTask.file_state='doing';
 await a.harness.refreshViewedTaskLifecycle('/demo','team','A-1',{force:true});
 const html=a.section.innerHTML;
 assert.match(html,/현재 담당: /);
 assert.match(html,/root\/controller\/nayong_1030_3m/);
 assert.match(html,/당시 배정: \/root\/controller\/digeuda/);
 assert.match(html,/기록 주체: \/root\/controller\/digeuda/);
 assert.match(html,/착수 Worker 식별 미확인/);
 assert.match(html,/legacy-controller-start/);
 assert.match(html,/아래는 과거 담당자와 기록 주체/);
});
