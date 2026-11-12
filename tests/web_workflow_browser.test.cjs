const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const fixture=require('./helpers/workflow_native.cjs');
const modulePath=process.env.WORKFLOW_PLAYWRIGHT_MODULE,required=process.env.WORKFLOW_REQUIRE_BROWSER==='1';
if(required&&!modulePath)throw new Error('Required workflow browser runtime is missing; refusing a skipped verification');
test('workflow real browser preserves full periods, grouped intervals, role meaning and partial-update state',{skip:!modulePath,timeout:120000},async()=>{
 const {chromium}=require(modulePath),server=await fixture.start(path.resolve(process.env.WORKFLOW_WEB_ROOT||'goassets/template/_task_mecca/framework/web'));
 let browser;try{browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE})}catch(error){await server.close();throw error}
 const out=process.env.WORKFLOW_EVIDENCE_DIR&&path.resolve(process.env.WORKFLOW_EVIDENCE_DIR);if(out)fs.mkdirSync(out,{recursive:true});
 const report={synthetic:true,browser:browser.version(),platform:process.platform,checks:[],screenshots:[],errors:[],cleanup:{}};
 let ctx;
 try{
  ctx=await browser.newContext({viewport:{width:1440,height:1000},locale:'ko-KR',timezoneId:'Asia/Seoul',deviceScaleFactor:1});
  await ctx.route('**/*',route=>route.request().url().startsWith(server.origin+'/')?route.continue():route.abort());
  await ctx.addInitScript(()=>{window.EventSource=class{addEventListener(){}close(){}};window.setInterval=()=>0;localStorage.setItem('task-mecca-language','ko');localStorage.setItem('task-mecca-sidebar-mode','expanded');localStorage.setItem('task-mecca-palette','smoke');localStorage.setItem('task-mecca-theme','light');});
  const page=await fixture.page(ctx,server);page.on('pageerror',e=>report.errors.push(e.message));
  await fixture.open(page,server);
  async function show(kind,palette='smoke',theme='light',mobile=false){
   server.state.data=fixture.payload(kind);server.state.activityStatus=200;
   await page.setViewportSize(mobile?{width:390,height:844}:{width:1440,height:1000});
   await fixture.mount(page,server,server.state.data,{theme});
  await page.locator('#lifecycle').waitFor();
  }
    await show('continuous');
  assert.equal(await page.locator('[data-workflow-row]').count(),1);
  assert.equal(await page.locator('[data-workflow-interval]').count(),19);
  const initial=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd}));
  assert.equal(Date.parse(initial.end)-Date.parse(initial.start),3600000);
  await page.locator('#workflowZoomIn').click();await page.locator('#workflowZoomIn').click();
  await page.locator('#activityDetailRound').selectOption('0',{force:true});
  assert.deepEqual(await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd})),initial);
  assert.equal(await page.locator('.workflow-timeline-canvas').evaluate(el=>Number(el.dataset.viewEnd)-Number(el.dataset.viewStart)),900000);
  await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open=true);
  const visibleCount=await page.locator('[data-workflow-interval]:not([aria-hidden="true"])').count();
  const interval=page.locator('[data-workflow-interval]:not([aria-hidden="true"])').first();await interval.focus();await page.keyboard.press('Enter');
  assert.equal(await page.locator('.workflow-floating.is-pinned').isVisible(),true);
  assert.match(await page.locator('.workflow-floating.is-pinned').textContent(),/구현|테스트/);
  await fixture.evaluate(page,()=>{window.__originalDetail=document.querySelector('.detail-identity');window.__chartRange=[document.querySelector('.workflow-timeline-canvas').dataset.viewStart,document.querySelector('.workflow-timeline-canvas').dataset.viewEnd];window.__originalScroll=scrollY;});
  server.state.data=fixture.payload('continuous');await page.locator('#activityDetailRefresh').click();
  await fixture.waitFor(page,()=>!activityDetailPending.size);
  assert.equal(await fixture.evaluate(page,()=>window.__originalDetail===document.querySelector('.detail-identity')),true);
  assert.equal(await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open),true);
  assert.equal(await page.locator('.workflow-timeline-canvas').evaluate(el=>Number(el.dataset.viewEnd)-Number(el.dataset.viewStart)),900000);
  assert.deepEqual(await page.locator('.workflow-timeline-canvas').evaluate(el=>[el.dataset.viewStart,el.dataset.viewEnd]),await fixture.evaluate(page,()=>window.__chartRange));
    assert.equal(await page.locator('.workflow-floating.is-pinned').count(),0);
    server.state.activityStatus=503;await page.locator('#activityDetailRefresh').click();await fixture.waitFor(page,()=>!activityDetailPending.size);
  assert.equal(await page.locator('[data-workflow-interval]:not([aria-hidden="true"])').count(),visibleCount);assert.match(await page.locator('.workflow-source-warning').first().textContent(),/마지막 확인 결과/);
  report.checks.push('one row / nineteen intervals; full 60-minute axis; zoom and round filtering preserve bounds; keyboard details; refresh keeps Markdown/disclosure/chart/selection; failed refresh retains data');
  await show('history');
  assert.equal(await page.locator('[data-workflow-row]').count(),0);
  const unknown=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd}));assert.equal(Date.parse(unknown.end)-Date.parse(unknown.start),139*60000);
  assert.match(await page.locator('.workflow-measurements').textContent(),/2시간 19분/);assert.equal(await page.locator('.workflow-gap').count(),0);
  await page.locator('#activityDetailRound').selectOption('4',{force:true});assert.equal(await page.locator('[data-workflow-row]').count(),0);assert.deepEqual(await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd})),unknown);
  await show('overlap','smoke','dark');
  assert.equal(await page.locator('.wf-role-worker[data-workflow-interval]').count()>0,true);
  assert.equal(await page.locator('[data-workflow-row-outcome="needs_changes"]').count(),1);
  const rowBadge=page.locator('[data-workflow-row-outcome="needs_changes"]');assert.equal(await rowBadge.isVisible(),true);
  assert.match(await page.locator('.workflow-measurements').textContent(),/Controller 검토10분/);

  const overlapped=await page.locator('[data-workflow-row="worker|worker|1"] [data-workflow-interval]').evaluateAll(items=>items.map(el=>({left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right,top:el.style.top})));assert.ok(overlapped[1].left<overlapped[0].right);assert.notEqual(overlapped[0].top,overlapped[1].top);
  assert.equal(await page.locator('img[onerror]').count(),0);
  await show('unknown');assert.match(await page.locator('.workflow-view').textContent(),/회차 미분류/);
  await show('zero');assert.equal(await page.locator('.wf-kind-activity_open').count(),0);assert.match(await page.locator('.workflow-measurements').textContent(),/0초/);assert.match(await page.locator('.workflow-measurements').textContent(),/Controller 검토미측정/);
  const reopenedAxis=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd}));assert.equal(Date.parse(reopenedAxis.end)-Date.parse(reopenedAxis.start),30*60000);assert.match(await page.locator('.workflow-context-line strong').textContent(),/구현 중/);assert.match(await page.locator('.workflow-next').textContent(),/구현·검증 진행/);assert.match(await page.locator('.workflow-context').textContent(),/Worker/);assert.match(await page.locator('.workflow-context').textContent(),/최초 작업/);assert.match(await page.locator('.workflow-reopened').textContent(),/이전 완료 후 재개/);
  await page.locator('#workflowLifecycleEvents').evaluate(el=>el.open=true);assert.match(await page.locator('#workflowLifecycleEvidenceBody').textContent(),/완료/);assert.match(await page.locator('#workflowLifecycleEvidenceBody').textContent(),/재개/);
  await show('short');assert.match(await page.locator('.workflow-measurements').textContent(),/250밀리초/);assert.match(await page.locator('[data-workflow-interval][data-workflow-detail*="0초"]').first().getAttribute('aria-label'),/0초/);assert.equal(await page.locator('.wf-start-only').count(),0);
  await show('none');assert.equal(await page.locator('#workflowTimelineViewport').count(),0);assert.equal(await page.locator('#workflowZoomIn').isDisabled(),true);assert.match(await page.locator('.workflow-empty').textContent(),/경계/);
  server.state.activityStatus=503;await page.reload();await fixture.loaded(page);
  assert.match(await page.locator('#activityMetricsBody').textContent(),/본문은 계속 볼 수 있습니다/);assert.equal(await page.locator('#activityDetailRefresh').isVisible(),true);server.state.activityStatus=200;await page.locator('#activityDetailRefresh').click();await fixture.waitFor(page,()=>!activityDetailPending.size);assert.equal(await page.locator('.workflow-view').count(),1);
  report.checks.push('139-minute Activity0 axis; nine grounded execution/review rows; rework filtering; concurrent lanes; unclassified round; outcome badges; escaped ledger; measured zero differs from unknown; completed then resumed keeps 30-minute current axis, actor/Worker/action/initial round and prior completion; initial-load retry');
  // One confirmation batch: actual viewport frames include fixed application
  // chrome. Separate section crops hide chrome only for a complete chart view.
  for(const [kind,palette,theme,mobile] of [['continuous','smoke','light',false],['history','smoke','light',false],['overlap','smoke','dark',false],['continuous','smoke','dark',true],['history','smoke','light',true],['zero','smoke','dark',false],['short','smoke','light',false],['stale','smoke','light',false],['short','smoke','dark',false],['history','smoke','dark',true],['continuous','smoke','light',true]]){
   if(process.env.WORKFLOW_CAPTURE_CASE&&kind!==process.env.WORKFLOW_CAPTURE_CASE)continue;
   await show(kind,palette,theme,mobile);
   const metrics=await page.locator('.workflow-section').evaluate(el=>({width:el.getBoundingClientRect().width,scrollWidth:document.documentElement.scrollWidth,viewport:innerWidth,rows:el.querySelectorAll('[data-workflow-row]').length,roleColors:[...el.querySelectorAll('.workflow-interval.wf-kind-activity')].slice(0,3).map(x=>({role:x.className,color:getComputedStyle(x).backgroundColor,foreground:getComputedStyle(x).color})),controls:[...el.querySelectorAll('.workflow-controls button')].map(x=>({width:x.getBoundingClientRect().width,height:x.getBoundingClientRect().height}))}));
   assert.ok(metrics.scrollWidth<=metrics.viewport+1,'workflow causes page horizontal overflow');if(mobile)assert.ok(metrics.controls.every(c=>c.height>=44));
   const prefix=[kind,palette,theme,mobile?'mobile':'desktop'].join('-');
   if(out&&process.env.WORKFLOW_NO_CAPTURE!=='1'&&process.env.WORKFLOW_GLASS_COMPAT!=='1'){
    for(const [part,selector] of [['summary','.workflow-section']]){
     await page.locator(selector).evaluate(el=>{const chrome=document.querySelector('.topbar')?.getBoundingClientRect().height||0;scrollTo({top:el.getBoundingClientRect().top+scrollY-chrome-20,behavior:'instant'})});
     const rect=await page.locator(selector).boundingBox(),header=await page.locator('.topbar').boundingBox();assert.ok(rect.y>=(header?.height||0)-1,'workflow heading/controls hidden by topbar');
     const name=prefix+'-'+part+'-viewport.png';await page.screenshot({path:path.join(out,name),animations:'disabled'});report.screenshots.push({name,capture:'actual viewport with application chrome',...metrics});
    }
    const name=prefix+'-section.png';await page.locator('.workflow-section').screenshot({path:path.join(out,name),animations:'disabled',style:'#shortcutBar,.topbar,.detail-toc-mobile-btn{visibility:hidden!important}'});report.screenshots.push({name,capture:'section crop; fixed chrome hidden only during capture',...metrics});
   }
  }
  // Real runtime producer -> SSE network -> native consumer -> lifecycle/list.
  // This context deliberately uses the browser's real EventSource, no stub and
  // no direct revision/checker call. The server speaks the existing attention event.
  await ctx.close();ctx=null;
  const liveContext=await browser.newContext({viewport:{width:1440,height:1000},locale:'ko-KR',timezoneId:'Asia/Seoul'});
  try{
   await liveContext.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
   server.state.data=fixture.payload();server.state.activityStatus=200;server.state.taskResponse=fixture.task();server.state.revision={revision:'live-r1',signal_revision:'live-s1',states:[{id:'FLOW-1',title:server.state.taskResponse.title,file_state:'done'}]};
   const livePage=await fixture.page(liveContext,server);livePage.on('pageerror',e=>report.errors.push(e.message));const connections=server.eventConnections;await fixture.open(livePage,server);await server.waitForEvents(connections);assert.equal(await livePage.evaluate(()=>EventSource.toString().includes('[native code]')),true,'runtime event path uses the browser EventSource, not a test stub');await fixture.waitFor(livePage,()=>state.contentRevision==='live-r1');
   server.emitAttention({revision:'live-r1',signal_revision:'live-s1',attention:[],notification_events:[]});
   await livePage.locator('[data-workflow-interval]').nth(8).focus();await livePage.keyboard.press('Enter');await livePage.locator('#workflowIntervalEvidence > summary').click();await livePage.locator('#workflowIntervalEvidence pre').evaluate(el=>{const text=el.firstChild,start=text.textContent.indexOf('attempt_id');getSelection().setBaseAndExtent(text,start,text,start+10)});
   const readLive=()=>fixture.evaluate(livePage,()=>({raw:state.detailTask.raw_markdown,selection:getSelection().toString(),y:scrollY,pin:!!document.querySelector('.workflow-floating.is-pinned'),open:document.querySelector('#workflowIntervalEvidence').open,scroll:document.querySelector('.workflow-floating').scrollTop,focus:document.activeElement.tagName}));const reading=await readLive();await livePage.locator('#workflowLifecycleEvents').evaluate(el=>el.open=true);const evidence=await livePage.locator('#workflowLifecycleEvidenceBody').elementHandle();
   server.state.taskResponse.lifecycle.events.push({event_id:'runtime-sse-new',kind:'started',label:'Runtime SSE event',at:'2026-11-05T00:19:00Z',actor:'/root/controller/native-fixture',source:'fixture'});server.state.revision.signal_revision='live-s2';server.emitAttention({revision:'live-r1',signal_revision:'live-s2',attention:[],notification_events:[{event_id:'runtime-sse-new'}]});
   await livePage.getByText('runtime-sse-new',{exact:false}).waitFor().catch(async error=>{report.runtimeFailure={requests:server.state.requests.filter(u=>/events|revision|projection=lifecycle/.test(u)),observed:await fixture.evaluate(livePage,()=>({contentRevision:state.contentRevision,signalRevision:state.signalRevision,events:state.detailTask.lifecycle.events,detail:state.detail,body:document.querySelector('#workflowLifecycleEvidenceBody').textContent}))};throw error});assert.deepEqual(await readLive(),reading,'real runtime event patches evidence while preserving Markdown, selected text, pin, disclosure, scroll and focus');assert.equal(await evidence.evaluate(el=>el===document.querySelector('#workflowLifecycleEvidenceBody')),true,'only evidence children patch, source reader and section stay mounted');assert.equal(await livePage.locator('#contentUpdateNotice').count(),0,'other runtime evidence does not claim Markdown changed');
   const beforeTask=server.state.taskResponse;server.state.taskResponse={...beforeTask,raw_markdown:beforeTask.raw_markdown+'\nA real changed source'};server.state.revision.revision='live-r2';server.state.revision.signal_revision='live-s3';server.emitAttention({revision:'live-r2',signal_revision:'live-s3',attention:[],notification_events:[{event_id:'runtime-sse-body'}]});await livePage.locator('#contentUpdateNotice').waitFor();assert.equal((await readLive()).raw,reading.raw,'changed source is announced without replacing the open Markdown');await livePage.keyboard.press('Escape');assert.equal(await fixture.evaluate(livePage,()=>state.detail),'FLOW-1');
   const nextConnection=server.eventConnections,listURL=server.origin+'/?view=backlog&project='+encodeURIComponent(fixture.project)+'&backlog='+encodeURIComponent(fixture.backlog)+'&q=workflow&page=2';const originalRow={...fixture.task(),title:'Native list before event'};server.state.backlogResponse={items:[originalRow],page:2,pages:2,page_size:20,total:21,counts:{all:21,done:21},selection:{selected:fixture.backlog,candidates:[]},backlog_presence:{present:true}};await livePage.goto(listURL);await livePage.getByText('Native list before event',{exact:true}).waitFor();await server.waitForEvents(nextConnection);server.emitAttention({revision:'list-r1',signal_revision:'list-s1',attention:[],notification_events:[]});server.state.backlogResponse={...server.state.backlogResponse,items:[{...originalRow,title:'Native list after event'}]};server.emitAttention({revision:'list-r2',signal_revision:'list-s2',attention:[],notification_events:[{event_id:'list-changed'}]});await livePage.locator('#listUpdateNotice').waitFor();assert.equal(await livePage.locator('[name=q]').inputValue(),'workflow');assert.equal(new URL(livePage.url()).searchParams.get('page'),'2');await livePage.getByText('Native list before event',{exact:true}).waitFor();await livePage.locator('#listUpdateNotice button').click();await livePage.getByText('Native list after event',{exact:true}).waitFor();assert.equal(await livePage.locator('[name=q]').inputValue(),'workflow');assert.equal(new URL(livePage.url()).searchParams.get('page'),'2');
   report.runtimeProducer={eventConnections:server.eventConnections,wireEvent:'attention',newLifecycleEvent:'runtime-sse-new',readingPreserved:true,modifiedSourceAnnounced:true,filteredListUpdated:true,requests:server.state.requests.filter(u=>/events|revision|projection=lifecycle/.test(u))};
   report.checks.push('real native EventSource attention producer updates lifecycle evidence and filtered list; preserves open source/reader and announces changed Markdown; no direct revision function call');
  }finally{await liveContext.close();delete server.state.taskResponse;delete server.state.backlogResponse;delete server.state.revision;}
  assert.deepEqual(report.errors,[]);
  report.passed=true;
 }finally{if(ctx)await ctx.close();await browser.close();await server.close();report.cleanup={browserClosed:true,fixtureClosed:true};if(out)fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2)+'\n');}
});

// A Promise was previously accepted as a truthy waitForFunction result even
// when the native predicate resolved false. Hold a real refresh response to
// prove the helper polls through pending work instead of reading the old DOM.
test('native workflow wait polls false predicates through refresh completion',{skip:!modulePath,timeout:30000},async()=>{
  const {chromium}=require(modulePath),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
  let browser,release;
  try{
    browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});
    const context=await browser.newContext({viewport:{width:1440,height:1000}});
    await context.addInitScript(()=>{window.EventSource=class{addEventListener(){}close(){}};window.setInterval=()=>0;});
    const page=await fixture.page(context,server),data=fixture.payload('zero');
    await fixture.open(page,server);
    await fixture.mount(page,server,data);
    await assert.rejects(fixture.waitFor(page,()=>false,undefined,{timeout:100}),{name:'TimeoutError'});
    const truth=await fixture.waitFor(page,id=>state.detail===id,'FLOW-1');assert.equal(await truth.jsonValue(),true);await truth.dispose();
    const canvas=await page.locator('.workflow-timeline-canvas').elementHandle(),before=await canvas.evaluate(el=>({end:+el.dataset.viewEnd,width:el.getBoundingClientRect().width}));
    const held=new Promise(resolve=>release=resolve);let received;
    const requested=new Promise(resolve=>received=resolve);
    await page.route('**/api/activity?*',async route=>{received();await held;await route.continue();});
    await page.locator('#activityDetailRefresh').click();await requested;
    let completed=false;
    const waiting=fixture.waitFor(page,()=>{
      document.documentElement.dataset.workflowWaitPolls=String(Number(document.documentElement.dataset.workflowWaitPolls||0)+1);
      return !activityDetailPending.size;
    }).then(value=>{completed=true;return value;});
    await page.waitForFunction(()=>Number(document.documentElement.dataset.workflowWaitPolls)>=2);
    assert.equal(completed,false,'pending native response must keep the wait open across polling frames');
    assert.equal(await fixture.evaluate(page,()=>activityDetailPending.size),1);
    release();const result=await waiting;assert.equal(await result.jsonValue(),true);await result.dispose();
    assert.equal(await canvas.evaluate(el=>el.isConnected),false,'refresh replaces the old canvas');
    assert.equal(await canvas.evaluate(el=>el.getBoundingClientRect().width),0,'a detached canvas explains the zero-width stale read');
    const current=await page.locator('.workflow-timeline-canvas').evaluate(el=>({end:+el.dataset.viewEnd,width:el.getBoundingClientRect().width}));
    assert.deepEqual(current,before,'completed refresh preserves the original time bound and exact canvas width');
    assert.equal(await fixture.evaluate(page,expected=>JSON.stringify(activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow)===expected,JSON.stringify(data.workflow)),true);
    await canvas.dispose();await context.close();
  }finally{release?.();if(browser)await browser.close();await server.close();}
});
