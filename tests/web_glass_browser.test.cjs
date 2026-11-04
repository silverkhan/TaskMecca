// Opt-in real Chromium test. All assets are the embedded app; all API data is synthetic.
const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const fixture=require('./helpers/glass_fixture.cjs');
const enabled=!!process.env.GLASS_PLAYWRIGHT_MODULE;
test('Glass browser contract: appearance, stable canvas, content, fallbacks and input',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),data=JSON.parse(JSON.stringify(fixture));data.logs=fixture.api(new URL('http://fixture/api/storage/logs'),'http://fixture');data.runtime=fixture.api(new URL('http://fixture/api/runtime/storage'),'http://fixture');
 const server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
 const browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});
 const out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR);if(out)fs.mkdirSync(out,{recursive:true});
 const report={syntheticProject:fixture.project,syntheticBacklog:fixture.backlog,origin:server.origin,errors:[],checks:[],screenshots:[],textSamples:[]};
 try{
 for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});
  await ctx.route('**/*',route=>route.request().url().startsWith(server.origin+'/')?route.continue():route.abort());
  await ctx.addInitScript(()=>{if(!localStorage.getItem('fixture-seeded')){localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');localStorage.setItem('task-mecca-language','ko');localStorage.setItem('fixture-seeded','1')}window.EventSource=class{addEventListener(){}close(){}}});
  const page=await ctx.newPage();page.on('pageerror',e=>report.errors.push(e.message));await page.goto(server.origin);await page.waitForTimeout(300);
  const seed=async(view='backlog')=>page.evaluate(({f,view})=>{Object.assign(state,{project:f.project,backlog:f.backlog,view,listData:f.list,snapshot:f.snapshot,hub:f.hub,storageProject:f.project,logStorage:f.logs,storageRuntime:f.runtime,storageBusy:false,commonUserAttention:{fixture:{project:f.project,backlog:f.backlog,rows:[{id:f.detail.id,title:'가상 검증 결과 확인',message:'실제 요청이 아닌 fixture 안내입니다.',action:'검증 결과와 근거 확인'}]}},detail:null,detailTask:null,raw:false});render()},{f:data,view});
  const shot=async name=>{await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));if(out){report.screenshots.push({name,mobile,...await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollY,sidebar:document.querySelector('.sidebar')?.getBoundingClientRect().toJSON(),main:document.querySelector('.main')?.getBoundingClientRect().toJSON(),topbar:document.querySelector('.topbar')?.getBoundingClientRect().toJSON()}))});await page.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-${name}.png`),fullPage:false,animations:'disabled'})}};
  await seed();assert.ok(await page.locator('#previewIdentityBanner').isVisible(),'fixture Preview identity renders');await page.evaluate(()=>scrollTo(0,0));
  // Use the actual custom select's click/option event path at least once.
  await page.evaluate(()=>{applyTheme('light');applyBackground('ambient')});
  for(const [palette,label,text]of [['mecca','Mecca','#292536'],['slate','Slate','#172033'],['glass','Glass','#30251f']]){
   await page.locator('.palette-picker .tm-combo-trigger').click();await page.locator('.tm-combo-option').filter({hasText:new RegExp('^'+label+'$')}).click();
   assert.equal(await page.evaluate(()=>document.documentElement.dataset.palette),palette);assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-palette')),palette);
   assert.equal(await page.locator('.background-picker').isVisible(),palette==='glass');
   assert.equal(await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--text').trim()),text);
   assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-background')),'ambient');
   assert.equal((await page.evaluate(()=>getComputedStyle(document.body,'::before').content))!=='none',palette==='glass');
   assert.equal(await page.locator('.task-list').evaluate(e=>getComputedStyle(e).backgroundColor==='rgba(0, 0, 0, 0)'),palette!=='glass');
  }
  await page.evaluate(()=>{applyTheme('dark');applyBackground('cozy')});
  const geometry=()=>page.evaluate(()=>{const s=getComputedStyle(document.body,'::before');return[s.width,s.height,s.left,s.top,s.backgroundSize,s.backgroundPosition,s.backgroundImage]});
  const initial=await geometry();await page.evaluate(()=>{document.body.style.minHeight='3000px';window.scrollTo(0,800)});assert.deepEqual(await geometry(),initial);
  await page.setViewportSize({width:mobile?390:1440,height:mobile?510:700});assert.deepEqual(await geometry(),initial);await page.setViewportSize(mobile?{width:844,height:390}:{width:1100,height:800});assert.deepEqual(await geometry(),initial);
  await shot('height-orientation');await page.setViewportSize(mobile?{width:390,height:844}:{width:1440,height:960});await page.evaluate(()=>{document.body.style.minHeight='';scrollTo(0,0)});
  for(const theme of ['dark','light']){
   await page.evaluate(theme=>applyTheme(theme),theme);await seed();await page.evaluate(()=>scrollTo(0,0));
   assert.equal(await page.locator('.task-row').count()>0,true,'real backlog rows render');
   await shot(theme+'-backlog');await page.locator('.common-attention summary').click();await shot(theme+'-attention');await page.locator('.common-attention summary').click();
   if(out){
    const samples=await page.evaluate(()=>{const w=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT),a=[];let n;while(n=w.nextNode()){if(!n.textContent.trim()||!n.parentElement||n.parentElement.closest('script,style'))continue;const r=document.createRange();r.selectNodeContents(n);let b=r.getBoundingClientRect();const p=n.parentElement.getBoundingClientRect(),s=getComputedStyle(n.parentElement);b={x:Math.max(0,b.x,p.x),y:Math.max(0,b.y,p.y),right:Math.min(innerWidth,b.right,p.right),bottom:Math.min(innerHeight,b.bottom,p.bottom)};b.width=b.right-b.x;b.height=b.bottom-b.y;if(b.width>8&&b.height>5&&b.y>=0&&b.bottom<innerHeight&&s.visibility!=='hidden'&&s.opacity!=='0'&&n.parentElement.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))a.push({text:n.textContent.trim().slice(0,60),rect:{x:b.x,y:b.y,width:b.width,height:b.height},color:s.color,fontSize:s.fontSize,fontWeight:s.fontWeight})}return a});
    const hide=await page.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'});await page.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-${theme}-background-samples.png`)});await hide.evaluate(e=>e.remove());report.textSamples.push({mobile,theme,samples});
   }
   for(const bg of ['ambient','none','cozy']){await page.evaluate(bg=>applyBackground(bg),bg);assert.equal(await page.evaluate(()=>document.documentElement.dataset.background),bg);await page.reload();await seed();assert.equal(await page.evaluate(()=>document.documentElement.dataset.background),bg)}
   await seed('workload');await page.locator('.runtime-root-card').first().scrollIntoViewIfNeeded();await shot(theme+'-workload');await page.locator('.runtime-history').first().evaluate(e=>e.open=true);await page.locator('.runtime-transition-list').first().scrollIntoViewIfNeeded();await shot(theme+'-runtime-logs');await page.locator('.worker-card').first().scrollIntoViewIfNeeded();await shot(theme+'-workers');
   assert.equal(await page.locator('.worker-card').count()>0,true,'actual workload cards render');
   assert.equal(await page.locator('.worker-card').first().evaluate(e=>getComputedStyle(e).backdropFilter),'none');
   await page.evaluate(()=>scrollTo(0,0));await seed('hub');await shot(theme+'-hub');await seed('storage');await page.waitForTimeout(100);assert.equal(await page.locator('.storage-overview-card').count(),3,'selected-project storage figures render');await shot(theme+'-storage');
   await page.evaluate(()=>scrollTo(0,0));await seed();await page.evaluate(f=>{state.detail=f.detail.id;state.detailTask=f.detail;state.raw=true;render()},data);await shot(theme+'-detail-raw');await page.locator('.raw').evaluate(e=>e.scrollIntoView());await shot(theme+'-raw-table');
   await page.evaluate(f=>{state.raw=false;activityDetailCache.set(activityDetailKey(f.detail.id),f.activity);render()},data);assert.ok(await page.locator('.activity-timeline-track').count()>0,'Gantt has real timed rows');await page.locator('.activity-timeline-scroll').scrollIntoViewIfNeeded();await shot(theme+'-detail-gantt');
  }
  await page.evaluate(()=>{applyTheme('system');applyBackground('cozy')});await page.emulateMedia({colorScheme:'dark'});await page.waitForFunction(()=>document.documentElement.dataset.theme==='dark');assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),'dark');await page.emulateMedia({colorScheme:'light'});await page.waitForFunction(()=>document.documentElement.dataset.theme==='light');assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),'light');
  await seed();await page.keyboard.press('Tab');assert.ok(await page.evaluate(()=>document.activeElement!==document.body));assert.equal(await page.evaluate(()=>getComputedStyle(document.activeElement).outlineStyle),'solid');
  const enlargement=await page.evaluate(()=>{
   const text=[...document.querySelectorAll('body *')].filter(e=>[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())).map(e=>[e,parseFloat(getComputedStyle(e).fontSize)]);
   for(const [e,size]of text){e.dataset.fixtureFont=e.style.fontSize;e.style.setProperty('font-size',(size*1.5)+'px','important')}
   return text.filter(([e,size])=>e.getBoundingClientRect().width>0).map(([e,size])=>({before:size,after:parseFloat(getComputedStyle(e).fontSize)}));
  });
  assert.ok(enlargement.length>10);assert.ok(enlargement.every(x=>x.after>=x.before*1.49),'visible text really enlarged');
  await page.evaluate(()=>scrollTo(0,0));await shot('large-font');assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'enlarged text viewport does not overflow');
  await page.evaluate(()=>{for(const e of document.querySelectorAll('[data-fixture-font]')){e.style.fontSize=e.dataset.fixtureFont;delete e.dataset.fixtureFont}});await seed();
  // Independent, real mobile navigation dialog scroll, then fixed canvas comparison.
  if(mobile){await page.evaluate(()=>toggleSidebar());const before=await geometry();await page.locator('#mobileSidebarPanel').evaluate(e=>{e.style.maxHeight='180px';e.style.overflow='auto';e.scrollTop=300});assert.deepEqual(await geometry(),before);await shot('navigation-dialog');await page.evaluate(()=>closeMobileNav())}
  const cdp=await ctx.newCDPSession(page);await cdp.send('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-transparency',value:'reduce'},{name:'prefers-reduced-motion',value:'reduce'}]});
  assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').backgroundImage),'none');assert.equal(await page.locator('.topbar').evaluate(e=>getComputedStyle(e).backdropFilter),'none');await shot('reduced-preferences');
  await cdp.send('Emulation.setEmulatedMedia',{features:[]});
  // Chromium supports blur. Force only the unsupported capability branch and remove
  // unsupported declarations, emulating how a browser lacking both properties parses CSS.
  await page.route('**/style.css',async route=>{const response=await route.fetch();let css=await response.text();css=css.replace(/@supports not \(\(backdrop-filter:blur\(1px\)\) or \(-webkit-backdrop-filter:blur\(1px\)\)\)/,'@supports (display:block)').replace(/(?:-webkit-)?backdrop-filter\s*:[^;}]+;?/g,'');await route.fulfill({response,body:css})});
  await page.reload();await seed();assert.ok((await geometry())[6].includes('cozy-beige-room.svg'));assert.equal(await page.locator('.topbar').evaluate(e=>getComputedStyle(e).backgroundColor),'rgb(245, 235, 221)');await shot('unsupported-blur-emulated');
  await page.unroute('**/style.css');for(const theme of ['dark','light']){
   await page.evaluate(theme=>localStorage.setItem('task-mecca-theme',theme),theme);await page.goto(server.origin+'/terminal?project=/fixtures/atlas');await page.waitForTimeout(200);
   assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),theme);assert.equal(await page.locator('.terminal-app').evaluate(e=>getComputedStyle(e).zIndex),'1');
   await page.locator('#terminalIdle').evaluate(e=>e.hidden=true);await page.locator('#terminalFallbackOutput').evaluate(e=>{e.hidden=false;e.textContent='[fixture only] 가상 로그 스트림\nDEMO-100 · verification completed\nNo shell process was launched.'});assert.ok(await page.locator('#terminalFallbackOutput').isVisible());await shot(theme+'-terminal');
  }
  report.checks.push({mobile,geometry:initial,passed:true});await ctx.close();
 }
 assert.deepEqual(report.errors,[]);
 }finally{if(out)fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2));await browser.close();await server.close()}
});
