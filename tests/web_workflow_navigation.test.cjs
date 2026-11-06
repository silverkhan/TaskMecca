const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const fixture=require('./helpers/workflow_fixture.cjs');
const {zoomActivityRange}=require('./helpers/workflow_input.cjs');
const modulePath=process.env.WORKFLOW_PLAYWRIGHT_MODULE;
if(process.env.WORKFLOW_REQUIRE_BROWSER==='1'&&!modulePath)throw new Error('Required workflow browser runtime missing');
test('representative workflow has compact context and plot-coordinate anchored time navigation',{skip:!modulePath,timeout:120000},async()=>{
  const {chromium}=require(modulePath),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
  let browser;
  const out=path.resolve(process.env.WORKFLOW_EVIDENCE_DIR||'handoff-evidence/a59-lifecycle-simple-zoom-20261105/milestone');fs.mkdirSync(out,{recursive:true});
  const report={synthetic:true,base:'6c6c7563ac115ff372f52dec002930fc9452af53',cases:[],errors:[],cleanup:{}};
  try{
    browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});report.browser=browser.version();
    for(const mobile of [false,true]){
      const context=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:1000},locale:'ko-KR',timezoneId:'Asia/Seoul',hasTouch:mobile,isMobile:mobile,deviceScaleFactor:1});
      await context.route('**/*',route=>route.request().url().startsWith(server.origin+'/')?route.continue():route.abort());
      await context.addInitScript(()=>{window.EventSource=class{addEventListener(){}close(){}};window.setInterval=()=>0;localStorage.setItem('task-mecca-language','ko');localStorage.setItem('task-mecca-sidebar-mode','expanded');});
      const page=await context.newPage();page.on('pageerror',error=>report.errors.push(error.message));
      await page.goto(server.origin+'/?project='+encodeURIComponent(fixture.project));await page.waitForFunction(()=>typeof state!=='undefined');
      server.state.data=fixture.payload();server.state.activityStatus=200;
      await page.evaluate(({data,task,project,backlog,mobile})=>{
        Object.assign(state,{project,backlog,view:'backlog',detail:task.id,detailTask:task,raw:false,loadError:'',language:'ko',sidebarMode:'expanded',snapshot:{all_items:{[task.id]:task},counts:{all:1,done:1},access:{status:'full',full_access_confirmed:true},workload:{agents:[]},attention:[]},listData:{items:[task],counts:{all:1,done:1}},hub:{projects:[{project,name:'Workflow fixture'}]}});
        activityDetailViews.clear();activityDetailCache.set(activityDetailKey(task.id,project),{...data,_client_fetched_at:'2026-11-05T03:00:00Z'});applyPalette('mecca');applyTheme(mobile?'dark':'light');render();
      },{data:server.state.data,task:fixture.task(server.state.data),project:fixture.project,backlog:fixture.backlog,mobile});
      const chart=page.locator('#workflowTimelineViewport'),canvas=page.locator('.workflow-timeline-canvas');
      await chart.waitFor();
      const settled=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      const windowRange=()=>canvas.evaluate(el=>({start:Number(el.dataset.viewStart),end:Number(el.dataset.viewEnd),fullStart:el.dataset.axisStart,fullEnd:el.dataset.axisEnd,width:el.getBoundingClientRect().width}));
      const projectedPosition=()=>canvas.evaluate(el=>{
        const start=Number(el.dataset.viewStart),end=Number(el.dataset.viewEnd),plot=el.querySelector('.workflow-axis-track').getBoundingClientRect(),button=[...el.querySelectorAll('[data-workflow-interval]')].find(b=>Number(b.dataset.workflowStart)>=start&&Number(b.dataset.workflowEnd)<=end&&b.getAttribute('aria-hidden')!=='true');
        if(!button)throw new Error('No complete visible interval for geometry verification');
        const expected=plot.left+(Number(button.dataset.workflowStart)-start)/(end-start)*plot.width,actual=button.getBoundingClientRect().left;return {expected,actual,error:Math.abs(expected-actual)};
      });
      const scrollSection=()=>page.locator('.workflow-section').evaluate(el=>{const header=document.querySelector('.topbar')?.getBoundingClientRect().height||0;scrollTo({top:el.getBoundingClientRect().top+scrollY-header-18,behavior:'instant'});});
      const sourceBefore=await page.evaluate(()=>JSON.stringify(activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow));
      const initial=await windowRange();assert.equal(initial.end-initial.start,3600000);assert.equal(await page.locator('[data-workflow-row]').count(),1);assert.equal(await page.locator('[data-workflow-interval]').count(),19);
      assert.equal(await page.locator('#activityDetailZoom').count(),0);assert.equal(await page.locator('#activityDetailFocus').count(),0);
      if(process.env.WORKFLOW_CORE_ONLY==='1'){
        const geometries=[];
        const tickGeometry=async label=>{
          const geometry=await page.locator('.workflow-axis-track').evaluate(el=>({plot:{left:el.getBoundingClientRect().left,right:el.getBoundingClientRect().right,width:el.getBoundingClientRect().width},ticks:[...el.children].map(t=>({position:Number(t.dataset.workflowTick),text:t.textContent,visible:getComputedStyle(t).visibility==='visible',left:t.getBoundingClientRect().left,right:t.getBoundingClientRect().right}))}));
          const visible=geometry.ticks.filter(t=>t.visible),first=geometry.ticks[0],last=geometry.ticks.at(-1);
          assert.equal(first.visible,true);assert.equal(last.visible,true);assert.ok(Math.abs(first.left-geometry.plot.left)<1);assert.ok(Math.abs(last.right-geometry.plot.right)<1);
          for(const t of visible){assert.ok(t.left>=geometry.plot.left-1&&t.right<=geometry.plot.right+1,'visible tick stays inside actual plot');if(t.position>0&&t.position<1)assert.ok(Math.abs((t.left+t.right)/2-(geometry.plot.left+geometry.plot.width*t.position))<1,'intermediate tick centers on its time coordinate');}
          for(let i=1;i<visible.length;i++)assert.ok(visible[i].left>=visible[i-1].right+7,'visible labels do not overlap');
          geometries.push({label,...geometry});return geometry;
        };
        await scrollSection();await settled();const prefix=mobile?'mobile390':'desktop1440';
        await tickGeometry(prefix+' whole');if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-whole.png'),animations:'disabled'});
        const interval=page.locator('[data-workflow-interval]').nth(8);await interval.focus();await page.keyboard.press('Enter');await zoomActivityRange(page);
        const precise=await windowRange();assert.ok(Math.abs(precise.end-precise.start-72000)<2);assert.ok(Math.abs(precise.start-initial.start-1074000)<2);assert.ok(Math.abs(precise.end-initial.start-1146000)<2);
        const label=await page.locator('#workflowVisibleRange').textContent();assert.match(label,/09:17:5[34]/);assert.match(label,/09:19:0[56]/);await tickGeometry(prefix+' 72seconds');
        await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open=true);await chart.focus();await page.keyboard.press('ArrowLeft');
        const afterLeft=await windowRange();assert.ok(Math.abs(afterLeft.start-(precise.start-14400))<2);
        assert.equal(await page.evaluate(()=>state.detail),'FLOW-1');assert.equal(await chart.count(),1);assert.equal(await page.locator('.workflow-floating.is-pinned').count(),0);assert.equal(await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open),true);
        await scrollSection();await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open=false);await settled();if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-after-arrowleft.png'),animations:'disabled'});
        await zoomActivityRange(page);await scrollSection();await settled();if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-72seconds.png'),animations:'disabled'});
        const keys=[];
        for(const key of ['ArrowRight','ArrowLeft','+','-','Home','+','0']){await chart.focus();await page.keyboard.press(key);assert.equal(await page.evaluate(()=>state.detail),'FLOW-1',key+' must not close task detail');keys.push({key,range:await windowRange()});}
        await zoomActivityRange(page);const descendantKeys=[];
        for(const key of ['ArrowLeft','ArrowRight','+','-','Home','+','0']){await page.locator('[data-workflow-interval]:not([aria-hidden="true"])').first().focus();await page.keyboard.press(key);assert.equal(await page.evaluate(()=>state.detail),'FLOW-1','focused interval '+key+' must not close task detail');descendantKeys.push({key,range:await windowRange(),focus:await page.evaluate(()=>document.activeElement?.getAttribute('data-workflow-interval')||document.activeElement?.id)});}
        await page.locator('#workflowFit').click();await page.locator('[data-workflow-interval]').nth(4).focus();await page.keyboard.press('Enter');assert.match(await page.locator('.workflow-floating.is-pinned').textContent(),/9:14:00/);
        await page.locator('[data-workflow-interval]').nth(8).focus();await page.keyboard.press('Space');assert.match(await page.locator('.workflow-floating.is-pinned').textContent(),/9:18:00/);
        // Fractional seconds use the same formatter for visible endpoints and ticks.
        await zoomActivityRange(page);await chart.focus();for(let i=0;i<8;i++)await page.keyboard.press('+');
        const subsecond=await windowRange(),subsecondLabel=await page.locator('#workflowVisibleRange').textContent();assert.ok(subsecond.end-subsecond.start<1000);assert.match(subsecondLabel,/\d{2}:\d{2}:\d{2}\.\d{3}/);await tickGeometry(prefix+' subsecond');
        if(mobile){await page.setViewportSize({width:430,height:844});await page.locator('#workflowFit').click();await scrollSection();await settled();await tickGeometry('mobile430 whole');if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,'mobile430-whole.png'),animations:'disabled'});await zoomActivityRange(page);await settled();await tickGeometry('mobile430 72seconds');assert.match(await page.locator('#workflowVisibleRange').textContent(),/09:17:5[34]/);if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,'mobile430-72seconds.png'),animations:'disabled'});}
        await chart.focus();await page.evaluate(()=>{window.__coreWheel=[];document.querySelector('#workflowTimelineViewport').addEventListener('wheel',e=>window.__coreWheel.push(e.defaultPrevented),{passive:true});});
        const rect=await page.locator('.workflow-axis-track').boundingBox();await page.mouse.move(rect.x+rect.width*.5,rect.y+rect.height/2);await page.mouse.wheel(0,15);await page.keyboard.down('Control');await page.mouse.wheel(0,-15);await page.keyboard.up('Control');await settled();assert.deepEqual(await page.evaluate(()=>window.__coreWheel),[false,true]);
        assert.equal(await page.evaluate(()=>JSON.stringify(activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow)),sourceBefore);
        await page.evaluate(()=>{document.activeElement.blur();});await page.keyboard.press('ArrowLeft');assert.equal(await page.evaluate(()=>state.detail),null,'existing ArrowLeft outside chart still closes detail');
        report.cases.push({mobile,precise,label,afterLeft,keys,descendantKeys,nativeEnterSpaceSelection:true,tickGeometries:geometries,subsecond,subsecondLabel,detailRetained:true,selectionRetained:true,sourceUnchanged:true,outsideShortcutUnchanged:true,verticalUnpreventedPlotCtrlWheelHandled:true});await context.close();continue;
      }
      await scrollSection();await settled();const prefix=mobile?'mobile-dark':'desktop-light';
      if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-overview.png'),animations:'disabled'});
      if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.locator('.workflow-section').screenshot({path:path.join(out,prefix+'-section.png'),animations:'disabled',style:'#shortcutBar,.topbar,.detail-toc-mobile-btn{visibility:hidden!important}'});
      // Buttons preserve the visible center. Keyboard uses the same time model.
      await page.locator('#workflowZoomIn').click();let zoomed=await windowRange();assert.equal(zoomed.end-zoomed.start,1800000);assert.equal((zoomed.start+zoomed.end)/2,(initial.start+initial.end)/2);
      await chart.focus();await page.keyboard.press('ArrowRight');let panned=await windowRange();assert.equal(panned.start,zoomed.start+360000);
      await page.keyboard.press('Home');assert.deepEqual(await windowRange(),initial);
      const plot=await page.locator('.workflow-axis-track').boundingBox(),row=await page.locator('.workflow-agent-track').first().boundingBox();
      const x=plot.x+plot.width*.7,y=row.y+row.height/2,anchor=initial.start+(initial.end-initial.start)*.7;
      await page.mouse.dblclick(x,y);zoomed=await windowRange();const ratio=(anchor-zoomed.start)/(zoomed.end-zoomed.start);
      assert.ok(Math.abs(ratio-.7)*plot.width<1,'pointer anchor uses plot width, excluding Agent label column');
      assert.equal(zoomed.width,initial.width,'zoom does not expand DOM width');
      // A label-column pointer cannot move the view.
      await page.mouse.dblclick(plot.x-35,y);assert.deepEqual(await windowRange(),zoomed);
      await page.locator('#workflowFit').click();
      // Direct Shift-drag zooms the selected time range, rather than scaling CSS width.
      await page.keyboard.down('Shift');await page.mouse.move(plot.x+plot.width*.25,y);await page.mouse.down();await page.mouse.move(plot.x+plot.width*.5,y,{steps:8});await page.mouse.up();await page.keyboard.up('Shift');await settled();
      const selectedRange=await windowRange();assert.ok(Math.abs((selectedRange.end-selectedRange.start)-900000)<1000);assert.ok(Math.abs(selectedRange.start-(initial.start+900000))<1000);
      if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-range-zoom.png'),animations:'disabled'});
      // Native horizontal wheel pans. Ordinary vertical wheel stays native; plot Ctrl-wheel is explicitly chart zoom.
      await page.mouse.move(plot.x+plot.width*.5,y);await page.mouse.wheel(40,0);await settled();const wheel=await windowRange();assert.ok(wheel.start>selectedRange.start);
      const wheelGeometry=await projectedPosition();assert.ok(wheelGeometry.error<1.5,'rendered interval follows actual time coordinates after buffered pan');
      await page.evaluate(()=>{window.__workflowWheels=[];document.querySelector('#workflowTimelineViewport').addEventListener('wheel',e=>window.__workflowWheels.push({ctrl:e.ctrlKey,prevented:e.defaultPrevented}),{passive:true});});
      await page.mouse.wheel(0,12);await page.keyboard.down('Control');await page.mouse.wheel(0,-12);await page.keyboard.up('Control');await settled();
      assert.deepEqual(await page.evaluate(()=>window.__workflowWheels.map(e=>e.prevented)),[false,true]);
      await scrollSection();await settled();const afterScrollPlot=await page.locator('.workflow-axis-track').boundingBox(),afterScrollRow=await page.locator('.workflow-agent-track').first().boundingBox();
      let touch=null;
      if(mobile){
        const session=await context.newCDPSession(page),tx=afterScrollPlot.x+afterScrollPlot.width*.7,ty=afterScrollRow.y+afterScrollRow.height/2,before=await windowRange();
        await session.send('Input.dispatchTouchEvent',{type:'touchStart',touchPoints:[{x:tx,y:ty}]});
        for(let i=1;i<=8;i++)await session.send('Input.dispatchTouchEvent',{type:'touchMove',touchPoints:[{x:tx-i*5,y:ty}]});
        await session.send('Input.dispatchTouchEvent',{type:'touchEnd',touchPoints:[]});await settled();touch=await windowRange();assert.ok(touch.start>before.start,'actual emulated touch pans horizontally');assert.ok((await projectedPosition()).error<1.5,'rendered touch pan keeps interval coordinates');await session.detach();
      }else{
        const before=await windowRange();await page.mouse.move(afterScrollPlot.x+afterScrollPlot.width*.6,afterScrollRow.y+afterScrollRow.height/2);await page.mouse.down();await page.mouse.move(afterScrollPlot.x+afterScrollPlot.width*.4,afterScrollRow.y+afterScrollRow.height/2,{steps:8});await page.mouse.up();await settled();assert.ok((await windowRange()).start>before.start,'mouse drag pans the plot');
      }
      // Interval selection provides a precise zoom target and exact evidence stays folded.
      await page.locator('#workflowFit').click();const interval=page.locator('[data-workflow-interval]').nth(8);await interval.focus();await page.keyboard.press('Enter');assert.equal(await page.locator('.workflow-floating.is-pinned').isVisible(),true);
      await zoomActivityRange(page);const precise=await windowRange();assert.ok(precise.end-precise.start<120000);assert.ok((initial.end-initial.start)/(precise.end-precise.start)>8);
      await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open=true);const beforeRefresh=await windowRange();
      await page.evaluate(()=>window.__workflowDetail=document.querySelector('.detail-head'));await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);
      assert.deepEqual(await windowRange(),beforeRefresh);assert.equal(await page.evaluate(()=>window.__workflowDetail===document.querySelector('.detail-head')),true);assert.equal(await page.locator('#workflowEvidenceDetails').evaluate(el=>el.open),true);assert.equal(await page.locator('.workflow-floating.is-pinned').count(),0);
      const beforeResize=await windowRange();await page.setViewportSize(mobile?{width:430,height:900}:{width:1280,height:900});await settled();const resized=await windowRange();assert.equal(resized.start,beforeResize.start);assert.equal(resized.end,beforeResize.end);
      assert.equal(await page.evaluate(()=>JSON.stringify(activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow)),sourceBefore,'original timestamps, gaps, overlap and elapsed calculations stay unchanged');
      assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'no page overflow');
      // An out-of-order unmeasured report does not change measured totals. Its
      // insertion before the selected activity must not change that activity's identity.
      await page.locator('[data-workflow-interval]:not([aria-hidden="true"])').first().focus();await page.keyboard.press('Enter');
      const selectedKey=await page.locator('.workflow-interval.is-selected').getAttribute('data-workflow-interval'),selectedText=await page.locator('.workflow-floating.is-pinned').textContent();
      const inserted=structuredClone(server.state.data);inserted.workflow.rows[0].intervals.unshift({id:'synthetic-late-arriving-report',kind:'execution_report',type:'unclassified_execution',start:'2026-11-05T00:01:00Z',measured:false,attempt_id:'late-report',end_evidence:'unobserved',lane:0});
      server.state.data=inserted;await page.locator('#workflowIntervalEvidence').evaluate(el=>el.open=true);await page.locator('.workflow-interval.is-selected').focus();
      await page.evaluate(()=>loadTaskActivity('FLOW-1',state.project));await page.waitForFunction(()=>!activityDetailPending.size);
      assert.equal(await page.locator('.workflow-interval.is-selected').getAttribute('data-workflow-interval'),selectedKey);assert.equal(await page.locator('.workflow-floating.is-pinned').textContent(),selectedText);assert.equal(await page.evaluate(()=>document.activeElement.dataset.workflowInterval),selectedKey);assert.equal(await page.locator('#workflowIntervalEvidence').evaluate(el=>el.open),false);
      // Existing round0 can be filtered and re-entered without changing the time window.
      const beforeFilter=await windowRange();await page.locator('#activityDetailRound').selectOption('0',{force:true});assert.deepEqual(await windowRange(),beforeFilter);assert.equal(await page.locator('.workflow-interval.is-selected').count(),1);
      server.state.activityStatus=503;await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);assert.deepEqual(await windowRange(),beforeFilter);assert.equal(await page.locator('.workflow-floating.is-pinned').count(),0);assert.match(await page.locator('.workflow-source-warning').first().textContent(),/마지막 확인/);
      server.state.activityStatus=200;server.state.data.workflow.control_snapshot.trust='stale';await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);assert.deepEqual(await windowRange(),beforeFilter);assert.match(await page.locator('.workflow-source-note').textContent(),/오래/);
      await page.locator('#workflowFit').click();await page.locator('#workflowZoomIn').click();await chart.focus();for(let i=0;i<12;i++)await page.keyboard.press('ArrowLeft');assert.equal((await windowRange()).start,initial.start);for(let i=0;i<24;i++)await page.keyboard.press('ArrowRight');assert.equal((await windowRange()).end,initial.end);
      // An open lifecycle can receive a newer current observation. Keeping a
      // view is the default; only the explicit Fit action follows the new bound.
      const open=fixture.payload('zero');server.state.data=open;
      await page.evaluate(({data,task})=>{activityDetailViews.clear();state.detailTask=task;activityDetailCache.set(activityDetailKey('FLOW-1',state.project),data);renderActivityDetailControls('FLOW-1',state.project);},{data:open,task:fixture.task(open)});
      const openBefore=await windowRange(),advanced=structuredClone(open),oldEnd=advanced.workflow.axis.end,newEnd=new Date(Date.parse(oldEnd)+60000).toISOString();
      advanced.workflow.axis.end=newEnd;advanced.workflow.axis.elapsed_ms+=60000;advanced.workflow.computed_at=newEnd;advanced.workflow.lifecycle.total_elapsed_ms+=60000;advanced.workflow.lifecycle.unattributed_elapsed_ms+=60000;advanced.workflow.measurement.unmeasured_ms+=60000;for(const gap of advanced.workflow.unmeasured_intervals)if(Date.parse(gap.end)===Date.parse(oldEnd))gap.end=newEnd;
      server.state.data=advanced;await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);let stayed=await windowRange();assert.equal(stayed.start,openBefore.start);assert.equal(stayed.end,openBefore.end);await page.locator('#workflowFit').click();assert.equal((await windowRange()).end,Date.parse(newEnd));
      const long=structuredClone(advanced),longEnd=new Date(Date.parse(long.workflow.axis.start)+3*86400000).toISOString(),delta=Date.parse(longEnd)-Date.parse(long.workflow.axis.end),priorEnd=long.workflow.axis.end;
      long.workflow.axis.end=longEnd;long.workflow.computed_at=longEnd;long.workflow.axis.elapsed_ms+=delta;long.workflow.lifecycle.total_elapsed_ms+=delta;long.workflow.lifecycle.unattributed_elapsed_ms+=delta;long.workflow.measurement.unmeasured_ms+=delta;for(const gap of long.workflow.unmeasured_intervals)if(Date.parse(gap.end)===Date.parse(priorEnd))gap.end=longEnd;
      server.state.data=long;await page.locator('#activityDetailRefresh').click();await page.waitForFunction(()=>!activityDetailPending.size);
      await page.evaluate(()=>{const wf=activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow,p=activityDetailPrefs('FLOW-1');workflowSetWindow(wf,p,Date.parse('2026-11-05T14:59:00Z'),Date.parse('2026-11-05T15:01:00Z'));renderActivityDetailControls('FLOW-1');});
      const midnight=await page.locator('#workflowVisibleRange').textContent();assert.match(midnight,/11\. 05\..*오후 11:59:00.*11\. 06\..*오전 12:01:00/);
      await page.evaluate(()=>{const wf=activityDetailCache.get(activityDetailKey('FLOW-1',state.project)).workflow,p=activityDetailPrefs('FLOW-1');workflowSetWindow(wf,p,Date.parse('2026-11-07T00:00:00Z'),Date.parse('2026-11-07T00:00:45Z'));renderActivityDetailControls('FLOW-1');});
      const laterDay=await page.locator('#workflowVisibleRange').textContent();assert.match(laterDay,/11\. 07\..*09:00:00.*09:00:45/);await page.locator('#workflowZoomIn').click();assert.match(await page.locator('#workflowVisibleRange').textContent(),/11\. 07\./);
      await scrollSection();await settled();if(process.env.WORKFLOW_NO_CAPTURE!=='1')await page.screenshot({path:path.join(out,prefix+'-later-day.png'),animations:'disabled'});
      await page.locator('#workflowFit').click();const longFit=await page.locator('#workflowVisibleRange').textContent();assert.match(longFit,/11\. 05\..*11\. 08\./);assert.equal((await windowRange()).end,Date.parse(longEnd));
      report.cases.push({mobile,initial,pointerZoom:zoomed,pointerAnchor:{time:anchor,ratio,pixelError:Math.abs(ratio-.7)*plot.width},directRange:selectedRange,horizontalWheel:wheel,wheelGeometry,touch,preciseZoom:precise,resizePreserved:true,partialRefreshPreserved:true,sourceUnchanged:true,outOfOrderInsertionSelectionFocusPreserved:true,filterAndErrorStalePreserved:true,actualBothBoundaryClamps:true,newObservationDoesNotAutoFollow:true,longLifecycle:{midnight,laterDay,longFit}});await context.close();
    }
    assert.deepEqual(report.errors,[]);report.passed=true;
  }finally{if(browser)await browser.close();await server.close();report.cleanup={browserClosed:!!browser,fixtureClosed:true};fs.writeFileSync(path.join(out,'navigation-contract.json'),JSON.stringify(report,null,2)+'\n');}
});
