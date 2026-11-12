// Deterministic, test-only schedule reproducer. Run from repository root.
// Holds one real activity response; does not replace native fetch/render/input.
const assert=require('node:assert/strict'),path=require('node:path');
const fixture=require(path.resolve('tests/helpers/workflow_native.cjs'));
const {chromium}=require(process.env.WORKFLOW_PLAYWRIGHT_MODULE);
const legacyWait=(page,fn,arg)=>page.waitForFunction(async({code,arg})=>(await import('/__test__/workflow-bridge.js')).run(code,arg),{code:fn.toString(),arg});
(async()=>{
 const server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
 let browser,release;
 try{
  browser=await chromium.launch({headless:true});const context=await browser.newContext({viewport:{width:1440,height:1000}});
  await context.addInitScript(()=>{window.EventSource=class{addEventListener(){}close(){}};window.setInterval=()=>0;});
  const page=await fixture.page(context,server);await fixture.open(page,server);
  const original=fixture.payload('zero');await fixture.mount(page,server,original);
  const started=Date.now(),falseResult=await legacyWait(page,()=>false);
  console.log(JSON.stringify({probe:'legacy always-false wait',result:await falseResult.jsonValue(),elapsedMs:Date.now()-started}));await falseResult.dispose();
  const oldCanvas=await page.locator('.workflow-timeline-canvas').elementHandle(),before=await oldCanvas.evaluate(el=>({end:+el.dataset.viewEnd,width:el.getBoundingClientRect().width}));
  const advanced=structuredClone(original),previous=advanced.workflow.axis.end,newEnd=new Date(Date.parse(previous)+60000).toISOString();
  advanced.workflow.axis.end=newEnd;advanced.workflow.axis.elapsed_ms+=60000;advanced.workflow.computed_at=newEnd;advanced.workflow.lifecycle.total_elapsed_ms+=60000;advanced.workflow.lifecycle.unattributed_elapsed_ms+=60000;advanced.workflow.measurement.unmeasured_ms+=60000;for(const gap of advanced.workflow.unmeasured_intervals)if(gap.end===previous)gap.end=newEnd;
  server.state.data=advanced;const source=JSON.stringify(advanced.workflow);
  let arrived;const requested=new Promise(resolve=>arrived=resolve),held=new Promise(resolve=>release=resolve);
  await page.route('**/api/activity?*',async route=>{arrived();await held;await route.continue();});
  await page.locator('#activityDetailRefresh').click();await requested;
  const broken=await legacyWait(page,()=>!activityDetailPending.size);
  const pending=await fixture.evaluate(page,()=>({pending:activityDetailPending.size,cacheEnd:activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow.axis.end}));
  const early=await page.locator('.workflow-timeline-canvas').evaluate(el=>({end:+el.dataset.viewEnd,width:el.getBoundingClientRect().width}));
  console.log(JSON.stringify({probe:'legacy refresh wait returns while response held',waitResult:await broken.jsonValue(),expectedEnd:Date.parse(newEnd),actual:early,...pending}));await broken.dispose();assert.equal(early.end,before.end);assert.equal(pending.pending,1);
  release();const complete=await fixture.waitFor(page,end=>!activityDetailPending.size&&activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow.axis.end===end,newEnd);await complete.dispose();
  const stale=await oldCanvas.evaluate(el=>({connected:el.isConnected,width:el.getBoundingClientRect().width})),current=await page.locator('.workflow-timeline-canvas').evaluate(el=>({end:+el.dataset.viewEnd,width:el.getBoundingClientRect().width}));
  console.log(JSON.stringify({probe:'native refresh replaces canvas; completed current sample',stale,current,expectedEnd:Date.parse(newEnd),expectedWidth:before.width}));
  assert.deepEqual(stale,{connected:false,width:0});assert.deepEqual(current,{end:Date.parse(newEnd),width:before.width});
  assert.equal(await fixture.evaluate(page,expected=>JSON.stringify(activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow)===expected,source),true);
  console.log('PASS: real native pending/cache/render predicates, exact width, advanced axis, and source bytes verified');await oldCanvas.dispose();await context.close();
 }finally{release?.();if(browser)await browser.close();await server.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
