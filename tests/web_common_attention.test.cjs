const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function page(fetch){
 const handlers={};const details={addEventListener:(name,handler)=>handlers[name]=handler};
 const element={hidden:true,innerHTML:'',querySelector:()=>details};
 const context=vm.createContext({fetch,URLSearchParams,location:{search:'?project=/demo'},navigator:{language:'ko'},document:{querySelector:selector=>selector==='#userAttention'?element:null},localStorage:{getItem:()=>null,setItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.app={state,storeCommonUserAttention,renderOperationBanner,renderUserAttention,refreshOperations};',context);
 return {...context.app,element,handlers};
}
const snapshot=(attention=true,time='2026-10-08T00:40:00Z')=>({snapshot_at:time,all_items:{'A-14':{id:'A-14',title:'정책 선택',file_state:'hold',state:'needs_user'}},attention:attention?[{id:'A-14',type:'user_intervention',message:'정책을 선택해 주세요.',resume_condition:'두 선택지를 확인해 주세요.'}]:[]});
const warning={id:'incident',project:'/demo',task_id:'A-35',attempt_id:'run-current',kind:'no_signal',evidence:'Hook 신호 확인 필요',action:'Controller가 실제 실행 근거를 확인하세요.'};
test('one disclosure defaults closed, separates types and retains choice across every common view',()=>{
 const app=page();app.storeCommonUserAttention(snapshot(),'/demo','team');app.renderOperationBanner({active:[warning],resolved_observations:[{...warning,recovered_at:'now'}],stages:[{stage:'controller_review'}]});
 assert.doesNotMatch(app.element.innerHTML,/<details class="common-attention" open/);
 assert.match(app.element.innerHTML,/사용자 판단 1건/);assert.match(app.element.innerHTML,/세션 경고 1건/);
 assert.match(app.element.innerHTML,/backlog=team/);assert.match(app.element.innerHTML,/#runtime-attempt-run-current/);
 app.handlers.toggle({target:{open:true}});
 for(const view of ['hub','backlog','workload','attention','issues','release-notes','manual','project-notifications']){
  app.state.view=view;app.state.project=view==='hub'||view==='release-notes'?'':'/other';app.renderUserAttention();
  assert.equal(app.element.hidden,false);assert.match(app.element.innerHTML,/<details class="common-attention" open/);
 }
});
test('resolution hides empty area and old, undated or equal timestamp responses cannot restore it',()=>{
 const app=page();app.storeCommonUserAttention(snapshot(),'/demo','');app.renderOperationBanner({active:[warning]});
 app.storeCommonUserAttention(snapshot(false,'2026-10-08T00:41:00Z'),'/demo','');app.renderOperationBanner({active:[],resolved_observations:[warning]});
 assert.equal(app.element.hidden,true);
 for(const time of ['2026-10-08T00:40:00Z','','2026-10-08T00:41:00Z'])app.storeCommonUserAttention(snapshot(true,time),'/demo','');
 assert.equal(app.element.hidden,true);
 for(const resolved of [{recovered_at:'now'},{resolution:{}},{recovery_evidence:'resolved'},{history:true},{active:false}]){app.renderOperationBanner({active:[{...warning,...resolved}]});assert.equal(app.element.hidden,true)}
});
test('late operations response cannot recreate warnings after newer empty snapshot',async()=>{
 const pending=[];const app=page(()=>new Promise(resolve=>pending.push(resolve)));
 const old=app.refreshOperations(),fresh=app.refreshOperations();
 pending[1]({ok:true,json:async()=>({active:[],projects:[]})});await fresh;
 pending[0]({ok:true,json:async()=>({active:[warning],projects:[]})});await old;
 assert.equal(app.element.hidden,true);
});
test('global attention response started before a current resolution cannot override it',async()=>{
 let resolveAttention;
 const app=page(url=>url==='/api/operations'?Promise.resolve({ok:true,json:async()=>({active:[],projects:[{path:'/demo'}]})}):new Promise(resolve=>resolveAttention=resolve));
 const refresh=app.refreshOperations();await new Promise(setImmediate);
 app.storeCommonUserAttention(snapshot(false),'/demo','team');
 resolveAttention({ok:true,json:async()=>snapshot()});await refresh;
 assert.equal(app.element.hidden,true);
});
test('source text and links escape safely without treating Controller review as user need',()=>{
 const app=page();const data=snapshot();data.all_items['A-14'].state='controller_review';app.storeCommonUserAttention(data,'/demo','');assert.equal(app.element.hidden,true);
 app.renderOperationBanner({active:[{...warning,evidence:'<script>bad</script>',action:'"<&',project:'/demo & policies'}]});
 assert.doesNotMatch(app.element.innerHTML,/<script>/);assert.match(app.element.innerHTML,/&lt;script&gt;/);assert.match(app.element.innerHTML,/project=%2Fdemo\+%26\+policies/);
});

test("default snapshot updates only its selected folder",()=>{const app=page();app.storeCommonUserAttention(snapshot(),"/demo","team");const data=snapshot(false,"2026-10-08T00:41:00Z");data.backlog_selection={selected:"other"};app.storeCommonUserAttention(data,"/demo","");assert.equal(app.element.hidden,false);assert.match(app.element.innerHTML,/backlog=team/);app.storeCommonUserAttention(snapshot(false,"2026-10-08T00:42:00Z"),"/demo","team");assert.equal(app.element.hidden,true)});

test('successful folder inventory prunes deleted sources while failed inventory preserves them',async()=>{
 let okay=false;const app=page(url=>Promise.resolve({ok:url==='/api/operations'||okay,json:async()=>url==='/api/operations'?{projects:[{path:'/demo'}],active:[]}:{...snapshot(false),backlog_selection:{selected:'other',candidates:[{path:'other'}]}}}));
 app.storeCommonUserAttention(snapshot(),'/demo','deleted');await app.refreshOperations();assert.equal(app.element.hidden,false);
 okay=true;await app.refreshOperations();assert.equal(app.element.hidden,true);
});
test('resolved history is not rendered in workload',()=>{assert.doesNotMatch(source.slice(source.indexOf('function workloadView('),source.indexOf('function workloadView(')+18000),/\$\{renderOperationHistory/)});

test('common notification controls retain native keyboard behavior',()=>{assert.match(source,/if\(document\.activeElement\?\.closest\?\.\('#userAttention'\)\)return/)});

test('global refresh reads each discovered folder and keeps same task IDs scoped',async()=>{
 const requests=[];const app=page(url=>{requests.push(url);return Promise.resolve({ok:true,json:async()=>{if(url==='/api/operations')return{projects:[{path:'/demo'}],active:[]};const folder=new URLSearchParams(url.split('?')[1]).get('backlog')||'first';return{...snapshot(),backlog_selection:{selected:folder,candidates:[{path:'first'},{path:'second'}]}}}})});
 await app.refreshOperations();assert.equal(requests.length,3);assert.match(requests[2],/backlog=second/);assert.match(app.element.innerHTML,/사용자 판단 2건/);assert.match(app.element.innerHTML,/backlog=first/);assert.match(app.element.innerHTML,/backlog=second/);
});
