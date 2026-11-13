// Offline actual-DOM/CSS test. Uses an already-installed Puppeteer module;
// never starts Task Mecca, loads its URL, or reads operational ledgers.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const puppeteer=require(process.env.PUPPETEER_MODULE||'puppeteer');
const web=path.resolve('goassets/template/_task_mecca/framework/web');
const source=fs.readFileSync(path.join(web,'app.js'),'utf8'),css=fs.readFileSync(path.join(web,'style.css'),'utf8');
const boot=source.slice(0,source.indexOf('\ntranslateChrome();'));
(async()=>{
 const browser=await puppeteer.launch({headless:true,pipe:true,executablePath:process.env.BROWSER_EXECUTABLE||undefined,args:[...(process.env.BROWSER_OS_SANDBOX==='1'?['--no-sandbox']:[]),'--disable-background-networking','--disable-component-update','--no-first-run','--disable-sync']});
 const results=[];let network=0;
 try{
 for(const language of ['ko','en'])for(const theme of ['light','dark'])for(const width of [390,1280]){
  const page=await browser.newPage();await page.setRequestInterception(true);page.on('request',request=>{network++;request.abort();});const errors=[];page.on('pageerror',error=>errors.push(String(error)));
  await page.setViewport({width,height:900,deviceScaleFactor:1});
  await page.setContent(`<!doctype html><html lang="${language}" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>${css}</style></head><body><main><div id="userAttention" class="user-attention"></div></main><script>Object.defineProperty(window,'localStorage',{value:{getItem:()=>null,setItem(){}}});${boot.replaceAll('</script>','<\\/script>')}\nstate.language=${JSON.stringify(language)};globalThis.fixtureItems=Array.from({length:20000},(_,i)=>({id:'fixture-'+i,project:i%2?'/offline/alpha/long-project-name':'/offline/beta',kind:i%7?'monitor_gap':'normal_exit',attempt_id:i%13?'run-'+i%100:'',detected_at:'2026-11-'+String(1+i%12).padStart(2,'0')+'T12:00:00Z',evidence:'offline observation only'}));globalThis.fixtureStart=performance.now();renderOperationBanner({snapshot_at:'2026-11-13T00:00:00Z',observation_history:fixtureItems,active:[{id:'warning',project:'/offline',kind:'no_signal',evidence:'Current warning retained'}],diagnostics:[{id:'diagnostic',project:'/offline',evidence:'Diagnosis retained'}],user_holds:[]});globalThis.closedMs=performance.now()-fixtureStart;</script></body></html>`);
  assert.deepEqual(errors,[]);assert.equal(await page.$$eval('.operation-raw-fields',els=>els.length),0);
  await page.click('.common-attention > summary');
  assert.equal(await page.$$eval('.operation-raw-fields',els=>els.length),0);
  // Genuine browser keyboard default action opens the lazy history disclosure.
  await page.focus('[data-observation-section="history"] > summary');await page.keyboard.press('Enter');
  await page.waitForSelector('[data-observation-select]');
  assert.equal(await page.$$eval('.operation-history-groups > li',els=>els.length),10);
  assert.equal(await page.$$eval('.operation-raw-fields',els=>els.length),0);
  const groupDomElements=await page.$$eval('*',nodes=>nodes.length);
  await page.focus('[data-observation-select]');await page.keyboard.press('Enter');await page.waitForSelector('.operation-raw-fields');
  assert.equal(await page.$$eval('.operation-raw-fields',els=>els.length),20);
  assert.equal(await page.evaluate(()=>document.activeElement.tagName),'H3');
  const selectedDomElements=await page.$$eval('*',nodes=>nodes.length);assert.ok(selectedDomElements<1000);
  await page.focus('[data-observation-page="raw"][data-page="2"]');await page.keyboard.press('Enter');
  await page.waitForFunction(()=>document.querySelector('.operation-history-raw [role="status"]').textContent.includes('21–40'));
  assert.match(await page.evaluate(()=>document.activeElement.dataset.observationFocus),/raw-next/);
  await page.focus('[data-observation-jump="raw"] input');await page.keyboard.press('ArrowUp');await page.keyboard.press('Enter');
  await page.waitForFunction(()=>document.querySelector('.operation-history-raw [role="status"]').textContent.includes('41–60'));
  await page.focus('.operation-raw-fields summary');await page.keyboard.press('Enter');assert.equal(await page.$eval('.operation-raw-fields',node=>node.open),true);
  // Scope change discards only the rendered selection, never the source data.
  await page.focus('[data-observation-project="history"]');await page.keyboard.type('/offline/beta');await page.keyboard.press('Enter');
  await page.waitForFunction(()=>!document.querySelector('.operation-raw-fields'));
  assert.equal(await page.evaluate(()=>fixtureItems.length),20000);
  const metrics=await page.evaluate(()=>({closedMs,domElements:document.querySelectorAll('*').length,rawRows:document.querySelectorAll('.operation-raw-fields').length,groupRows:document.querySelectorAll('.operation-history-groups > li').length,width:innerWidth,scrollWidth:document.documentElement.scrollWidth,source:document.querySelector('.operation-history-source').textContent,note:document.querySelector('.operation-history-note').textContent,background:getComputedStyle(document.body).backgroundColor,focus:document.activeElement.dataset.observationFocus,controls:[...document.querySelectorAll('.operation-history-pager button,.operation-history-scope input')].map(node=>({height:node.getBoundingClientRect().height,outline:getComputedStyle(node).outlineStyle})),warning:document.body.textContent.includes('Current warning retained')}));
  assert.ok(metrics.domElements<1000,JSON.stringify(metrics));assert.ok(metrics.scrollWidth<=width+1,JSON.stringify(metrics));assert.ok(metrics.closedMs<2000,JSON.stringify(metrics));assert.ok(metrics.warning);assert.match(metrics.source,/UTC/);assert.match(metrics.focus,/project/);if(width===390)assert.ok(metrics.controls.every(control=>control.height>=44));
  await page.keyboard.press('Tab');assert.equal(await page.evaluate(()=>document.activeElement.hasAttribute('data-observation-project-apply')),true);results.push({language,theme,width,groupDomElements,selectedDomElements,...metrics});await page.close();
 }
 assert.equal(network,0);console.log(JSON.stringify({networkRequests:network,cases:results},null,2));
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
