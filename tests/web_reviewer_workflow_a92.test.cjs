// Shipped renderer, synthetic evidence only. No API, service or real ledger.
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const web=path.resolve(__dirname,'../goassets/template/_task_mecca/framework/web');
const source=fs.readFileSync(path.join(web,'app.js'),'utf8');
const spec=require('./fixtures/workflow/reviewer-role-a92.json');
const base=require('./fixtures/workflow/continuous-activity.json');
const names=['state','activityMetricsBody','activityDetailPrefs','workflowPlotHTML','workflowActivityLayout','workflowViewWindow','workflowSetWindow','workflowZoomWindow','activityDetailCache','activityDetailKey','bindWorkflowControls','workflowActorLabel','activityDisplayRole','reviewPresentationMap','reviewPresentationLookup','reviewRowPresentation','reviewGenerationLabel','reviewRoundLabel','reviewPresentationDetail','reviewPresentationSummary','reviewStateLabel','reviewNecessityLabel','rolePolicyPanel','workloadSessionCard','workloadSessionName','workloadSessionKey','workflowExecutionCoverage'];
function load(language='ko'){
 const context=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('Network forbidden');},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a92={'+names.join(',')+'};',context);
 Object.assign(context.a92.state,{language,project:'/fixtures/a92',detail:'A92-FIXTURE'});return context.a92;
}
function fixture(){
 const data=structuredClone(base),wf=data.workflow,template=wf.rows.find(r=>r.intervals.some(i=>i.measured))||wf.rows[0];
 wf.task_id='A92-FIXTURE';wf.current.phase='controller_review';wf.current.processing_role='reviewer';wf.current.processing_agent='fixture-formal-review-13';const by_attempt={};
 wf.rows=spec.actors.map((actor,index)=>{
  const row=structuredClone(template);row.id=actor.role+'|'+actor.native+'|0';row.role=actor.role;row.agent_id=actor.native;row.display_name=actor.name;row.display_name_source='bound_runtime_agent_path';row.round=0;row.name_evidence=[{attempt_id:actor.attempt,agent_path:'/root/controller/'+actor.name,status:'readable_bound_runtime_path'}];row.role_facts=[];row.gaps=[];
  row.intervals=row.intervals.slice(0,1).map(i=>({...i,id:'a92-'+index,attempt_id:actor.attempt,type:actor.purpose==='review'?'controller_review':'implementation'}));
  by_attempt[actor.attempt]={attempt_id:actor.attempt,native_id:actor.native,actor_name:actor.name,actor_name_source:'bound_runtime_agent_path',task_id:'A92-FIXTURE',official_name:`D1112_01_${actor.role==='reviewer'?'Reviewer':'Worker'}_${actor.name}_G${actor.generation||1}`,name_verified:true,name_source:'persisted_root_name',name_observed_at:'2026-11-19T00:00:00Z',purpose:actor.purpose,purpose_verified:actor.purpose==='review',necessity:'required',actual_state:actor.state,chain_id:actor.round===null?'':'fixture-chain',review_round:actor.round,session_generation:actor.generation,context_status:'unknown',continuity_ref:'',context_ref:'',source_delta:[],errors:[]};return row;
 });
 wf.control_snapshot={...(wf.control_snapshot||{}),trust:'verified',reviewer_presentation:{version:1,by_attempt,task:{necessity:'required',actual_state:'reviewing',reason:'Synthetic bound review observation'}}};
 return data;
}
function freeze(value){if(value&&typeof value==='object'){Object.freeze(value);Object.values(value).forEach(freeze);}return value;}
if(require.main===module){
test('formal Reviewer and original historical Worker identity stay separate from verified purpose and G/round',()=>{
 const a=load('en'),data=freeze(fixture()),before=JSON.stringify(data),html=a.activityMetricsBody(data,'A92-FIXTURE');
 assert.equal(a.activityDisplayRole('reviewer',false),'Reviewer');
 for(const [index,actor] of spec.actors.entries()){
  const row=data.workflow.rows[index],p=a.reviewRowPresentation(data.workflow,row);
  assert.ok(p);assert.equal(row.role,actor.role);assert.equal(p.session_generation,actor.generation);
  if(actor.generation)assert.match(html,new RegExp(actor.name+' · G'+actor.generation));
 }
 assert.match(html,/Worker · pikachyu · G11/);assert.match(html,/Reviewer · pikachyu · G13/);
 assert.match(html,/Review round 1/);assert.match(html,/Review round 3/);assert.doesNotMatch(html,/Review round 11/);
 assert.match(html,/Review round unknown/);assert.equal(JSON.stringify(data),before);
 const visibleLabels=html.match(/class="workflow-row-label"[^>]*>[\s\S]*?<\/button>/g).map(button=>button.slice(button.indexOf('>')+1).replace(/<[^>]+>/g,''));
 assert.doesNotMatch(visibleLabels.join(''),/D1112_01_/,'full official name belongs in selected detail, not visible row text');
});
test('lookup requires exact attempt AND native; missing, mismatched or ambiguous evidence never establishes a review',()=>{
 const a=load('en'),data=fixture(),row=data.workflow.rows[3],p=a.reviewRowPresentation(data.workflow,row);
 assert.equal(a.reviewPresentationLookup(data.workflow,['unknown'],row.agent_id),null);
 assert.equal(a.reviewPresentationLookup(data.workflow,[p.attempt_id],'another-native'),null);
 data.workflow.control_snapshot.reviewer_presentation.by_attempt[p.attempt_id].attempt_id='different-attempt';assert.equal(a.reviewRowPresentation(data.workflow,row),null);
 delete data.workflow.control_snapshot.reviewer_presentation;assert.equal(a.reviewRowPresentation(data.workflow,row),null);
 const text=a.reviewPresentationDetail(null,false);assert.match(text,/unknown/);assert.doesNotMatch(text,/0%|accepted|G13/);
 assert.equal(a.reviewGenerationLabel({official_name:'D1112_01_Reviewer_pikachyu_G99'},false),'G unknown');
 assert.equal(a.reviewRoundLabel({session_generation:99,review_round:4},false),'Review round unknown');
 const exact=a.reviewPresentationLookup({reviewer_presentation:spec.interface_example},['run-fixture'],'native-fixture');
 assert.ok(exact);assert.equal(a.reviewGenerationLabel(exact,false),'G unknown');assert.equal(a.reviewRoundLabel(exact,false),'Review round unknown');
 assert.match(a.reviewPresentationDetail(exact,false),/D1112_01_Worker_pikachyu_G14/,'literal official G remains evidence, never parsed into nullable generation');
});
test('necessity is separate from every actual state; running/ACK/plans are unknown',()=>{
 const a=load('en');
 assert.deepEqual(['required','recommended','optional'].map(s=>a.reviewNecessityLabel({level:s},false)),['Required','Recommended','Optional']);
 assert.equal(new Set(spec.states.map(s=>a.reviewStateLabel(s,false))).size,9);
 assert.equal(a.reviewStateLabel('reviewing',false),'Review in progress');assert.match(a.reviewStateLabel('result_recorded',false),/acceptance unverified/);
 for(const state of ['running','ACK','planned','completed',null])assert.equal(a.reviewStateLabel(state,false),'Review state unknown');
});
test('valid omission has policy reason and no reviewer placeholder or missing-review warning; errors do not establish omission',()=>{
 const a=load('en'),task={role_policy:{implementation:'worker',review:'omitted',review_reason:'Small verified text correction',review_requirement:{level:'optional',status:'omitted'},policy:{implementation_owner:{role:'worker'},operational_owner:{role:'controller'}}}};
 const html=a.rolePolicyPanel(task);assert.match(html,/Optional/);assert.match(html,/Omitted by policy/);assert.match(html,/Small verified text correction/);assert.doesNotMatch(html,/Separate reviewer|Independent review requires|role="status"/);
 task.role_policy.error='invalid policy';assert.match(a.rolePolicyPanel(task),/Review state unknown/);
 const planned={role_policy:{review:'unknown',policy:{review_owner:{role:'reviewer',native_id:'planned'}}}};
 assert.doesNotMatch(a.rolePolicyPanel(planned),/>Review accepted</);
});
test('metadata changes neither geometry, timeline intervals, denominator nor measurement; refreshed source replaces purpose',()=>{
 const encode=value=>JSON.stringify(value,(_,v)=>typeof v==='bigint'?String(v):v);
 const a=load('en'),data=fixture(),before=JSON.stringify({axis:data.workflow.axis,measurement:data.workflow.measurement,rows:data.workflow.rows}),row=data.workflow.rows[3],layout=encode(a.workflowActivityLayout(row,data.workflow.axis.end));
 a.activityMetricsBody(data,'A92-FIXTURE');assert.equal(encode(a.workflowActivityLayout(row,data.workflow.axis.end)),layout);
 delete data.workflow.control_snapshot.reviewer_presentation;const html=a.activityMetricsBody(data,'A92-FIXTURE');assert.match(html,/Review state unknown/);assert.doesNotMatch(html,/Review purpose verified/);
 assert.equal(JSON.stringify({axis:data.workflow.axis,measurement:data.workflow.measurement,rows:data.workflow.rows}),before);
});
test('A93 coverage is detail-only, retains errors and missing duration, and never allocates time per task',()=>{
 const a=load('en'),data=fixture();data.workflow.reporting.roles=[{role:'reviewer',execution_coverage:{version:1,items:[{id:'closed-phase',role:'reviewer',category:'closed_phase',start:data.workflow.axis.start,end:data.workflow.axis.end,task_ids:['A-1','A-2'],actor_id:'/root/controller/reviewer',reason:'Fixture boundary <unsafe>'},{id:'open',category:'open_start',start:data.workflow.axis.start}],collection_errors:['fixture read failed']}}];
 const before=JSON.stringify(data),html=a.workflowExecutionCoverage(data.workflow,false);
 assert.match(html,/separate from axis and Activity totals/);assert.match(html,/not summed as exclusive time per task/);assert.match(html,/Unmeasured/);assert.match(html,/Collection errors/);assert.match(html,/&lt;unsafe&gt;/);assert.doesNotMatch(html,/NaN|0s/);assert.equal(JSON.stringify(data),before);
});
test('Workload consumes explicit response map, preserves separate natives and uses continuity generation, not official-name suffix',()=>{
 const a=load('en'),data=fixture(),map=data.workflow.control_snapshot.reviewer_presentation;
 a.state.workloadSessions={reviewer_presentation:map,active:{items:[]},historical:{items:[]}};
 const roots=spec.actors.slice(1,5).map(actor=>({root_session_id:actor.native,provider_session_id:actor.native,display_name:map.by_attempt[actor.attempt].official_name,display_name_source:'t3_title',display_name_status:'verified',relation_status:'verified',role:actor.role,provider:'codex',status:'terminal'}));
 const html=roots.map(root=>a.workloadSessionCard(root)).join('');
 assert.equal((html.match(/data-runtime-root-key=/g)||[]).length,4);assert.match(html,/Worker · pikachyu · G11/);assert.match(html,/Reviewer · pikachyu · G14/);
 assert.equal(a.workloadSessionName({...roots[0],display_name:'D1112_01_Worker_pikachyu_G999'},null),'Worker · Name unverified · G unknown');
 assert.equal(a.workloadSessionName({...roots[0],display_name:'D1112_01_Reviewer_wrong_G999'},map.by_attempt['fixture-review-11']),'Worker · pikachyu · G11','typed bound name wins; title supplies neither alias, role nor G');
 assert.equal(a.reviewPresentationMap({reviewer_presentation:map}),map);
 assert.equal(a.reviewPresentationMap({workflow:{control_snapshot:{reviewer_presentation:map}}}),null,'no hidden cache or wrong envelope fallback');
});

}
module.exports={fixture,load,source,web,names};
