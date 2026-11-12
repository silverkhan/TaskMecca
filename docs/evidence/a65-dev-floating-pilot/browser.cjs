'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {start,project,backlog}=require('../a65-performance/fixture.cjs');
const wf=require('../../../tests/helpers/workflow_fixture.cjs');
const {chromium}=require('/Users/hanati/.codex/worktrees/pr-348-glass-refinement/TaskMecca/handoff-evidence/pr348/runtime/node_modules/playwright');
const phase=process.argv[2]||'batch',out=path.join(__dirname,phase);if(fs.existsSync(out))throw Error('preserve existing output');fs.mkdirSync(out,{recursive:true});
const task=wf.task();task.document.contract_kind='defined';
const paragraphs=(label,n)=>Array.from({length:n},(_,i)=>`${label} ${i+1}. 기존 dev UI와 동일한 내용으로 연속 문서·앵커 탐색을 확인합니다. The same synthetic content is used in both views.`).join('\n\n');
task.document.requirements={background:paragraphs('Background',8),goal:'기존 메뉴와 본문을 그대로 유지한 플로팅 소재 시안',requirements:paragraphs('Requirement',12),scope_in:paragraphs('Scope',3),scope_out:'운영 서비스 변경 제외',constraints:'No optical runtime. No row blur.',acceptance:'- [ ] 기존 앵커와 플로팅 인덱스 보존'};task.document.notes=paragraphs('Notes',8);task.document.result=paragraphs('Result',8);task.document.verification=paragraphs('Verification',8);task.raw_markdown=Object.values(task.document.requirements).join('\n\n');
fs.writeFileSync(path.join(out,'fixture.json'),JSON.stringify(task,null,2));
const web=path.resolve('goassets/template/_task_mecca/framework/web');const sources={dev:'/private/tmp/task-mecca-a65-baseline-f0a732e3/goassets/template/_task_mecca/framework/web',pilot:web};
let browser;const servers=[],records=[],signatures={};
(async()=>{try{
 browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 if(phase.startsWith('batch-pilot')||phase==='confirm'){for(const r of JSON.parse(fs.readFileSync(path.join(__dirname,'batch/results.json')))){signatures[r.width+'-'+r.theme]=r.signature;}}
 for(const [variant,dir] of Object.entries(sources)){if((phase.startsWith('batch-pilot')||phase==='confirm')&&variant==='dev')continue;const server=await start(dir);servers.push(server);
 for(const width of [1440,1280,320,390,430,800])for(const theme of ['light','dark']){
 const ctx=await browser.newContext({viewport:{width,height:width<=430?844:1000},colorScheme:theme,hasTouch:width<=800,isMobile:width<=800,serviceWorkers:'block'});
 await ctx.route('**/*',r=>{const u=new URL(r.request().url());if(u.origin!==server.origin)return r.abort();if(u.pathname.startsWith('/api/tasks/'))return r.fulfill({json:{...task,id:decodeURIComponent(u.pathname.split('/').pop())}});return r.continue()});
 await ctx.addInitScript(({theme,backlog})=>{localStorage.setItem('task-mecca-theme',theme);localStorage.setItem('task-mecca-language','ko');localStorage.setItem('task-mecca-backlog-folder',backlog);localStorage.setItem('task-mecca-list-page-size','20')},{theme,backlog});
 const page=await ctx.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
 const params=new URLSearchParams({project,backlog,...(variant==='pilot'?{floating:'1'}:{})});const url=server.origin+'/tasks/FLOW-060?'+params;
 await page.goto(url);await page.locator('.detail-toc-nav a').first().waitFor();await page.waitForFunction(()=>document.querySelector('#activityMetricsBody')?.textContent.length>80);await page.evaluate(()=>document.fonts.ready);
 const signature=await page.evaluate(()=>({nav:[...document.querySelectorAll('#nav .nav-item')].map(n=>({text:n.textContent.trim(),view:n.dataset.view,icon:n.querySelector('svg')?.outerHTML})),sections:[...document.querySelectorAll('.detail-section[id]')].map(n=>({id:n.id,label:n.dataset.tocLabel,head:n.querySelector('h2')?.textContent})),anchors:[...document.querySelectorAll('.detail-toc-nav a')].map(n=>({href:n.getAttribute('href'),text:n.textContent})),tables:[...document.querySelectorAll('table')].map(n=>n.querySelectorAll('tr').length),inputs:[...document.querySelectorAll('input,select,textarea')].map(n=>({tag:n.tagName,type:n.type,id:n.id})),goal:document.querySelector('#requirements')?.textContent||document.querySelector('#legacy-task')?.textContent}));
 const key=width+'-'+theme;if(variant==='dev')signatures[key]=signature;else assert.deepEqual(JSON.parse(JSON.stringify(signature)),signatures[key],'unchanged dev semantic DOM');
 const geometry=await page.evaluate(()=>({overflow:document.documentElement.scrollWidth-innerWidth,sidebar:document.querySelector('.sidebar').getBoundingClientRect().toJSON(),header:document.querySelector('.topbar').getBoundingClientRect().toJSON(),detail:document.querySelector('.detail-layout').getBoundingClientRect().toJSON(),filterNodes:[...document.querySelectorAll('body *')].filter(e=>{const s=getComputedStyle(e);return s.filter!=='none'||s.backdropFilter!=='none'}).map(e=>({tag:e.tagName,cls:e.className,filter:getComputedStyle(e).filter,backdrop:getComputedStyle(e).backdropFilter})),pilot:document.documentElement.dataset.floatingPilot||null}));
 assert.ok(geometry.overflow<=1,'no root overflow');for(const part of ['sidebar','header']){assert.ok(geometry[part].x>=0&&geometry[part].right<=width+1,part+' stays inside viewport');}if(variant==='pilot'){assert.equal(geometry.pilot,'true');assert.equal(await page.locator('.topbar').evaluate(e=>getComputedStyle(e).backdropFilter),'none','pilot header adds no backdrop filtering');assert.ok(geometry.filterNodes.every(e=>e.cls!=='topbar'),'only inherited dev filters remain');assert.equal(await page.locator('[data-light-surface],.material-transmission,svg filter').count(),0,'no old optics mounted');}
 await page.screenshot({path:path.join(out,`${variant}-${key}.png`)});
 // Actual dev floating-index controls and anchor traversal, never a replacement navigator.
 if(width<=1280){await page.locator('.detail-toc-fab').click();await page.locator('.detail-layout.toc-open').waitFor();}
 const target=await page.locator('.detail-toc-nav a').last().getAttribute('data-toc-target');await page.locator('.detail-toc-nav a').last().click();await page.waitForFunction(id=>decodeURIComponent(location.hash.slice(1))===id,target);await page.evaluate(()=>new Promise(r=>setTimeout(r,400)));assert.ok(await page.locator('#'+target).isVisible());
 await page.reload();await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(decodeURIComponent(new URL(page.url()).hash.slice(1)),target);
 await page.setViewportSize({width:width===390?430:width,height:760});assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
 if(variant==='pilot'){
  await page.goto(url+'&background=solid');await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>getComputedStyle(document.body,'::before').content),'none');
  await page.goto(url+'&background=image');await page.locator('.detail-toc-nav a').first().waitFor();assert.ok((await page.evaluate(()=>getComputedStyle(document.body,'::before').backgroundImage)).includes('cozy-beige'));
  await page.goto(url.replace('floating=1','floating=0'));await page.locator('.detail-toc-nav a').first().waitFor();assert.equal(await page.evaluate(()=>document.documentElement.hasAttribute('data-floating-pilot')),false);
 }
 assert.deepEqual(errors,[]);records.push({variant,width,theme,signature,geometry,anchorReload:true,resize:true,errors});fs.writeFileSync(path.join(out,'results.json'),JSON.stringify(records,null,2));console.log('PASS',variant,width,theme);await ctx.close();
 }
 }
}finally{await browser?.close();for(const s of servers)await s.close();fs.writeFileSync(path.join(out,'cleanup.json'),JSON.stringify({browserClosed:!browser?.isConnected(),servers:servers.map(s=>({origin:s.origin,listening:s.isListening()}))},null,2));}})().catch(e=>{fs.writeFileSync(path.join(out,'failure.txt'),String(e.stack));console.error(e);process.exitCode=1});
