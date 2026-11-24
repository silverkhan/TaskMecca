// Offline synthetic evidence through the actual shipped renderer.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/app.js'),'utf8');
function load(language='ko'){
 const context=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden')},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a97={state,activityMetricsBody,workflowActorLabel};',context);
 context.a97.state.language=language;return context.a97;
}
function row(native){return {id:native,role:'controller',agent_id:native,display_name:'Controller',display_name_source:'bound_runtime_agent_path',round:0,intervals:[{id:'activity',kind:'activity',type:'coordination',start:'2026-11-05T00:10:00.000Z',end:'2026-11-05T00:20:00.000Z',measured:true,duration_ms:600000,attempt_id:'attempt-1',assignment_id:'assignment'}],session_identity:{status:'verified',provider:'codex',native_id:native,aliases:['/root/controller',native],attempts:[{attempt_id:'attempt-1',thread_id:'exact-t3',assignment_id:'assignment'}]}};}
test('same role/name sessions have distinct visible and accessible labels in KO/EN',()=>{
 for(const lang of ['ko','en']){const a=load(lang),one=row('01234567-same-prefix-native-first'),two=row('01234567-same-prefix-native-second');
 assert.notEqual(a.workflowActorLabel(one,lang==='ko'),a.workflowActorLabel(two,lang==='ko'));
 const p=structuredClone(require('./fixtures/workflow/fact-gap-details-a89.json').cases.mixed);p.workflow.rows=[one,two];
 const before=JSON.stringify(p),html=a.activityMetricsBody(p,'A97-FIXTURE');
 assert.match(html,/data-workflow-row-select="01234567-same-prefix-native-first"/);assert.match(html,/aria-label="[^"]*01234567…-first/);assert.match(html,/01234567…second/);assert.equal(JSON.stringify(p),before);
 }
});
test('unverified alias never displays a verified native label and identity is escaped',()=>{
 const a=load(),r=row('native<script>');r.session_identity.status='unverified';
 assert.match(a.workflowActorLabel(r,true),/세션 연결 미확인/);assert.doesNotMatch(a.workflowActorLabel(r,true),/<script>/);
 r.session_identity.status='verified';const p=structuredClone(require('./fixtures/workflow/fact-gap-details-a89.json').cases.mixed);p.workflow.rows=[r];const html=a.activityMetricsBody(p,'A97-XSS');assert.doesNotMatch(html,/<script>/);assert.match(html,/native&lt;script&gt;/);
});
