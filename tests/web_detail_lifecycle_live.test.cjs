const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const stamp='2026-10-14T12:00:00Z';
function session(fetch){
 const section={
   innerHTML:'Original lifecycle',
   height:50,
   rectBottom:100,
   getBoundingClientRect(){return {height:this.height,bottom:this.rectBottom}}
 };
 const scrolled=[];
 const context=vm.createContext({
   fetch,URLSearchParams,location:{search:'?project=/demo'},
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
 assert.match(a.section.innerHTML,/lifecycle/);
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
