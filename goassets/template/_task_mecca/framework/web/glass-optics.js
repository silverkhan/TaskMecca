/* Local optics for the known, fixed Cozy photograph. No DOM/screen capture.
   A shared photo sampler feeds a few curved rims; foreground DOM stays intact. */
(function(){
 'use strict';
 const root=document.documentElement,MAX_LENSES=2,MAX_PIXELS=350000,RINGS=4;
 const entries=new Map(),reduced=window.matchMedia('(prefers-reduced-transparency: reduce)');
 const panels=new Map(),materials=new Map(),MAX_MATERIAL_PIXELS=480000,MATERIAL_BEVEL=18,MATERIAL_BEND=12,MATERIAL_REFLECTION=1;
 const readerBackingSupported=CSS.supports('selector(:has(*))');
 let sampleTheme='',sampleBackground='',sampleSource='';
 let samplePixels=null,adaptCount=0,totalAdaptMs=0,geometryFrame=0,failedURL='';
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
  const start=performance.now(),g=geometry(),hosts=[...document.querySelectorAll('.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.tm-combo-popup,.notification-panel,.common-attention-body,.modal-card,.channel-switch-modal,.detail-toc,.mobile-sidebar-panel,.preview-identity-banner,.user-attention')],keep=new Set(hosts);
  for(const host of panels.keys())if(!keep.has(host)){delete host.dataset.glassTone;host.style.removeProperty('--glass-alpha');host.style.removeProperty('--glass-reflection-rgb');panels.delete(host)}
  for(const host of hosts){
   // The reader's fixed-size sticky backing is the actual surface behind long tables.
   const plane=host.querySelector(':scope > .glass-reader-material'),planeRect=plane?.getBoundingClientRect(),r=(planeRect?.width>8&&planeRect?.height>8)?planeRect:host.getBoundingClientRect(),left=Math.max(0,r.left),top=Math.max(0,r.top),right=Math.min(innerWidth,r.right),bottom=Math.min(innerHeight,r.bottom);
   if(right-left<8||bottom-top<8||getComputedStyle(host).visibility==='hidden')continue;
   const key=[generation,left,top,right,bottom].map((n,i)=>i?Math.round(n/12):n).join('|'),previous=panels.get(host);if(previous?.key===key)continue;
   let sum=0,min=1,max=0,rgb=[0,0,0],count=0;
   for(let y=0;y<7;y++)for(let x=0;x<11;x++){
    const sx=Math.min(sampler.width-1,Math.max(0,Math.floor((left+(x+.5)*(right-left)/11)/g.width*sampler.width))),sy=Math.min(sampler.height-1,Math.max(0,Math.floor((top+(y+.5)*(bottom-top)/7)/g.height*sampler.height))),i=(sy*sampler.width+sx)*4;
    const v=[samplePixels[i],samplePixels[i+1],samplePixels[i+2]],l=.2126*linearChannel(v[0])+.7152*linearChannel(v[1])+.0722*linearChannel(v[2]);sum+=l;min=Math.min(min,l);max=Math.max(max,l);v.forEach((n,k)=>rgb[k]+=n);count++;
   }
   const luminance=sum/count;
   // The authored theme owns polarity and density. Scene measurements are
   // diagnostic only: reference fidelity must not be corrected for contrast.
   const tone=root.dataset.theme==='dark'?'dark':'light',role=materialRole(host);
   const mixed=max-min>.38,alpha=role==='reader'?.30:role==='floating'?.26:.18;
   if(host.dataset.glassTone!==tone)host.dataset.glassTone=tone;
   if(host.style.getPropertyValue('--glass-alpha')!==String(alpha))host.style.setProperty('--glass-alpha',alpha);
   const color=rgb.map(n=>Math.round(n/count)),reflection=color.map((n,i)=>Math.round(n*.18+[246,248,247][i]*.82)).join(',');
   if(host.style.getPropertyValue('--glass-reflection-rgb')!==reflection)host.style.setProperty('--glass-reflection-rgb',reflection);
   panels.set(host,{key,role,tone,theme:root.dataset.theme,luminance,range:max-min,color,mixed,alpha,rect:{left,top,width:right-left,height:bottom-top}});adaptCount++;
  }
  totalAdaptMs+=performance.now()-start;
 }
 function backing(owner,terminal=false){
  if(!readerBackingSupported)return null; // Keep the CSS pseudo fallback on older selectors.
  let plane=owner.querySelector(':scope > .glass-reader-material');if(plane)return plane;
  plane=document.createElement('div');plane.className='glass-reader-material'+(terminal?' terminal-material':'');plane.setAttribute('aria-hidden','true');plane.dataset.glassKind='reader';owner.prepend(plane);return plane;
 }
 function removeCanvas(entry){if(entry.scrollPlane)entry.scrollPlane.remove();else entry.canvas.remove()}
 function remove(entry){removeCanvas(entry);entries.delete(entry.host);}
 function removeMaterial(entry){entry.profileCache=null;entry.nativeProfile=null;delete entry.host.dataset.glassNative;delete entry.host.dataset.glassPositioned;removeCanvas(entry);delete entry.host.dataset.glassMaterial;materials.delete(entry.host);}
 function deactivate(reason){
  enabled=false;for(const e of [...entries.values()])remove(e);for(const e of [...materials.values()])removeMaterial(e);document.querySelectorAll('.glass-reader-material').forEach(e=>e.remove());root.dataset.glassOptics=reason;
  image=null;imageURL='';loading='';sampler=null;samplePixels=null;stageKey='';sampleTheme='';sampleBackground='';sampleSource='';
  for(const host of panels.keys()){delete host.dataset.glassTone;host.style.removeProperty('--glass-alpha');host.style.removeProperty('--glass-reflection-rgb')}panels.clear();
 }
 function candidates(){
  const a=[];const add=(selector,kind)=>{for(const host of document.querySelectorAll(selector))a.push({host,kind})};
  add('.tm-combo-popup','popup');add('.topbar,.terminal-header','chrome');
  // One bounded reader plane; no per-row optical layers.
  for(const owner of document.querySelectorAll('.content,.terminal-main'))backing(owner,owner.classList.contains('terminal-main'));
  return a.filter((item,index,list)=>list.findIndex(x=>x.host===item.host)===index).filter(({host,kind})=>{
   const r=host.getBoundingClientRect(),s=getComputedStyle(host);
   if(host.closest('details:not([open])'))return false;
   if(host.checkVisibility&&!host.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))return false;
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none';
  }).slice(0,MAX_LENSES);
 }
 function materialCandidates(){
  return [...document.querySelectorAll('.tm-combo-popup,.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.mobile-sidebar-panel,.preview-identity-banner,.user-attention,.common-attention-body,.notification-panel,.modal-card,.channel-switch-modal,.detail-toc')].filter(host=>{
   const r=host.getBoundingClientRect(),s=getComputedStyle(host);
   if(host.matches('.detail-toc')&&!(innerWidth<=1280&&host.closest('.toc-open')))return false;
   if(host.closest('details:not([open])'))return false;
   if(host.checkVisibility&&!host.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))return false;
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none'&&!(innerWidth<=680&&host.classList.contains('sidebar'))&&!(innerWidth>680&&host.classList.contains('sidebar-head'))&&(readerBackingSupported||!host.matches('.content,.terminal-main'));
  });
 }
 function materialRole(host){return host.matches('.content,.terminal-main')?'reader':host.matches('.tm-combo-popup,.mobile-sidebar-panel,.common-attention-body,.notification-panel,.modal-card,.channel-switch-modal,.detail-toc')?'floating':'chrome'}
 // Component-proven native material: scene sampling and compositing remain
 // on Canvas. Cache only current rounded paths, never foreground or pixel coefficients.
 function nativeMaterialGeometry(entry,w,h,radius){
  const key=[w,h,radius,MATERIAL_BEVEL,MATERIAL_BEND].join('|');if(entry.nativeProfile?.key===key)return entry.nativeProfile;
  const band=Math.min(MATERIAL_BEVEL,w/4,h/3),rings=[],lights=[];
  for(let i=0;i<40;i++){
   const a=i*band/40,b=(i+1)*band/40,path=new Path2D();rounded(path,a,a,w-2*a,h-2*a,Math.max(0,radius-a));rounded(path,b,b,w-2*b,h-2*b,Math.max(0,radius-b));
   const t=(i+.5)/40;rings.push({path,curve:(1-t)**2,alpha:.82*(1-t)**2});
  }
  for(let i=0;i<48;i++){
   const d=(i+.5)*band/48,t=d/band,path=new Path2D();rounded(path,d,d,w-2*d,h-2*d,Math.max(0,radius-d));
   const width=band/48+1,coverage=(band/48)/width,energy=target=>1-(1-target)**coverage;
   lights.push({path,width,shine:energy(.74*Math.exp(-1*((t-.34)/.30)**2)),secondary:energy(.42*Math.exp(-1*((t-.30)/.28)**2)),shade:energy(.09*Math.exp(-1*((t-.81)/.20)**2))});
  }
  entry.nativeProfile={key,band,rings,lights};return entry.nativeProfile;
 }
 function paintNativeMaterial(entry,r){
  const {host,canvas}=entry,w=r.width,h=r.height;if(w<8||h<8)return;
  const radius=Math.min(parseFloat(getComputedStyle(host).borderTopLeftRadius)||28,w/2,h/2),tint=panels.get(host)?.alpha??.18,key=['native-sidebar',generation,r.x,r.y,w,h,radius,tint].join('|');if(entry.key===key)return;
  const started=performance.now(),scale=Math.min(1,Math.sqrt(MAX_MATERIAL_PIXELS/(w*h))),cw=Math.max(1,Math.floor(w*scale)),ch=Math.max(1,Math.floor(h*scale));if(canvas.width!==cw)canvas.width=cw;if(canvas.height!==ch)canvas.height=ch;
  const c=canvas.getContext('2d'),g=geometry();c.setTransform(scale,0,0,scale,0,0);c.clearRect(0,0,w,h);
  // One native softened photograph, no strip/self-blur/CPU pixel readback chain.
  c.save();c.filter='blur(9px)';c.drawImage(sampler,-r.left,-r.top,g.width,g.height);c.restore();
  const neutral=root.dataset.theme==='light'?[249,250,248]:[24,28,29];c.fillStyle=`rgba(${neutral},${tint})`;c.fillRect(0,0,w,h);
  const profile=nativeMaterialGeometry(entry,w,h,radius);
  // Each subpixel ring samples the still-unmodified inner face. Its smooth
  // inward bend is retained while native Canvas handles the compositing.
  for(const ring of profile.rings){const bend=MATERIAL_BEND*ring.curve;c.save();c.clip(ring.path,'evenodd');c.globalAlpha=ring.alpha;c.translate(w/2,h/2);c.scale(1+bend/(w/2),1+bend/(h/2));c.translate(-w/2,-h/2);c.drawImage(canvas,0,0,canvas.width,canvas.height,0,0,w,h);c.restore()}
  // Localized corner sources, not a bright constant rail along a tall side.
  // Coverage-compensated subpixel strokes retain one intended reflection energy.
  const extent=Math.max(Math.min(Math.min(w,h)*2.4,Math.max(w,h)*.42),radius*2.5),light=c.createRadialGradient(radius*.3,radius*.35,0,radius*.3,radius*.35,extent);light.addColorStop(0,'rgba(255,255,255,.98)');light.addColorStop(.20,'rgba(255,255,255,.88)');light.addColorStop(.55,'rgba(255,255,255,.25)');light.addColorStop(1,'rgba(255,255,255,0)');
  const secondary=c.createRadialGradient(w-radius*.4,h-radius*.4,0,w-radius*.4,h-radius*.4,extent*.85);secondary.addColorStop(0,'rgba(255,255,255,.85)');secondary.addColorStop(.4,'rgba(255,255,255,.30)');secondary.addColorStop(1,'rgba(255,255,255,0)');
  // The inner return belongs to the same local source as the convex cap;
  // a diagonal all-around shade would read as an inset grey rail.
  const shade=c.createRadialGradient(radius,radius,0,radius,radius,extent*1.1);shade.addColorStop(0,'rgba(10,12,13,.75)');shade.addColorStop(.35,'rgba(10,12,13,.35)');shade.addColorStop(1,'rgba(10,12,13,0)');
  for(const lobe of profile.lights){c.lineWidth=lobe.width;c.strokeStyle=light;c.globalAlpha=lobe.shine*MATERIAL_REFLECTION;c.stroke(lobe.path);c.strokeStyle=secondary;c.globalAlpha=lobe.secondary*MATERIAL_REFLECTION;c.stroke(lobe.path);c.strokeStyle=shade;c.globalAlpha=lobe.shade;c.stroke(lobe.path)}c.globalAlpha=1;
  const lip=new Path2D();rounded(lip,.7,.7,w-1.4,h-1.4,radius-.7);c.lineWidth=1.1;c.strokeStyle=light;c.globalAlpha=.88*MATERIAL_REFLECTION;c.stroke(lip);c.strokeStyle=secondary;c.globalAlpha=.45*MATERIAL_REFLECTION;c.stroke(lip);c.globalAlpha=1;
  const face=c.createLinearGradient(0,0,w*.7,h);face.addColorStop(0,'rgba(255,255,255,.11)');face.addColorStop(.45,'rgba(255,255,255,.015)');face.addColorStop(1,'rgba(255,255,255,0)');c.fillStyle=face;c.fillRect(0,0,w,h);
  entry.profileCache=null;host.dataset.glassNative=host.matches('.sidebar')?'sidebar':host.matches('.topbar')?'topbar':host.matches('.content,.terminal-main')?'reader':'surface';entry.key=key;entry.rect={left:r.x,top:r.y,width:w,height:h};entry.role=materialRole(host);entry.tint=tint;entry.radius=radius;entry.bevel=profile.band;entry.amplitude=MATERIAL_BEND;entry.backend='native-canvas';entry.paintMs=performance.now()-started;entry.totalPaintMs=(entry.totalPaintMs||0)+entry.paintMs;entry.draws++;paintCount++;totalPaintMs+=entry.paintMs;
 }
 function paint(entry,r){
  if(!sampler)return;
  const {host,canvas,kind}=entry,style=getComputedStyle(host),radius=Math.min(parseFloat(style.borderTopLeftRadius)||16,r.width/2,r.height/2);
  const key=[r.x,r.y,r.width,r.height,radius,generation,host.dataset.glassTone,devicePixelRatio].map(x=>typeof x==='number'?Math.round(x*100)/100:x).join('|');if(entry.key===key)return;
  const start=performance.now(),w=r.width,h=r.height,dpr=Math.min(devicePixelRatio||1,1.25,Math.sqrt(MAX_PIXELS/(w*h)));
  const width=Math.max(1,Math.floor(w*dpr)),height=Math.max(1,Math.floor(h*dpr));if(canvas.width!==width)canvas.width=width;if(canvas.height!==height)canvas.height=height;const c=canvas.getContext('2d');c.setTransform(1,0,0,1,0,0);c.clearRect(0,0,width,height);c.scale(dpr,dpr);
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
  c.save();c.strokeStyle=(()=>{const light=c.createLinearGradient(0,0,w*.72,h);light.addColorStop(0,`rgba(255,255,255,${strength})`);light.addColorStop(.12,`rgba(235,239,237,${strength*.5})`);light.addColorStop(.32,'rgba(240,243,241,0)');light.addColorStop(.7,'rgba(240,243,241,0)');light.addColorStop(.9,`rgba(213,219,216,${strength*.65})`);light.addColorStop(1,`rgba(255,255,255,${strength*.3})`);return light})();c.lineWidth=.75;c.stroke(lip);c.restore();
  entry.key=key;entry.rect={left:r.left,top:r.top,width:w,height:h};entry.rim=rim;entry.amplitude=amp;entry.draws++;paintCount++;totalPaintMs+=performance.now()-start;
 }
 function ensureMaterial(host){
  if(materials.has(host))return;
  if(getComputedStyle(host).position==='static')host.dataset.glassPositioned='true';
  const plane=host.querySelector(':scope > .glass-reader-material')||host,canvas=document.createElement('canvas');canvas.className='glass-material-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');plane.prepend(canvas);host.dataset.glassMaterial='true';materials.set(host,{host,plane,canvas,key:'',draws:0});
 }
 let savedListReading=null;
 function readerScrollHost(){const owner=document.querySelector('.content');return root.dataset.palette==='glass'&&owner&&getComputedStyle(owner).overflowY==='auto'?owner:null}
 function layoutReaderPorts(){
  if(root.dataset.palette!=='glass')return;
  for(const owner of document.querySelectorAll('.content')){const style=getComputedStyle(owner),values={width:owner.clientWidth,height:owner.clientHeight,top:parseFloat(style.paddingTop)||0,left:parseFloat(style.paddingLeft)||0};for(const [name,value]of Object.entries(values)){const key='--glass-reader-port-'+name,v=value+'px';if(owner.style.getPropertyValue(key)!==v)owner.style.setProperty(key,v)}
   // Fixed descendants are still clipped by the native reading port. Keep
   // their controls in its actual visible paddingbox, including viewport pan.
   const r=owner.getBoundingClientRect(),v=window.visualViewport,left=Math.max(0,v?.offsetLeft||0,r.left+owner.clientLeft)+8,top=Math.max(0,v?.offsetTop||0,r.top+owner.clientTop)+8,right=Math.min(innerWidth,(v?.offsetLeft||0)+(v?.width||innerWidth),r.left+owner.clientLeft+owner.clientWidth)-8,bottom=Math.min(innerHeight,(v?.offsetTop||0)+(v?.height||innerHeight),r.top+owner.clientTop+owner.clientHeight)-8;
   for(const [name,value]of Object.entries({left,top,right,bottom,width:Math.max(0,right-left),height:Math.max(0,bottom-top)})){const key='--glass-toc-'+name,next=value+'px';if(owner.style.getPropertyValue(key)!==next)owner.style.setProperty(key,next)}
  }
 }
 function syncGeometry(){
  geometryFrame=0;if(!enabled||!sampler)return;layoutReaderPorts();
  if(root.dataset.background==='cozy'){const desired=materialCandidates(),keep=new Set(desired);for(const e of [...materials.values()])if(!keep.has(e.host))removeMaterial(e);for(const host of desired)ensureMaterial(host);}
  const all=[...materials.values(),...entries.values()].filter(e=>e.canvas.isConnected&&e.host.isConnected),ports=new Map();
  // A canvas inside an overflow port must stay with its viewport, not with
  // scrolled text. Read dimensions first, then apply only changed port metrics.
  for(const e of all)if(!ports.has(e.host)){const s=getComputedStyle(e.host),port=((e.plane||e.host)===e.host||e.host.matches('.content'))&&/^(auto|scroll)$/.test(s.overflowY);ports.set(e.host,port?{width:e.host.clientWidth,height:e.host.clientHeight,top:parseFloat(s.paddingTop)||0,left:parseFloat(s.paddingLeft)||0}:null)}
  for(const e of all){const port=ports.get(e.host),key=port?Object.values(port).join('|'):'';if(e.portKey===key)continue;e.portKey=key;
   if(port&&!e.scrollPlane&&(e.plane||e.host)===e.host){const plane=document.createElement('div');plane.className='glass-scroll-plane';plane.setAttribute('aria-hidden','true');e.canvas.before(plane);plane.append(e.canvas);e.scrollPlane=plane}
   if(!port&&e.scrollPlane){e.scrollPlane.before(e.canvas);e.scrollPlane.remove();e.scrollPlane=null}
   e.canvas.classList.toggle('glass-scroll-optics',!!port);for(const name of ['width','height','top','left'])if(port)e.canvas.style.setProperty('--glass-scroll-'+name,port[name]+'px');else e.canvas.style.removeProperty('--glass-scroll-'+name)
  }
  // These viewport boxes and body::before use the same coordinate space.
  // Sample the canvas itself: its border/padding offset differs from its owner.
  const boxes=all.map(entry=>({entry,rect:entry.canvas.getBoundingClientRect()}));
  adaptPanels();
  for(const {entry,rect}of boxes){if(rect.width<8||rect.height<8||rect.bottom<=0||rect.right<=0||rect.top>=innerHeight||rect.left>=innerWidth)continue;if(materials.get(entry.host)===entry){paintNativeMaterial(entry,rect)}else paint(entry,rect)}
 }
 function scheduleGeometry(){if(!geometryFrame&&!frame)geometryFrame=requestAnimationFrame(syncGeometry)}
 // App renders replace foreground children. Keep an existing decorative scene
 // on a surviving reader owner through that transaction, before another paint.
 function captureReaders(){return [...document.querySelectorAll('.content,.terminal-main')].map(owner=>({owner,plane:owner.querySelector(':scope > .glass-reader-material'),scrollTop:owner.scrollTop,scrollLeft:owner.scrollLeft})).filter(({owner,plane})=>plane||owner===readerScrollHost())}
 function restoreReaders(saved){
  if(root.dataset.palette!=='glass')return;
  let restored=false;
  for(const {owner,plane,scrollTop,scrollLeft}of saved||[])if(owner.isConnected){if(enabled&&plane&&!plane.isConnected&&!owner.querySelector(':scope > .glass-reader-material')){owner.prepend(plane);restored=true}if(owner.scrollTop!==(scrollTop||0))owner.scrollTop=scrollTop||0;if(owner.scrollLeft!==(scrollLeft||0))owner.scrollLeft=scrollLeft||0}
  if(enabled&&restored){if(geometryFrame){cancelAnimationFrame(geometryFrame);geometryFrame=0}syncGeometry()}
 }
 function update(){
  frame=0;layoutReaderPorts();const active=root.dataset.palette==='glass'&&!reduced.matches&&supports;
  if(geometryFrame){cancelAnimationFrame(geometryFrame);geometryFrame=0}
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
  for(const e of [...materials.values()])if(!cozy||!materialKeep.has(e.host)||!e.host.isConnected||!e.plane.isConnected||!e.canvas.isConnected||getComputedStyle(e.host).display==='none')removeMaterial(e);
  for(const host of materialHosts){if(host.dataset.glassPositioned==='true'){delete host.dataset.glassPositioned;if(getComputedStyle(host).position==='static')host.dataset.glassPositioned='true';}ensureMaterial(host);}
  const desired=cozy?possible:[],keep=new Set(desired.map(x=>x.host));for(const entry of [...entries.values()])if(!keep.has(entry.host)||!entry.host.isConnected)remove(entry);
  for(const {host,kind}of desired){let entry=entries.get(host);if(!entry){const canvas=document.createElement('canvas');canvas.className='glass-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');host.prepend(canvas);entry={host,canvas,kind,key:'',draws:0};entries.set(host,entry)}}
  syncGeometry();
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(update)}
 new MutationObserver(schedule).observe(root,{attributes:true,attributeFilter:['data-palette','data-theme','data-background']});
 new MutationObserver(schedule).observe(document.body,{childList:true,subtree:true});
 // Native details/hidden changes do not necessarily insert nodes or move focus.
 new MutationObserver(schedule).observe(document.body,{attributes:true,subtree:true,attributeFilter:['open','hidden']});
 window.addEventListener('resize',schedule,{passive:true});document.addEventListener('scroll',scheduleGeometry,{passive:true,capture:true});document.addEventListener('focusin',schedule);document.addEventListener('focusout',schedule);
 window.visualViewport?.addEventListener('resize',schedule,{passive:true});window.visualViewport?.addEventListener('scroll',scheduleGeometry,{passive:true});reduced.addEventListener?.('change',schedule);
 window.TaskMeccaGlass=Object.freeze({prepareReaderLayout:layoutReaderPorts,scrollHost:readerScrollHost,rememberListReading(key){const owner=readerScrollHost();if(owner)savedListReading={key,x:owner.scrollLeft,y:owner.scrollTop}},restoreListReading(key){const saved=savedListReading,owner=readerScrollHost();if(owner&&saved?.key===key){owner.scrollTo({left:saved.x,top:saved.y,behavior:'auto'});savedListReading=null}},get scrollY(){return readerScrollHost()?.scrollTop??window.scrollY},get scrollX(){return readerScrollHost()?.scrollLeft??window.scrollX},scrollTo(...args){(readerScrollHost()||window).scrollTo(...args)},readingViewport(){const owner=readerScrollHost();return owner?{top:owner.getBoundingClientRect().top,height:owner.clientHeight,paddingBottom:parseFloat(getComputedStyle(owner).paddingBottom)||0}:null},refresh:schedule,captureReaders,restoreReaders,diagnostics:()=>({savedListReading:savedListReading?{...savedListReading}:null,mode:root.dataset.glassOptics||'material',photo:imageURL,sample:{generation,theme:sampleTheme,background:sampleBackground,source:sampleSource},loading:!!loading,pending:!!frame||!!geometryFrame,readerBackingSupported,stage:geometry(),sampler:sampler?{width:sampler.width,height:sampler.height}:null,paintCount,totalPaintMs,adaptCount,totalAdaptMs,sampleBytes:samplePixels?.byteLength||0,panels:[...panels.values()],materialCount:materials.size,materialPixelsTotal:[...materials.values()].reduce((n,e)=>n+e.canvas.width*e.canvas.height,0),nativeGeometryObjects:[...materials.values()].reduce((n,e)=>n+(e.nativeProfile?e.nativeProfile.rings.length+e.nativeProfile.lights.length:0),0),profileCacheBytes:[...materials.values()].reduce((n,e)=>n+(e.profileCache?.bytes||0),0),profileCacheCount:[...materials.values()].filter(e=>e.profileCache).length,materials:[...materials.values()].map(e=>({backend:e.backend||'cpu-pixels',nativeGeometryObjects:e.nativeProfile?e.nativeProfile.rings.length+e.nativeProfile.lights.length:0,cacheBytes:e.profileCache?.bytes||0,cachePoints:e.profileCache?.points||0,cacheBuilds:e.profileCacheBuilds||0,cacheHits:e.profileCacheHits||0,cacheBuildMs:e.profileCacheBuildMs||0,host:e.host.className,role:e.role,tint:e.tint,radius:e.radius,bevel:e.bevel,bend:MATERIAL_BEND,profile:e.backend==='native-canvas'?'native-convex':'continuous-convex',reflection:MATERIAL_REFLECTION,rect:e.rect,amplitude:e.amplitude,paintMs:e.paintMs,totalPaintMs:e.totalPaintMs,draws:e.draws,pixels:e.canvas.width*e.canvas.height})),lenses:[...entries.values()].map(e=>({kind:e.kind,rect:e.rect,rim:e.rim,amplitude:e.amplitude,draws:e.draws,pixels:e.canvas.width*e.canvas.height}))})});
 if(typeof ResizeObserver!=='undefined'){const resize=new ResizeObserver(schedule);document.querySelectorAll('.topbar,.terminal-header,.sidebar-head').forEach(e=>resize.observe(e))}
 document.fonts?.ready.then(schedule);
 schedule();
})();
