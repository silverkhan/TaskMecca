/* Local optics for the known, fixed Cozy photograph. No DOM/screen capture.
   A shared photo sampler feeds a few curved rims; foreground DOM stays intact. */
(function(){
 'use strict';
 const root=document.documentElement,MAX_LENSES=2,MAX_PIXELS=350000,RINGS=4;
 const entries=new Map(),reduced=window.matchMedia('(prefers-reduced-transparency: reduce)');
 let frame=0,image=null,imageURL='',loading='',sampler=null,stageKey='',generation=0,paintCount=0,totalPaintMs=0,enabled=false;
 const supports=typeof Path2D!=='undefined'&&!!document.createElement('canvas').getContext?.('2d')&&
  (CSS.supports('backdrop-filter','blur(1px)')||CSS.supports('-webkit-backdrop-filter','blur(1px)'));
 function geometry(){
  const s=getComputedStyle(root);return{width:parseFloat(s.getPropertyValue('--glass-stage-width'))||innerWidth,height:parseFloat(s.getPropertyValue('--glass-stage-height'))||innerHeight};
 }
 function photoURL(){return getComputedStyle(root).getPropertyValue('--glass-room-image').match(/url\(["']?([^"')]+)/)?.[1]||'';}
 function rounded(path,x,y,w,h,r){
  r=Math.max(0,Math.min(r,w/2,h/2));path.moveTo(x+r,y);path.arcTo(x+w,y,x+w,y+h,r);path.arcTo(x+w,y+h,x,y+h,r);path.arcTo(x,y+h,x,y,r);path.arcTo(x,y,x+w,y,r);path.closePath();
 }
 function paintSampler(){
  if(!image)return;
  const g=geometry(),dark=root.dataset.theme==='dark',key=[g.width,g.height,dark,imageURL].join('|');if(key===stageKey)return;
  const scale=Math.min(1,1536/g.width,1536/g.height);sampler=document.createElement('canvas');sampler.width=Math.ceil(g.width*scale);sampler.height=Math.ceil(g.height*scale);
  const c=sampler.getContext('2d');c.scale(scale,scale);
  const cover=Math.max(g.width/image.naturalWidth,g.height/image.naturalHeight),w=image.naturalWidth*cover,h=image.naturalHeight*cover;
  c.drawImage(image,(g.width-w)/2,(g.height-h)/2,w,h);
  // The same global shade as body::before, sampled before local lens mapping.
  if(dark){const dx=Math.sin(2*Math.PI/3),dy=-Math.cos(2*Math.PI/3),len=Math.abs(g.width*dx)+Math.abs(g.height*dy),cx=g.width/2,cy=g.height/2;
   const shade=c.createLinearGradient(cx-dx*len/2,cy-dy*len/2,cx+dx*len/2,cy+dy*len/2);shade.addColorStop(0,'rgba(8,12,10,.10)');shade.addColorStop(.6,'rgba(8,12,10,.02)');shade.addColorStop(1,'rgba(8,12,10,.12)');c.fillStyle=shade;c.fillRect(0,0,g.width,g.height);
  }
  stageKey=key;generation++;
 }
 function backing(owner,terminal=false){
  let plane=owner.querySelector(':scope > .glass-reader-material');if(plane)return plane;
  plane=document.createElement('div');plane.className='glass-reader-material'+(terminal?' terminal-material':'');plane.setAttribute('aria-hidden','true');plane.dataset.glassKind='reader';owner.prepend(plane);return plane;
 }
 function remove(entry){entry.canvas.remove();entries.delete(entry.host);}
 function deactivate(reason){
  enabled=false;for(const e of [...entries.values()])remove(e);document.querySelectorAll('.glass-reader-material').forEach(e=>e.remove());root.dataset.glassOptics=reason;
  image=null;imageURL='';loading='';sampler=null;stageKey='';
 }
 function candidates(){
  const a=[];const add=(selector,kind)=>{for(const host of document.querySelectorAll(selector))a.push({host,kind})};
  add('.tm-combo-popup','popup');add('.topbar,.terminal-header','chrome');
  // Reader material stays a single quiet plane, with no optical canvas.
  for(const owner of document.querySelectorAll('.content,.terminal-main'))backing(owner,owner.classList.contains('terminal-main'));
  return a.filter((item,index,list)=>list.findIndex(x=>x.host===item.host)===index).filter(({host,kind})=>{
   const r=host.getBoundingClientRect(),s=getComputedStyle(host);
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none';
  }).slice(0,MAX_LENSES);
 }
 function paint(entry){
  if(!sampler)return;
  const {host,canvas,kind}=entry,r=canvas.getBoundingClientRect(),style=getComputedStyle(host),radius=Math.min(parseFloat(style.borderTopLeftRadius)||16,r.width/2,r.height/2);
  const key=[r.x,r.y,r.width,r.height,radius,generation,devicePixelRatio].map(x=>typeof x==='number'?Math.round(x*100)/100:x).join('|');if(entry.key===key)return;
  const start=performance.now(),w=r.width,h=r.height,dpr=Math.min(devicePixelRatio||1,1.25,Math.sqrt(MAX_PIXELS/(w*h)));
  const width=Math.max(1,Math.ceil(w*dpr)),height=Math.max(1,Math.ceil(h*dpr));if(canvas.width!==width)canvas.width=width;if(canvas.height!==height)canvas.height=height;const c=canvas.getContext('2d');c.setTransform(1,0,0,1,0,0);c.clearRect(0,0,width,height);c.scale(dpr,dpr);
  const rim=Math.min(2,radius*.18,h*.08),amp=1.6,g=geometry();
  for(let i=0;i<RINGS;i++){
   const a=i*rim/RINGS,b=(i+1)*rim/RINGS,ring=new Path2D();rounded(ring,a,a,w-2*a,h-2*a,radius-a);
   if(w>2*b&&h>2*b)rounded(ring,b,b,w-2*b,h-2*b,radius-b);
   const curvature=Math.pow(1-(i+.5)/RINGS,2),mx=1+amp*curvature/(w/2),my=1+amp*curvature/(h/2);
   c.save();c.clip(ring,'evenodd');c.globalAlpha=.42;c.translate(w/2,h/2);c.scale(mx,my);c.translate(-w/2,-h/2);
   c.drawImage(sampler,-r.left,-r.top,g.width,g.height);c.restore();
  }
  entry.key=key;entry.rect={left:r.left,top:r.top,width:w,height:h};entry.rim=rim;entry.amplitude=amp;entry.draws++;paintCount++;totalPaintMs+=performance.now()-start;
 }
 function update(){
  frame=0;const active=root.dataset.palette==='glass'&&root.dataset.background==='cozy'&&!reduced.matches&&supports;
  if(!active){if(enabled||root.dataset.glassOptics!==(reduced.matches?'solid':'material'))deactivate(reduced.matches?'solid':'material');return;}
  const url=photoURL();if(!url){deactivate('material');return;}
  if(imageURL!==url){
   if(loading===url)return;loading=url;const next=new Image();next.decoding='async';next.src=url;
   next.decode().then(()=>{if(loading!==url)return;image=next;imageURL=url;loading='';stageKey='';schedule()}).catch(()=>{if(loading===url){loading='';deactivate('material')}});return;
  }
  enabled=true;root.dataset.glassOptics='active';
  const toolbar=document.querySelector('.topbar'),top=toolbar?Math.ceil(toolbar.getBoundingClientRect().height)+(innerWidth<=680?20:24):88;
  if(root.style.getPropertyValue('--glass-reader-top')!==top+'px')root.style.setProperty('--glass-reader-top',top+'px');
  paintSampler();
  const desired=candidates(),keep=new Set(desired.map(x=>x.host));for(const entry of [...entries.values()])if(!keep.has(entry.host)||!entry.host.isConnected)remove(entry);
  for(const {host,kind}of desired){let entry=entries.get(host);if(!entry){const canvas=document.createElement('canvas');canvas.className='glass-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');host.prepend(canvas);entry={host,canvas,kind,key:'',draws:0};entries.set(host,entry)}paint(entry)}
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(update)}
 new MutationObserver(schedule).observe(root,{attributes:true,attributeFilter:['data-palette','data-theme','data-background']});
 new MutationObserver(schedule).observe(document.body,{childList:true,subtree:true});
 window.addEventListener('resize',schedule,{passive:true});document.addEventListener('scroll',schedule,{passive:true,capture:true});document.addEventListener('focusin',schedule);document.addEventListener('focusout',schedule);
 window.visualViewport?.addEventListener('resize',schedule,{passive:true});window.visualViewport?.addEventListener('scroll',schedule,{passive:true});reduced.addEventListener?.('change',schedule);
 window.TaskMeccaGlass=Object.freeze({refresh:schedule,diagnostics:()=>({mode:root.dataset.glassOptics||'material',photo:imageURL,stage:geometry(),sampler:sampler?{width:sampler.width,height:sampler.height}:null,paintCount,totalPaintMs,lenses:[...entries.values()].map(e=>({kind:e.kind,rect:e.rect,rim:e.rim,amplitude:e.amplitude,draws:e.draws,pixels:e.canvas.width*e.canvas.height}))})});
 if(typeof ResizeObserver!=='undefined'){const resize=new ResizeObserver(schedule);document.querySelectorAll('.topbar,.terminal-header,.sidebar-head').forEach(e=>resize.observe(e))}
 document.fonts?.ready.then(schedule);
 schedule();
})();
