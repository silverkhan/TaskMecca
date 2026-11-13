const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function page(){
 const element={hidden:true,innerHTML:'',querySelector:()=>({addEventListener(){}})};
 const context=vm.createContext({URLSearchParams,location:{search:'?project=/fixture'},navigator:{language:'ko'},document:{querySelector:selector=>selector==='#userAttention'?element:null},localStorage:{getItem:()=>null,setItem(){}}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.app={state,renderOperationBanner,storeCommonUserAttention,operationHistoryView,operationHistoryProjection};',context);
 return {...context.app,element};
}
const stamp='2026-11-13T02:00:00Z';
function incident(id,task,category){return {id,task_id:task,project:'/fixture',attempt_id:'attempt-'+id,assignment_id:'assignment-'+id,kind:'runtime_unknown',quality:'verification_required',evidence:'<unverified evidence>',last_observed_at:stamp,detected_at:stamp,category};}
function payload(){return {snapshot_at:stamp,active:[incident('current','F-DOING','current_operation')],user_holds:Array.from({length:5},(_,i)=>({...incident('hold-'+i,'F-HOLD','user_hold'),hold_reason:'Explicit user decision',resume_condition:'Wait for user choice'})),observation_history:Array.from({length:11},(_,i)=>incident('done-'+i,'F-DONE','observation_history')),diagnostics:Array.from({length:4},(_,i)=>incident('old-'+i,'','insufficient_evidence'))};}
test('audit-shaped records separate current warnings, one hold representative and all histories',()=>{
 const app=page(),data=payload();app.renderOperationBanner(data);let html=app.element.innerHTML;
 assert.match(html,/사용자 판단 1건/);assert.match(html,/세션 경고 1건/);
 assert.match(html,/보류 관련 관측 이력 · 1 그룹 · 5 원시 사건/);assert.match(html,/과거 관측 이력 · 1 그룹 · 11 원시 사건/);assert.match(html,/판단 근거 부족 · 진단 · 1 그룹 · 4 원시 사건/);
 assert.equal((html.match(/<details/g)||[]).length,4);assert.doesNotMatch(html,/<details[^>]+ open/);assert.equal((html.match(/operation-raw-fields/g)||[]).length,0);
 for(const [key,items] of [['hold-/fixture-F-HOLD',data.user_holds],['history',data.observation_history],['diagnostics',data.diagnostics]]){const v=app.operationHistoryView(key);v.open=true;v.selected=app.operationHistoryProjection(items).groups[0].key;}
 app.renderOperationBanner(data);html=app.element.innerHTML;
 for(let i=0;i<5;i++)assert.match(html,new RegExp('attempt-hold-'+i));
 for(let i=0;i<11;i++)assert.match(html,new RegExp('assignment-done-'+i));
 assert.match(html,/당시 실행 상태는 미확인/);assert.match(html,/Explicit user decision/);assert.match(html,/Wait for user choice/);
 assert.doesNotMatch(html,/<unverified evidence>/);assert.match(html,/&lt;unverified evidence&gt;/);
 assert.equal((html.match(/operation-raw-fields/g)||[]).length,20);
});
test('new current task changes count, history is not a resolution tombstone and stale responses stay ignored',()=>{
 const app=page(),data=payload();app.renderOperationBanner(data);
 const newer={...data,snapshot_at:'2026-11-13T02:01:00Z',active:[...data.active,incident('new-current','F-NEXT','current_operation')]};
 app.renderOperationBanner(newer);assert.match(app.element.innerHTML,/세션 경고 2건/);
 app.renderOperationBanner(data);assert.match(app.element.innerHTML,/세션 경고 2건/);
 // The same source may become actionable on a later scan; category is not deletion.
 app.renderOperationBanner({...newer,snapshot_at:'2026-11-13T02:02:00Z',active:[...newer.active,{...data.observation_history[0],category:'current_operation'}],observation_history:data.observation_history.slice(1)});
 assert.match(app.element.innerHTML,/세션 경고 3건/);
});
test('existing user decision and hold histories use one representative, diagnostics remain accessible alone',()=>{
 const app=page();app.storeCommonUserAttention({snapshot_at:stamp,all_items:{'F-HOLD':{id:'F-HOLD',title:'Choose policy',file_state:'hold',state:'needs_user'}},attention:[{id:'F-HOLD',type:'user_intervention',message:'Choose',resume_condition:'User chooses'}]},'/fixture','');
 app.renderOperationBanner(payload());assert.match(app.element.innerHTML,/사용자 판단 1건/);
 const next=page();next.renderOperationBanner({diagnostics:[incident('unknown','','insufficient_evidence')]});
 assert.equal(next.element.hidden,false);assert.match(next.element.innerHTML,/세션 경고 0건/);assert.match(next.element.innerHTML,/진단 · 1/);
 next.state.language='en';next.renderOperationBanner({diagnostics:[incident('unknown','','insufficient_evidence')]});assert.match(next.element.innerHTML,/Insufficient evidence/);
});
test('history uses native disclosure and CSS accommodates long identities and localization',()=>{
 const app=page(),data=payload();data.user_holds[0].attempt_id='한글🙂العربية'+('x'.repeat(500));app.renderOperationBanner(data);
 assert.match(app.element.innerHTML,/보류 관련 관측 이력/);assert.doesNotMatch(app.element.innerHTML,/한글🙂العربية/);const v=app.operationHistoryView('hold-/fixture-F-HOLD');v.open=true;v.selected=app.operationHistoryProjection(data.user_holds).groups[0].key;app.renderOperationBanner(data);assert.match(app.element.innerHTML,/한글🙂العربية/);
 const css=fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8');assert.match(css,/operation-observation-details dd[^}]*overflow-wrap: anywhere/s);assert.match(css,/operation-observation-details dl > div[^}]*flex-wrap: wrap/s);
});
