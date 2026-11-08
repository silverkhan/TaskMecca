const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const fixture=require('./helpers/a65_fixture.cjs'),runtime=process.env.WORKFLOW_PLAYWRIGHT_MODULE;
if(process.env.WORKFLOW_REQUIRE_BROWSER==='1'&&!runtime)throw Error('Required isolated return-to-Dev browser runtime missing');
const target=()=>({kind:'pr',channel:'dev',pr_number:9001,head_sha:'1'.repeat(40),original_dev_version:'0.0.0-dev.synthetic',original_dev_sha256:'2'.repeat(64),original_dev_backup:'/synthetic/retained-dev',installed_at:'2026-11-08T07:00:00Z'});
const env=t=>({target:t,transition:null,switch_supported:true,os:'darwin'});
const health=(t,boot='synthetic-pr-boot')=>({ok:true,instance_id:'synthetic-fixture-instance',boot_id:boot,version:t.kind==='dev'?'0.0.0-dev.synthetic':'synthetic-pr-build',execution_target:t,preview_mode:false});
test('synthetic native PR banner and consent/return/error flows use only owned ephemeral fixture',{skip:!runtime,timeout:90000},async()=>{
 const server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web')),out=path.resolve(process.env.RETURN_DEV_EVIDENCE_DIR||'docs/evidence/a65-return-dev/browser');fs.mkdirSync(out,{recursive:true});let browser;
 const report={synthetic:true,operating_environment_mutations:0,mechanism:'Native Playwright interactions with same-origin synthetic API routes on owned ephemeral fixture; no actual managed return executed.',cases:[],pageErrors:[],outsideFixtureRequests:[]};
 try{
  browser=await require(runtime).chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});
  async function open({viewport={width:1440,height:1000},mode='light',language='ko',postStatus=200,blockPost=false,badHealth=false,transition=null}={}){
   const context=await browser.newContext({viewport,hasTouch:viewport.width<600,isMobile:viewport.width<600,serviceWorkers:'block'});let t=target(),e=env(t),h=health(t),posts=0,dialogCount=0,releasePost,signalPost;
   e.transition=transition;if(badHealth)delete h.boot_id;
   const postStarted=new Promise(r=>signalPost=r),postGate=blockPost?new Promise(r=>releasePost=r):Promise.resolve();
   await context.addInitScript(({mode,language})=>{localStorage.setItem('task-mecca-smoke-mode',mode);localStorage.setItem('task-mecca-language',language)}, {mode,language});
   await context.route('**/*',async route=>{const request=route.request(),u=new URL(request.url());if(!request.url().startsWith(server.origin+'/')){report.outsideFixtureRequests.push(request.url());await route.abort();return}
    if(u.pathname==='/api/environment')return route.fulfill({json:e});
    if(u.pathname==='/api/health')return route.fulfill({json:h});
    if(u.pathname==='/api/environment/return-dev'){
     assert.equal(request.method(),'POST');assert.equal(request.headers()['x-task-mecca-action'],'1');assert.equal(request.headers()['origin'],server.origin);assert.deepEqual(request.postDataJSON(),{approve:true});posts++;signalPost();await postGate;
     if(postStatus===403)return route.fulfill({status:403,json:{error:'Synthetic unmanaged fixture: PR switching requires managed Dev Web'}});
     if(postStatus===409)return route.fulfill({status:409,json:{error:'Synthetic retained Dev baseline missing'}});
     t={kind:'dev',channel:'dev'};e=env(t);h=health(t,'synthetic-dev-boot');return route.fulfill({json:{ok:true,restarting:true}});
    }
    return route.continue();
   });
   const page=await context.newPage();page.on('pageerror',error=>report.pageErrors.push(error.message));await page.goto(server.origin);await page.locator('#executionEnvironment').waitFor();
   await page.waitForFunction(()=>!document.querySelector('#executionRefresh')?.disabled);
   await page.evaluate(()=>{const n=document.createElement('aside');n.id='syntheticFixtureLabel';n.textContent='SYNTHETIC FIXTURE · 실제 환경 전환 없음';n.style.cssText='position:fixed;bottom:4px;right:8px;z-index:1000;max-width:calc(100% - 16px);padding:3px 7px;font:11px sans-serif;background:Canvas;color:CanvasText;border:1px solid GrayText';document.body.append(n)});
   return{page,context,postStarted,release:()=>releasePost?.(),posts:()=>posts,dialogs:()=>dialogCount,dialog:accept=>{page.once('dialog',async d=>{dialogCount++;assert.equal(d.type(),'confirm');assert.match(d.message(),/9001/);assert.match(d.message(),/0\.0\.0-dev\.synthetic/);accept?await d.accept():await d.dismiss()})}};
  }
  for(const [name,viewport,mode,language] of [
   ['desktop-light-ko',{width:1440,height:1000},'light','ko'],['desktop-dark-en',{width:1280,height:900},'dark','en'],['mobile-light-ko',{width:390,height:844},'light','ko'],['mobile-dark-en',{width:320,height:720},'dark','en']]){
   const x=await open({viewport,mode,language}),{page}=x;await page.locator('#prReturnDev:enabled').waitFor();assert.equal(await page.locator('.execution-sha').innerText(),'1'.repeat(40));assert.match(await page.locator('#executionEnvironment').innerText(),/PR #9001/);
   const size=await page.locator('#prReturnDev').boundingBox();assert.ok(size.height>=44);assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),true);
   await page.screenshot({path:path.join(out,name+'.png'),fullPage:false});
   x.dialog(false);await page.locator('#prReturnDev').click();await page.waitForFunction(()=>document.activeElement?.id==='prReturnDev');assert.equal(x.posts(),0);assert.equal(x.dialogs(),1);
   report.cases.push({name,fullPRSHAVisible:true,overflow:false,returnTarget44:true,nativeCancelPostCount:x.posts(),focusRestored:true});await x.context.close();
  }
  const denied=await open({postStatus:403});denied.dialog(true);await denied.page.locator('#prReturnDev').click();await denied.page.waitForFunction(()=>document.querySelector('.execution-status')?.textContent.includes('Synthetic unmanaged'));assert.equal(denied.posts(),1);assert.equal(await denied.page.locator('#prReturnDev').isDisabled(),true);assert.match(await denied.page.locator('#executionEnvironment').innerText(),/거부|rejected/);report.cases.push({name:'unmanaged403',nativeConfirm:true,postCount:denied.posts(),success:false,reloaded:false});await denied.context.close();
  const unknown=await open({badHealth:true});assert.equal(await unknown.page.locator('#prReturnDev').isDisabled(),true);assert.match(await unknown.page.locator('.execution-status').innerText(),/미확인|확인하지/);assert.equal(unknown.posts(),0);report.cases.push({name:'missing-identity',postCount:0,success:false});await unknown.context.close();
  const pending=await open({transition:{phase:'rollback_required'}});assert.equal(await pending.page.locator('#prReturnDev').isDisabled(),true);assert.match(await pending.page.locator('.execution-status').innerText(),/rollback_required/);assert.equal(pending.posts(),0);report.cases.push({name:'pending-transition',postCount:0});await pending.context.close();
  const positive=await open({blockPost:true});positive.dialog(true);await positive.page.locator('#prReturnDev').click();await positive.postStarted;assert.equal(await positive.page.locator('#prReturnDev').isDisabled(),true);const r=await positive.page.locator('#prReturnDev').boundingBox();await positive.page.mouse.click(r.x+r.width/2,r.y+r.height/2);await positive.page.keyboard.press('Enter');assert.equal(positive.posts(),1);positive.release();await positive.page.waitForFunction(()=>!document.querySelector('#prReturnDev'));assert.equal(positive.posts(),1);assert.equal(positive.dialogs(),1);report.cases.push({name:'native-accepted-return',nativeConfirm:true,headerAndApproveBody:true,postCount:1,duplicatePostCount:0,freshDevBootObserved:true,syntheticReload:true});await positive.context.close();
  assert.deepEqual(report.pageErrors,[]);assert.deepEqual(report.outsideFixtureRequests,[]);
 }finally{if(browser)await browser.close();await server.close();report.cleanup={browserClosed:!!browser,fixtureClosed:true};fs.writeFileSync(path.join(out,'browser-contract.json'),JSON.stringify(report,null,2)+'\n')}
});
