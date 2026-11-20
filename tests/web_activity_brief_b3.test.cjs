// Synthetic exact-origin DTO consumer proof. Backend projection is verified separately.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const spec=require('./fixtures/workflow/activity-brief-b3.json');
const web=path.resolve(__dirname,'../goassets/template/_task_mecca/framework/web');
const source=fs.readFileSync(path.join(web,'app.js'),'utf8');
function load(language='en'){
 const context=vm.createContext({URLSearchParams,AbortController,location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}},fetch(){throw Error('No network');}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.b3={state,workflowBrief,workflowFloatingContent,workflowActivityLayout,workflowPlotHTML,activityDetailPrefs,activityMetricsBody};',context);
 context.b3.state.language=language;return context.b3;
}
function fixture(){
 const p=structuredClone(require('./fixtures/workflow/continuous-activity.json')),row=p.workflow.rows[0],origin=spec.origin;
 row.agent_id=origin.agent_id;row.display_name='togepi';row.display_name_source='bound_runtime_agent_path';
 Object.assign(row.intervals[0],{id:origin.event_id,attempt_id:origin.attempt_id,assignment_id:origin.assignment_id,explanation:spec.comments.short,evidence:'/fixture/long/source/path/original-evidence'});
 row.intervals[1].explanation=spec.comments.english;
 row.role_facts=[{id:'b3-fact',kind:'assigned',at:p.workflow.axis.end,source:'fixture',reason:'Original assignment observed',attempt_id:origin.attempt_id,assignment_id:origin.assignment_id}];
 row.gaps=[{start:row.intervals[0].start,end:row.intervals[0].end,status:'confirmed_wait',kind:'reported_wait',evidence:[{source:'fixture',reason:'Await exact source review',event_id:'b3-wait-original',attempt_id:origin.attempt_id}]}];
 return p;
}
test('exact selected Activity comment wins over long evidence without changing raw content or geometry',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0],i=row.intervals[0],before=JSON.stringify(p),layout=JSON.stringify(a.workflowActivityLayout(row,p.workflow.axis.end),(_,v)=>typeof v==='bigint'?String(v):v);
 const html=a.workflowFloatingContent(row,i,null,false);
 assert.ok(html.indexOf(spec.comments.short)<html.indexOf(i.evidence));assert.match(html,/Worker · togepi/);assert.match(html,/Start/);assert.match(html,/End/);
 assert.equal(a.workflowBrief(i,i.kind,false).text,spec.comments.short);
 assert.equal(JSON.stringify(p),before);assert.equal(JSON.stringify(a.workflowActivityLayout(row,p.workflow.axis.end),(_,v)=>typeof v==='bigint'?String(v):v),layout);
});
test('shared badges preserve the supplied verified reviewer generation without parsing native or official name',()=>{
 const a=load(),p=fixture(),row={...p.workflow.rows[0],role:'reviewer'},i=row.intervals[0];
 const html=a.workflowFloatingContent(row,i,null,false,{session_generation:13,official_name:'D1112_01_Reviewer_togepi_G99'});
 assert.match(html,/Reviewer · togepi · G13/);assert.doesNotMatch(html,/G99/);
});
test('missing and whitespace comments never borrow another interval or current actor description',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0],i=row.intervals[2];
 for(const explanation of [undefined,null,'',' \n ',42]){i.explanation=explanation;const html=a.workflowFloatingContent(row,i,null,false);assert.match(html,/Work description not recorded/);assert.ok(!html.includes(spec.comments.short));assert.ok(!html.includes(spec.comments.english));}
 assert.equal(a.workflowBrief({explanation:spec.comments.short},'execution_report',false).recorded,false);
});
test('facts, wait, unmeasured and collection failures show original evidence without inventing work',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 const fact={row,kind:'fact',value:row.role_facts[0]};assert.match(a.workflowFloatingContent(row,null,fact,false),/Original assignment observed/);
 for(const status of ['confirmed_wait','unmeasured','native_observed','collection_failed']){
  const gap={row,kind:'gap',value:{...row.gaps[0],status,explanation:'MUST NOT INFER WORK'}};
  const html=a.workflowFloatingContent(row,null,gap,false);assert.match(html,/Work description not recorded/);assert.match(html,/Await exact source review/);assert.doesNotMatch(html,/MUST NOT INFER WORK/);
 }
});
test('open and terminal-bounded missing-end phases retain exact explanation and unknown timing',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 for(const kind of ['activity_open','activity_unreported']){const i={...row.intervals[0],kind,end:null,measured:false,duration_ms:null},before=JSON.stringify(i),html=a.workflowFloatingContent(row,i,null,false);assert.match(html,/End unobserved/);assert.ok(html.includes(spec.comments.short));assert.equal(JSON.stringify(i),before);}
});
test('KO/EN long and hostile text is escaped, preserved and isolated from markup',()=>{
 for(const language of ['ko','en']){const a=load(language),p=fixture(),row=p.workflow.rows[0];
  for(const explanation of Object.values(spec.comments)){const i={...row.intervals[0],explanation},html=a.workflowFloatingContent(row,i,null,language==='ko');assert.doesNotMatch(html,/<img|onerror="window/);assert.equal(a.workflowBrief(i,i.kind,language==='ko').text,explanation);}
  const html=a.workflowFloatingContent(row,{...row.intervals[0],explanation:''},null,language==='ko');assert.ok(html.includes(language==='ko'?'작업 설명 미기록':'Work description not recorded'));
 }
});
const runtime=process.env.WORKFLOW_PLAYWRIGHT_MODULE;
if(process.env.WORKFLOW_REQUIRE_BROWSER==='1'&&!runtime)throw Error('Required offline browser runtime missing');
test('shipped DOM: same exact content across hover/focus/pin/tap, safe bounds and refresh',{skip:!runtime,timeout:120000},async()=>{
 const transport=require(runtime),server=await transport.fixture.start(web),out=process.env.WORKFLOW_EVIDENCE_DIR;let browser;
 const report={synthetic:true,cases:[],limits:['No physical touch, assistive technology, other browser engines or OS clipboard proof','Projected fixtures verify consumer only; actual B1 DTO integration requires separate sealed proof']};
 try{
  browser=await transport.chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});report.browser=browser.version();
  for(const language of ['ko','en'])for(const mobile of [false,true]){
   const context=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:1000},hasTouch:true,isMobile:mobile,locale:language==='ko'?'ko-KR':'en-US'}),page=await context.newPage(),errors=[];page.on('pageerror',e=>errors.push(e.message));
   await page.goto(server.origin+'?project='+encodeURIComponent(transport.fixture.project));await page.waitForFunction(()=>typeof state!=='undefined');
   const mount=async data=>{server.state.data=data;await page.evaluate(({data,task,language,mobile,project,backlog})=>{Object.assign(state,{project,backlog,view:'backlog',detail:task.id,detailTask:task,raw:false,loadError:'',language,sidebarMode:'expanded',snapshot:{all_items:{[task.id]:task},counts:{all:1,done:1},access:{status:'full'},workload:{agents:[]},attention:[]},listData:{items:[task],counts:{all:1,done:1}},hub:{projects:[]}});activityDetailViews.clear();activityDetailCache.set(activityDetailKey(task.id,project),data);applyPalette(mobile?'slate':'mecca');applyTheme(mobile?'dark':'light');render();},{data,task:transport.fixture.task(data),language,mobile,project:transport.fixture.project,backlog:transport.fixture.backlog});await page.locator('.workflow-section').evaluate(el=>scrollTo(0,el.getBoundingClientRect().top+scrollY-(document.querySelector('.topbar')?.getBoundingClientRect().height||0)-18));};
   const p=fixture();await mount(p);const bar=page.locator('[data-workflow-interval]').first(),floating=page.locator('.workflow-floating'),content=()=>floating.locator('.workflow-floating-content').textContent();
   await bar.hover();const hover=await content();assert.ok(hover.includes(spec.comments.short));await bar.focus();assert.equal(await content(),hover);await page.keyboard.press('Enter');assert.equal(await content(),hover);assert.equal(await page.locator('#workflowIntervalEvidence').evaluate(el=>el.open),false);await page.keyboard.press('Escape');assert.equal(await bar.evaluate(el=>el===document.activeElement),true);
   await bar.tap();assert.equal(await content(),hover);await page.locator('#workflowIntervalEvidence>summary').click();const raw=await page.locator('#workflowIntervalEvidence pre').textContent();for(const value of Object.values(spec.origin).filter(v=>v!=='FLOW-1'))assert.ok(raw.includes(value));await page.keyboard.press('Escape');
   const marker=page.locator('.workflow-fact-point').first();await marker.focus();const factContent=await content();assert.match(factContent,/Original assignment observed/);await page.keyboard.press('Enter');assert.equal(await content(),factContent);await page.keyboard.press('Escape');
   const gap=page.locator('.workflow-gap-paint[data-workflow-gap="confirmed_wait"]').first();await page.mouse.move(0,0);await gap.focus();const gapContent=await content();assert.match(gapContent,/Await exact source review/);assert.ok(!gapContent.includes(spec.comments.short));await page.keyboard.press('Enter');assert.equal(await content(),gapContent);await page.keyboard.press('Escape');
   for(const key of ['special','long']){const data=fixture();data.workflow.rows[0].intervals[0].explanation=spec.comments[key];await mount(data);await bar.focus();await page.keyboard.press('Enter');assert.equal(await floating.locator('.workflow-brief').textContent(),spec.comments[key]);assert.equal(await floating.locator('img,script').count(),0);assert.equal(await page.evaluate(()=>window.__b3xss),undefined);const bounds=await floating.evaluate(el=>({r:el.getBoundingClientRect().toJSON(),w:innerWidth,h:innerHeight,overflow:getComputedStyle(el).overflowY}));assert.ok(bounds.r.left>=0&&bounds.r.right<=bounds.w+1&&bounds.r.top>=0&&bounds.r.bottom<=bounds.h+1);assert.equal(bounds.overflow,'auto');assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));if(out)await page.screenshot({path:path.join(out,`b3-${language}-${mobile?'mobile':'desktop'}-${key}.png`),animations:'disabled'});await page.keyboard.press('Escape');}
   for(const mode of ['missing','open']){const data=fixture(),i=data.workflow.rows[0].intervals[0];if(mode==='missing')delete i.explanation;else Object.assign(i,{kind:mode==='open'?'activity_open':'activity_unreported',end:null,measured:false,duration_ms:null});await mount(data);await page.mouse.move(0,0);await bar.focus();const text=await content();assert.ok(text.includes(mode==='missing'?(language==='ko'?'작업 설명 미기록':'Work description not recorded'):spec.comments.short));await page.keyboard.press('Enter');assert.equal(await content(),text);await page.keyboard.press('Escape');assert.equal(await bar.evaluate(el=>el===document.activeElement),true);}
   const data=fixture();await mount(data);await bar.focus();await page.keyboard.press('Enter');data.workflow.rows[0].intervals[0].explanation='Changed exact source';server.state.data=data;await page.evaluate(()=>loadTaskActivity('FLOW-1',state.project));await page.waitForFunction(()=>!activityDetailPending.size);assert.equal(await page.locator('.workflow-floating.is-pinned').count(),0);assert.deepEqual(errors,[]);report.cases.push({language,mobile,sameContent:true,exactOrigin:true,markerEvidence:true,xssSafe:true,bounds:true,changedSourceDismisses:true});await context.close();
  }
  report.passed=true;
 }finally{if(browser)await browser.close();await server.close();if(out)fs.writeFileSync(path.join(out,'b3-browser.json'),JSON.stringify(report,null,2)+'\n');}
});
module.exports={fixture,load};
