const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function page(){
 const element={hidden:true,innerHTML:'',setAttribute(){},querySelectorAll(){return []}};
 const storage=new Map(), streams=[];
 class EventSource { static CLOSED=2; constructor(){this.listeners={};streams.push(this)} addEventListener(type,listener){this.listeners[type]=listener} close(){} }
 const context=vm.createContext({EventSource,URLSearchParams,location:{search:'?project=/repos/demo'},navigator:{language:'ko'},document:{querySelector:selector=>selector==='#userAttention'?element:null},localStorage:{getItem:key=>storage.get(key)||null,setItem:(key,value)=>storage.set(key,String(value))}});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.app={state,currentUserAttention,updateCurrentUserAttention,clearCurrentUserAttention,listNotificationPayload,userAttentionMarkup,renderUserAttention,ensureAttentionStream};',context);
 return {...context.app,element,streams};
}
function snapshot(time='2026-10-07T14:00:00Z'){
 return {snapshot_at:time,all_items:{'A-14':{id:'A-14',title:'정책 판단',file_state:'hold',state:'needs_user',location:'active',hold_review:null}},attention:[{id:'A-14',type:'user_intervention',message:'중복 ID 표기 정책을 결정해 주세요.',resume_condition:'프로젝트명 예외 또는 링크 표시 확대 중 선택'}]};
}
test('typed current policy hold shows exact task and concrete decision even without hold review',()=>{
 const app=page();app.updateCurrentUserAttention(snapshot());
 assert.equal(app.element.hidden,false);
 assert.match(app.element.innerHTML,/A-14/);assert.match(app.element.innerHTML,/프로젝트명 예외/);
 assert.match(app.element.innerHTML,/\/tasks\/A-14\?project=%2Frepos%2Fdemo/);
});
test('resolved history, controller recovery and inactive tasks never appear',()=>{
 const app=page();
 for(const reason of [{resolved:true},{history:true},{active:false},{status:'resolved'},{audience:'controller'},{type:'runtime_stalled'}]){
 const data=snapshot();Object.assign(data.attention[0],reason);assert.equal(app.currentUserAttention(data).length,0);
 }
 for(const task of [{file_state:'done'},{location:'archive'},{state:'controller_review'},{state:'controller_recovery'}]){
 const data=snapshot();Object.assign(data.all_items['A-14'],task);assert.equal(app.currentUserAttention(data).length,0);
 }
});
test('resolution replaces snapshot and stale or undated responses cannot resurrect it',()=>{
 const app=page();app.updateCurrentUserAttention(snapshot());
 app.updateCurrentUserAttention({...snapshot('2026-10-07T14:01:00Z'),attention:[]});
 assert.equal(app.element.hidden,true);assert.equal(app.element.innerHTML,'');
 app.updateCurrentUserAttention(snapshot());app.updateCurrentUserAttention(snapshot(''));
 assert.equal(app.element.hidden,true);
 app.clearCurrentUserAttention();app.updateCurrentUserAttention(snapshot(''));assert.equal(app.element.hidden,false);
 app.state.project='/other';app.renderUserAttention();assert.equal(app.element.hidden,true);
});
test('global attention survives pagination and safely escapes all visible source text',()=>{
 const app=page(),data=snapshot();const task=data.all_items['A-14'];
 task.title='<img src=x onerror=alert(1)>';data.attention[0].message='<script>bad</script>';data.attention[0].resume_condition='"<& 선택';
 const payload=app.listNotificationPayload({snapshot_at:data.snapshot_at,items:[],attention_items:data.all_items,attention:data.attention});
 app.state.backlog='team & policy';app.updateCurrentUserAttention(payload);
 assert.doesNotMatch(app.element.innerHTML,/<img|<script>/);assert.match(app.element.innerHTML,/&lt;script&gt;/);
 assert.match(app.element.innerHTML,/backlog=team\+%26\+policy/);assert.equal(payload.snapshot_at,data.snapshot_at);
});
test('late SSE from previous project or backlog cannot overwrite current attention',()=>{
 const app=page();app.ensureAttentionStream();const old=app.streams[0];
 app.state.backlog='another-folder';
 old.listeners.attention({data:JSON.stringify(snapshot())});
 assert.equal(app.element.hidden,true);
 app.ensureAttentionStream();const current=app.streams[1];
 app.state.project='/other';
 current.listeners.attention({data:JSON.stringify(snapshot())});
 assert.equal(app.element.hidden,true);
});
