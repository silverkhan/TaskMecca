const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const start=source.indexOf('async function openUpgradeDetails(');
const end=source.indexOf('function renderGlobalUpdateIndicator()',start);
const performStart=source.indexOf('async function performUpgrade()');
const performEnd=source.indexOf('async function loadRuntimeHistory(',performStart);
if(start<0||end<0||performStart<0||performEnd<0)throw Error('Missing upgrade flow sources');

function createFlow(fetch,loadReleaseNoteDetail=async()=>({version:'0.2.53-dev.200'})){
 const stages=[],modal=[];
 const state={releaseNotePopup:null,releaseNotePopupMode:'installed',versionInfo:{cli:{current:'0.2.53-dev.199',latest:'0.2.53-dev.200'}}};
 const context=vm.createContext({state,fetch,loadReleaseNoteDetail,stages,modal,
   setInterval:()=>17,clearInterval:()=>{},setTimeout:()=>{},Date,Promise,
   renderGlobalUpdateIndicator:()=>{},renderReleaseNoteModal:()=>modal.push(state.releaseNotePopupMode),
   waitForRestartedWeb:async()=>false,refreshVersionInfo:async()=>({ok:true}),
   upgradeFlowCopy:()=>({restartError:'Web restart failed'}),
   stopUpgradeFlowTimers:()=>{},
   setUpgradeFlowStage:phase=>{stages.push(phase);vm.runInContext('upgradeFlowMode='+JSON.stringify(phase),context)},
   closeUpgradeFlow:()=>vm.runInContext("upgradeFlowMode='idle';upgradeFlowSession++",context)
 });
 const init="let upgradeFlowMode='idle',upgradeFlowSession=0,upgradeFlowStartedAt=0,upgradeFlowTarget='',upgradeFlowPollTimer=null,upgradeFlowPollBusy=false,upgradeFlowOwnRequest=false,upgradeFlowRestartWatching=false; const upgradeFlowPhases=['checking','downloading','verifying','installing','restarting'];";
 vm.runInContext(init+'\n'+source.slice(start,end)+'\n'+source.slice(performStart,performEnd)+
   '\nglobalThis.api={openUpgradeDetails,performUpgrade,mode:()=>upgradeFlowMode,session:()=>upgradeFlowSession};',context);
 return {state,stages,modal,api:context.api,context};
}
const tick=()=>new Promise(setImmediate);
test('clicking an update instantly opens checking UI and only fetches release notes once',async()=>{
 let finish;let noteCalls=0;
 const f=createFlow(()=>{throw Error('must not install while reading release notes')},()=>{
  noteCalls++;return new Promise(resolve=>finish=resolve);
 });
 const pending=f.api.openUpgradeDetails('0.2.53-dev.200');
 const duplicate=f.api.openUpgradeDetails('0.2.53-dev.200');
 assert.deepEqual(f.stages,['notes']);assert.equal(noteCalls,1);
 finish({version:'0.2.53-dev.200'});await Promise.all([pending,duplicate]);
 assert.deepEqual(f.modal,['available']);
 assert.equal(f.state.releaseNotePopup.version,'0.2.53-dev.200');
});
test('missing release notes requires explicit choice and never starts an installation',async()=>{
 const f=createFlow(()=>{throw Error('unexpected installation')},async()=>null);
 await f.api.openUpgradeDetails('0.2.53-dev.200');
 assert.equal(f.api.mode(),'no-notes');
 assert.deepEqual(f.stages,['notes','no-notes']);
});
test('multiple update presses create only one POST and a visible stage immediately',async()=>{
 let resolve;let posts=0;
 const f=createFlow(()=>{posts++;return new Promise(done=>resolve=done)});
 const a=f.api.performUpgrade();
 const b=f.api.performUpgrade();
 assert.equal(posts,1);assert.equal(f.api.mode(),'checking');
 resolve({ok:true,status:200,json:async()=>({from:'0.2.53-dev.199',to:'0.2.53-dev.200',restart_required:false})});
 await Promise.all([a,b]);
 assert.deepEqual(f.stages,['checking','completed']);
});
test('HTTP failure stays in panel and retry issues one new POST',async()=>{
 let posts=0;
 const f=createFlow(async()=>{posts++;return {ok:false,status:500,json:async()=>({error:'download timed out'})}});
 await f.api.performUpgrade();
 assert.equal(f.api.mode(),'failed');assert.equal(posts,1);
 await f.api.performUpgrade();
 assert.equal(posts,2);assert.equal(f.api.mode(),'failed');
});
test('409 attaches to an existing update instead of showing a false failure',async()=>{
 const f=createFlow(async()=>({ok:false,status:409,json:async()=>({status:{active:true,phase:'downloading'},error:'update already in progress'})}));
 await f.api.performUpgrade();
 assert.equal(f.api.mode(),'downloading');
 assert.deepEqual(f.stages,['checking','downloading']);
});
test('restart displays its own phase and a distinct timeout error',async()=>{
 const f=createFlow(async()=>({ok:true,status:200,json:async()=>({from:'0.2.53-dev.199',to:'0.2.53-dev.200',restart_required:true})}));
 await f.api.performUpgrade();
 assert.deepEqual(f.stages,['checking','restarting','failed']);
});
