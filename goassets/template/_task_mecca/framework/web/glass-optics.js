/* Local optics for the known, fixed Cozy photograph. No DOM/screen capture.
   A shared photo sampler feeds a few curved rims; foreground DOM stays intact. */
(function(){
 'use strict';
 const root=document.documentElement,MAX_LENSES=2,MAX_PIXELS=350000,RINGS=4;
 const entries=new Map(),reduced=window.matchMedia('(prefers-reduced-transparency: reduce)');
 const panels=new Map(),materials=new Map(),MAX_MATERIAL_PIXELS=480000;
 const readerBackingSupported=CSS.supports('selector(:has(*))');
 let sampleTheme='',sampleBackground='',sampleSource='';
 let samplePixels=null,adaptCount=0,totalAdaptMs=0,scrollTimer=0,failedURL='';
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
  stageKey=key;generation++;samplePixels=c.getImageData(0,0,sampler.width,sampler.height).data;sampleTheme=root.dataset.theme;sampleBackground=root.dataset.background;sampleSource=imageURL;
 }
 // Parse the resolved, authored CSS gradient layers, so sampling follows the actual
 // stage colors/positions rather than assigning a tone from a preset name.
 function splitLayers(value){let depth=0,start=0,out=[];for(let i=0;i<value.length;i++){if(value[i]==='(')depth++;if(value[i]===')')depth--;if(value[i]===','&&!depth){out.push(value.slice(start,i).trim());start=i+1}}out.push(value.slice(start).trim());return out;}
 function paintMaterialSampler(){
  const g=geometry(),style=getComputedStyle(document.body,'::before'),key=[g.width,g.height,style.backgroundImage,style.backgroundColor].join('|');if(key===stageKey)return true;
  const scale=Math.min(1,1536/g.width,1536/g.height);sampler??=document.createElement('canvas');sampler.width=Math.ceil(g.width*scale);sampler.height=Math.ceil(g.height*scale);
  const c=sampler.getContext('2d');c.scale(scale,scale);c.fillStyle=style.backgroundColor;c.fillRect(0,0,g.width,g.height);
  const layers=splitLayers(style.backgroundImage).reverse();
  for(const layer of layers){
   if(layer==='none')continue;
   const radial=layer.startsWith('radial-gradient('),linear=layer.startsWith('linear-gradient(');if(!radial&&!linear)return false;
   const parts=splitLayers(layer.slice(layer.indexOf('(')+1,-1));let shape=parts.shift(),gradient,fill=[0,0,g.width,g.height];
   c.save();
   if(radial){
    // CSS serializes the implicit ellipse as "at x% y%". Its default
    // farthest-corner ellipse keeps the farthest-side aspect ratio, then grows
    // through the farthest corner; neither its center nor its radii are fixed.
    if(!/^(?:ellipse\b|at\b|farthest-corner\b|[\d.]+(?:%|px))/.test(shape)){parts.unshift(shape);shape='';}
    let size=shape.replace(/^ellipse(?:\s+|$)/,'').trim(),cx=g.width/2,cy=g.height/2;
    const position=size.match(/(?:^|\s)at\s+([\d.]+)(%|px)\s+([\d.]+)(%|px)$/);
    if(position){cx=Number(position[1])*(position[2]==='%'?g.width/100:1);cy=Number(position[3])*(position[4]==='%'?g.height/100:1);size=size.slice(0,position.index).trim();}
    else if(size.includes('at')){c.restore();return false;}
    let rx,ry;
    if(!size||size==='farthest-corner'){rx=Math.max(Math.abs(cx),Math.abs(g.width-cx))*Math.SQRT2;ry=Math.max(Math.abs(cy),Math.abs(g.height-cy))*Math.SQRT2;}
    else{const radii=size.match(/^([\d.]+)(%|px)\s+([\d.]+)(%|px)$/);if(!radii){c.restore();return false}rx=Number(radii[1])*(radii[2]==='%'?g.width/100:1);ry=Number(radii[3])*(radii[4]==='%'?g.height/100:1);}
    if(!(rx>0&&ry>0)){c.restore();return false;}
    c.translate(cx,cy);c.scale(rx,ry);gradient=c.createRadialGradient(0,0,0,0,0,1);fill=[-cx/rx,-cy/ry,g.width/rx,g.height/ry];
   }
   else{if(!/^[\d.]+deg$/.test(shape)){c.restore();return false}const angle=parseFloat(shape)*Math.PI/180,dx=Math.sin(angle),dy=-Math.cos(angle),length=Math.abs(g.width*dx)+Math.abs(g.height*dy);gradient=c.createLinearGradient(g.width/2-dx*length/2,g.height/2-dy*length/2,g.width/2+dx*length/2,g.height/2+dy*length/2);}
   const stops=parts.map(part=>{const stop=part.match(/^(.*?)(?:\s+([\d.]+)%)?$/);return{color:stop[1],at:stop[2]===undefined?null:Number(stop[2])/100}});
   if(stops.length<2){c.restore();return false;}stops[0].at??=0;stops[stops.length-1].at??=1;
   let previous=0;for(const stop of stops)if(stop.at!==null){stop.at=Math.max(previous,stop.at);previous=stop.at;}
   for(let i=1;i<stops.length-1;i++)if(stops[i].at===null){const start=i-1;let end=i;while(stops[end].at===null)end++;for(;i<end;i++)stops[i].at=stops[start].at+(stops[end].at-stops[start].at)*(i-start)/(end-start);}
   // CSS gradients interpolate premultiplied sRGB; Canvas gradients interpolate
   // straight RGB. In particular, fading a color to transparent black must not
   // introduce a dark fringe. Subdivide only varying-alpha segments, using the
   // premultiplied color at each stop (authored color-to-transparent is exact).
   const colors=stops.map(stop=>{const match=stop.color.match(/^rgba?\(([^)]+)\)$/);if(!match)return null;const channels=match[1].split(',').map(Number);return channels.length===3||channels.length===4?[...channels.slice(0,3),channels[3]??1]:null});
   if(colors.some(color=>!color||color.some(n=>!Number.isFinite(n)))){c.restore();return false;}
   for(let i=0;i<stops.length-1;i++){
    const a=colors[i],b=colors[i+1],steps=a[3]===b[3]?1:32;
    for(let j=0;j<=steps;j++){const t=j/steps,alpha=a[3]*(1-t)+b[3]*t,rgb=a.slice(0,3).map((v,k)=>alpha?(v*a[3]*(1-t)+b[k]*b[3]*t)/alpha:(a[3]?v:b[k]));gradient.addColorStop(stops[i].at+(stops[i+1].at-stops[i].at)*t,`rgba(${rgb.join(',')},${alpha})`);}
   }
   c.fillStyle=gradient;c.fillRect(...fill);c.restore();
  }
  samplePixels=c.getImageData(0,0,sampler.width,sampler.height).data;stageKey=key;generation++;sampleTheme=root.dataset.theme;sampleBackground=root.dataset.background;sampleSource=style.backgroundImage;return true;
 }
 const linearChannel=v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4};
 function adaptPanels(){
  if(!samplePixels)return;
  const start=performance.now(),g=geometry(),hosts=[...document.querySelectorAll('.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.tm-combo-popup,.notification-panel,.common-attention-body,.modal-card,.mobile-sidebar-panel,.preview-identity-banner,.user-attention')],keep=new Set(hosts);
  for(const host of panels.keys())if(!keep.has(host)){delete host.dataset.glassTone;host.style.removeProperty('--glass-alpha');host.style.removeProperty('--glass-reflection-rgb');panels.delete(host)}
  for(const host of hosts){
   // The reader's fixed-size sticky backing is the actual surface behind long tables.
   const plane=host.querySelector(':scope > .glass-reader-material'),r=(plane||host).getBoundingClientRect(),left=Math.max(0,r.left),top=Math.max(0,r.top),right=Math.min(innerWidth,r.right),bottom=Math.min(innerHeight,r.bottom);
   if(right-left<8||bottom-top<8||getComputedStyle(host).visibility==='hidden')continue;
   const key=[generation,left,top,right,bottom].map((n,i)=>i?Math.round(n/12):n).join('|'),previous=panels.get(host);if(previous?.key===key)continue;
   let sum=0,min=1,max=0,rgb=[0,0,0],count=0;
   for(let y=0;y<7;y++)for(let x=0;x<11;x++){
    const sx=Math.min(sampler.width-1,Math.max(0,Math.floor((left+(x+.5)*(right-left)/11)/g.width*sampler.width))),sy=Math.min(sampler.height-1,Math.max(0,Math.floor((top+(y+.5)*(bottom-top)/7)/g.height*sampler.height))),i=(sy*sampler.width+sx)*4;
    const v=[samplePixels[i],samplePixels[i+1],samplePixels[i+2]],l=.2126*linearChannel(v[0])+.7152*linearChannel(v[1])+.0722*linearChannel(v[2]);sum+=l;min=Math.min(min,l);max=Math.max(max,l);v.forEach((n,k)=>rgb[k]+=n);count++;
   }
   const luminance=sum/count;
   // A wide dead band retains panel polarity across leaves, threshold scroll and resize.
   const prior=previous?.theme===root.dataset.theme?previous.tone:root.dataset.theme,tone=luminance>.48?'light':luminance<.28?'dark':prior;
   // Broad Cozy material transmits the scene without any text-specific backing.
   // Non-photographic and fallback panels retain their established contrast fill.
   const mixed=max-min>.38?true:max-min<.24?false:(previous?.mixed??true),alpha=host.dataset.glassMaterial==='true'?.14:(mixed?.86:.78);
   if(host.dataset.glassTone!==tone)host.dataset.glassTone=tone;
   if(host.style.getPropertyValue('--glass-alpha')!==String(alpha))host.style.setProperty('--glass-alpha',alpha);
   const color=rgb.map(n=>Math.round(n/count)),reflection=color.map((n,i)=>Math.round(n*.18+[240,249,255][i]*.82)).join(',');
   if(host.style.getPropertyValue('--glass-reflection-rgb')!==reflection)host.style.setProperty('--glass-reflection-rgb',reflection);
   panels.set(host,{key,tone,theme:root.dataset.theme,luminance,range:max-min,color,mixed,alpha,rect:{left,top,width:right-left,height:bottom-top}});adaptCount++;
  }
  totalAdaptMs+=performance.now()-start;
 }
 function backing(owner,terminal=false){
  if(!readerBackingSupported)return null; // Keep the CSS pseudo fallback on older selectors.
  let plane=owner.querySelector(':scope > .glass-reader-material');if(plane)return plane;
  plane=document.createElement('div');plane.className='glass-reader-material'+(terminal?' terminal-material':'');plane.setAttribute('aria-hidden','true');plane.dataset.glassKind='reader';owner.prepend(plane);return plane;
 }
 function remove(entry){entry.canvas.remove();entries.delete(entry.host);}
 function removeMaterial(entry){entry.canvas.remove();delete entry.host.dataset.glassMaterial;materials.delete(entry.host);}
 function deactivate(reason){
  enabled=false;for(const e of [...entries.values()])remove(e);for(const e of [...materials.values()])removeMaterial(e);document.querySelectorAll('.glass-reader-material').forEach(e=>e.remove());root.dataset.glassOptics=reason;
  image=null;imageURL='';loading='';sampler=null;samplePixels=null;stageKey='';sampleTheme='';sampleBackground='';sampleSource='';
  for(const host of panels.keys()){delete host.dataset.glassTone;host.style.removeProperty('--glass-alpha');host.style.removeProperty('--glass-reflection-rgb')}panels.clear();
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
 function materialCandidates(){
  return [...document.querySelectorAll('.tm-combo-popup,.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.mobile-sidebar-panel,.preview-identity-banner,.user-attention,.common-attention-body')].filter(host=>{
   const r=host.getBoundingClientRect(),s=getComputedStyle(host);
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none'&&!(innerWidth<=680&&host.classList.contains('sidebar'))&&!(innerWidth>680&&host.classList.contains('sidebar-head'))&&(readerBackingSupported||!host.matches('.content,.terminal-main'));
  });
 }
 function paintMaterial(entry){
  const {host,canvas,plane}=entry,r=plane.getBoundingClientRect(),w=r.width,h=r.height;if(w<8||h<8)return;
  // Material depends only on the scene and panel geometry, never on ink.
  const key=[generation,r.x,r.y,w,h,host.dataset.glassTone].join("|");if(entry.key===key)return;
  const start=performance.now(),scale=Math.min(1,Math.sqrt(MAX_MATERIAL_PIXELS/(w*h))),width=Math.ceil(w*scale),height=Math.ceil(h*scale),g=geometry();
  if(canvas.width!==width)canvas.width=width;if(canvas.height!==height)canvas.height=height;
  const c=canvas.getContext('2d');c.setTransform(scale,0,0,scale,0,0);c.clearRect(0,0,w,h);
  // One continuous field: the central wave flows into a shallow curved bevel.
  // Edge samples return smoothly to the interior instead of copying a photo rim.
  const sx=sampler.width/g.width,sy=sampler.height/g.height,amplitude=8,edge=Math.min(28,w*.12,h*.22),bend=Math.min(6,edge*.22),fall=d=>Math.max(0,Math.min(1,1-d/edge))**2;
  const cuts=[0,edge*.2,edge*.5,edge*.8,edge,w-edge,w-edge*.8,w-edge*.5,w-edge*.2,w];
  const mapY=y=>r.top+y+Math.sin(y/h*Math.PI)*4+bend*(fall(y)-fall(h-y));
  for(let y=-4;y<h+4;y+=4){const dx=Math.sin((y+2)/h*Math.PI*2)*amplitude,top=mapY(y),bottom=mapY(y+5),mapX=x=>r.left-w*.012+x*1.024+dx+bend*(fall(x)-fall(w-x));
   for(let i=0;i<cuts.length-1;i++){const x=cuts[i],right=cuts[i+1],leftSample=mapX(x),rightSample=mapX(right);
    c.drawImage(sampler,leftSample*sx,top*sy,(rightSample-leftSample)*sx,(bottom-top)*sy,x,y,right-x,5);
   }
  }
  const light=host.dataset.glassTone==='light',neutral=light?[255,255,255]:[12,20,25],tint=host.matches('.mobile-sidebar-panel,.common-attention-body')?.22:.14;
  c.fillStyle=`rgba(${neutral},${tint})`;c.fillRect(0,0,w,h);
  // Directional, neutral reflection. Chromatic dispersion remains only on the
  // separate narrow floating lip; text and focus never pass through this layer.
  const reflection=c.createLinearGradient(0,0,w*.8,h);reflection.addColorStop(0,'rgba(255,255,255,.16)');reflection.addColorStop(.25,'rgba(255,255,255,.015)');reflection.addColorStop(.7,'rgba(255,255,255,0)');reflection.addColorStop(1,'rgba(3,12,18,.07)');c.fillStyle=reflection;c.fillRect(0,0,w,h);
  if(host.matches('.content,.terminal-main')){const bevel=new Path2D();rounded(bevel,1.5,1.5,w-3,h-3,16.5);const light=c.createLinearGradient(0,0,w,h);light.addColorStop(0,'rgba(255,255,255,.5)');light.addColorStop(.28,'rgba(255,255,255,.06)');light.addColorStop(.6,'rgba(3,12,18,0)');light.addColorStop(1,'rgba(3,12,18,.24)');c.strokeStyle=light;c.lineWidth=1.25;c.stroke(bevel)}
  entry.key=key;entry.rect={left:r.x,top:r.y,width:w,height:h};entry.amplitude=amplitude;entry.paintMs=performance.now()-start;entry.totalPaintMs=(entry.totalPaintMs||0)+entry.paintMs;entry.draws++;paintCount++;totalPaintMs+=entry.paintMs;
 }
 function paint(entry){
  if(!sampler)return;
  const {host,canvas,kind}=entry,r=canvas.getBoundingClientRect(),style=getComputedStyle(host),radius=Math.min(parseFloat(style.borderTopLeftRadius)||16,r.width/2,r.height/2);
  const key=[r.x,r.y,r.width,r.height,radius,generation,host.dataset.glassTone,devicePixelRatio].map(x=>typeof x==='number'?Math.round(x*100)/100:x).join('|');if(entry.key===key)return;
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
  const lip=new Path2D();rounded(lip,.65,.65,w-1.3,h-1.3,radius-.65);
  const panel=panels.get(host),strength=.22+.28*(1-(panel?.luminance??.5));
  c.save();c.strokeStyle=(()=>{const light=c.createLinearGradient(0,0,w*.72,h);light.addColorStop(0,`rgba(99,190,222,${strength})`);light.addColorStop(.12,`rgba(122,191,223,${strength*.5})`);light.addColorStop(.32,'rgba(220,233,236,0)');light.addColorStop(.7,'rgba(220,233,236,0)');light.addColorStop(.9,`rgba(226,184,117,${strength*.65})`);light.addColorStop(1,`rgba(238,213,169,${strength*.3})`);return light})();c.lineWidth=.75;c.stroke(lip);c.restore();
  entry.key=key;entry.rect={left:r.left,top:r.top,width:w,height:h};entry.rim=rim;entry.amplitude=amp;entry.draws++;paintCount++;totalPaintMs+=performance.now()-start;
 }
 function update(){
  frame=0;const active=root.dataset.palette==='glass'&&!reduced.matches&&supports;
  if(!active){if(enabled||root.dataset.glassOptics!==(reduced.matches?'solid':'material'))deactivate(reduced.matches?'solid':'material');return;}
  const cozy=root.dataset.background==='cozy',url=cozy?photoURL():'';
  if(cozy&&(!url||failedURL===url)){deactivate('material');return;}
  if(cozy&&imageURL!==url){
   if(loading===url)return;loading=url;const next=new Image();next.decoding='async';next.src=url;
   next.decode().then(()=>{if(loading!==url)return;image=next;imageURL=url;loading='';stageKey='';schedule()}).catch(()=>{if(loading===url){failedURL=url;deactivate('material')}});return;
  }
  enabled=true;root.dataset.glassOptics='active';
  const toolbar=document.querySelector('.topbar'),top=toolbar?Math.ceil(toolbar.getBoundingClientRect().height)+(innerWidth<=680?20:24):88;
  if(root.style.getPropertyValue('--glass-reader-top')!==top+'px')root.style.setProperty('--glass-reader-top',top+'px');
  try{if(cozy)paintSampler();else if(!paintMaterialSampler()){deactivate('material');return;}}catch{deactivate('material');return;}
  const possible=candidates(),materialHosts=cozy?materialCandidates():[],materialKeep=new Set(materialHosts);
  for(const e of [...materials.values()])if(!materialKeep.has(e.host)||!e.host.isConnected||!e.plane.isConnected||!e.canvas.isConnected)removeMaterial(e);
  for(const host of materialHosts){if(!materials.has(host)){const plane=host.querySelector(':scope > .glass-reader-material')||host,canvas=document.createElement('canvas');canvas.className='glass-material-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');plane.prepend(canvas);host.dataset.glassMaterial='true';materials.set(host,{host,plane,canvas,key:'',draws:0})}}
  adaptPanels();for(const entry of materials.values())paintMaterial(entry);
  const desired=cozy?possible:[],keep=new Set(desired.map(x=>x.host));for(const entry of [...entries.values()])if(!keep.has(entry.host)||!entry.host.isConnected)remove(entry);
  for(const {host,kind}of desired){let entry=entries.get(host);if(!entry){const canvas=document.createElement('canvas');canvas.className='glass-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');host.prepend(canvas);entry={host,canvas,kind,key:'',draws:0};entries.set(host,entry)}paint(entry)}
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(update)}
 new MutationObserver(schedule).observe(root,{attributes:true,attributeFilter:['data-palette','data-theme','data-background']});
 new MutationObserver(schedule).observe(document.body,{childList:true,subtree:true});
 window.addEventListener('resize',schedule,{passive:true});document.addEventListener('scroll',()=>{if(!scrollTimer)scrollTimer=setTimeout(()=>{scrollTimer=0;schedule()},100)},{passive:true,capture:true});document.addEventListener('focusin',schedule);document.addEventListener('focusout',schedule);
 window.visualViewport?.addEventListener('resize',schedule,{passive:true});window.visualViewport?.addEventListener('scroll',schedule,{passive:true});reduced.addEventListener?.('change',schedule);
 window.TaskMeccaGlass=Object.freeze({refresh:schedule,diagnostics:()=>({mode:root.dataset.glassOptics||'material',photo:imageURL,sample:{generation,theme:sampleTheme,background:sampleBackground,source:sampleSource},loading:!!loading,pending:!!frame||!!scrollTimer,readerBackingSupported,stage:geometry(),sampler:sampler?{width:sampler.width,height:sampler.height}:null,paintCount,totalPaintMs,adaptCount,totalAdaptMs,sampleBytes:samplePixels?.byteLength||0,panels:[...panels.values()],materials:[...materials.values()].map(e=>({rect:e.rect,amplitude:e.amplitude,paintMs:e.paintMs,totalPaintMs:e.totalPaintMs,draws:e.draws,pixels:e.canvas.width*e.canvas.height})),lenses:[...entries.values()].map(e=>({kind:e.kind,rect:e.rect,rim:e.rim,amplitude:e.amplitude,draws:e.draws,pixels:e.canvas.width*e.canvas.height}))})});
 if(typeof ResizeObserver!=='undefined'){const resize=new ResizeObserver(schedule);document.querySelectorAll('.topbar,.terminal-header,.sidebar-head').forEach(e=>resize.observe(e))}
 document.fonts?.ready.then(schedule);
 schedule();
})();
