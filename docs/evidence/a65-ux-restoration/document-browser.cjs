const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {start,project,backlog}=require('../a65-performance/fixture.cjs'),wf=require('../../../tests/helpers/workflow_fixture.cjs');
const {chromium}=require('/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright');
const root=path.resolve(__dirname,'../../..'),phase=process.argv[2],out=path.join(__dirname,phase);if(!['batch','confirm','reference','mechanics'].includes(phase)||fs.existsSync(out))throw Error('fresh phase required');fs.mkdirSync(out,{recursive:true});
const paragraph=(prefix,n)=>Array.from({length:n},(_,i)=>`${prefix} ${i+1}. Synthetic reading evidence, same source data in both interfaces. This is not operating user data.`).join('\n\n');
const task=wf.task();task.document.contract_kind='defined';task.document.requirements={background:paragraph('Background',24),goal:'Keep all content on one continuous page.',requirements:paragraph('Requirement',24),scope_in:paragraph('Included',6),scope_out:'Operating service changes excluded.',constraints:'No row-level blur.',acceptance:'- [ ] Existing anchors and floating index remain usable.'};task.document.notes=paragraph('Notes',18);task.document.result=paragraph('Execution',18);task.document.verification=paragraph('Verification',18);task.raw_markdown=Object.values(task.document.requirements).join('\n\n')+'\n\n'+task.document.notes;
const sources={...(phase==='reference'?{}:{candidate:path.join(root,'goassets/template/_task_mecca/framework/web')}),...(phase==='mechanics'?{}:{gui:'/private/tmp/task-mecca-a65-baseline-f0a732e3/goassets/template/_task_mecca/framework/web'})};const records=[],servers=[];let browser;
(async()=>{try{
 browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 for(const [variant,dir]of Object.entries(sources)){
  const server=await start(dir);servers.push(server);
  for(const width of variant==='gui'?[1440,390]:[1440,1280,320,390,430])for(const theme of ['light','dark']){
   const ctx=await browser.newContext({viewport:{width,height:width<760?844:1000},colorScheme:theme,serviceWorkers:'block'});await ctx.route('**/*',r=>{const u=new URL(r.request().url());if(u.origin!==server.origin)return r.abort();if(u.pathname.startsWith('/api/tasks/'))return r.fulfill({json:{...task,id:decodeURIComponent(u.pathname.split('/').pop())}});return r.continue()});
   await ctx.addInitScript(({theme,backlog})=>{for(const[k,v]of Object.entries({'task-mecca-language':'en','task-mecca-theme':theme,'task-mecca-smoke-mode':theme,'task-mecca-list-page-size':'20','task-mecca-list-page-mode-v1':'manual','task-mecca-backlog-folder':backlog}))localStorage.setItem(k,v)},{theme,backlog});
   const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));const list=server.origin+'/?view=backlog&project='+encodeURIComponent(project)+'&backlog='+encodeURIComponent(backlog)+'&page_size=20';await page.goto(list);await page.locator('.task-row').first().waitFor();
   const capture=phase!=='mechanics'&&[1440,390].includes(width);if(capture)await page.screenshot({path:path.join(out,`${variant}-${width}-${theme}-backlog.png`)});
   if(variant==='candidate'){
    const icons=await page.locator('.material-nav .rail-link .material-nav-icon path').evaluateAll(nodes=>nodes.map(n=>n.getAttribute('d')));assert.equal(new Set(icons).size,icons.length,'menu meanings use distinct paths');
   }
   await page.locator(variant==='gui'?'.task-row':'.task-row a').first().click();await page.locator('.detail-toc-nav a').first().waitFor({state:'attached'});
   if(variant==='candidate'){
    await page.waitForFunction(()=>document.querySelector('#activityMetricsBody .workflow-root')||document.querySelector('#activityMetricsBody .activity-empty')||document.querySelector('#activityMetricsBody')?.textContent.length>80);
    for(const id of ['human-summary','background','requirements','scope','acceptance','lifecycle','progress-result','execution','verification','operations','source'])assert.equal(await page.locator('#'+id).count(),1,id+' remains in one DOM');
    if(phase==='mechanics')assert.equal(await page.evaluate(()=>!!(document.querySelector('#lifecycle').compareDocumentPosition(document.querySelector('#background'))&Node.DOCUMENT_POSITION_FOLLOWING)),true,'workflow retains incumbent order before contract body');
    assert.ok(await page.locator('#background').innerText().then(s=>s.includes('Background 24')));assert.ok(await page.locator('#progress-result').innerText().then(s=>s.includes('Notes 18')));
    assert.equal(await page.locator('.detail-reading .markdown').evaluateAll(nodes=>nodes.every(n=>getComputedStyle(n).overflowY==='visible')),true,'no nested document scroll traps');
    const optical=await page.locator('.material-reading-plane').evaluate(n=>({height:n.getBoundingClientRect().height,viewport:innerHeight,filters:n.querySelectorAll('.material-transmission').length,descendantLenses:document.querySelectorAll('.detail-reading .material-transmission').length}));assert.ok(optical.height<=optical.viewport);assert.equal(optical.filters,1);assert.equal(optical.descendantLenses,0);
    if(width<=1280){await page.locator('.detail-toc-fab').click();assert.equal(await page.locator('.detail-toc-fab').getAttribute('aria-expanded'),'true')}
    await page.locator('.detail-toc-nav [data-detail-target=verification]').click();assert.ok((await page.locator('#verification').boundingBox()).y<844);assert.ok((await page.url()).endsWith('#verification'));assert.equal(await page.locator('#requirements').count(),1);
    if(width<=1280){assert.equal(await page.locator('.detail-toc-fab').getAttribute('aria-expanded'),'false');await page.locator('.detail-toc-fab').click();await page.keyboard.press('Escape');assert.equal(await page.locator('.detail-toc-fab').getAttribute('aria-expanded'),'false')}
    await page.reload();await page.locator('.detail-toc-nav a').first().waitFor({state:'attached'});await page.waitForFunction(()=>{const r=document.querySelector('#verification')?.getBoundingClientRect();return r&&r.top<innerHeight&&r.bottom>0});
    await page.evaluate(()=>document.querySelector('#human-summary').scrollIntoView());
   }else await page.evaluate(()=>document.querySelector('#human-summary').scrollIntoView());
   if(capture)await page.screenshot({path:path.join(out,`${variant}-${width}-${theme}-detail.png`)});
   const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);assert.equal(overflow,false,'root overflow');assert.equal(errors.length,0,errors.join('\n'));
   records.push({variant,width,theme,errors,overflow,documentHeight:await page.evaluate(()=>document.documentElement.scrollHeight),continuous:variant==='candidate',anchorReload:variant==='candidate',floatingIndex:variant==='candidate'&&width<=1280});fs.writeFileSync(path.join(out,'results.json'),JSON.stringify(records,null,2));await ctx.close();
  }
 }
}finally{await browser?.close();for(const server of servers)await server.close();fs.writeFileSync(path.join(out,'cleanup.json'),JSON.stringify({browserClosed:!browser?.isConnected(),servers:servers.map(s=>({origin:s.origin,listening:s.isListening()}))},null,2))}})().catch(e=>{console.error(e);process.exitCode=1});
