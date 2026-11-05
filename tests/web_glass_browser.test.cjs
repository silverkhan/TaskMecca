// Opt-in real Chromium test. All assets are the embedded app; all API data is synthetic.
const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const fixture=require('./helpers/glass_fixture.cjs');
const {installOpticalOracle,assertOpticalComparison,captureOpticalWitness}=require('./helpers/glass_optical_oracle.cjs');
const {waitForGlass}=require('./helpers/glass_state.cjs');
const enabled=!!process.env.GLASS_PLAYWRIGHT_MODULE;
test('Glass browser contract: appearance, stable canvas, content, fallbacks and input',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),data=JSON.parse(JSON.stringify(fixture));data.logs=fixture.api(new URL('http://fixture/api/storage/logs'),'http://fixture');data.runtime=fixture.api(new URL('http://fixture/api/runtime/storage'),'http://fixture');
 const server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
 const browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});
 const out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR);if(out)fs.mkdirSync(out,{recursive:true});
 const report={syntheticProject:fixture.project,syntheticBacklog:fixture.backlog,origin:server.origin,errors:[],checks:[],screenshots:[],textSamples:[]};
 try{
 for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1,locale:'en-US'});
  await ctx.route('**/*',route=>route.request().url().startsWith(server.origin+'/')?route.continue():route.abort());
  await ctx.addInitScript(()=>{if(!localStorage.getItem('fixture-seeded')){localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');localStorage.setItem('task-mecca-language','ko');localStorage.setItem('fixture-seeded','1')}window.EventSource=class{addEventListener(){}close(){}}});
  const page=await ctx.newPage();page.on('pageerror',e=>report.errors.push(e.message));await page.goto(server.origin);await page.waitForTimeout(300);
  const seed=async(view='backlog')=>page.evaluate(({f,view})=>{Object.assign(state,{project:f.project,backlog:f.backlog,view,listData:f.list,snapshot:f.snapshot,hub:f.hub,storageProject:f.project,logStorage:f.logs,storageRuntime:f.runtime,storageBusy:false,commonUserAttention:{fixture:{project:f.project,backlog:f.backlog,rows:[{id:f.detail.id,title:'가상 검증 결과 확인',message:'실제 요청이 아닌 fixture 안내입니다.',action:'검증 결과와 근거 확인'}]}},detail:null,detailTask:null,raw:false});render()},{f:data,view});
  const shot=async name=>{await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));if(out){report.screenshots.push({name,mobile,...await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollY,sidebar:document.querySelector('.sidebar')?.getBoundingClientRect().toJSON(),main:document.querySelector('.main')?.getBoundingClientRect().toJSON(),topbar:document.querySelector('.topbar')?.getBoundingClientRect().toJSON()}))});await page.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-${name}.png`),fullPage:false,animations:'disabled'})}};
  await seed();assert.ok(await page.locator('#previewIdentityBanner').isVisible(),'fixture Preview identity renders');await page.evaluate(()=>scrollTo(0,0));
  // Use the actual custom select's click/option event path at least once.
  await page.evaluate(()=>{applyTheme('light');applyBackground('ambient')});
  for(const [palette,label,text]of [['mecca','Mecca','#292536'],['slate','Slate','#172033'],['glass','Glass','#1c2730']]){
   await page.locator('.palette-picker .tm-combo-trigger').click();await page.locator('.tm-combo-option').filter({hasText:new RegExp('^'+label+'$')}).click();
   await page.waitForFunction(value=>value==='glass'||!document.querySelector('.glass-reader-material,.glass-optics'),palette);
   if(palette==='glass'){report.paletteStates??=[];report.paletteStates.push(await waitForGlass(page));if(out)fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2));}
   assert.equal(await page.evaluate(()=>document.documentElement.dataset.palette),palette);assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-palette')),palette);
   assert.equal(await page.locator('.background-picker').isVisible(),palette==='glass');
   assert.equal(await page.evaluate(()=>getComputedStyle(document.documentElement).getPropertyValue('--text').trim()),text);
   assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-background')),'ambient');
   assert.equal((await page.evaluate(()=>getComputedStyle(document.body,'::before').content))!=='none',palette==='glass');
   assert.equal(await page.locator('.content').evaluate(e=>!!e.querySelector('.glass-reader-material')||getComputedStyle(e,'::before').content!=='none'),palette==='glass');
  }
  await page.evaluate(()=>{applyTheme('dark');applyBackground('cozy')});
  await page.waitForFunction(()=>document.documentElement.dataset.glassOptics==='active');
  await page.waitForFunction(()=>{const e=document.querySelector('.glass-reader-material');return e?.isConnected&&getComputedStyle(e).backdropFilter.includes('blur(')});
  const reader=await page.locator('.content > .glass-reader-material').evaluate(e=>({height:e.getBoundingClientRect().height,width:e.getBoundingClientRect().width,filter:getComputedStyle(e).backdropFilter,position:getComputedStyle(e).position,ownFilter:getComputedStyle(e.parentElement).backdropFilter}));
  assert.ok(reader.filter.includes('blur('));assert.equal(reader.position,'sticky');assert.ok(reader.height<=await page.evaluate(()=>innerHeight));assert.equal(reader.ownFilter,'none');report.readerPlanes??=[];report.readerPlanes.push({mobile,...reader,optics:await page.evaluate(()=>TaskMeccaGlass.diagnostics())});
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
   assert.equal(await page.locator('.worker-card').first().evaluate(e=>getComputedStyle(e).backdropFilter),'none');assert.ok(await page.locator('.glass-reader-material').first().evaluate(e=>e.getBoundingClientRect().height<=innerHeight));
   await page.evaluate(()=>scrollTo(0,0));await seed('hub');await shot(theme+'-hub');await seed('storage');await page.waitForTimeout(100);assert.equal(await page.locator('.storage-overview-card').count(),3,'selected-project storage figures render');await shot(theme+'-storage');
   await page.evaluate(()=>scrollTo(0,0));await seed();await page.evaluate(f=>{state.detail=f.detail.id;state.detailTask=f.detail;state.raw=true;render()},data);await shot(theme+'-detail-raw');await page.locator('.raw').evaluate(e=>e.scrollIntoView());await shot(theme+'-raw-table');
   await page.evaluate(f=>{state.raw=false;activityDetailCache.set(activityDetailKey(f.detail.id),f.activity);render()},data);if(mobile){const fixed=await page.locator('.detail-toc-fab').boundingBox();assert.ok(fixed?.width>0);await page.evaluate(()=>scrollBy(0,160));assert.deepEqual(await page.locator('.detail-toc-fab').boundingBox(),fixed,'TOC stays fixed across reader scroll')}assert.ok(await page.locator('.activity-timeline-track').count()>0,'Gantt has real timed rows');await page.locator('.activity-timeline-scroll').scrollIntoViewIfNeeded();await shot(theme+'-detail-gantt');
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
  if(mobile){await page.evaluate(()=>toggleSidebar());await waitForGlass(page);const navigationLayer=await page.locator('#mobileSidebarPanel').evaluate(panel=>{const target=panel.querySelector('.nav-item'),r=target.getBoundingClientRect(),top=document.elementFromPoint(r.x+r.width*.5,r.y+r.height*.5);return{foregroundHit:panel.contains(top),topClass:top?.className,scrimVisible:!document.getElementById('mobileSidebarScrim').hidden}});assert.equal(navigationLayer.foregroundHit,true,'mobile navigation remains above its scrim and receives hit testing');assert.equal(navigationLayer.scrimVisible,true);report.navigationLayer=navigationLayer;const before=await geometry();await page.locator('#mobileSidebarPanel').evaluate(e=>{e.style.maxHeight='180px';e.style.overflow='auto';e.scrollTop=300});assert.deepEqual(await geometry(),before);await shot('navigation-dialog');await page.evaluate(()=>closeMobileNav())}
  const cdp=await ctx.newCDPSession(page);await cdp.send('Emulation.setEmulatedMedia',{features:[{name:'prefers-reduced-transparency',value:'reduce'},{name:'prefers-reduced-motion',value:'reduce'}]});
  assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').backgroundImage),'none');assert.equal(await page.locator('.topbar').evaluate(e=>getComputedStyle(e).backdropFilter),'none');await shot('reduced-preferences');
  await cdp.send('Emulation.setEmulatedMedia',{features:[]});
  // Chromium supports blur. Force only the unsupported capability branch and remove
  // unsupported declarations, emulating how a browser lacking both properties parses CSS.
  await page.evaluate(()=>localStorage.setItem('fixture-no-backdrop','1'));await page.addInitScript(()=>{if(localStorage.getItem('fixture-no-backdrop')==='1'){const original=CSS.supports.bind(CSS);CSS.supports=(p,v)=>String(p).includes('backdrop-filter')?false:original(p,v)}});
  await page.route('**/glass.css',async route=>{const response=await route.fetch();let css=await response.text();css=css.replace(/@supports not \(\(backdrop-filter:blur\(1px\)\) or \(-webkit-backdrop-filter:blur\(1px\)\)\)/,'@supports (display:block)').replace(/(?:-webkit-)?backdrop-filter\s*:[^;}]+;?/g,'');await route.fulfill({response,body:css})});
  await page.reload();await seed();assert.ok((await geometry())[6].includes('cozy-beige-room-photo-1920.webp'));assert.equal(await page.locator('.topbar').evaluate(e=>getComputedStyle(e).backgroundColor),'rgb(238, 243, 245)');assert.equal(await page.locator('.glass-optics').count(),0);await shot('unsupported-blur-emulated');await page.evaluate(()=>localStorage.removeItem('fixture-no-backdrop'));
  await page.unroute('**/glass.css');for(const theme of ['dark','light']){
   await page.evaluate(theme=>localStorage.setItem('task-mecca-theme',theme),theme);await page.goto(server.origin+'/terminal?project=/fixtures/atlas');await page.waitForTimeout(200);
   assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),theme);assert.equal(await page.locator('.terminal-app').evaluate(e=>getComputedStyle(e).zIndex),'1');
   await page.locator('#terminalIdle').evaluate(e=>e.hidden=true);await page.locator('#terminalFallbackOutput').evaluate(e=>{e.hidden=false;e.textContent='[fixture only] 가상 로그 스트림\nDEMO-100 · verification completed\nNo shell process was launched.'});assert.ok(await page.locator('#terminalFallbackOutput').isVisible());await shot(theme+'-terminal');
  }
  report.checks.push({mobile,geometry:initial,passed:true});await ctx.close();
 }
 assert.deepEqual(report.errors,[]);
 }finally{if(out)fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2));await browser.close();await server.close()}
});
test('photographic source responds to mobile decode budget and desktop DPR',{skip:!enabled,timeout:60000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web'));
 const browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true}),records=[];
 try{for(const mobile of [true,false]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:mobile?3:2});
  await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','light');window.EventSource=class{addEventListener(){}close(){}}});
  const page=await ctx.newPage();await page.goto(server.origin);await page.waitForTimeout(300);
  const image=await page.evaluate(async()=>{const css=getComputedStyle(document.body,'::before').backgroundImage,url=css.match(/url\(["']?([^"')]+)/)[1],start=performance.now(),image=new Image();image.src=url;await image.decode();const resource=performance.getEntriesByName(image.src).find(e=>e.initiatorType==='css');return{url,width:image.naturalWidth,height:image.naturalHeight,decodeMs:performance.now()-start,resource:resource?{name:resource.name,encodedBodySize:resource.encodedBodySize,decodedBodySize:resource.decodedBodySize,transferSize:resource.transferSize}:null}});
  assert.equal(image.width,mobile?1920:3840);assert.equal(image.height,mobile?1080:2160);assert.ok(image.resource,'CSS requested actual photo');assert.equal(image.resource.encodedBodySize,mobile?451256:1158428);
  const response=await ctx.request.get(new URL(image.url,server.origin).href);assert.equal(response.headers()['content-type'],'image/webp');assert.ok(response.headers()['cache-control'].includes('immutable'));
  records.push({mobile,dpr:mobile?3:2,...image,rgbaDecodeEstimateBytes:image.width*image.height*4,contentType:response.headers()['content-type'],cache:response.headers()['cache-control']});await ctx.close();
 }}finally{if(process.env.GLASS_EVIDENCE_DIR)fs.writeFileSync(path.resolve(process.env.GLASS_EVIDENCE_DIR,'photo-delivery.json'),JSON.stringify(records,null,2));await browser.close();await server.close()}
});

test('Glass optical layers preserve representative menu and native controls',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web')),browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});
 const out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR),data=JSON.parse(JSON.stringify(fixture)),report=[];if(out)fs.mkdirSync(out,{recursive:true});
 try{for(const mobile of [false,true]){
  const c=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1,locale:'en-US'});await c.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
  await c.addInitScript(installOpticalOracle);
  await c.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');localStorage.setItem('task-mecca-language','ko');window.EventSource=class{addEventListener(){}close(){}}});
  const p=await c.newPage();await p.goto(server.origin);await p.waitForTimeout(350);await p.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().mode==='active');
  const seed=()=>p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null,detailTask:null});render()},data);
  const shot=async(name)=>{await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));if(out)await p.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-optical-${name}.png`),animations:'disabled'})};
  await seed();
  for(const theme of ['dark','light']){
   await p.evaluate(theme=>applyTheme(theme),theme);await p.locator('.palette-picker .tm-combo-trigger').click();await p.waitForFunction(()=>TaskMeccaGlass.diagnostics().lenses.some(e=>e.kind==='popup'));
   const d=await p.evaluate(()=>TaskMeccaGlass.diagnostics());assert.ok(d.lenses.length<=2);const lens=d.lenses.find(e=>e.kind==='popup');assert.ok(lens.amplitude>0&&lens.amplitude<=2&&lens.rim>0&&lens.rim<=2&&lens.draws>0);
   const pixels=await p.locator('.tm-combo-popup .glass-optics').evaluate(e=>{const c=e.getContext('2d'),p=c.getImageData(0,0,e.width,e.height).data;let painted=0;for(let i=3;i<p.length;i+=4)if(p[i])painted++;return{painted,total:e.width*e.height,center:c.getImageData(Math.floor(e.width/2),Math.floor(e.height/2),1,1).data[3]}});
   assert.ok(pixels.painted>0&&pixels.painted<pixels.total/2);assert.equal(pixels.center,0,'foreground center is not captured/warped');
   const box=await p.locator('.tm-combo-popup').boundingBox(),v=p.viewportSize();assert.ok(box.x>=0&&box.y>=0&&box.x+box.width<=v.width+1&&box.y+box.height<=v.height+1);
   if(theme==='light'){
    const remap=await p.locator('.tm-combo-popup .glass-optics').evaluate(canvas=>window.__glassOpticalCompare(canvas));
    report.push({mobile,refractionComparison:remap});if(out)fs.writeFileSync(path.join(out,'liquid-glass-contract.json'),JSON.stringify(report,null,2));
    assertOpticalComparison(remap);
   }
   await shot(theme+'-menu');if(out){await p.locator('.tm-combo-popup .glass-optics').evaluate(e=>e.style.visibility='hidden');await shot(theme+'-menu-no-optical-rim');await p.locator('.tm-combo-popup .glass-optics').evaluate(e=>e.style.visibility='')}
   await p.keyboard.press('Escape');assert.equal(await p.locator('.palette-picker .tm-combo-trigger').getAttribute('aria-expanded'),'false');assert.ok(await p.locator('.palette-picker .tm-combo-trigger').evaluate(e=>e===document.activeElement));
   report.push({mobile,theme,box,pixels,optics:d});
  }
  const witness=await captureOpticalWitness({browser,origin:server.origin,mobile});report.push({mobile,opticalWitness:witness});if(out)fs.writeFileSync(path.join(out,'liquid-glass-contract.json'),JSON.stringify(report,null,2));assertOpticalComparison(witness,{witness:true});
  const negative=await captureOpticalWitness({browser,origin:server.origin,mobile,zeroDisplacement:true});report.push({mobile,zeroDisplacementControl:negative});if(out)fs.writeFileSync(path.join(out,'liquid-glass-contract.json'),JSON.stringify(report,null,2));assert.throws(()=>assertOpticalComparison(negative,{witness:true}),/bounded nonzero displacement/,'zero displacement must fail the optical contract');
  const combo=p.locator('.palette-picker .tm-combo-trigger');await combo.press('Enter');await p.keyboard.press('Home');await p.keyboard.press('Enter');assert.equal(await p.evaluate(()=>state.palette),'mecca');await p.waitForFunction(()=>document.querySelectorAll('.glass-optics').length===0);assert.equal(await p.locator('.glass-optics').count(),0);await p.evaluate(()=>applyPalette('glass'));await p.waitForFunction(()=>TaskMeccaGlass.diagnostics().mode==='active');
  // Real select enhancement: searchable list, disabled option, keyboard choice.
  await p.evaluate(()=>{const l=document.createElement('label');l.id='fixtureSelectLabel';l.style.cssText='position:fixed;top:180px;left:20px;z-index:2000;width:280px';l.textContent='가상 선택 검색 ';const s=document.createElement('select');s.id='fixtureSearchSelect';s.setAttribute('aria-label','가상 선택 검색');for(let i=0;i<16;i++){const o=document.createElement('option');o.value='v'+i;o.textContent='가상 항목 '+i;o.disabled=i===3;s.append(o)}l.append(s);document.body.append(l);TaskMeccaSelectUI.sync()});
  await p.locator('#fixtureSelectLabel .tm-combo-trigger').click();await p.locator('.tm-combo-search').fill('항목 12');await p.keyboard.press('ArrowDown');await p.keyboard.press('Enter');assert.equal(await p.locator('#fixtureSearchSelect').inputValue(),'v12');assert.ok(await p.locator('#fixtureSelectLabel .tm-combo-trigger').evaluate(e=>e===document.activeElement));
  await p.locator('#fixtureSelectLabel .tm-combo-trigger').click();await p.keyboard.press('Home');await p.keyboard.press('ArrowDown');await p.keyboard.press('ArrowDown');await p.keyboard.press('ArrowDown');await p.keyboard.press('Enter');assert.equal(await p.locator('#fixtureSearchSelect').inputValue(),'v4','disabled option is skipped');
 await p.locator('#fixtureSelectLabel .tm-combo-trigger').click();await p.setViewportSize({width:350,height:600});await p.waitForTimeout(50);const resized=await p.locator('.tm-combo-popup').boundingBox();assert.ok(resized.x>=0&&resized.y>=0&&resized.x+resized.width<=351&&resized.y+resized.height<=601);await p.keyboard.press('Escape');await p.setViewportSize(mobile?{width:390,height:844}:{width:1440,height:960});
 await p.evaluate(()=>{const input=document.createElement('input');input.id='fixtureInvalidInput';input.setAttribute('aria-invalid','true');document.getElementById('fixtureSelectLabel').append(input)});const invalid=await p.locator('#fixtureInvalidInput').evaluate(e=>({outline:getComputedStyle(e).outlineWidth,border:getComputedStyle(e).borderColor}));assert.equal(invalid.outline,'1px');assert.ok(invalid.border);await p.locator('#fixtureInvalidInput').evaluate(e=>e.remove());
 await p.locator('#fixtureSearchSelect').evaluate(e=>e.disabled=true);await p.evaluate(()=>TaskMeccaSelectUI.sync());assert.ok(await p.locator('#fixtureSelectLabel .tm-combo-trigger').isDisabled());await p.locator('#fixtureSelectLabel').evaluate(e=>e.remove());
  // Activity's actual native checkbox and notifications' actual role=switch.
  await p.evaluate(f=>{state.detail=f.detail.id;state.detailTask=f.detail;activityDetailCache.set(activityDetailKey(f.detail.id),f.activity);state.raw=false;render()},data);
  const group=p.locator('#activityDetailGroupAgents');await group.scrollIntoViewIfNeeded();await group.focus();const was=await group.isChecked();await p.keyboard.press('Space');assert.equal(await p.locator('#activityDetailGroupAgents').isChecked(),!was);await shot('activity-checkbox');
  await p.evaluate(f=>{Object.assign(state,{detail:null,detailTask:null,view:'notifications',notificationCenterTab:'settings',projectNotificationSettings:[{path:f.project,name:'Atlas · 가상 알림',status:{project_enabled:true,configured:false,connected:false}}]});syncBackgroundPush=async()=>false;render();renderNotificationPanel()},data);
  const toggle=p.locator('#centerGlobalWebChannel');await toggle.scrollIntoViewIfNeeded();const checked=await toggle.getAttribute('aria-checked');await toggle.click();await p.waitForFunction(value=>document.querySelector('#centerGlobalWebChannel')?.getAttribute('aria-checked')!==value,checked);await shot('notification-switches');
  report.push({mobile,selectionInventory:await p.evaluate(()=>({nativeRadios:document.querySelectorAll('input[type=radio]').length,switches:document.querySelectorAll('[role=switch]').length,disabledSwitches:document.querySelectorAll('[role=switch]:disabled').length})),providerBoundary:'Push provider stub; native handlers and local state used'});
  await c.close();
 }}finally{if(out)fs.writeFileSync(path.join(out,'liquid-glass-contract.json'),JSON.stringify(report,null,2));await browser.close();await server.close()}
});

test('Glass backgrounds have distinct actual light distributions in Dark and Light',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});
 try{await require('./helpers/glass_background_review.cjs').capture({browser,root:path.resolve('goassets/template/_task_mecca/framework/web'),out:process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR)})}finally{await browser.close()}
});
