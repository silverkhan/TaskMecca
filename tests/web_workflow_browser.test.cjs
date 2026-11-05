const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const fixture=require('./helpers/workflow_fixture.cjs');
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
  await ctx.addInitScript(()=>{window.EventSource=class{addEventListener(){}close(){}};window.setInterval=()=>0;localStorage.setItem('task-mecca-language','ko');localStorage.setItem('task-mecca-sidebar-mode','expanded');localStorage.setItem('task-mecca-palette','mecca');localStorage.setItem('task-mecca-theme','light');});
  await ctx.addInitScript(value=>{window.__workflowGlassCompat=value},process.env.WORKFLOW_GLASS_COMPAT==='1');
  const page=await ctx.newPage();page.on('pageerror',e=>report.errors.push(e.message));
  await page.goto(server.origin+'/?project='+encodeURIComponent(fixture.project));
  await page.waitForFunction(()=>typeof workflowMetricsBody==='function'&&typeof render==='function');
  async function show(kind,palette='mecca',theme='light',mobile=false){
   server.state.data=fixture.payload(kind);server.state.activityStatus=200;
   await page.setViewportSize(mobile?{width:390,height:844}:{width:1440,height:1000});
   await page.evaluate(({data,task,project,backlog,palette,theme})=>{
    Object.assign(state,{project,backlog,view:'backlog',detail:task.id,detailTask:task,raw:false,loadError:'',language:'ko',sidebarMode:'expanded',snapshot:{all_items:{[task.id]:task},counts:{all:1,done:1},access:{status:'full',full_access_confirmed:true},workload:{agents:[]},attention:[]},listData:{items:[task],counts:{all:1,done:1}},hub:{projects:[{project,name:'Workflow fixture',status:'active'}]}});
      activityDetailViews.clear();activityDetailCache.set(activityDetailKey(task.id,project),{...data,_client_fetched_at:'2026-11-05T03:00:00Z'});applyPalette(window.__workflowGlassCompat?'glass':palette);if(window.__workflowGlassCompat)applyBackground('cozy');applyTheme(theme);render();
   },{data:server.state.data,task:fixture.task(server.state.data),project:fixture.project,backlog:fixture.backlog,palette,theme});
   await page.locator('#lifecycle').waitFor();
  }
    const checkGlass=async phase=>{
      if(process.env.WORKFLOW_GLASS_COMPAT!=='1')return;
      await page.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().mode==='active'&&!window.TaskMeccaGlass.diagnostics().loading);
      const observed=await page.evaluate(()=>({palette:document.documentElement.dataset.palette,background:document.documentElement.dataset.background,diagnostics:window.TaskMeccaGlass.diagnostics()}));
      assert.equal(observed.palette,'glass');assert.equal(observed.background,'cozy');assert.equal(observed.diagnostics.mode,'active');assert.ok(observed.diagnostics.paintCount>0);assert.ok(observed.diagnostics.materials.length+observed.diagnostics.lenses.length>0);
      (report.glassCompatibility??=[]).push({phase,...observed});
    };
    await show('continuous');await checkGlass('workflow rows before interaction');
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
  assert.equal(await page.locator('#activityTimelineSelection').isVisible(),true);
  assert.match(await page.locator('#activityTimelineSelection').textContent(),/実行|실행:/);
  await page.evaluate(()=>{window.__originalDetail=document.querySelector('.detail-head');window.__chartRange=[document.querySelector('.workflow-timeline-canvas').dataset.viewStart,document.querySelector('.workflow-timeline-canvas').dataset.viewEnd];window.__originalScroll=scrollY;});
  server.state.data=fixture.payload('continuous');await page.locator('#activityDetailRefresh').click();
  await page.waitForFunction(()=>!activityDetailPending.size);
  assert.equal(await page.evaluate(()=>window.__originalDetail===document.querySelector('.detail-head')),true);
  assert.equal(await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open),true);
  assert.equal(await page.locator('.workflow-timeline-canvas').evaluate(el=>Number(el.dataset.viewEnd)-Number(el.dataset.viewStart)),900000);
  assert.deepEqual(await page.locator('.workflow-timeline-canvas').evaluate(el=>[el.dataset.viewStart,el.dataset.viewEnd]),await page.evaluate(()=>window.__chartRange));
    assert.equal(await page.locator('#activityTimelineSelection').isVisible(),true);await checkGlass('row keyboard selection and partial refresh preserved');
    server.state.activityStatus=503;await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);
  assert.equal(await page.locator('[data-workflow-interval]:not([aria-hidden="true"])').count(),visibleCount);assert.match(await page.locator('.workflow-source-warning').first().textContent(),/마지막 확인 결과/);
  report.checks.push('one row / nineteen intervals; full 60-minute axis; zoom and round filtering preserve bounds; keyboard details; refresh keeps Markdown/disclosure/chart/selection; failed refresh retains data');
  await show('history');
  assert.equal(await page.locator('[data-workflow-row]').count(),9);
  const unknown=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd}));assert.equal(Date.parse(unknown.end)-Date.parse(unknown.start),139*60000);
  assert.match(await page.locator('.workflow-measurements').textContent(),/2시간 19분/);assert.equal(await page.locator('.workflow-gap').count(),1);
  await page.locator('#activityDetailRound').selectOption('4',{force:true});assert.equal(await page.locator('[data-workflow-row]').count(),1);assert.deepEqual(await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd})),unknown);
  await show('overlap','slate','dark');
  assert.equal(await page.locator('.wf-role-worker[data-workflow-interval]').count()>0,true);
  assert.equal(await page.locator('[data-workflow-row-outcome="needs_changes"]').count(),1);
  const rowBadge=page.locator('[data-workflow-row-outcome="needs_changes"]');assert.equal(await rowBadge.isVisible(),true);
  assert.match(await page.locator('.workflow-measurements').textContent(),/Controller 검토10분/);

  const overlapped=await page.locator('[data-workflow-row="worker|worker|1"] [data-workflow-interval]').evaluateAll(items=>items.map(el=>({left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right,top:el.style.top})));assert.ok(overlapped[1].left<overlapped[0].right);assert.notEqual(overlapped[0].top,overlapped[1].top);
  assert.equal(await page.locator('img[onerror]').count(),0);
  await show('unknown');assert.match(await page.locator('.workflow-view').textContent(),/회차 미분류/);
  await show('zero');assert.equal(await page.locator('.wf-kind-activity_open').count(),0);assert.match(await page.locator('.workflow-measurements').textContent(),/0초/);assert.match(await page.locator('.workflow-measurements').textContent(),/Controller 검토미측정/);
  const reopenedAxis=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:el.dataset.axisStart,end:el.dataset.axisEnd}));assert.equal(Date.parse(reopenedAxis.end)-Date.parse(reopenedAxis.start),30*60000);assert.match(await page.locator('.workflow-context-line strong').textContent(),/구현 중/);assert.match(await page.locator('.workflow-next').textContent(),/구현·검증 진행/);assert.match(await page.locator('.workflow-context').textContent(),/Worker.*w/);assert.match(await page.locator('.workflow-context').textContent(),/최초 작업/);assert.match(await page.locator('.workflow-reopened').textContent(),/이전 완료 후 재개/);
  await page.locator('#workflowLifecycleEvents').evaluate(el=>el.open=true);assert.match(await page.locator('#workflowLifecycleEvidenceBody').textContent(),/완료/);assert.match(await page.locator('#workflowLifecycleEvidenceBody').textContent(),/재개/);
  await show('short');assert.match(await page.locator('.workflow-measurements').textContent(),/250밀리초/);assert.match(await page.locator('[data-workflow-interval][data-workflow-detail*="0초"]').first().getAttribute('aria-label'),/0초/);assert.equal(await page.locator('.wf-start-only').count(),1);
  await show('none');assert.equal(await page.locator('#workflowTimelineViewport').count(),0);assert.equal(await page.locator('#workflowZoomIn').isDisabled(),true);assert.match(await page.locator('.workflow-empty').textContent(),/경계/);
  await page.evaluate(()=>{activityDetailCache.set(activityDetailKey('FLOW-1',state.project),{error:'synthetic first-load HTTP 503'});renderActivityDetailControls('FLOW-1',state.project)});
  assert.match(await page.locator('#activityMetricsBody').textContent(),/본문은 계속 볼 수 있습니다/);assert.equal(await page.locator('#activityDetailRefresh').isVisible(),true);await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);assert.equal(await page.locator('.workflow-view').count(),1);
  report.checks.push('139-minute Activity0 axis; nine grounded execution/review rows; rework filtering; concurrent lanes; unclassified round; outcome badges; escaped ledger; measured zero differs from unknown; completed then resumed keeps 30-minute current axis, actor/Worker/action/initial round and prior completion; initial-load retry');
  // One confirmation batch: actual viewport frames include fixed application
  // chrome. Separate section crops hide chrome only for a complete chart view.
  for(const [kind,palette,theme,mobile] of [['continuous','mecca','light',false],['history','mecca','light',false],['overlap','slate','dark',false],['continuous','mecca','dark',true],['history','slate','light',true],['zero','slate','dark',false],['short','mecca','light',false],['stale','slate','light',false],['short','mecca','dark',false],['history','slate','dark',true],['continuous','mecca','light',true]]){
   if(process.env.WORKFLOW_CAPTURE_CASE&&kind!==process.env.WORKFLOW_CAPTURE_CASE)continue;
   await show(kind,palette,theme,mobile);
   const metrics=await page.locator('.workflow-section').evaluate(el=>({width:el.getBoundingClientRect().width,scrollWidth:document.documentElement.scrollWidth,viewport:innerWidth,rows:el.querySelectorAll('[data-workflow-row]').length,roleColors:[...el.querySelectorAll('.workflow-interval.wf-kind-activity')].slice(0,3).map(x=>({role:x.className,color:getComputedStyle(x).backgroundColor,foreground:getComputedStyle(x).color})),controls:[...el.querySelectorAll('.workflow-controls button')].map(x=>({width:x.getBoundingClientRect().width,height:x.getBoundingClientRect().height}))}));
   assert.ok(metrics.scrollWidth<=metrics.viewport+1,'workflow causes page horizontal overflow');if(mobile)assert.ok(metrics.controls.every(c=>c.height>=44));
   const prefix=[kind,palette,theme,mobile?'mobile':'desktop'].join('-');
   if(out&&process.env.WORKFLOW_GLASS_COMPAT!=='1'){
    for(const [part,selector] of [['summary','.workflow-section']]){
     await page.locator(selector).evaluate(el=>{const chrome=document.querySelector('.topbar')?.getBoundingClientRect().height||0;scrollTo({top:el.getBoundingClientRect().top+scrollY-chrome-20,behavior:'instant'})});
     const rect=await page.locator(selector).boundingBox(),header=await page.locator('.topbar').boundingBox();assert.ok(rect.y>=(header?.height||0)-1,'workflow heading/controls hidden by topbar');
     const name=prefix+'-'+part+'-viewport.png';await page.screenshot({path:path.join(out,name),animations:'disabled'});report.screenshots.push({name,capture:'actual viewport with application chrome',...metrics});
    }
    const name=prefix+'-section.png';await page.locator('.workflow-section').screenshot({path:path.join(out,name),animations:'disabled',style:'#shortcutBar,.topbar,.detail-toc-mobile-btn{visibility:hidden!important}'});report.screenshots.push({name,capture:'section crop; fixed chrome hidden only during capture',...metrics});
   }
  }
  assert.deepEqual(report.errors,[]);
  report.passed=true;
 }finally{if(ctx)await ctx.close();await browser.close();await server.close();report.cleanup={browserClosed:true,fixtureClosed:true};if(out)fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2)+'\n');}
});
