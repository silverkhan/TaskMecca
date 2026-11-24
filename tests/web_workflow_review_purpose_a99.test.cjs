const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/app.js'),'utf8');
function load(language='ko'){const c=vm.createContext({URLSearchParams,AbortController,fetch(){throw Error('network forbidden')},location:{search:''},navigator:{language},history:{pushState(){}},queueMicrotask,setTimeout:()=>1,clearTimeout,document:{querySelector:()=>null,addEventListener(){}},localStorage:{getItem:()=>null,setItem(){},removeItem(){}}});vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.a99={state,workflowActorLabel,reviewRowPresentation,workflowFloatingContent,workflowSessionDisplayDetail,reviewPresentationDetail,workflowPhaseLabel,workflowActionLabel,workflowIntervalType,workflowReviewPresentationDetail};',c);c.a99.state.language=language;return c.a99;}
const row=()=>({id:'worker',role:'worker',agent_id:'native',round:0,intervals:[{attempt_id:'attempt'}],display_name:'isanghaessi',display_name_source:'bound_runtime_agent_path'});
test('ordinary Worker auxiliary DTO does not add review generation/state/context badges',()=>{for(const lang of ['ko','en']){const a=load(lang),r=row(),wf={control_snapshot:{reviewer_presentation:{version:1,by_attempt:{attempt:{attempt_id:'attempt',native_id:'native',purpose:'unknown',purpose_verified:false,actual_state:'unknown',context_status:'unknown',session_generation:null}}}}};assert.ok(a.reviewRowPresentation(wf,r));const label=a.workflowActorLabel(r,lang==='ko',a.reviewRowPresentation(wf,r));assert.doesNotMatch(label,/G 미확인|G unknown|검수|Review/);}});
test('verified Worker review purpose is separate from stored role in actual renderer',()=>{for(const lang of ['ko','en']){const a=load(lang),r=row(),p={purpose:'review',purpose_verified:true,linkage_verified:true,stored_role:'worker',actual_state:'reviewing',session_generation:1};const label=a.workflowActorLabel(r,lang==='ko',p);assert.match(label,/Worker/);assert.match(label,lang==='ko'?/독립 검수/:/Independent review/);assert.match(a.workflowPhaseLabel('independent_review',lang==='ko'),lang==='ko'?/독립 검수/:/Independent review/);assert.doesNotMatch(a.workflowActionLabel('await_independent_review',lang==='ko'),/Implement and verify|구현·검증/);}});
test('official metadata retains exact source/time and unsupported context has no failure implication',()=>{const a=load(),r=row();r.session_display={name_verified:true,official_name:'D1123_01_Worker_isanghaessi_G1',name_source:'t3_title',name_observed_at:'2026-11-24T01:50:21.41163Z',context_support:'unsupported',native_id:'native',session_generation:1,name_evidence:{native_name:'D1123_01_Worker_isanghaessi_G1',t3_thread_id:'exact-thread',host_project_id:'project'}};const before=JSON.stringify(r),label=a.workflowActorLabel(r,true),detail=a.workflowSessionDisplayDetail(r,true);assert.match(label,/isanghaessi.*G1/);assert.doesNotMatch(label,/G 미확인|검수/);assert.match(detail,/t3_title/);assert.match(detail,/01:50:21.41163Z/);assert.match(detail,/지원하지 않습니다/);assert.doesNotMatch(detail,/identity failure|실행 실패/);assert.equal(JSON.stringify(r),before);});

test('context unsupported, not collected and unknown remain separate from review identity',()=>{const a=load('en'),base={purpose:'review',purpose_verified:true,actual_state:'reviewing',context_status:'unknown'};assert.match(a.reviewPresentationDetail({...base,context_support:'unsupported'},false),/Unsupported for this role/);assert.match(a.reviewPresentationDetail({...base,context_support:'not_collected'},false),/Not collected/);assert.match(a.reviewPresentationDetail(base,false),/Unknown/);});

test('only source-verified individual Activity receives the independent-review label',()=>{const a=load('en'),raw={kind:'activity',type:'other',explanation:'review keyword alone'};assert.doesNotMatch(a.workflowIntervalType(raw,false),/Independent review/);assert.match(a.workflowIntervalType({...raw,purpose:'review',purpose_verified:true},false),/Independent review/);assert.equal(raw.type,'other');});

test('review detail applicability keeps ordinary actors free of reviewer-only unknown fields',()=>{
 for(const language of ['ko','en']){const a=load(language),ko=language==='ko';
  assert.equal(a.reviewPresentationDetail(null,ko,{role:'worker'}),'');
  assert.equal(a.reviewPresentationDetail({chain_id:'unverified-chain',purpose:'unknown',purpose_verified:false,stored_role:'worker'},ko,{role:'worker'}),'');
  assert.match(a.reviewPresentationDetail(null,ko,{role:'reviewer'}),ko?/검수 목적·연결/:/Review purpose, linkage/);assert.match(a.reviewPresentationDetail({stored_role:'reviewer',purpose:'unknown',purpose_verified:false},ko,{}),ko?/검수 상태 미확인/:/Review state unknown/);
  assert.match(a.reviewPresentationDetail(null,ko,{role:'worker',purpose:'review',purpose_verified:true}),ko?/검수 목적·연결/:/Review purpose, linkage/);
 }
});

test('actual pinned detail helper applies to row, interval and exact fact attempt without guessing review purpose',()=>{
 for(const language of ['ko','en']){const a=load(language),ko=language==='ko',r=row(),interval={id:'measured',kind:'activity',type:'implementation',attempt_id:'attempt'},fact={kind:'fact',value:{attempt_id:'attempt'}};
  for(const p of [null,{attempt_id:'attempt',native_id:'native',purpose:'unknown',purpose_verified:false,chain_id:'not-proof'}]){
   const wf={control_snapshot:{reviewer_presentation:{version:1,by_attempt:p?{attempt:p}:{}}}};
   for(const [i,m] of [[null,null],[interval,null],[null,fact]])assert.equal(a.workflowReviewPresentationDetail(wf,r,i,m,ko),'');
   assert.match(a.workflowReviewPresentationDetail(wf,{...r,role:'reviewer'},interval,null,ko),ko?/검수/:/Review/);
  }
  const review={attempt_id:'attempt',native_id:'native',purpose:'review',purpose_verified:true,stored_role:'worker',actual_state:'reviewing'},wf={control_snapshot:{reviewer_presentation:{version:1,by_attempt:{attempt:review}}}};
  const other={...interval,type:'other',purpose:'review',purpose_verified:true};
  assert.match(a.workflowReviewPresentationDetail(wf,r,other,null,ko),ko?/검수 목적 확인/:/Review purpose verified/);
  assert.equal(a.workflowReviewPresentationDetail(wf,r,interval,null,ko),'','verified attempt DTO does not promote an unverified individual interval');
  assert.match(a.workflowReviewPresentationDetail(wf,r,null,fact,ko),ko?/검수 목적 확인/:/Review purpose verified/);
  assert.equal(a.workflowReviewPresentationDetail(wf,r,null,{kind:'fact',value:{attempt_id:'foreign'}},ko),'');assert.equal(a.workflowReviewPresentationDetail(wf,r,null,{kind:'fact',value:{}},ko),'','missing exact fact attempt cannot borrow the row review purpose');
  assert.equal(r.role,'worker');assert.equal(other.type,'other');
 }
 assert.match(source,/insertAdjacentHTML\('beforebegin',workflowReviewPresentationDetail\(wf,row,interval,marker,ko\)\)/,'real pinned renderer uses the applicability helper');
});
