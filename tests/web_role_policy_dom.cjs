// Real offline DOM only: no HTTP fixture server, live API, or operating service.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'/tmp/taskmecca-a7275-browser/node_modules/playwright-core');
const web=path.resolve('goassets/template/_task_mecca/framework/web'),source=fs.readFileSync(path.join(web,'app.js'),'utf8'),css=fs.readFileSync(path.join(web,'style.css'),'utf8');
const panel=source.slice(source.indexOf('function rolePolicyPanel('),source.indexOf('function detailView('));
assert.ok(panel.includes('rolePolicyPanel'));
const policy={implementation_owner:{role:'worker',agent_path:'/root/controller/kkobugi',native_id:'actual-implementer-native'},operational_owner:{role:'controller',agent_path:'/root/controller'},review_owner:{role:'worker',agent_path:'/root/controller/reviewer',native_id:'actual-separate-native'},workspace:'/isolated/actual/source/'+('long-segment-'.repeat(15)),method:'worker',scope:['internal/product/source.go','tests/source.test.cjs']};
const cases=[
 {policy,implementation:'worker',review:'unknown',review_reason:'actual_reviewer_and_source_required'},
 {policy,implementation:'worker',review:'self',review_reason:'same_native_self_review'},
 {policy,implementation:'worker',review:'independent',review_reason:'supported_acceptance_receipt_current_source'},
 {policy:{...policy,implementation_owner:{role:'controller',agent_path:'/root/controller',native_id:'actual-implementer-native'},method:'controller_exception',exception:{reason:'Scoped repair <img src=x onerror="window.injected=true">',evidence:'actual user decision reference',scope:policy.scope,resume_when:'Bind exact reviewer and approve restricted policy.'}},implementation:'controller_exception',review:'unknown',exception_approved:false},
 {implementation:'worker_default',review:'unknown',legacy:true},
 {policy,implementation:'worker',review:'unknown',review_reason:'review_result_report_or_source_changed'},
 {implementation:'unknown',review:'unknown',error:'role_policy_supplement_requires_current_contract_confirmation'},
];
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE,args:['--no-sandbox','--disable-background-networking','--disable-component-update','--disable-sync','--no-first-run']});
 let network=0;const results=[];fs.mkdirSync('.artifacts/a78/render',{recursive:true});
 try{for(const language of ['ko','en'])for(const theme of ['light','dark'])for(const width of [390,1280]){
  const page=await browser.newPage({viewport:{width,height:900},deviceScaleFactor:1});const errors=[];page.on('pageerror',e=>errors.push(String(e)));await page.route('**/*',r=>{network++;return r.abort()});
  await page.setContent(`<!doctype html><html lang="${language}" data-theme="${theme}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>${css}</style></head><body><main style="padding:20px;max-width:1100px;margin:auto" id="mount"></main><script>const state={language:${JSON.stringify(language)}};const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));${panel.replaceAll('</script>','<\\/script>')}document.querySelector('#mount').innerHTML=${JSON.stringify(cases)}.map((role_policy,i)=>rolePolicyPanel({id:'fixture-'+i,role_policy}).replace('id="role-policy"','id="role-policy-'+i+'"')).join('');</script></body></html>`);
  assert.equal(await page.locator('.role-policy-section').count(),cases.length);assert.equal(await page.locator('img').count(),0);assert.equal(await page.evaluate(()=>!!window.injected),false);
  const text=await page.locator('#mount').textContent();assert.match(text,language==='ko'?/독립 검수 확인/:/Independent review verified/);assert.match(text,language==='ko'?/자기 검수/:/Self review/);assert.match(text,language==='ko'?/승인 미확인 · 보류/:/Approval unknown · hold/);assert.match(text,language==='ko'?/과거 기록/:/legacy record/);
  assert.equal(await page.locator('[role="status"]').count(),1);
  const details=page.locator('.role-policy-section').nth(5).locator('details');await details.locator('summary').focus();await page.keyboard.press('Enter');assert.equal(await details.evaluate(n=>n.open),true);assert.match(await details.textContent(),/review_result_report_or_source_changed/);await page.keyboard.press('Enter');assert.equal(await details.evaluate(n=>n.open),false);
  const metrics=await page.evaluate(()=>({width:innerWidth,scrollWidth:document.documentElement.scrollWidth,dom:document.querySelectorAll('*').length,background:getComputedStyle(document.body).backgroundColor,color:getComputedStyle(document.body).color,fontSize:getComputedStyle(document.body).fontSize,focusOutline:getComputedStyle(document.activeElement).outlineStyle}));
  assert.ok(metrics.scrollWidth<=width+1,JSON.stringify(metrics));assert.ok(metrics.dom<500);assert.notEqual(metrics.focusOutline,'none');assert.deepEqual(errors,[]);
  await page.screenshot({path:`.artifacts/a78/render/${language}-${theme}-${width}.png`,fullPage:true});results.push({language,theme,states:cases.length,...metrics});await page.close();
 }}finally{await browser.close()}
 assert.equal(network,0);fs.writeFileSync('.artifacts/a78/dom-results.json',JSON.stringify({networkRequests:network,cases:results},null,2));console.log(JSON.stringify({networkRequests:network,cases:results},null,2));
})().catch(e=>{console.error(e);process.exitCode=1});
