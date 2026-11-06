// Observation only: never forces a tone, generation, DOM or animation outcome.
function installGlassState(){
 if(window.__glassReadState)return;
 const ids=new WeakMap();let next=0,previous='',repetitions=0;
 const id=e=>{if(!e)return null;if(!ids.has(e))ids.set(e,++next);return ids.get(e)};
 const hash=value=>{let h=2166136261;for(let i=0;i<value.length;i++)h=Math.imul(h^value.charCodeAt(i),16777619);return{length:value.length,fnv32:(h>>>0).toString(16)}};
 window.__glassReadState=(mode='active')=>{
  const root=document.documentElement,d=window.TaskMeccaGlass?.diagnostics(),style=getComputedStyle(root),url=style.getPropertyValue('--glass-room-image').match(/url\(["']?([^"')]+)/)?.[1]||'',source=root.dataset.background==='cozy'?url:getComputedStyle(document.body,'::before').backgroundImage;
  const panels=[...document.querySelectorAll('.topbar,.content,.terminal-main,.preview-identity-banner,.user-attention,.tm-combo-popup')].filter(e=>{const r=e.getBoundingClientRect();return e.checkVisibility()&&r.width>8&&r.height>8&&r.bottom>0&&r.top<innerHeight}).map(e=>{const plane=e.querySelector(':scope > .glass-reader-material'),s=getComputedStyle(e),r=e.getBoundingClientRect(),pseudo=getComputedStyle(e,'::before').content;return{id:id(e),kind:e.className,plane:id(plane),planeConnected:!!plane?.isConnected,pseudo,tone:e.dataset.glassTone||null,foreground:s.color,fill:getComputedStyle(plane||e,plane?null:'::before').backgroundColor,rect:[r.x,r.y,r.width,r.height],backed:!e.matches('.content,.terminal-main')||!!plane?.isConnected||!['none','normal'].includes(pseudo)||((mode!=='active'||d?.readerBackingSupported===false)&&/^rgb\(/.test(s.backgroundColor))}});
  const texts=[],walk=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let n;
  while(n=walk.nextNode()){const e=n.parentElement;if(!n.textContent.trim()||e.closest('script,style')||!e.checkVisibility())continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect();if(b.width<2||b.height<2||b.bottom<0||b.y>innerHeight)continue;const s=getComputedStyle(e);let opacity=1;for(let ancestor=e;ancestor;ancestor=ancestor.parentElement)opacity*=Number(getComputedStyle(ancestor).opacity);texts.push([id(n),n.textContent.trim(),s.color,String(opacity),b.x,b.y,b.width,b.height]);}
  const animations=document.getAnimations().filter(a=>a.playState==='running'||a.pending).map(a=>({target:id(a.effect?.target),property:a.transitionProperty||a.animationName||'animation',time:a.currentTime,progress:a.effect?.getComputedTiming().progress}));
  const sourceMatches=!!d&&d.sample?.source===source&&d.sample?.theme===root.dataset.theme&&d.sample?.background===root.dataset.background&&(root.dataset.background!=='cozy'||d.photo===url);
  const materialReady=[...document.querySelectorAll('[data-glass-material="true"]')].every(host=>{const plane=host.querySelector(':scope > .glass-reader-material')||host,canvas=plane.querySelector(':scope > .glass-material-optics,:scope > .glass-scroll-plane > .glass-material-optics');if(!canvas?.isConnected||canvas.width<1||canvas.height<1)return false;const pr=plane.getBoundingClientRect(),owner=pr.width>8&&pr.height>8?plane:host,r=owner.getBoundingClientRect(),c=canvas.getBoundingClientRect(),style=getComputedStyle(owner);return Math.abs(c.left-r.left-parseFloat(style.borderLeftWidth))<1.1&&Math.abs(c.top-r.top-parseFloat(style.borderTopWidth))<1.1&&Math.abs(c.width-owner.clientWidth)<1.1&&Math.abs(c.height-owner.clientHeight)<1.1});
  const ready=!!d&&(mode!=='active'||root.dataset.palette==='glass')&&d.mode===mode&&!d.pending&&!d.loading&&panels.every(p=>p.backed)&&materialReady&&document.fonts.status==='loaded'&&!animations.length&&(mode!=='active'||(sourceMatches&&panels.every(p=>p.tone)));
  const sample=d?.sample?{generation:d.sample.generation,theme:d.sample.theme,background:d.sample.background,source:hash(d.sample.source)}:null;
  const viewport={width:innerWidth,height:innerHeight,dpr:devicePixelRatio};
  // Native identities remain in diagnostics; visual equivalence retains every
  // measured value while ignoring replacement nodes with identical pixels.
  const visualPanels=panels.map(({id,plane,...visual})=>visual),visualTexts=texts.map(([, ...visual])=>visual);
  const fingerprint=JSON.stringify({viewport,palette:root.dataset.palette,theme:root.dataset.theme,background:root.dataset.background,mode:d?.mode,sample,panels:visualPanels,texts:visualTexts,paintCount:d?.paintCount});
  return{ready,viewport,palette:root.dataset.palette,theme:root.dataset.theme,background:root.dataset.background,mode:d?.mode,selectedSource:hash(source),decodedPhoto:hash(d?.photo||''),sourceMatches,sample,pending:d?.pending,loading:d?.loading,panels,texts,animations,paintCount:d?.paintCount,fingerprint};
 };
 window.__glassReady=mode=>{const s=window.__glassReadState(mode);if(s.ready&&s.fingerprint===previous)repetitions++;else repetitions=0;previous=s.fingerprint;return s.ready&&repetitions>=2;};
}
async function waitForGlass(page,mode='active'){
 await page.evaluate(installGlassState);
 await page.waitForFunction(mode=>window.__glassReady(mode),mode);
 return page.evaluate(mode=>window.__glassReadState(mode),mode);
}
async function captureGlass(page,capture,observations=[]){
 for(let attempt=0;attempt<3;attempt++){
  const before=await waitForGlass(page),value=await capture(),after=await page.evaluate(()=>window.__glassReadState());
  const stable=before.ready&&after.ready&&before.fingerprint===after.fingerprint;
  observations.push({attempt,stable,before,after});if(stable)return value;
 }
 throw Error('Glass snapshot changed DOM/sample/foreground while screenshots were captured; see observations');
}
// Install before application scripts: execution-time gating also catches a
// native rAF that was queued while the gate was still disabled.
function installGlassUpdateGate(){
 if(window.__glassUpdateGate)return;
 const request=window.requestAnimationFrame.bind(window),cancel=window.cancelAnimationFrame.bind(window),queued=new Map(),held=new Map(),events=[];let active=false;
 const hash=value=>{let h=2166136261;for(let i=0;i<value.length;i++)h=Math.imul(h^value.charCodeAt(i),16777619);return(h>>>0).toString(16)};
 const execute=(task,time,released)=>{task.callback(time);events.push({phase:released?'released-executed':'executed',id:task.id,pending:window.TaskMeccaGlass?.diagnostics().pending,plane:!!document.querySelector('.content > .glass-reader-material')})};
 window.requestAnimationFrame=callback=>{
  if(callback.name!=='update'||!callback.toString().includes('glassOptics'))return request(callback);
  const task={callback,id:null,signature:hash(callback.toString())};task.id=request(time=>{queued.delete(task.id);if(active){held.set(task.id,task);events.push({phase:'held',id:task.id,signature:task.signature});return;}execute(task,time,false)});queued.set(task.id,task);events.push({phase:'queued',id:task.id,signature:task.signature});return task.id;
 };
 window.cancelAnimationFrame=id=>{queued.delete(id);held.delete(id);cancel(id)};
 window.__glassUpdateGate={
  activate(){active=true;events.push({phase:'activated',queued:[...queued.keys()]});return this.state()},
  release(){active=false;const tasks=[...held.values()];held.clear();for(const task of tasks)request(time=>execute(task,time,true));events.push({phase:'released',ids:tasks.map(task=>task.id)});return tasks.map(task=>task.id)},
  state(){return{active,queued:[...queued.keys()],held:[...held.keys()],events:events.map(event=>({...event}))}}
 };
}
module.exports={installGlassState,waitForGlass,captureGlass,installGlassUpdateGate};

// Native mobile disclosure is a real user step, not a forced product setting.
async function openGlassPreferences(page){const b=page.locator('#glassAppearanceToggle');if(await b.isVisible()&&await b.getAttribute('aria-expanded')!=='true'){await b.click();await waitForGlass(page)}}
async function openGlassFilters(page){const b=page.locator('#glassFilterToggle');if(await b.isVisible()&&await b.getAttribute('aria-expanded')!=='true'){await b.click();await waitForGlass(page)}}
module.exports.openGlassPreferences=openGlassPreferences;module.exports.openGlassFilters=openGlassFilters;
