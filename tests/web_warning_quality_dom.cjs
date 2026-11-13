// Offline DOM/CSS verification only: no app startup, service control or HTTP.
const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const puppeteer=require(process.env.PUPPETEER_MODULE||'puppeteer');
const root=path.resolve(__dirname,'..'),web=path.join(root,'goassets/template/_task_mecca/framework/web');
const source=fs.readFileSync(path.join(web,'app.js'),'utf8'),css=fs.readFileSync(path.join(web,'style.css'),'utf8');
const out=path.resolve(process.argv[2]||path.join(root,'.artifacts/a71-pairi/dom'));fs.mkdirSync(out,{recursive:true});
function markup(language){
 const element={innerHTML:'',querySelector:()=>({addEventListener(){}})};
 const ctx=vm.createContext({URLSearchParams,location:{search:''},navigator:{language},document:{querySelector:s=>s==='#userAttention'?element:null},localStorage:{getItem:()=>null,setItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.app={state,renderOperationBanner};',ctx);
 ctx.app.state.language=language;
 const row=(i,task,category)=>({id:'fixture-'+i,project:'/fixture',task_id:task,attempt_id:'fixture-'+i+'漢字🙂العربية'+('x'.repeat(200)),kind:'runtime_unknown',quality:'verification_required',evidence:'Evidence <must stay text>',detected_at:'2026-11-13T00:00:00Z',last_observed_at:'2026-11-13T00:00:00Z',category});
 ctx.app.renderOperationBanner({active:[row(0,'F-DOING','current_operation')],user_holds:Array.from({length:5},(_,i)=>({...row(i+1,'F-HOLD','user_hold'),hold_reason:'Explicit user decision',resume_condition:'User chooses policy'})),observation_history:Array.from({length:11},(_,i)=>row(i+6,'F-DONE','observation_history')),diagnostics:Array.from({length:4},(_,i)=>row(i+17,'','insufficient_evidence'))});
 return element.innerHTML;
}
(async()=>{
 const browser=await puppeteer.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE||'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',args:['--disable-background-networking','--disable-component-update','--no-first-run']});
 const reports=[];try{
 const page=await browser.newPage();let network=0;await page.setRequestInterception(true);page.on('request',r=>{network++;r.abort()});
 for(const [id,width,language,theme,zoom] of [['desktop',1440,'ko','light',1],['narrow',390,'en','dark',1],['mobile',320,'ko','light',1],['zoom',640,'en','dark',2]]){
  await page.setViewport({width,height:1000,deviceScaleFactor:1});
  await page.setContent(`<html lang="${language}" data-theme="${theme}"><head><meta charset="utf-8"><style>${css}</style></head><body style="zoom:${zoom}"><div id="userAttention">${markup(language)}</div></body></html>`);
  assert.equal(await page.$$eval('.operation-observation-details',els=>els.length),3);
  await page.focus('.common-attention > summary');await page.keyboard.press('Enter');assert.equal(await page.$eval('.common-attention',el=>el.open),true);
  await page.$$eval('.operation-observation-details',els=>els.forEach(el=>{el.open=true}));
  const metrics=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,rows:[...document.querySelectorAll('.operation-observation-details')].map(el=>el.querySelectorAll('li').length),allDetailsOpen:[...document.querySelectorAll('details')].every(el=>el.open),text:document.querySelector('#userAttention').textContent}));
  assert.ok(metrics.scrollWidth<=width+1,`${id}: horizontal overflow ${metrics.scrollWidth}`);assert.deepEqual(metrics.rows,[5,11,4]);assert.ok(metrics.allDetailsOpen);assert.match(metrics.text,/Evidence <must stay text>/);
  await page.screenshot({path:path.join(out,id+'.png'),fullPage:true});reports.push({id,width,language,theme,zoom,scrollWidth:metrics.scrollWidth,rows:metrics.rows,keyboardDisclosure:true});
 }
 assert.equal(network,0);fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({networkRequests:network,cases:reports},null,2));console.log(JSON.stringify({networkRequests:network,cases:reports}));
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exitCode=1});
