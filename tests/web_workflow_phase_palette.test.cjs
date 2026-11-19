// Offline regression against the shipped renderer, source objects and CSS.
// This test never starts a server, sends a request, or reads an operating ledger.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm'),crypto=require('node:crypto');
const root=(process.env.WORKFLOW_PHASE_WEB_ROOT||'goassets/template/_task_mecca/framework/web').replace(/\/$/,'')+'/';
const source=fs.readFileSync(root+'app.js','utf8'),css=fs.readFileSync(root+'style.css','utf8');
const fixtureBytes=fs.readFileSync('tests/fixtures/workflow/phase-palette.json');
const fixture=()=>JSON.parse(fixtureBytes);
const types=['requirements','research','implementation','targeted_test','full_regression','build_check','controller_review','controller_rereview','handoff','recovery','other'];
function page(language='en'){
  const context=vm.createContext({URLSearchParams,AbortController,fetch:()=>{throw Error('offline test forbids fetch');},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
  vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.paletteTest={state,activityMetricsBody,activityDetailPrefs,activityDisplayType,workflowPlotHTML,workflowActivityLayout,workflowZoomWindow,workflowSetWindow,workflowViewWindow,workflowIntervalType};',context);
  const api=context.paletteTest;Object.assign(api.state,{project:'/fixtures/workflow',detail:'FLOW-1',language});return api;
}
const buttons=html=>html.match(/<button\b[^>]*data-workflow-interval="[^>]*>[\s\S]*?<\/button>/g)||[];
const decode=s=>s.replace(/&quot;/g,'"').replace(/&amp;/g,'&').replace(/&lt;/g,'<').replace(/&gt;/g,'>');
function attribute(tag,name){return decode(tag.match(new RegExp(' '+name+'="([^"]*)"'))?.[1]||'');}
function freeze(value){if(value&&typeof value==='object'){Object.freeze(value);Object.values(value).forEach(freeze);}return value;}

test('B644-shaped single Worker and other roles retain per-entry actual type identity',()=>{
  const a=page(),data=fixture(),html=a.activityMetricsBody(data,'FLOW-1'),tags=buttons(html);
  for(const type of types){const tag=tags.find(t=>JSON.parse(attribute(t,'data-workflow-interval'))[1]==='phase-'+type);assert.ok(tag,'rendered '+type);assert.match(tag,new RegExp('wf-type-'+type+'(?:[ "\\s])'),'type palette '+type);assert.equal(attribute(tag,'data-workflow-type'),type);assert.ok(attribute(tag,'aria-label').includes(a.activityDisplayType(type,false)));}
  const same=tags.filter(t=>attribute(t,'data-workflow-type')==='implementation');assert.ok(same.length>=4);assert.ok(same.every(t=>t.includes('wf-type-implementation')),'same meaning across roles');
  for(const role of ['worker','controller','registrar','sub_controller'])assert.match(html,new RegExp('workflow-role-swatch wf-role-'+role));
  assert.match(html,/data-workflow-row-expand=/,'concurrent source intervals stay inspectable');assert.doesNotMatch(html,/<not markup>/);assert.match(html,/&lt;not markup&gt;/);
});

test('legend, selected entry and cumulative bars share distinct localized type labels',()=>{
  for(const language of ['ko','en']){const a=page(language),html=a.activityMetricsBody(fixture(),'FLOW-1');
    assert.notEqual(a.activityDisplayType('targeted_test',language==='ko'),a.activityDisplayType('full_regression',language==='ko'));
    const legend=html.match(/<[^>]*data-workflow-type-legend[^>]*>[\s\S]*?<\/ul>/)?.[0];assert.ok(legend,'visible type legend');
    for(const type of types){const label=a.activityDisplayType(type,language==='ko');assert.ok(legend.includes(label),'legend '+label);assert.match(legend,new RegExp('wf-type-'+type));assert.match(html,new RegExp('activity-type-row wf-type-'+type));const tag=buttons(html).find(t=>attribute(t,'data-workflow-type')===type);assert.ok(attribute(tag,'data-workflow-title').includes(label),'selected detail uses '+label);}
    assert.match(html,language==='ko'?/미측정은 작업이 없었다는 뜻이 아닙니다/:/Unmeasured does not mean no work/);
  }
});

test('open Activity, execution facts, missing terminal, waits and gaps keep separate meanings',()=>{
  const a=page(),data=fixture(),html=a.activityMetricsBody(data,'FLOW-1'),open=buttons(html).find(t=>JSON.parse(attribute(t,'data-workflow-interval'))[1]==='open');
  assert.ok(open);assert.match(open,/wf-kind-activity_open/);assert.match(open,/wf-type-implementation/);assert.match(attribute(open,'aria-label'),/end unverified|End unobserved/);
  assert.match(html,/data-workflow-state="measured"/);assert.match(html,/data-workflow-state="open"/);assert.match(html,/data-workflow-state="execution"/);assert.match(html,/data-workflow-state="unreported"/);assert.match(html,/data-workflow-state="wait"/);assert.match(html,/data-workflow-state="unmeasured"/);
  assert.match(html,/native_terminal/);assert.match(html,/t3_structured/);assert.match(html,/phase-end|Phase-end/);assert.match(html,/registration_queue/);assert.match(html,/dispatch_wait/);
  const unmeasured=html.match(/<[^>]+data-workflow-state="unmeasured"[^>]*>/g)||[];assert.ok(unmeasured.length);assert.ok(unmeasured.every(t=>!t.includes('wf-type-implementation')&&!t.includes('wf-type-targeted_test')));
});

test('unknown raw type remains escaped and uses neutral unknown palette',()=>{
  const a=page(),html=a.activityMetricsBody(fixture(),'FLOW-1'),tag=buttons(html).find(t=>JSON.parse(attribute(t,'data-workflow-interval'))[1]==='unknown');assert.ok(tag);assert.match(tag,/wf-type-unknown/);assert.equal(attribute(tag,'data-workflow-type'),'future<phase>');assert.match(attribute(tag,'aria-label'),/future<phase>/);assert.doesNotMatch(html,/<phase>/);
});

test('A85 actual additive fields render neutral points and evidenced gaps without synthesizing spans',()=>{
  const a=page(),data=freeze(fixture()),before=JSON.stringify(data),html=a.activityMetricsBody(data,'FLOW-1');
  const point=html.match(/<button[^>]+data-workflow-fact="registration-point"[^>]*>/)?.[0];assert.ok(point);assert.match(point,/workflow-fact-point/);assert.doesNotMatch(point,/width:|wf-type-/);assert.ok(attribute(point,'data-workflow-marker'));assert.match(attribute(point,'data-workflow-fingerprint'),/registration queue is not active time/);assert.match(html.match(/<details class="workflow-disclosure workflow-gap-evidence">[\s\S]*?<\/details>/)?.[0]||'',/registration queue is not active time/);
  assert.match(html,/data-workflow-row="controller\|fact-only\|1"/);assert.match(html,/data-workflow-fact="controller-observed-point"/);
  for(const status of ['unmeasured','confirmed_wait','native_observed','collection_failed'])assert.match(html,new RegExp('data-workflow-gap="'+status+'"'));
  assert.match(html,/workflow-gap-evidence/);assert.match(html,/observed_evidence_only/);assert.match(html,/Synthetic explicit wait followed by resume/);assert.match(html,/Synthetic collector failure &lt;not markup&gt;/);assert.doesNotMatch(html,/<not markup>/);
  assert.equal(JSON.stringify(data),before,'role facts, gap evidence and reporting untouched');
  const old=fixture();for(const row of old.workflow.rows){delete row.role_facts;delete row.gaps;}delete old.workflow.reporting.roles;delete old.workflow.reporting.scope;assert.doesNotThrow(()=>a.activityMetricsBody(old,'FLOW-1'),'older payloads keep supported rendering');
});

test('actual A85 generated model fixtures keep source hashes, role facts and unmeasured metrics intact',()=>{
  for(const entry of fixture().generated_cases){assert.equal(crypto.createHash('sha256').update(entry.raw_json).digest('hex'),entry.sha256,entry.name+' original bytes');const data=freeze(JSON.parse(entry.raw_json)),before=JSON.stringify(data),a=page(),html=a.activityMetricsBody(data,'FLOW-1');
    for(const row of data.workflow.rows){assert.ok(html.includes('data-workflow-row="'+row.id.replace(/&/g,'&amp;').replace(/"/g,'&quot;')+'"'),entry.name+' '+row.id);for(const fact of row.role_facts||[]){const tag=html.match(new RegExp('<button[^>]+data-workflow-fact="'+fact.id+'"[^>]*>'))?.[0];assert.ok(tag,entry.name+' fact '+fact.id);assert.doesNotMatch(tag,/wf-type-|width:/);assert.ok(attribute(tag,'data-workflow-marker'),entry.name+' selectable fact');assert.ok(attribute(tag,'data-workflow-fingerprint').includes(JSON.stringify(fact)),entry.name+' exact original fact');const escapedReason=fact.reason.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');const sourceDetails=html.match(/<details class="workflow-disclosure workflow-gap-evidence">[\s\S]*?<\/details>/)?.[0]||'';assert.ok(sourceDetails.includes(escapedReason),entry.name+' original reason in collapsed source evidence');assert.doesNotMatch(html,/<not markup>/);}for(const gap of row.gaps||[])assert.ok(html.includes('data-workflow-gap="'+gap.status+'"'));}
    assert.equal(buttons(html).length,data.workflow.rows.flatMap(r=>r.intervals).filter(i=>['activity','activity_open'].includes(i.kind)).length,'facts/native bounds never become measured activity buttons');
    assert.equal(JSON.stringify(data),before,entry.name+' original events/normalized time/union unmeasured metrics unchanged');
  }
});

test('zoom, pan, concurrency and round collapse preserve exact source members and bytes',()=>{
  const bytesHash=crypto.createHash('sha256').update(fixtureBytes).digest('hex'),data=freeze(fixture()),before=JSON.stringify(data),a=page(),p=a.activityDetailPrefs('FLOW-1'),wf=data.workflow;
  const layout=a.workflowActivityLayout(wf.rows[0],wf.axis.end);assert.equal(layout.entries.length,wf.rows[0].intervals.length);assert.ok(layout.lanes>1);
  const mixed=layout.runs.find(r=>r.entries.length>3);assert.ok(mixed,'A63 exact-touch source wrapper retained');assert.equal(new Set(mixed.entries.map(e=>e.interval.type)).size,mixed.entries.length);
  const initial=a.activityMetricsBody(data,'FLOW-1');assert.equal(buttons(initial).length,wf.rows.flatMap(r=>r.intervals).filter(i=>['activity','activity_open'].includes(i.kind)).length);
  const from=Date.parse(wf.axis.start);a.workflowSetWindow(wf,p,from+700000,from+701000);const short=a.activityMetricsBody(data,'FLOW-1');assert.match(short,/250ms/);assert.match(short,/data-workflow-type="targeted_test"/);
  a.workflowZoomWindow(wf,p,2,from+700250,.25);assert.equal(p.window.end-p.window.start,2000);
  p.round='1';const filtered=a.activityMetricsBody(data,'FLOW-1');assert.equal(buttons(filtered).length,0);p.round='all';p.window=null;p.windowMode='fit';p.expandedRows={'worker|phase|0':true};assert.match(a.activityMetricsBody(data,'FLOW-1'),/aria-expanded="true"/);
  assert.equal(JSON.stringify(data),before,'all normalized times, metrics, raw events and acceptance unchanged');assert.equal(crypto.createHash('sha256').update(fs.readFileSync('tests/fixtures/workflow/phase-palette.json')).digest('hex'),bytesHash);
});

test('shipped CSS paints actual segments, never an opaque role run, and supports non-color reading',()=>{
  const surface=css.match(/\.workflow-run-surface\s*\{[^}]*\}/g)||[];assert.ok(surface.length);assert.ok(surface.every(rule=>!rule.includes('background:var(--wf-role-color)')),'run must not mask phase boundaries with role paint');
  const paint=css.match(/\.workflow-type-paint\s*\{[^}]*\}/)?.[0];assert.ok(paint,'production per-entry paint');assert.match(paint,/--wf-type-color/);assert.match(paint,/border|box-shadow/,'visible segment boundary');
  for(const type of types){const rule=css.match(new RegExp('\\.wf-type-'+type+'\\s*\\{[^}]*\\}'))?.[0];assert.ok(rule,'actual schema type '+type);assert.match(rule,/--wf-type-color/);}
  assert.match(css,/\.wf-type-unknown\s*\{[^}]*--wf-type-color/);assert.match(css,/workflow-state-unmeasured[^{]*\{[^}]*background:color-mix[^}]*border-block-end:1px dotted/);assert.doesNotMatch(css.match(/\.workflow-state-unmeasured\s*\{[^}]*\}/)?.[0]||"",/repeating-linear-gradient/,"unmeasured bounds stay inspectable without dense stripes");assert.match(css,/@media\s*\(forced-colors:\s*active\)/);assert.match(css,/workflow-type-swatch/);
  assert.match(css,/\.workflow-run \.workflow-interval>span\s*\{[^}]*background:var\(--wf-type-color/,'visible text has solid type backing instead of low-contrast pattern strokes');assert.match(css,/\.workflow-run \.wf-kind-activity_open>span\s*\{[^}]*background:var\(--surface\)/,'open labels retain theme text contrast');
});

test('palette paint stays inside original source capsules when the view clips both endpoints',()=>{
 const a=page(),data=freeze(fixture()),wf=data.workflow,before=JSON.stringify(data),prefs=a.activityDetailPrefs('FLOW-1'),start=Date.parse(wf.axis.start);
 a.workflowSetWindow(wf,prefs,start+180000,start+240000);
 const html=a.workflowPlotHTML(wf,prefs,false),run=html.match(/<div class="workflow-run wf-role-worker"[^>]*data-workflow-from-ns="([^"]*)" data-workflow-to-ns="([^"]*)" style="left:([^;]*)%;width:([^;]*)%[^>]*><div class="workflow-run-surface[^"]*"[^>]*>([\s\S]*?)<\/div>/);
 assert.ok(run,'production DOM keeps the capsule directly inside its full source wrapper');
 assert.equal(BigInt(run[1]),BigInt(start)*1000000n);assert.equal(BigInt(run[2]),BigInt(start+660000)*1000000n);
 for(const plotWidth of [181.875,672.8]){const left=Number(run[3])/100*plotWidth,right=(Number(run[3])+Number(run[4]))/100*plotWidth;assert.ok(left<0&&right>plotWidth,'source capsule endpoints remain outside clipped viewport, not fabricated rounded window ends');}
 const paints=[...run[5].matchAll(/workflow-type-paint wf-type-([^" ]+)" style="left:([\d.eE+-]+)%;width:([\d.eE+-]+)%/g)];assert.equal(paints.length,11);
 paints.forEach((paint,index)=>{assert.ok(Math.abs(Number(paint[2])-index/11*100)<1e-9);assert.ok(Math.abs(Number(paint[3])-100/11)<1e-9);});
 assert.match(css,/\.workflow-run-surface\s*\{[^}]*left:0;right:0;/,'capsule stretches over the source wrapper, not the viewport');
 assert.equal(JSON.stringify(data),before,'source timestamps, overlap, metrics, gaps and events remain unchanged');
});
