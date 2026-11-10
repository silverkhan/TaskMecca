'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),path=require('node:path'),fs=require('node:fs');
const runtime=process.env.WORKFLOW_PLAYWRIGHT_MODULE;if(process.env.WORKFLOW_REQUIRE_BROWSER==='1'&&!runtime)throw Error('Required browser runtime missing');
test('PR and standalone Preview offer one-click pilot/dev comparison while ordinary dev remains unchanged',{skip:!runtime,timeout:240000},async()=>{
 const {start,project,backlog}=require('../docs/evidence/a65-performance/fixture.cjs'),wf=require('./helpers/workflow_fixture.cjs'),{chromium}=require(runtime),server=await start(path.resolve('goassets/template/_task_mecca/framework/web'));
 const sha='2bb105cdba180dbc98d6d2c009205243a1b3ecf9',task=wf.task();task.document.contract_kind='defined';let browser;const cases=[];
 const sameRoute=(actual,previous,choice)=>{const a=new URL(actual),b=new URL(previous);assert.equal(a.origin,b.origin);assert.equal(a.pathname,b.pathname);assert.equal(a.hash,b.hash);for(const[k,v]of b.searchParams)if(k!=='floating')assert.equal(a.searchParams.get(k),v,k);assert.equal(a.searchParams.get('floating'),choice)};
 try{browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});
 for(const kind of ['dev','unconfirmed','pr','preview'])for(const width of [1440,320])for(const theme of ['light','dark']){
  const ctx=await browser.newContext({viewport:{width,height:844},hasTouch:width===320,isMobile:width===320,colorScheme:theme,reducedMotion:'reduce',serviceWorkers:'block'});
  await ctx.route('**/*',r=>{const u=new URL(r.request().url());if(u.origin!==server.origin)return r.abort();if(u.pathname==='/api/health')return r.fulfill({json:kind==='preview'?{preview_mode:true,preview_pr:361,preview_sha:sha,preview_parent_url:'http://127.0.0.1:18765',boot_id:'synthetic'}:{boot_id:'synthetic'}});if(u.pathname==='/api/environment')return r.fulfill({json:{target:(kind==='pr'||kind==='unconfirmed')?{kind:'pr',pr_number:361,head_sha:kind==='pr'?sha:'unconfirmed'}:{kind:'dev'},switch_supported:true,os:'synthetic'}});if(u.pathname.startsWith('/api/tasks/'))return r.fulfill({json:{...task,id:decodeURIComponent(u.pathname.split('/').pop())}});return r.continue()});
  await ctx.addInitScript(({theme,backlog})=>{for(const[k,v]of Object.entries({'task-mecca-theme':theme,'task-mecca-language':'ko','task-mecca-backlog-folder':backlog,'task-mecca-list-page-size':'20','task-mecca-list-page-mode-v1':'manual'}))localStorage.setItem(k,v)},{theme,backlog});
  const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));const environmentReady=page.waitForResponse(r=>r.url().includes('/api/environment'));const params=new URLSearchParams({project,backlog,keep:'a b'});await page.goto(server.origin+'/tasks/FLOW-060?'+params+'#requirements');await page.locator('.detail-toc-nav a').first().waitFor();
  if(kind==='dev'||kind==='unconfirmed'){
   await environmentReady;
   assert.equal(await page.locator('[data-floating-review]').count(),0);assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);
  }else{
   const banner=page.locator(kind==='pr'?'#prExecutionBanner':'#previewIdentityBanner');await banner.waitFor({state:'visible'});const pilot=banner.locator('[data-floating-review="1"]'),dev=banner.locator('[data-floating-review="0"]');assert.equal(await pilot.textContent(),'플로팅 시안 보기');assert.equal(await dev.textContent(),'기존 UI 비교');
   const initial=page.url();await pilot.scrollIntoViewIfNeeded();const box=await pilot.boundingBox();assert.ok(box.height>=44&&box.width>=44);assert.ok(box.x>=0&&box.x+box.width<=width+1,'mobile control fits viewport');
   if(width===320)await pilot.tap();else{await pilot.focus();await page.keyboard.press('Enter');}
   await page.waitForFunction(()=>document.documentElement.dataset.floatingPilot==='true');await page.locator('.detail-toc-nav a').first().waitFor();sameRoute(page.url(),initial,'1');assert.equal(await page.locator((kind==='pr'?'#prExecutionBanner':'#previewIdentityBanner')+' [data-floating-review="1"]').getAttribute('aria-pressed'),'true');
   // Navigate inside the existing dev SPA before comparing: controls must use
   // the route at click time, not the route when the PR banner first loaded.
   await page.locator('#backBtn').click();await page.locator('.task-row').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.dataset.floatingPilot),'true');const listURL=page.url();
   const compare=page.locator((kind==='pr'?'#prExecutionBanner':'#previewIdentityBanner')+' [data-floating-review="0"]');if(width===320)await compare.tap();else await compare.click();await page.locator('.task-row').first().waitFor();sameRoute(page.url(),listURL,'0');assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);
   await page.locator('.task-row').first().click();await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);await page.reload();await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);
  }
  assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));assert.deepEqual(errors,[]);cases.push({kind,width,theme,ordinaryDevUnchanged:kind==='dev',unconfirmedIdentityHidden:kind==='unconfirmed',oneClick:kind==='pr'||kind==='preview',urlContextAndHashPreserved:kind==='pr'||kind==='preview',navigationChoicePersists:kind==='pr'||kind==='preview',mobileHit:width===320&&(kind==='pr'||kind==='preview'),errors});await ctx.close();
 }
 }finally{await browser?.close();await server.close();const out=process.env.WORKFLOW_EVIDENCE_DIR;if(out){fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'review-entry.json'),JSON.stringify({cases,syntheticPRIdentity:true,actualServiceAccess:false,browserClosed:!browser?.isConnected(),serverListening:server.isListening()},null,2));}}
});
