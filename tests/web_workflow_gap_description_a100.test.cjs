// Actual shipped renderer; offline fixtures are not live measurement evidence.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/app.js'),'utf8'),cut=source.slice(0,source.indexOf('\ntranslateChrome();'));
const fixture=require('./fixtures/workflow/gap-description-a100.json');
function load(language='ko'){
 const c=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden')},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(cut+'\nglobalThis.a100={state,activityMetricsBody,workflowActivityLayout,workflowFloatingContent,workflowExplanationOrigin,workflowBrief,workflowRowSummary,workflowIntervalType};',c);c.a100.state.language=language;return c.a100;
}
test('A100 actual renderer includes open, execution-only, operation and unreported intervals in KO/EN',()=>{
 for(const language of ['ko','en']){const a=load(language),p=structuredClone(fixture),before=JSON.stringify(p),r=p.workflow.rows[0],layout=a.workflowActivityLayout(r,p.workflow.axis.end,true);assert.equal(layout.entries.length,5);assert.ok(layout.lanes>=2);
  const html=a.activityMetricsBody(p,'A100-FIXTURE');assert.equal((html.match(/data-workflow-interval=/g)||[]).length,2);assert.equal((html.match(/data-workflow-execution=/g)||[]).length,3);for(const kind of ['activity_open','operation_execution','execution_report','activity_unreported'])assert.match(html,new RegExp('wf-kind-'+kind));assert.match(html,language==='ko'?/상태 근거가 있는 시간/:/Time with state evidence/);assert.equal(JSON.stringify(p),before);
 }
});
test('A100 own interval explanation, exact marker reason, origins and session summary stay distinct',()=>{
 for(const language of ['ko','en']){const a=load(language),ko=language==='ko',r=fixture.workflow.rows[0];
  const interval=a.workflowFloatingContent(r,r.intervals[0],null,ko);assert.match(interval,/실제 구현 설명 &lt;원문&gt;/);assert.match(interval,ko?/작성된 설명/:/Written explanation/);
  assert.match(a.workflowFloatingContent(r,r.intervals[1],null,ko),ko?/유형 기본 설명/:/Type default explanation/);
  assert.match(a.workflowExplanationOrigin({explanation:'old'},ko),ko?/출처 미확인/:/origin unknown/);
  assert.match(a.workflowExplanationOrigin({explanation:'구현',explanation_origin:'written'},ko),ko?/작성된/:/Written/);
  const row=a.workflowFloatingContent(r,null,null,ko);assert.match(row,ko?/이 세션의 기록 현황/:/This session’s recorded scope/);assert.match(row,/등록 operation 확인/);
  const empty=a.workflowRowSummary({...r,intervals:[]},ko);assert.match(empty,ko?/관측 사건/:/Observed facts/);assert.doesNotMatch(empty,/Work description not recorded|모든 작업설명/);
  const marker=a.workflowFloatingContent(r,null,{kind:'gap',value:{status:'collection_failed',reason:'exact write denied',start:r.intervals[0].start,end:r.intervals[0].end,evidence:[]}},ko);assert.match(marker,/exact write denied/);assert.doesNotMatch(marker,/실제 구현 설명/);
 }
});
test('A100 immutable B653 and A96 source renders without rewriting historical evidence',()=>{
 const historical=require('./fixtures/workflow/b653-a96-regression-a100.json'),a=load();assert.equal(historical.a96_gaps.length,8);assert.equal(historical.historical_boundaries.stop_start_gap_ms,491.949);assert.equal(historical.historical_boundaries.after_task_activity_ms,67124);
 for(const sample of historical.samples){const before=JSON.stringify(sample),p={workflow:sample.workflow,events:sample.events,summary:sample.summary};assert.match(sample.sha256,/^[a-f0-9]{64}$/);assert.match(a.activityMetricsBody(p,sample.workflow.task_id),/workflow/);assert.equal(JSON.stringify(sample),before);}
});

// Optional real Chrome harness: CDP pipes + file URL, no listener or installation.
test('A100 real browser keyboard, touch, overlap and narrow zoom',{skip:!process.env.A100_CHROME,timeout:90000},async()=>{
 const {spawn}=require('node:child_process');const out=path.resolve(process.env.A100_BROWSER_EVIDENCE);fs.mkdirSync(out,{recursive:true});
 const profile=path.join(out,'chrome-profile');const chrome=spawn(process.env.A100_CHROME,['--headless=new','--remote-debugging-pipe','--no-first-run','--no-default-browser-check','--disable-background-networking','--disable-component-update','--disable-sync','--disable-extensions','--disable-breakpad','--disable-crash-reporter','--metrics-recording-only','--no-proxy-server','--host-resolver-rules=MAP * ~NOTFOUND','--user-data-dir='+profile,'about:blank'],{stdio:['ignore','ignore','pipe','pipe','pipe']});
 let stderr='';chrome.stderr.on('data',b=>stderr+=b);let id=0,buffer='',pending=new Map();let exited=false;chrome.stdio[3].on('error',()=>{});
 chrome.on('exit',(code,signal)=>{exited=true;fs.writeFileSync(path.join(out,'chrome.stderr'),stderr);for(const p of pending.values())p.reject(Error('Chrome exited '+code+' '+signal+': '+stderr.slice(-1500)));pending.clear();});
 chrome.stdio[4].on('data',b=>{buffer+=b;let at;while((at=buffer.indexOf('\0'))>=0){const raw=buffer.slice(0,at);buffer=buffer.slice(at+1);if(!raw)continue;const m=JSON.parse(raw),p=pending.get(m.id);if(p){pending.delete(m.id);m.error?p.reject(Error(JSON.stringify(m.error))):p.resolve(m.result)}}});
 const call=(method,params={},sessionId)=>new Promise((resolve,reject)=>{const n=++id;pending.set(n,{resolve,reject});chrome.stdio[3].write(JSON.stringify({id:n,method,params,...(sessionId?{sessionId}:{})})+'\0');});
 let timer;const timeout=new Promise((_,reject)=>timer=setTimeout(()=>reject(Error('Chrome pipe timed out: '+stderr.slice(-1500))),70000));
 try {await Promise.race([(async()=>{
  const {targetId}=await call('Target.createTarget',{url:'about:blank'}),{sessionId}=await call('Target.attachToTarget',{targetId,flatten:true});const cmd=(method,params)=>call(method,params,sessionId);
  await cmd('Page.enable');await cmd('Runtime.enable');await cmd('Network.enable');await cmd('Network.setBlockedURLs',{urls:['http://*','https://*','ws://*','wss://*']});
  const html=path.join(out,'fixture.html'),css=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/style.css'),'utf8');
  fs.writeFileSync(html,'<!doctype html><html lang="ko" data-theme="light"><meta charset="utf-8"><style>'+css+'</style><body><main id="fixture"></main><script>'+cut.replaceAll('</script','<\\/script')+'\nwindow.fixture='+JSON.stringify(fixture).replaceAll('<','\\u003c')+';window.ready=true;</script>');
  await cmd('Page.navigate',{url:'file://'+html});
  const evaluate=async(expression)=>{const r=await cmd('Runtime.evaluate',{expression,returnByValue:true,awaitPromise:true});if(r.exceptionDetails)throw Error(JSON.stringify(r.exceptionDetails));return r.result.value;};
  // Page load completes before evaluation via the next protocol command in this local file.
  let ready=false;for(let i=0;i<20&&!ready;i++){ready=await evaluate('window.ready===true');if(!ready)await new Promise(r=>setTimeout(r,30));}assert.ok(ready,'fixture load');
  const setup=language=>evaluate(`(()=>{state.language=${JSON.stringify(language)};state.project='fixture';activityDetailCache.set(activityDetailKey('A100','fixture'),fixture);document.querySelector('#fixture').innerHTML=activityMetricsBody(fixture,'A100','fixture');bindWorkflowControls('A100','fixture',document.querySelector('#fixture'));return {activity:document.querySelectorAll('[data-workflow-interval]').length,execution:document.querySelectorAll('[data-workflow-execution]').length,entries:document.querySelectorAll('[data-workflow-entry]').length}})()`);
  const report={transport:'CDP pipe + file URL; network blocked; no HTTP listener',checks:[]};
  for(const language of ['ko','en']){
   await cmd('Emulation.setDeviceMetricsOverride',{width:390,height:844,deviceScaleFactor:1,mobile:true});assert.deepEqual(await setup(language),{activity:2,execution:3,entries:5});
   const bounds=await evaluate(`(()=>{const b=document.querySelector('[data-workflow-interval]');b.focus();return {x:b.getBoundingClientRect().x,y:b.getBoundingClientRect().y}})()`);
   await cmd('Input.dispatchKeyEvent',{type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});await cmd('Input.dispatchKeyEvent',{type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13});
   assert.equal(await evaluate("Boolean(document.querySelector('.workflow-floating.is-pinned:not([hidden])'))"),true);
   assert.match(await evaluate("document.querySelector('.workflow-floating').textContent"),/실제 구현 설명/);
   await cmd('Input.dispatchKeyEvent',{type:'keyDown',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});await cmd('Input.dispatchKeyEvent',{type:'keyUp',key:'Escape',code:'Escape',windowsVirtualKeyCode:27});
   const before=await evaluate("JSON.stringify(activityDetailPrefs('A100','fixture').window)");await evaluate("document.querySelector('#workflowZoomIn').click()");await evaluate("new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))");assert.notEqual(await evaluate("JSON.stringify(activityDetailPrefs('A100','fixture').window)"),before);
   await evaluate("document.querySelector('#workflowFit').click()");await evaluate("new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))");assert.equal(await evaluate("JSON.stringify(activityDetailPrefs('A100','fixture').window)"),before,'Fit restores the full pre-zoom window');
   const point=await evaluate("(()=>{const b=document.querySelector('[data-workflow-row-select]'),r=b.getBoundingClientRect();return {x:r.x+r.width/2,y:r.y+r.height/2}})()");
   await cmd('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:point.x,y:point.y}]});await cmd('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});
   await evaluate("new Promise(r=>requestAnimationFrame(r))");assert.match(await evaluate("document.querySelector('.workflow-floating').textContent"),language==='ko'?/이 세션의 기록 현황/:/This session’s recorded scope/);
   const screenshot=await cmd('Page.captureScreenshot',{format:'png'});fs.writeFileSync(path.join(out,language+'-narrow.png'),Buffer.from(screenshot.data,'base64'));report.checks.push(language+': actual keyboard pin/Escape, zoom, touch row summary, overlap, 390px rendered');
  }
  fs.writeFileSync(path.join(out,'browser-report.json'),JSON.stringify(report,null,2));
 })(),timeout]);}finally{clearTimeout(timer);if(!exited)await Promise.race([call('Browser.close').catch(()=>{}),new Promise(r=>setTimeout(r,500))]);chrome.kill();fs.writeFileSync(path.join(out,'chrome.stderr'),stderr);}
});

test('A100 missing terminal fields stay unknown in actual labels and accessible renderer text',()=>{
 for(const language of ['ko','en']){const a=load(language),ko=language==='ko',p=structuredClone(fixture),before=JSON.stringify(p),v=p.workflow.rows[0].intervals.find(i=>i.kind==='activity_unreported');
  const label=a.workflowIntervalType(v,ko);assert.match(label,ko?/실행 종료 근거 미확인/:/Execution end evidence unknown/);assert.doesNotMatch(label,/undefined|실행 종료 관측|Execution end observed/);
  const html=a.activityMetricsBody(p,'A100-TERMINAL');assert.doesNotMatch(html,/종료 관측 \(undefined\)|end observed \(undefined\)/);assert.match(html,ko?/실행 종료 근거 미확인/:/Execution end evidence unknown/);
  assert.match(a.workflowIntervalType({...v,end_evidence:'native_terminal',end_evidence_source:'hook'},ko),/hook/);
  assert.match(a.workflowIntervalType({...v,end_evidence:'native_terminal',end_evidence_source:'unknown'},ko),ko?/근거 미확인/:/evidence unknown/);assert.match(a.workflowIntervalType({...v,runtime_terminal:{},end_evidence_source:'hook'},ko),ko?/근거 미확인/:/evidence unknown/);
  assert.equal(JSON.stringify(p),before,'no manufactured terminal state in source fixture');
 }
});

test('A100 overlap geometry wrappers pass hit testing to actual interval controls',()=>{
 const css=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/style.css'),'utf8');
 assert.match(css,/\.workflow-run\{[^}]*pointer-events:none/,'full-height geometry wrapper cannot intercept another lane');
 assert.match(css,/\.workflow-run \.workflow-interval\{[^}]*pointer-events:auto/,'actual buttons retain hover, click and bubbling viewport gestures');
 const a=load(),html=a.activityMetricsBody(structuredClone(fixture),'A100-HIT-STRUCTURE');
 assert.equal((html.match(/data-workflow-entry=/g)||[]).length,5);assert.equal((html.match(/data-workflow-interval=/g)||[]).length,2);assert.equal((html.match(/data-workflow-execution=/g)||[]).length,3);
 // This checks the CSS contract and real renderer structure, not browser geometry.
});
