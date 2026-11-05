// Real embedded assets, synthetic APIs, and exploratory headless observations.
// Paint here is main-thread trace work, never GPU/battery measurement.
const fs=require('node:fs'),path=require('node:path'),fixture=require('./glass_fixture.cjs');
async function paneBox(cdp,page){
 const real=await page.locator('.glass-reader-material').first().count();if(real)return await page.locator('.glass-reader-material').first().evaluate(e=>({kind:'element',width:e.getBoundingClientRect().width,height:e.getBoundingClientRect().height,rect:e.getBoundingClientRect().toJSON(),boundedToViewport:e.getBoundingClientRect().height<=innerHeight,viewport:{width:innerWidth,height:innerHeight,scrollY},optics:window.TaskMeccaGlass?.diagnostics()}));
 const {root}=await cdp.send('DOM.getDocument'),{nodeId}=await cdp.send('DOM.querySelector',{nodeId:root.nodeId,selector:'#content'});
 const {node}=await cdp.send('DOM.describeNode',{nodeId,depth:1});
 const pseudo=node.pseudoElements?.find(n=>n.pseudoType==='before');if(!pseudo)return null;
 const {model}=await cdp.send('DOM.getBoxModel',{backendNodeId:pseudo.backendNodeId});
 const viewport=await page.evaluate(()=>({width:innerWidth,height:innerHeight,scrollY})),left=Math.max(0,model.border[0]),top=Math.max(0,model.border[1]),right=Math.min(viewport.width,model.border[4]),bottom=Math.min(viewport.height,model.border[5]);
 return{...model,viewport,visibleIntersection:{left,top,width:Math.max(0,right-left),height:Math.max(0,bottom-top)},boundedToViewport:model.height<=viewport.height};
}
(async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),out=path.resolve(process.env.GLASS_EVIDENCE_DIR||'glass-measurements');fs.mkdirSync(out,{recursive:true});
 const root=path.resolve(process.env.GLASS_ASSET_ROOT||'goassets/template/_task_mecca/framework/web'),server=await fixture.start(root),browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});
 const data=JSON.parse(JSON.stringify(fixture)),report={browser:browser.version(),root,syntheticProject:fixture.project,origin:server.origin,captureOnly:!!process.env.GLASS_CAPTURE_ONLY,observations:[]};
 try{for(const mobile of [false,true]){
  const context=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});
  await context.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
  await context.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');localStorage.setItem('task-mecca-language','ko');window.EventSource=class{addEventListener(){}close(){}}});
  const page=await context.newPage(),cdp=await context.newCDPSession(page);await page.goto(server.origin);await page.waitForTimeout(300);
  await page.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},data);
  const optical=[];
  for(const theme of ['dark','light']){
   await page.evaluate(v=>{applyTheme(v);scrollTo(0,0)},theme);await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
   const prefix=`${mobile?'mobile':'desktop'}-${theme}`;await page.screenshot({path:path.join(out,prefix+'-backlog.png'),animations:'disabled'});
  await page.locator('.palette-picker .tm-combo-trigger').click();await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await page.screenshot({path:path.join(out,prefix+'-menu.png'),animations:'disabled'});await page.keyboard.press('Escape');
   await page.evaluate(()=>{const r=document.querySelector('.task-list').getBoundingClientRect();scrollTo(0,scrollY+r.y-240)});
   await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));
   const clip=await page.locator('.task-list').evaluate(e=>{const r=e.getBoundingClientRect();return{x:r.x+8,y:r.y+scrollY+24,width:Math.min(r.width-16,680),height:Math.min(280,innerHeight-r.y-72)}});
   const hide=await page.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'});
   await page.screenshot({path:path.join(out,prefix+'-frost-crop.png'),clip,animations:'disabled'});await hide.evaluate(e=>e.remove());
   optical.push({theme,clip,pane:await paneBox(cdp,page),styles:await page.locator('.content').evaluate(e=>{const material=e.querySelector('.glass-reader-material'),s=material?getComputedStyle(material):getComputedStyle(e,'::before');return{kind:material?'element':'pseudo',filter:s.backdropFilter,width:s.width,height:s.height,position:s.position,fill:s.backgroundColor,ownFilter:getComputedStyle(e).backdropFilter}})});
  }
  await page.evaluate(()=>{applyTheme('dark');state.view='workload';render()});await page.locator('.worker-card').first().scrollIntoViewIfNeeded();
  const cardFilters=await page.locator('.worker-card').evaluateAll(a=>a.map(e=>getComputedStyle(e).backdropFilter));await page.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-workers.png`),animations:'disabled'});
  await page.evaluate(()=>{state.view='backlog';render();scrollTo(0,300)});
  const geometry=await page.evaluate(()=>({innerWidth,innerHeight,dpr:devicePixelRatio,sidebar:document.querySelector('.sidebar').getBoundingClientRect().toJSON(),main:document.querySelector('.main').getBoundingClientRect().toJSON(),content:document.querySelector('.content').getBoundingClientRect().toJSON(),scrollWidth:document.documentElement.scrollWidth}));
  const observation={mobile,optical,cardFilters,geometry,readerBefore:await paneBox(cdp,page)};
  if(!process.env.GLASS_CAPTURE_ONLY){
   await cdp.send('Performance.enable');const before=await cdp.send('Performance.getMetrics'),traceDone=new Promise(r=>cdp.once('Tracing.tracingComplete',r));await cdp.send('Tracing.start',{categories:'devtools.timeline',transferMode:'ReturnAsStream'});
   const frames=await page.evaluate(async()=>{let gaps=[],previous=performance.now();await new Promise(resolve=>{const end=previous+1500;function frame(t){gaps.push(t-previous);previous=t;scrollTo(0,(Math.sin(t/100)+1)*800);if(t<end)requestAnimationFrame(frame);else resolve()}requestAnimationFrame(frame)});return{frames:gaps.length,maxGap:Math.max(...gaps),meanGap:gaps.reduce((a,b)=>a+b,0)/gaps.length}});
   const after=await cdp.send('Performance.getMetrics');await cdp.send('Tracing.end');const {stream}=await traceDone;let raw='';for(;;){const chunk=await cdp.send('IO.read',{handle:stream});raw+=chunk.data;if(chunk.eof)break}await cdp.send('IO.close',{handle:stream});
   const paints=JSON.parse(raw).traceEvents.filter(e=>e.name==='Paint'&&e.ph==='X');
   const switching=await page.evaluate(async()=>{let times=[];for(let i=0;i<8;i++){const t=performance.now();applyTheme(i%2?'dark':'light');await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));times.push(performance.now()-t)}return times});
   Object.assign(observation,{frames,paint:{events:paints.length,totalMs:paints.reduce((sum,e)=>sum+(e.dur||0),0)/1000},before:before.metrics,after:after.metrics,switching,readerAfter:await paneBox(cdp,page)});
  }
  const first=await page.evaluate(()=>window.TaskMeccaGlass?.diagnostics()||null);await page.waitForTimeout(250);const second=await page.evaluate(()=>window.TaskMeccaGlass?.diagnostics()||null);observation.optics=second;observation.idleOpticalPaints=first&&second?second.paintCount-first.paintCount:null;observation.canvasRgbaEstimateBytes=second?((second.sampler?.width||0)*(second.sampler?.height||0)+second.lenses.reduce((n,l)=>n+l.pixels,0))*4:0;
  report.observations.push(observation);await context.close();
 }}finally{fs.writeFileSync(path.join(out,'observations.json'),JSON.stringify(report,null,2));await browser.close();await server.close()}
})().catch(e=>{console.error(e);process.exit(1)});
