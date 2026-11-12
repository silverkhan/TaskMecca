const fs=require('node:fs'),path=require('node:path');
const fixture=require(process.cwd()+'/tests/helpers/workflow_native.cjs'),nativePage=fixture.page;
fixture.page=async function(context,server){
 const page=await nativePage(context,server);
 page.on('console',m=>{if(m.text().startsWith('RACE'))console.log(m.text());});
 await context.addInitScript(()=>{
  window.__raceTrace=[];window.__race=(kind,extra={})=>{const c=document.querySelector('.workflow-timeline-canvas');window.__raceTrace.push({at:performance.now(),kind,start:c?.dataset.viewStart,end:c?.dataset.viewEnd,...extra});if(window.__raceTrace.length>100)window.__raceTrace.shift();};
  const rect=Element.prototype.getBoundingClientRect;
  Element.prototype.getBoundingClientRect=function(){const r=rect.call(this);if(this.matches('.workflow-timeline-canvas')&&r.width===0){const current=document.querySelector('.workflow-timeline-canvas');console.log('RACE '+JSON.stringify({kind:'zero-width',connected:this.isConnected,currentSame:current===this,currentWidth:current?rect.call(current).width:null,trace:window.__raceTrace}));}return r;};
 });
 await context.route('**/ui/components/workflow.js',r=>{let source=fs.readFileSync(path.join(server.webRoot,'ui/components/workflow.js'),'utf8');
 source=source.replace('async function loadTaskActivity(id,project=state.project,runtimeDiagnostic=false){','async function loadTaskActivity(id,project=state.project,runtimeDiagnostic=false){window.__race?.("load-enter",{pending:activityDetailPending.size});');
 source=source.replace('function renderActivityDetailControls(taskID,project=state.project){','function renderActivityDetailControls(taskID,project=state.project){window.__race?.("render-enter",{pending:activityDetailPending.size,cacheEnd:activityDetailCache.get(activityDetailKey(taskID,project))?.workflow?.axis?.end});');
 source=source.replace('activityDetailPending.delete(key)','activityDetailPending.delete(key);window.__race?.("pending-delete",{pending:activityDetailPending.size})');
 return r.fulfill({contentType:'text/javascript',body:fixture.appendExports(source,fixture.observed)});
 });
 const session=await context.newCDPSession(page);await session.send('Emulation.setCPUThrottlingRate',{rate:4});
 return page;
};
