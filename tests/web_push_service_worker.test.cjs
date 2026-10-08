const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');

const sw=fs.readFileSync('goassets/template/_task_mecca/framework/web/sw.js','utf8');

function worker(){
 const listeners={};const shown=[];
 const self={registration:{showNotification:async(title,options)=>shown.push({title,options})},
  addEventListener:(name,callback)=>{listeners[name]=callback}};
 vm.runInNewContext(sw,{self,clients:{matchAll:async()=>[],openWindow:async()=>null}});
 return {listeners,shown};
}
test('background Push shows a canonical notification without an open tab',async()=>{
 const {listeners,shown}=worker();
 let promise;listeners.push({
  data:{json:()=>({title:'Task · A-1 완료',body:'테스트 완료',tag:'task-mecca:e1',url:'/tasks/A-1?project=demo'})},
  waitUntil(value){promise=value}
 });
 await promise;
 assert.equal(shown.length,1);
 assert.equal(shown[0].title,'Task · A-1 완료');
 assert.equal(shown[0].options.data.url,'/tasks/A-1?project=demo');
});
test('Push URL must be same-origin and malformed payload cannot display',async()=>{
 const {listeners,shown}=worker();
 let awaited;listeners.push({
  data:{json:()=>({title:'A',url:'//outside.example/path'})},waitUntil(value){awaited=value}
 });
 await awaited;
 assert.equal(shown[0].options.data.url,'/?view=notifications');
 listeners.push({data:{json:()=>{throw Error('invalid JSON')}},waitUntil(){throw Error('unexpected display')}});
 assert.equal(shown.length,1);
});
