// Scoped diagnosis, never the immutable 48-group acceptance gate.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const {start,rows,project,backlog}=require('../a65-performance/fixture.cjs');
const {chromium}=require('/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright');
const root=path.resolve(__dirname,'../../..'),phase=process.argv[2];
if(!['before','after'].includes(phase))throw Error('before|after required');
const out=path.join(__dirname,phase);if(fs.existsSync(out))throw Error('immutable output exists');fs.mkdirSync(out,{recursive:true});
const web=path.join(root,'goassets/template/_task_mecca/framework/web');
const sources={candidate:web,...(phase==='before'?{gui:'/private/tmp/task-mecca-a65-baseline-f0a732e3/goassets/template/_task_mecca/framework/web'}:{})};
const all=[],servers=[];let browser;
const metrics=async c=>Object.fromEntries((await c.send('Performance.getMetrics')).metrics.map(x=>[x.name,x.value]));
(async()=>{try{
 browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 for(const [variant,dir] of Object.entries(sources)){
  const server=await start(dir);servers.push(server);
  for(const width of [1440,390])for(const theme of ['light','dark']){
   const context=await browser.newContext({viewport:{width,height:1000},colorScheme:theme,serviceWorkers:'block'});
   await context.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
   await context.addInitScript(({theme,backlog})=>{for(const [k,v]of Object.entries({'task-mecca-language':'en','task-mecca-list-page-size':'20','task-mecca-list-page-mode-v1':'manual','task-mecca-theme':theme,'task-mecca-smoke-mode':theme,'task-mecca-backlog-folder':backlog}))localStorage.setItem(k,v)},{theme,backlog});
   const page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
   const cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');
   await page.goto(server.origin+'/?view=backlog&project='+encodeURIComponent(project)+'&backlog='+encodeURIComponent(backlog)+'&page_size=20');
   await page.locator('.task-list .task-row').first().waitFor();await page.waitForTimeout(250);
   const ids=await page.locator('.task-list .task-row').evaluateAll(nodes=>nodes.map(n=>n.textContent));
   if(ids.length!==20)throw Error('Expected 20 identical dataset rows');
   await page.screenshot({path:path.join(out,`${variant}-${width}-${theme}.png`),fullPage:false});
   const stats=await page.evaluate(()=>({surfaces:document.querySelectorAll('.material-surface').length,transmissions:document.querySelectorAll('.material-transmission').length,filters:document.querySelectorAll('filter[id^="material-face"]').length,overflow:document.documentElement.scrollWidth>innerWidth,textFilters:[...document.querySelectorAll('main p,main h1,main td')].filter(n=>getComputedStyle(n).filter!=='none').length}));
   const samples=[];
   if(variant==='candidate')for(let repeat=0;repeat<4;repeat++)for(const action of ['scroll','input']){
    await page.evaluate(()=>scrollTo(0,0));await page.waitForTimeout(80);
    await page.evaluate(()=>{window.__frames=[];window.__record=true;let last;function frame(t){if(!window.__record)return;if(last)window.__frames.push(t-last);last=t;requestAnimationFrame(frame)}requestAnimationFrame(frame)});
    const a=await metrics(cdp),t=Date.now();
    if(action==='scroll')await page.evaluate(()=>scrollTo(0,400));else for(let i=0;i<12;i++)await page.mouse.move(100+i*12,160+i*9);
    await page.waitForTimeout(Math.max(0,1000-(Date.now()-t)));
    const b=await metrics(cdp),frames=await page.evaluate(()=>{window.__record=false;return window.__frames});
    samples.push({repeat,action,windowMs:Date.now()-t,TaskDurationMs:(b.TaskDuration-a.TaskDuration)*1000,ScriptDurationMs:(b.ScriptDuration-a.ScriptDuration)*1000,LayoutDurationMs:(b.LayoutDuration-a.LayoutDuration)*1000,JSHeapUsedSize:b.JSHeapUsedSize,frames,scrollY:await page.evaluate(()=>scrollY)});
   }
   all.push({variant,width,theme,ids,stats,errors,samples});fs.writeFileSync(path.join(out,'raw.json'),JSON.stringify(all,null,2));await context.close();
  }
 }
 fs.writeFileSync(path.join(out,'manifest.json'),JSON.stringify({phase,scope:'scoped 1-second scroll/input windows; not full gate; desktop synthetic mouse on mobile viewport is not touch or physical device',at:new Date().toISOString(),browser:browser.version(),sources,assets:Object.fromEntries(['ui/optics.js','ui/expansion.js','ui/expansion.css'].map(p=>[p,crypto.createHash('sha256').update(fs.readFileSync(path.join(web,p))).digest('hex')]))},null,2));
}finally{await browser?.close();for(const s of servers)await s.close();fs.writeFileSync(path.join(out,'cleanup.json'),JSON.stringify({browserClosed:!browser?.isConnected(),servers:servers.map(s=>({origin:s.origin,listening:s.isListening()}))},null,2));}})().catch(e=>{console.error(e);process.exitCode=1});
