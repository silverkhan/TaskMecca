'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const web=path.resolve('goassets/template/_task_mecca/framework/web'),fixtures=path.join(__dirname,'fixtures/a65'),baseline=require('./fixtures/a65/dev-floating-baseline.json');
const runtime=process.env.WORKFLOW_PLAYWRIGHT_MODULE;if(process.env.WORKFLOW_REQUIRE_BROWSER==='1'&&!runtime)throw Error('Required browser runtime missing');
test('approved pilot preserves exact origin/dev application, controls, styles and entry DOM',()=>{
 for(const [file,expected]of Object.entries(baseline.sha256)){
  let bytes=fs.readFileSync(path.join(web,file));
  if(file==='app.js'){
   let source=bytes.toString();assert.equal((source.match(/\+devFloatingReviewControls\(\)/g)||[]).length,2,'only the two actual PR identity banners add controls');
   source=source.replace(/\/\/ A-65 review controls:[\s\S]*?\/\/ End A-65 review controls\.\n\n/,'').replaceAll('+devFloatingReviewControls()','').replace("document.addEventListener('click',handleDevFloatingReview);\n",'');bytes=Buffer.from(source);
  }
  assert.equal(crypto.createHash('sha256').update(bytes).digest('hex'),expected,file+' remains dev apart from the bounded review-entry addition');
 }
 const addition='  <!-- Opt-in material pilot: dev DOM and app stay unchanged. -->\n  <script src="/ui/dev-floating/pilot.js"></script>\n  <link rel="stylesheet" href="/ui/dev-floating/pilot.css" />\n';
 const html=fs.readFileSync(path.join(web,'index.html'),'utf8');assert.ok(html.includes(addition));assert.equal(html.replace(addition,''),fs.readFileSync(path.join(fixtures,'dev-index-f0a732e3.html'),'utf8'));
 const js=fs.readFileSync(path.join(web,'ui/dev-floating/pilot.js'),'utf8');assert.doesNotMatch(js,/requestAnimationFrame|addEventListener|setInterval|ResizeObserver|MutationObserver|canvas|feDisplacementMap/);
 const css=fs.readFileSync(path.join(web,'ui/dev-floating/pilot.css'),'utf8');assert.doesNotMatch(css,/filter:\s*(?:blur|url)|text-shadow|text-stroke|will-change|animation:/);
});
test('actual dev entry retains search, native menus, continuous anchors, theme and mobile controls in the pilot',{skip:!runtime,timeout:180000},async()=>{
 const {start,project,backlog}=require('../docs/evidence/a65-performance/fixture.cjs'),{chromium}=require(runtime),server=await start(web);let browser;const cases=[];
 try{browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});
 for(const width of [1440,390])for(const theme of ['light','dark']){
  const ctx=await browser.newContext({viewport:{width,height:844},hasTouch:width===390,isMobile:width===390,colorScheme:theme,reducedMotion:'reduce',serviceWorkers:'block'});
  await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
  await ctx.addInitScript(({theme,backlog})=>{for(const[k,v]of Object.entries({'task-mecca-theme':theme,'task-mecca-language':'en','task-mecca-backlog-folder':backlog,'task-mecca-list-page-size':'20','task-mecca-list-page-mode-v1':'manual'}))localStorage.setItem(k,v)},{theme,backlog});
  const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));const query=new URLSearchParams({project,backlog,floating:'1',view:'backlog',page_size:'20'});
  await page.goto(server.origin+'/?'+query);await page.waitForFunction(()=>document.querySelectorAll('.task-row').length===20);await page.keyboard.press('/');assert.equal(await page.locator('#search').evaluate(e=>e===document.activeElement),true);await page.locator('#search').fill('Needle');await page.waitForFunction(()=>{const rs=[...document.querySelectorAll('.task-row')];return rs.length===20&&rs.every(r=>r.textContent.includes('Needle'))});
  const plate=await page.locator('#content').evaluate(e=>{const b=e.getBoundingClientRect(),h=document.querySelector('.topbar').getBoundingClientRect();return {background:getComputedStyle(e).backgroundColor,x:b.x,right:b.right,headerX:h.x,headerRight:h.right,rowFilters:[...e.querySelectorAll('.task-row')].some(r=>getComputedStyle(r).filter!=='none'||getComputedStyle(r).backdropFilter!=='none')};});
  assert.notEqual(plate.background,'rgba(0, 0, 0, 0)','toolbar, columns and rows have a shared reading face');assert.ok(Math.abs(plate.x-plate.headerX)<1&&Math.abs(plate.right-plate.headerRight)<1);assert.equal(plate.rowFilters,false);
  await page.locator('.task-row').first().focus();assert.equal(await page.locator('.task-row').first().evaluate(e=>e.matches(':focus-visible')&&getComputedStyle(e).outlineStyle==='solid'&&parseFloat(getComputedStyle(e).outlineWidth)>=2),true,'keyboard focus remains distinct from the quieter selected row');
  // Exercise the unchanged dev Auto fitter separately from the fixed-size
  // historical workflow fixture, including a viewport-height change.
  await page.locator('#listPageSize').selectOption('auto',{force:true});
  await page.waitForFunction(()=>state.listPageMode==='auto'&&state.autoListPageSize>0&&state.autoListPageSize<20&&document.querySelectorAll('.task-row').length===Math.min(20,state.autoListPageSize));
  const autoBefore=await page.evaluate(()=>state.autoListPageSize);await page.setViewportSize({width,height:600});await page.waitForFunction(before=>state.autoListPageSize<before||before===1,autoBefore);
  await page.waitForFunction(()=>document.querySelectorAll('.task-row').length===Math.min(20,state.autoListPageSize));
  const autoSmall=await page.evaluate(()=>state.autoListPageSize);
  await page.setViewportSize({width,height:844});
  // Dev deliberately retains a conservative row-height budget. Restoring height
  // must grow the page and fit it, not reproduce the earlier count exactly.
  await page.waitForFunction(({small,before})=>{const rows=[...document.querySelectorAll('.task-row')],last=rows.at(-1),bar=document.querySelector('.shortcut-bar');return state.listPageMode==='auto'&&!state.listRevalidating&&(state.autoListPageSize>small||before===1)&&rows.length===Math.min(20,state.autoListPageSize)&&last.getBoundingClientRect().bottom<=innerHeight-(bar?.getBoundingClientRect().height||0);},{small:autoSmall,before:autoBefore});
  const autoRestored=await page.evaluate(()=>({size:state.autoListPageSize,rowHeightBudget:autoPageRowHeightBudget,height:innerHeight,lastBottom:document.querySelector('.task-row:last-child').getBoundingClientRect().bottom}));
await page.locator('#listPageSize').locator('..').locator('button.tm-combo-trigger').click();await page.getByRole('listbox').getByRole('option',{name:'20',exact:true}).click();await page.waitForFunction(()=>state.listPageMode==='manual'&&document.querySelectorAll('.task-row').length===20);
  // Real pointer opens the native custom-combo; Escape closes it without a route change.
  const picker=page.locator('#palettePicker').locator('..').locator('button').first();await picker.click();await page.keyboard.press('Escape');
  await page.locator('[data-theme-choice="system"]').click();await page.emulateMedia({colorScheme:theme==='light'?'dark':'light'});await page.waitForFunction(t=>document.documentElement.dataset.theme===t,theme==='light'?'dark':'light');
  if(width===390){await page.locator('#sidebarToggle').click();await page.locator('.app-shell.mobile-nav-open').waitFor();assert.equal(await page.locator('#sidebarToggle').getAttribute('aria-expanded'),'true');await page.locator('#sidebarToggle').click();await page.waitForFunction(()=>!document.querySelector('.app-shell').classList.contains('mobile-nav-open'));}
  else{await page.locator('#sidebarToggle').click();await page.locator('#sidebarToggle').click();}
  await page.locator('.task-row').first().click();await page.locator('.detail-toc-nav a').first().waitFor();const signature=await page.evaluate(()=>({ids:[...document.querySelectorAll('.detail-section[id]')].map(e=>e.id),links:[...document.querySelectorAll('.detail-toc-nav a')].map(e=>e.getAttribute('href')),icons:[...document.querySelectorAll('#nav .nav-icon')].map(e=>e.innerHTML)}));
  assert.ok(signature.ids.length>=7);if(width===390){await page.locator('.detail-toc-fab').click();await page.locator('.detail-layout.toc-open').waitFor();}
  const link=page.locator('.detail-toc-nav a').last(),target=await link.getAttribute('data-toc-target');await link.click();await page.waitForFunction(t=>location.hash==='#'+t,target);
  await page.reload();await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.dataset.floatingPilot),'true');assert.ok(page.url().includes('#'+target));
  const url=new URL(page.url());url.searchParams.set('floating','0');await page.goto(url.toString());await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);assert.deepEqual(await page.evaluate(()=>({ids:[...document.querySelectorAll('.detail-section[id]')].map(e=>e.id),links:[...document.querySelectorAll('.detail-toc-nav a')].map(e=>e.getAttribute('href')),icons:[...document.querySelectorAll('#nav .nav-icon')].map(e=>e.innerHTML)})),signature);
  assert.deepEqual(errors,[]);assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));cases.push({width,theme,search:true,keyboard:true,comboPointer:true,system:true,sidebar:true,anchorReload:true,defaultDevParity:true,sharedListReadingFace:true,autoAndResize:true,autoBefore,autoSmall,autoRestored,keyboardFocus:true,errors});await ctx.close();
 }
 }finally{await browser?.close();await server.close();const out=process.env.WORKFLOW_EVIDENCE_DIR;if(out){fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'dev-floating-mechanics.json'),JSON.stringify({cases,sourceBaseline:baseline.originDev,browserClosed:!browser?.isConnected(),serverListening:server.isListening()},null,2));}}
});
