// Targeted nonvisual follow-up + short input on/off trace. No screenshot/polish loop.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),crypto=require('node:crypto');
const {chromium}=require(process.env.WORKFLOW_PLAYWRIGHT_MODULE||'/tmp/taskmecca-perceptual-runtime/node_modules/playwright');
const url=process.argv[2]||'http://127.0.0.1:57069/ui/material-review.html',out=path.resolve(process.argv[3]||'docs/evidence/a65-material-slice/technical-fix01');
if(fs.existsSync(out)&&fs.readdirSync(out).length)throw Error('Evidence already exists');fs.mkdirSync(out,{recursive:true});
const report={url,synthetic:true,api_response_routing:'none; actual Go HTTP',scope:'First material slice only; short absolute input on/off comparison, not whole-core 10% gate',system:[],traces:[],errors:[],assets:[]};
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const pick=metrics=>Object.fromEntries(metrics.metrics.map(m=>[m.name,m.value]));
const delta=(a,b,key)=>((b[key]||0)-(a[key]||0))*1000;
const median=a=>{const s=[...a].sort((a,b)=>a-b);return s.length?s.length%2?s[(s.length-1)/2]:(s[s.length/2-1]+s[s.length/2])/2:null};
(async()=>{let browser;try{
  for(const name of ['material-review.html','material/components.js','material/proximity.js','material/review.js','material/material.css']){
    const r=await fetch(new URL('/ui/'+name,url));const data=Buffer.from(await r.arrayBuffer()),local=fs.readFileSync(path.resolve('goassets/template/_task_mecca/framework/web/ui',name));assert.equal(r.status,200);assert.equal(Buffer.compare(data,local),0);report.assets.push({name,mime:r.headers.get('content-type'),sha256:crypto.createHash('sha256').update(data).digest('hex')});
  }
  browser=await chromium.launch({headless:true});report.browser=browser.version();
  for(const [width,height,touch]of[[1440,1000,false],[390,844,true]])for(const initial of['dark','light']){
    const context=await browser.newContext({viewport:{width,height},hasTouch:touch,isMobile:touch,colorScheme:initial});const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));
    await page.goto(url+'?theme='+initial);await page.locator('#material-task-card').waitFor();await page.locator('[data-material-mode="system"]').click();
    await page.emulateMedia({colorScheme:'light'});await page.waitForFunction(()=>document.documentElement.dataset.materialTheme==='light'&&matchMedia('(prefers-color-scheme:light)').matches);
    await page.emulateMedia({colorScheme:'dark'});await page.waitForFunction(()=>document.documentElement.dataset.materialTheme==='dark'&&matchMedia('(prefers-color-scheme:dark)').matches);
    assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-smoke-mode')),null);assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-theme')),null);
    report.system.push({width,initial,actual_light_change:true,actual_dark_change:true,original_preferences_untouched:true,passed:true});await context.close();
  }
  // Two counterbalanced pairs on the identical live fixture. Every run has a fresh context.
  for(const[order,enabled]of[['AB-on',true],['AB-off',false],['BA-off',false],['BA-on',true]]){
    const context=await browser.newContext({viewport:{width:1440,height:1000},colorScheme:'dark',serviceWorkers:'block'});const page=await context.newPage();page.on('pageerror',e=>report.errors.push(e.message));let apiRequests=0;page.on('request',r=>{if(r.url().includes('/api/'))apiRequests++});
    await page.goto(url+'?theme=dark&rail=expanded');await page.locator('#material-card-slot[aria-busy="false"]').waitFor();await page.locator('#material-task-card').waitFor();await page.evaluate(()=>document.fonts.ready);await page.mouse.move(1438,998);await sleep(450);
    if(!enabled)await page.evaluate(()=>window.taskMeccaMaterialReview.disableProximity());
    const box=await page.locator('#material-task-card').boundingBox(),cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');
    await page.evaluate(()=>{const m=window.__materialTrace={gaps:[],events:0,last:null,running:true,id:0};m.listener=()=>m.events++;document.addEventListener('pointermove',m.listener,{passive:true});const next=t=>{if(m.last!==null)m.gaps.push(t-m.last);m.last=t;if(m.running)m.id=requestAnimationFrame(next)};m.id=requestAnimationFrame(next)});
    const before=pick(await cdp.send('Performance.getMetrics')),beforeStats=await page.evaluate(()=>window.taskMeccaMaterialReview.inspect()),started=Date.now();
    const points=[];
    for(let i=0;i<80;i++){
      const f=i/79,x=box.x+24+f*(box.width-48),y=box.y+24+((Math.sin(f*Math.PI*2)+1)/2)*(box.height-48);
      points.push({x,y});await page.mouse.move(x,y);await sleep(12);
    }
    const remaining=2200-(Date.now()-started);if(remaining>0)await sleep(remaining);
    const after=pick(await cdp.send('Performance.getMetrics')),afterStats=await page.evaluate(()=>window.taskMeccaMaterialReview.inspect());
    const meter=await page.evaluate(()=>{const m=window.__materialTrace;m.running=false;cancelAnimationFrame(m.id);document.removeEventListener('pointermove',m.listener);return {gaps:m.gaps,events:m.events}});
    const trace={order,enabled,points,elapsedMs:Date.now()-started,windowTargetMs:2200,taskDurationMs:delta(before,after,'TaskDuration'),scriptDurationMs:delta(before,after,'ScriptDuration'),layoutDurationMs:delta(before,after,'LayoutDuration'),styleDurationMs:delta(before,after,'RecalcStyleDuration'),frameGapsMs:meter.gaps,eventCount:meter.events,moduleFrames:afterStats.frames-beforeStats.frames,moduleEvents:afterStats.events-beforeStats.events,moduleWrites:afterStats.writes-beforeStats.writes,moduleStats:afterStats,rawCounters:{before,after}};
    assert.equal(trace.eventCount,80);assert.ok(trace.taskDurationMs>=0);if(enabled){assert.equal(trace.moduleEvents,80);assert.ok(trace.moduleFrames>0&&trace.moduleFrames<=80);assert.ok(trace.moduleWrites>0)}else{assert.equal(trace.moduleFrames,0);assert.equal(trace.moduleWrites,0)}
    // Stop the explicit measurement rAF before testing the product's idle behavior.
    await page.mouse.move(1438,998);await sleep(500);const idleBefore=await page.evaluate(()=>{window.__materialIdleMutations=0;window.__materialIdleObserver=new MutationObserver(r=>window.__materialIdleMutations+=r.length);window.__materialIdleObserver.observe(document.querySelector('#material-review'),{attributes:true,childList:true,subtree:true,characterData:true});return window.taskMeccaMaterialReview.inspect()});const apiBefore=apiRequests;const idleCountersBefore=pick(await cdp.send('Performance.getMetrics'));await sleep(1100);const idleCountersAfter=pick(await cdp.send('Performance.getMetrics'));const idleAfter=await page.evaluate(()=>{window.__materialIdleObserver.disconnect();return{inspect:window.taskMeccaMaterialReview.inspect(),mutations:window.__materialIdleMutations}});
    trace.idle={windowMs:1100,frames:idleAfter.inspect.frames-idleBefore.frames,writes:idleAfter.inspect.writes-idleBefore.writes,events:idleAfter.inspect.events-idleBefore.events,pending:idleAfter.inspect.pending,domMutations:idleAfter.mutations,apiRequests:apiRequests-apiBefore,taskDurationMs:delta(idleCountersBefore,idleCountersAfter,'TaskDuration')};
    assert.equal(trace.idle.frames,0);assert.equal(trace.idle.writes,0);assert.equal(trace.idle.pending,false);assert.equal(trace.idle.domMutations,0);assert.equal(trace.idle.apiRequests,0);report.traces.push(trace);await context.close();
  }
  const on=report.traces.filter(t=>t.enabled),off=report.traces.filter(t=>!t.enabled);
  report.summary={onTaskDurationMs:on.map(t=>t.taskDurationMs),offTaskDurationMs:off.map(t=>t.taskDurationMs),onMedianTaskDurationMs:median(on.map(t=>t.taskDurationMs)),offMedianTaskDurationMs:median(off.map(t=>t.taskDurationMs)),absoluteMedianDifferenceMs:median(on.map(t=>t.taskDurationMs))-median(off.map(t=>t.taskDurationMs)),onFrameMedianMs:median(on.flatMap(t=>t.frameGapsMs)),offFrameMedianMs:median(off.flatMap(t=>t.frameGapsMs)),noRelativePercentClaim:true,wholeCoreGate:'NOT RUN for this first slice',limits:['two short counterbalanced pairs only','same synthetic actual Go fixture; warm data/fonts','no physical pointer/touch device, energy or total CPU/GPU/memory inference','the explicit rAF frame meter runs only during timing and is stopped before idle checks']};
  assert.deepEqual(report.errors,[]);report.passed=true;
}catch(error){report.failure=error.stack;report.passed=false;process.exitCode=1}finally{await browser?.close();fs.writeFileSync(path.join(out,'verification.json'),JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:report.passed,system:report.system,summary:report.summary,failure:report.failure}))}})();
