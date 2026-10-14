const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function extract(start,end){
 const a=source.indexOf(start),b=source.indexOf(end,a+start.length);
 assert.ok(a>=0&&b>a, 'function not found: '+start);
 return source.slice(a,b);
}
const functionSource=extract('async function refreshOnce() {','\nasync function refresh() {');
test('direct detail route never waits for the full backlog list',async()=>{
 let detailCalls=0,listCalls=0,revisionCalls=0;
 const state={project:'/sample',backlog:'/sample/_task_mecca/data/backlog',view:'backlog',detail:'B-59'};
 const context=vm.createContext({
  state,URLSearchParams,closeAttentionStream(){},
  refreshHub:async()=>{},
  loadTaskDetail:async()=>{detailCalls++},
  refreshList:async()=>{listCalls++;await new Promise(()=>{})},
  checkContentRevision:async()=>{revisionCalls++},
  queueMicrotask,fetch:()=>{throw Error('unexpected fetch')},
 });
 vm.runInContext(functionSource+'\nglobalThis.run=refreshOnce',context);
 await Promise.race([context.run(),new Promise((_,reject)=>setTimeout(()=>reject(Error('detail blocked')),500))]);
 assert.equal(detailCalls,1);
 assert.equal(listCalls,0);
 assert.equal(revisionCalls,1);
});
test('first list preview is provisional and authoritative page remains requested',()=>{
 assert.match(source,/fastParams\.set\('projection','initial'\)/);
 assert.match(source,/!first\.pending_reconciliation/);
 assert.match(source,/reconciled=true;if\(!isCurrent\(\)\)return/);
 assert.match(source,/preliminary\?\(state\.language==='ko'\?'확인 중':'Checking'\)/);
 assert.match(source,/if\(state\.detail\)loadTaskDetail\(state\.detail\);\s*else \{refreshList\(\);ensureAttentionStream\(\);\}/);
});
