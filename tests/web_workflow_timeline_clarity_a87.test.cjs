// Shipped renderer, offline synthetic payloads; no service or operating journal writes.
const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const web=process.env.WORKFLOW_PHASE_WEB_ROOT||'goassets/template/_task_mecca/framework/web';
const source=fs.readFileSync(web+'/app.js','utf8'),css=fs.readFileSync(web+'/style.css','utf8');
const fixtures=JSON.parse(fs.readFileSync('tests/fixtures/workflow/timeline-clarity-a87.json'));
function page(language='en'){
 const context=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden');},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a87={state,activityMetricsBody,activityDetailPrefs,workflowIntervalPresentation:typeof workflowIntervalPresentation==="function"?workflowIntervalPresentation:null,workflowActivityLayout,workflowPlotHTML,workflowSetWindow,workflowZoomWindow};',context);
 const a=context.a87;Object.assign(a.state,{project:'/synthetic',detail:'A87-FIXTURE',language});return a;
}
const data=name=>structuredClone(fixtures.cases[name]);
const legend=html=>html.match(/<ul[^>]*data-workflow-type-legend[^>]*>[\s\S]*?<\/ul>/)?.[0]||'';
const types=html=>[...legend(html).matchAll(/<li[^>]*data-workflow-legend-type="([^"]+)"/g)].map(m=>m[1]);
test('kind and measurement distinguish known Activity, explicit legacy facts, unknown Activity and typeless gaps',()=>{
 const a=page(),classify=i=>a.workflowIntervalPresentation(i).category;
 assert.equal(classify({kind:'activity',type:'requirements',measured:true}),'known');
 assert.equal(classify({kind:'activity_open',type:'targeted_test',measured:false}),'known');
 assert.equal(classify({kind:'handoff_review',type:'unclassified_review',measured:false}),'legacy_fact');
 assert.equal(classify({kind:'execution_report',type:'registration_authorization',measured:false}),'legacy_fact');
 assert.equal(classify({kind:'activity',type:'registration_authorization',measured:true}),'unknown','label alone never establishes a supported Activity phase');
 assert.equal(classify({kind:'activity',type:'future<phase>',measured:true}),'unknown');
 assert.equal(classify({kind:'activity_open',measured:false}),'unknown','explicit Activity without type needs diagnosis');
 assert.equal(classify({kind:'started',at:'2026-11-05T00:00:00Z'}),'fact');
 assert.equal(classify({kind:'activity_unmeasured',start:'x',end:'y'}),'gap');
 assert.equal(classify({kind:'native_observed',type:'unclassified_execution',measured:false}),'legacy_fact');
});
test('default legend shows only current Activity types; technical reference is named native disclosure',()=>{
 for(const lang of ['ko','en']){
  const html=page(lang).activityMetricsBody(data('known'),'A87-FIXTURE');
  assert.deepEqual(types(html),['requirements','implementation','targeted_test']);
  assert.doesNotMatch(legend(html),/unknown|controller_review|원문 유지|raw label retained/);
  assert.match(html,/<details class="workflow-legend-help"[^>]*><summary>/);
  assert.match(html,lang==='ko'?/전체 활동 유형과 측정 상태/:/All activity types and measurement states/);
  assert.doesNotMatch(html,/<details class="workflow-legend-help"[^>]*\bopen\b/);
  assert.match(html,/data-workflow-reference-type="controller_review"/);
  assert.match(html,/data-workflow-state="unmeasured"/);
 }
});
test('facts and long gaps never generate unknown Activity legend or measured source members',()=>{
 const a=page(),p=data('facts'),before=JSON.stringify(p),html=a.activityMetricsBody(p,'A87-FIXTURE');
 assert.equal(legend(html),'');assert.doesNotMatch(html,/data-workflow-legend-type="unknown"|data-workflow-interval=/);
 assert.match(html,/data-workflow-fact="registrar-fact-0"/);assert.match(html,/data-workflow-gap="unmeasured"/);
 assert.equal(JSON.stringify(p),before);
});
test('real unknown Activity stays neutral and raw escaped value remains in source details',()=>{
 const html=page().activityMetricsBody(data('unknown'),'A87-FIXTURE');
 assert.ok(types(html).includes('unknown'));assert.match(html,/wf-type-unknown/);
 assert.match(html,/future&lt;phase&gt; with a very long unregistered label/);assert.doesNotMatch(html,/<phase>|raw label retained|원문 유지/);
 assert.match(html,/Unknown activity type/);
});
test('round filtering changes current legend without changing raw payload, exact members or nanosecond layout',()=>{
 const a=page(),p=data('overlap'),before=JSON.stringify(p),wf=p.workflow,pref=a.activityDetailPrefs('A87-FIXTURE');
 const initial=a.workflowActivityLayout(wf.rows[0],wf.axis.end);assert.equal(initial.lanes,2);
 const geometry=initial.runs.map(r=>[String(r.fromNS),String(r.toNS),r.lane,r.entries.map(e=>e.interval.id)]);
 a.workflowSetWindow(wf,pref,Date.parse(wf.axis.start)+1000,Date.parse(wf.axis.start)+2000);
 assert.match(a.activityMetricsBody(p,'A87-FIXTURE'),/data-workflow-interval=/);
 a.workflowZoomWindow(wf,pref,2,Date.parse(wf.axis.start)+1250,.25);assert.equal(pref.window.end-pref.window.start,2000);
 pref.round='1';assert.equal(legend(a.activityMetricsBody(p,'A87-FIXTURE')),'');
 pref.round='all';pref.expandedRows={[wf.rows[0].id]:true};assert.match(a.activityMetricsBody(p,'A87-FIXTURE'),/aria-expanded="true"/);
 assert.deepEqual(a.workflowActivityLayout(wf.rows[0],wf.axis.end).runs.map(r=>[String(r.fromNS),String(r.toNS),r.lane,r.entries.map(e=>e.interval.id)]),geometry);
 assert.equal(JSON.stringify(p),before);
});
test('shipped CSS quiets unmeasured paint and test stripes while preserving semantic colors and focus',()=>{
 const gap=css.match(/\.workflow-state-unmeasured\s*\{[^}]*\}/)?.[0];assert.ok(gap);assert.doesNotMatch(gap,/repeating-linear-gradient/);
 const targeted=css.match(/\.wf-type-targeted_test\s*\{[^}]*\}/)?.[0];assert.match(targeted,/#9b5a00/);assert.match(targeted,/--wf-type-pattern:none/);
 assert.match(css,/\.workflow-legend-help[^{]*[\s\S]*focus-visible/);
 assert.match(css,/\.wf-type-requirements\{--wf-type-color:#315cac/);assert.match(css,/\.wf-type-implementation\{--wf-type-color:#00796b/);
});
