// Counterfactual observer diagnosis. Same fixture/browser, original vs current
// modules supplied by route interception; no services/checkouts are modified.
const fs=require('node:fs'),path=require('node:path'),{execFileSync}=require('node:child_process');
const {start,project,backlog}=require('../a65-performance/fixture.cjs');
const {chromium}=require('/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright');
const root=path.resolve(__dirname,'../../..'),web=path.join(root,'goassets/template/_task_mecca/framework/web');
const out=path.join(__dirname,'mutation.json');if(fs.existsSync(out))throw Error('immutable result exists');
const metrics=async c=>Object.fromEntries((await c.send('Performance.getMetrics')).metrics.map(x=>[x.name,x.value]));
(async()=>{const server=await start(web);let browser;const results=[];try{
browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
for(const variant of ['original','current']){
 const context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});
 await context.route('**/*',async r=>{const u=new URL(r.request().url());if(u.origin!==server.origin)return r.abort();
 if(variant==='original'&&['/ui/expansion.js','/ui/optics.js','/ui/expansion.css'].includes(u.pathname))return r.fulfill({body:execFileSync('git',['show','27dbe42e:goassets/template/_task_mecca/framework/web'+u.pathname],{cwd:root}),contentType:u.pathname.endsWith('.css')?'text/css':'text/javascript'});return r.continue()});
 const page=await context.newPage();await page.goto(server.origin+'/?view=backlog&project='+encodeURIComponent(project)+'&backlog='+encodeURIComponent(backlog));await page.locator('.task-row').first().waitFor();await page.waitForTimeout(100);
 await page.evaluate(()=>{window.__queries=0;for(const proto of [Document.prototype,Element.prototype]){const original=proto.querySelectorAll;proto.querySelectorAll=function(selector){if(selector.includes('material-surface')||selector.includes('data-light-surface'))window.__queries++;return original.call(this,selector)}}});
 const cdp=await context.newCDPSession(page);await cdp.send('Performance.enable');const before=await metrics(cdp);
 await page.evaluate(async()=>{const text=document.querySelector('.task-title')||document.querySelector('.task-row');const probe=document.createElement('span');text.append(probe);for(let i=0;i<60;i++){probe.textContent=String(i);await new Promise(r=>setTimeout(r,0))}probe.remove()});
 const after=await metrics(cdp),queries=await page.evaluate(()=>window.__queries);
 await page.evaluate(()=>document.querySelector('main').insertAdjacentHTML('beforeend','<section id="readonly-registry-proof" class="management-sheet"><p>Isolated fixture content</p></section>'));
 await page.locator('#readonly-registry-proof>.material-transmission').waitFor();
 await page.evaluate(()=>document.querySelector('#readonly-registry-proof').remove());
 await page.waitForFunction(()=>!document.querySelector('#readonly-registry-proof')&&document.querySelectorAll('filter[id^="material-face"]').length===document.querySelectorAll('.material-transmission').length);
 results.push({variant,queries,TaskDurationMs:(after.TaskDuration-before.TaskDuration)*1000,ScriptDurationMs:(after.ScriptDuration-before.ScriptDuration)*1000,insertedReadonlySurface:true,removedSurfaceCleaned:true});await context.close();
}
}finally{await browser?.close();await server.close();fs.writeFileSync(out,JSON.stringify({scope:'60 synthetic DOM text updates, one run each; observer causality diagnostic, not full gate or typical user workload',results,cleanup:{browserClosed:!browser?.isConnected(),serverListening:server.isListening()}},null,2))}})().catch(e=>{console.error(e);process.exitCode=1});
