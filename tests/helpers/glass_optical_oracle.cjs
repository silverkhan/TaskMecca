const assert=require('node:assert/strict');

// Test-only Canvas observer. Replay the actual sampler, clip paths, transforms,
// alpha and drawing calls twice: once exactly, once without centered lens scale.
// This avoids a second, differently scaled/opaque photo-rendering implementation.
function installOpticalOracle(){
 const proto=CanvasRenderingContext2D.prototype,original={},entries=new WeakMap();
 for(const name of ['setTransform','clearRect','scale','translate','save','restore','clip','drawImage','stroke'])original[name]=proto[name];
 function entry(context){
  const canvas=context.canvas;if(!canvas.classList?.contains('glass-optics'))return null;
  let e=entries.get(canvas);
  if(!e||e.width!==canvas.width||e.height!==canvas.height){
   const make=()=>{const c=document.createElement('canvas');c.width=canvas.width;c.height=canvas.height;return c.getContext('2d')};
   e={width:canvas.width,height:canvas.height,replay:make(),identity:make(),operations:[],scales:[]};entries.set(canvas,e);
  }
  return e;
 }
 for(const name of Object.keys(original))proto[name]=function(...args){
  const e=entry(this),before=name==='scale'?this.getTransform():null;
  const lens=name==='scale'&&(Math.abs(before.e)>1e-8||Math.abs(before.f)>1e-8);
  if(e){
   if(name==='clearRect'){e.scales=[];e.operations=[];}
   const state=name==='drawImage'?{globalAlpha:this.globalAlpha,globalCompositeOperation:this.globalCompositeOperation,imageSmoothingEnabled:this.imageSmoothingEnabled,imageSmoothingQuality:this.imageSmoothingQuality}:name==='stroke'?{strokeStyle:this.strokeStyle,lineWidth:this.lineWidth,lineCap:this.lineCap,lineJoin:this.lineJoin,globalAlpha:this.globalAlpha}:null;
   e.operations.push({name,args,state});
   if(name==='stroke')for(const c of [e.replay,e.identity]){c.strokeStyle=this.strokeStyle;c.lineWidth=this.lineWidth;c.lineCap=this.lineCap;c.lineJoin=this.lineJoin;c.globalAlpha=this.globalAlpha}
   if(name==='drawImage')for(const c of [e.replay,e.identity]){c.globalAlpha=this.globalAlpha;c.globalCompositeOperation=this.globalCompositeOperation;c.imageSmoothingEnabled=this.imageSmoothingEnabled;c.imageSmoothingQuality=this.imageSmoothingQuality}
   original[name].apply(e.replay,args);
   if(!lens)original[name].apply(e.identity,args);
   if(lens)e.scales.push({x:args[0],y:args[1],centerX:before.e/before.a,centerY:before.f/before.d});
  }
  return original[name].apply(this,args);
 };
 window.__glassSpectralControl=(canvas,restore=false)=>{
  const e=entries.get(canvas),c=canvas.getContext('2d');if(!e)throw Error('Unobserved real optical canvas');
  // Replay the exact operations into the SAME context. Neither 8-bit readback
  // nor a canvas-to-canvas copy is pixel-neutral in every headless renderer.
  original.save.call(c);original.setTransform.call(c,1,0,0,1,0,0);c.globalAlpha=1;c.globalCompositeOperation='source-over';
  for(const op of e.operations){if(op.state)Object.assign(c,op.state);if(op.name!=='stroke'||restore===true||restore==='copy')original[op.name].apply(c,op.args);}
  original.restore.call(c);
 };
 window.__glassOpticalCompare=canvas=>{
  const e=entries.get(canvas);if(!e)return null;
  const actual=canvas.getContext('2d').getImageData(0,0,e.width,e.height).data,replay=e.replay.getImageData(0,0,e.width,e.height).data,identity=e.identity.getImageData(0,0,e.width,e.height).data;
  let validPixels=0,weight=0,weightedDifference=0,replayWeightedDifference=0,changedWeight=0,maxAlpha=0,legacyPixels=0,maxAlphaDifference=0,replayMismatchBytes=0,replayMaxDifference=0;
  for(let i=0;i<actual.length;i+=4){
   const alpha=actual[i+3],w=Math.min(alpha,identity[i+3])/255;maxAlpha=Math.max(maxAlpha,alpha);if(alpha>100)legacyPixels++;
   maxAlphaDifference=Math.max(maxAlphaDifference,Math.abs(alpha-identity[i+3]));
   let colorDifference=0,replayColorDifference=0;
   for(let k=0;k<4;k++){const delta=Math.abs(actual[i+k]-replay[i+k]);if(delta)replayMismatchBytes++;replayMaxDifference=Math.max(replayMaxDifference,delta);if(k<3){colorDifference+=Math.abs(actual[i+k]-identity[i+k]);replayColorDifference+=delta}}
   if(w>0){validPixels++;weight+=w;weightedDifference+=w*colorDifference/3;replayWeightedDifference+=w*replayColorDifference/3;if(colorDifference>0)changedWeight+=w}
  }
  const rect=canvas.getBoundingClientRect(),maxDisplacement=Math.max(0,...e.scales.map(s=>Math.hypot(rect.width/2*(1-1/s.x),rect.height/2*(1-1/s.y))));
  return{width:e.width,height:e.height,validPixels,alphaWeight:weight,maxAlpha,legacyPixels,legacyChannels:legacyPixels*3,maxAlphaDifference,replayMismatchBytes,replayMaxDifference,meanWeightedDifference:weight?weightedDifference/weight:null,meanReplayDifference:weight?replayWeightedDifference/weight:null,signalToReplayError:weight?(weightedDifference/weight)/Math.max(1/255,replayWeightedDifference/weight):null,changedWeightFraction:weight?changedWeight/weight:null,maxDisplacement,scales:e.scales,control:'actual draw-call replay; same source, geometry, sampler, clip and alpha; centered scale removed only'};
 };
}
function assertOpticalComparison(result,{witness=false}={}){
 assert.ok(result,'actual optical draw was observed');
 // Canvas implementations can differ by one 8-bit color rounding step.
 assert.ok(result.replayMaxDifference<=1,'exact replay stays within one color quantization step');
 assert.equal(result.maxAlphaDifference,0,'identity reference has the same antialiased mask and alpha');
 assert.ok(result.validPixels>200&&result.alphaWeight>50,'enough alpha-weighted rim samples');
 assert.ok(Number.isFinite(result.meanWeightedDifference),'finite optical comparison');
 assert.ok(result.scales.length>0&&result.maxDisplacement>.5&&result.maxDisplacement<3,'actual lens mapping has bounded nonzero displacement');
 assert.ok(result.changedWeightFraction>0&&result.signalToReplayError>4,'sampling change exceeds exact-replay quantization noise');
 if(witness){
  assert.ok(result.changedWeightFraction>.1,'high contrast witness changes a substantial rim area');
  assert.ok(result.meanWeightedDifference>2,'high contrast witness detects a meaningful sampling shift');
 }
}
async function captureOpticalWitness({browser,origin,mobile,zeroDisplacement=false}){
 const c=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});
 try{
  await c.addInitScript(installOpticalOracle);
  await c.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','light');window.EventSource=class{addEventListener(){}close(){}}});
  await c.route('**/*',async route=>{
   const url=route.request().url();if(!url.startsWith(origin+'/'))return route.abort();
   if(url.includes('/cozy-beige-room-photo-'))return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="1920" height="1080"><defs><pattern id="p" width="16" height="16" patternUnits="userSpaceOnUse"><rect width="16" height="16" fill="white"/><path d="M0 0h8v8H0zM8 8h8v8H8z" fill="black"/></pattern></defs><rect width="1920" height="1080" fill="url(#p)"/></svg>'});
   if(zeroDisplacement&&url.endsWith('/glass-optics.js')){const response=await route.fetch(),source=await response.text();assert.ok(source.includes('amp=1.6'),'negative control targets the actual production amplitude');return route.fulfill({response,body:source.replace('amp=1.6','amp=0')})}
   return route.continue();
  });
  const p=await c.newPage();await p.goto(origin);await p.waitForTimeout(350);await p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},JSON.parse(JSON.stringify(require('./glass_fixture.cjs'))));await p.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().mode==='active');await require('./glass_state.cjs').openGlassPreferences(p);await p.locator('.palette-picker .tm-combo-trigger').click();await p.waitForFunction(()=>TaskMeccaGlass.diagnostics().lenses.some(e=>e.kind==='popup'));
  return await p.locator('.tm-combo-popup .glass-optics').evaluate(canvas=>window.__glassOpticalCompare(canvas));
 }finally{await c.close()}
}
module.exports={installOpticalOracle,assertOpticalComparison,captureOpticalWitness};
