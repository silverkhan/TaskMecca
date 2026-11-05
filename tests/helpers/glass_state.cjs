// Observation only: never forces a tone, generation, DOM or animation outcome.
function installGlassState(){
 if(window.__glassReadState)return;
 const ids=new WeakMap();let next=0,previous='',repetitions=0;
 const id=e=>{if(!e)return null;if(!ids.has(e))ids.set(e,++next);return ids.get(e)};
 const hash=value=>{let h=2166136261;for(let i=0;i<value.length;i++)h=Math.imul(h^value.charCodeAt(i),16777619);return{length:value.length,fnv32:(h>>>0).toString(16)}};
 window.__glassReadState=(mode='active')=>{
  const root=document.documentElement,d=window.TaskMeccaGlass?.diagnostics(),style=getComputedStyle(root),url=style.getPropertyValue('--glass-room-image').match(/url\(["']?([^"')]+)/)?.[1]||'',source=root.dataset.background==='cozy'?url:getComputedStyle(document.body,'::before').backgroundImage;
  const panels=[...document.querySelectorAll('.topbar,.content,.terminal-main,.preview-identity-banner,.user-attention,.tm-combo-popup')].filter(e=>{const r=e.getBoundingClientRect();return e.checkVisibility()&&r.width>8&&r.height>8&&r.bottom>0&&r.top<innerHeight}).map(e=>{const plane=e.querySelector(':scope > .glass-reader-material'),s=getComputedStyle(e),r=e.getBoundingClientRect(),pseudo=getComputedStyle(e,'::before').content;return{id:id(e),kind:e.className,plane:id(plane),planeConnected:!!plane?.isConnected,pseudo,tone:e.dataset.glassTone||null,foreground:s.color,fill:getComputedStyle(plane||e,plane?null:'::before').backgroundColor,rect:[r.x,r.y,r.width,r.height],backed:!e.matches('.content,.terminal-main')||!!plane?.isConnected||!['none','normal'].includes(pseudo)}});
  const texts=[],walk=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);let n;
  while(n=walk.nextNode()){const e=n.parentElement;if(!n.textContent.trim()||e.closest('script,style')||!e.checkVisibility())continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect();if(b.width<2||b.height<2||b.bottom<0||b.y>innerHeight)continue;const s=getComputedStyle(e);texts.push([id(n),n.textContent.trim(),s.color,s.opacity,b.x,b.y,b.width,b.height]);}
  const animations=document.getAnimations().filter(a=>a.playState==='running'||a.pending).map(a=>({target:id(a.effect?.target),property:a.transitionProperty||a.animationName||'animation',time:a.currentTime,progress:a.effect?.getComputedTiming().progress}));
  const sourceMatches=!!d&&d.sample?.source===source&&d.sample?.theme===root.dataset.theme&&d.sample?.background===root.dataset.background&&(root.dataset.background!=='cozy'||d.photo===url);
  const ready=!!d&&(mode!=='active'||root.dataset.palette==='glass')&&d.mode===mode&&!d.pending&&!d.loading&&panels.every(p=>p.backed)&&document.fonts.status==='loaded'&&!animations.length&&(mode!=='active'||(sourceMatches&&panels.every(p=>p.tone)));
  const sample=d?.sample?{generation:d.sample.generation,theme:d.sample.theme,background:d.sample.background,source:hash(d.sample.source)}:null;
  const viewport={width:innerWidth,height:innerHeight,dpr:devicePixelRatio};
  const fingerprint=JSON.stringify({viewport,palette:root.dataset.palette,theme:root.dataset.theme,background:root.dataset.background,mode:d?.mode,sample,panels,texts,paintCount:d?.paintCount});
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
module.exports={installGlassState,waitForGlass,captureGlass};
