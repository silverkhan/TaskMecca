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
  const start=performance.now(),g=geometry(),hosts=[...document.querySelectorAll('.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.tm-combo-popup,.notification-panel,.common-attention-body,.modal-card,.channel-switch-modal,.mobile-sidebar-panel,.preview-identity-banner,.user-attention')],keep=new Set(hosts);
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
 function removeMaterial(entry){entry.profileCache=null;removeCanvas(entry);delete entry.host.dataset.glassMaterial;materials.delete(entry.host);}
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
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none';
  }).slice(0,MAX_LENSES);
 }
 function materialCandidates(){
  return [...document.querySelectorAll('.tm-combo-popup,.topbar,.sidebar,.sidebar-head,.content,.terminal-header,.terminal-main,.mobile-sidebar-panel,.preview-identity-banner,.user-attention,.common-attention-body,.notification-panel,.modal-card,.channel-switch-modal')].filter(host=>{
   const r=host.getBoundingClientRect(),s=getComputedStyle(host);
   return r.width>8&&r.height>8&&r.bottom>0&&r.right>0&&r.top<innerHeight&&r.left<innerWidth&&s.visibility!=='hidden'&&s.display!=='none'&&!(innerWidth<=680&&host.classList.contains('sidebar'))&&!(innerWidth>680&&host.classList.contains('sidebar-head'))&&(readerBackingSupported||!host.matches('.content,.terminal-main'));
  });
 }
 function materialRole(host){return host.matches('.content,.terminal-main')?'reader':host.matches('.tm-combo-popup,.mobile-sidebar-panel,.common-attention-body,.notification-panel,.modal-card,.channel-switch-modal')?'floating':'chrome'}
 // One continuous convex face, not concentric photo bands or stroke rails.
 // Geometry/lighting coefficients belong to one current-geometry cache per entry.
 // Scroll changes scene coordinates, so the actual scene is resampled every paint.
 function materialProfile(entry,w,h,radius){
  const cw=entry.canvas.width,ch=entry.canvas.height,band=Math.min(MATERIAL_BEVEL,w/4,h/3);
  if(!band)return null;
  const key=[cw,ch,w,h,radius,MATERIAL_BEVEL,MATERIAL_BEND,MATERIAL_REFLECTION].join('|'),cached=entry.profileCache;
  if(cached?.key===key){entry.profileCacheHits=(entry.profileCacheHits||0)+1;return cached;}
  const start=performance.now(),px=cw/w,py=ch/h,bx=Math.ceil(band*px),by=Math.ceil(band*py),tiles=[];
  entry.profileCache=null; // Release the previous geometry; never retain history.
  for(const [tx,ty,tw,th]of [[0,0,cw,by],[0,ch-by,cw,by],[0,by,bx,ch-2*by],[cw-bx,by,bx,ch-2*by]]){
   if(tw<1||th<1)continue;
   const coefficients=new Float64Array(tw*th*8);let count=0;
   for(let y=0;y<th;y++)for(let x=0;x<tw;x++){
    const xx=(tx+x+.5)/px,yy=(ty+y+.5)/py,dx=xx-w/2,dy=yy-h/2,qx=Math.abs(dx)-(w/2-radius),qy=Math.abs(dy)-(h/2-radius);
    const ax=Math.max(qx,0),ay=Math.max(qy,0),length=Math.hypot(ax,ay),distance=radius-length-Math.min(Math.max(qx,qy),0);
    if(distance<0||distance>band)continue;
    const nx=length?ax/length*Math.sign(dx):qx>qy?Math.sign(dx):0,ny=length?ay/length*Math.sign(dy):qy>=qx?Math.sign(dy):0;
    const width=band*(.64+.36*Math.abs(nx));if(distance>width)continue;
    const t=distance/width,fall=(1-t)**2,bend=MATERIAL_BEND*fall,mix=.68*fall;
    const light=Math.max(0,-nx*.65-ny*.76),position=.38+.62*Math.exp(-1*((xx/w*.7+yy/h*.3-.12)*2.5)**2);
    // One broad lobe returns continuously through neutral inner shade.
    const lobe=.82*Math.exp(-1*((t-.31)/.22)**2)*(.16+.84*light**3)*position;
    const lip=.38*Math.exp(-distance/.55)*(.25+.75*light),shine=Math.min(.92,(lobe+lip)*MATERIAL_REFLECTION),shade=.13*Math.exp(-1*((t-.79)/.17)**2)*(.35+.65*(1-light));
    coefficients.set([(y*tw+x)*4,xx,yy,nx*bend,ny*bend,mix,shine,shade],count++*8);
   }
   tiles.push({tx,ty,tw,th,coefficients,count});
  }
  const profile={key,band,tiles,bytes:tiles.reduce((n,t)=>n+t.coefficients.byteLength,0),points:tiles.reduce((n,t)=>n+t.count,0)};
  entry.profileCacheBuilds=(entry.profileCacheBuilds||0)+1;entry.profileCacheBuildMs=(entry.profileCacheBuildMs||0)+performance.now()-start;entry.profileCache=profile;return profile;
 }
 function paintMaterialBevel(entry,c,r,radius){
  const profile=materialProfile(entry,r.width,r.height,radius);if(!profile)return 0;
  const g=geometry(),sw=sampler.width,sh=sampler.height,pixels=samplePixels,neutral=root.dataset.theme==='light'?[249,250,248]:[24,28,29],tint=panels.get(entry.host)?.alpha??.18;
  for(const {tx,ty,tw,th,coefficients:points,count}of profile.tiles){
   const field=c.getImageData(tx,ty,tw,th),data=field.data;
   for(let i=0;i<count*8;i+=8){
    const at=points[i],xx=points[i+1],yy=points[i+2],bendX=points[i+3],bendY=points[i+4],mix=points[i+5],shine=points[i+6],shade=points[i+7];
    const sceneX=r.left+xx-bendX,sceneY=r.top+yy-bendY;
    const cx=Math.min(sw-1,Math.max(0,Math.round(sceneX/g.width*sw))),cy=Math.min(sh-1,Math.max(0,Math.round(sceneY/g.height*sh)));
    const lx=Math.min(sw-1,Math.max(0,Math.round((sceneX-4)/g.width*sw))),rx=Math.min(sw-1,Math.max(0,Math.round((sceneX+4)/g.width*sw))),uy=Math.min(sh-1,Math.max(0,Math.round((sceneY-4)/g.height*sh))),dy=Math.min(sh-1,Math.max(0,Math.round((sceneY+4)/g.height*sh)));
    const center=(cy*sw+cx)*4,left=(cy*sw+lx)*4,right=(cy*sw+rx)*4,up=(uy*sw+cx)*4,down=(dy*sw+cx)*4;
    // Same five-tap arithmetic, with scene coordinates and Canvas dimensions
    // reused rather than crossing the native Canvas getters per channel/tap.
    for(let k=0;k<3;k++){const scene=pixels[center+k]*.5+pixels[left+k]*.125+pixels[right+k]*.125+pixels[up+k]*.125+pixels[down+k]*.125,transmitted=scene*(1-tint)+neutral[k]*tint,value=data[at+k]*(1-mix)+transmitted*mix;data[at+k]=(value+(255-value)*shine)*(1-shade)}
   }
   c.putImageData(field,tx,ty);
  }
  return profile.band;
 }
 function paintMaterial(entry,r){
  const {host,canvas}=entry,w=r.width,h=r.height;if(w<8||h<8)return;
  // Material depends only on the scene and panel geometry, never on ink.
  const radius=Math.min(parseFloat(getComputedStyle(entry.plane).borderTopLeftRadius)||28,w/2,h/2),role=materialRole(host);
  const key=[generation,r.x,r.y,w,h,radius,host.dataset.glassTone,panels.get(host)?.alpha].join("|");if(entry.key===key)return;
  const start=performance.now(),scale=Math.min(1,Math.sqrt(MAX_MATERIAL_PIXELS/(w*h))),width=Math.max(1,Math.floor(w*scale)),height=Math.max(1,Math.floor(h*scale)),g=geometry();
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
  // Soften transmission before the bevel is drawn, keeping its lip crisp.
  c.save();c.setTransform(1,0,0,1,0,0);c.filter='blur(12px)';c.globalCompositeOperation='copy';c.drawImage(canvas,0,0);c.restore();
  const light=host.dataset.glassTone==='light',neutral=light?[249,250,248]:[24,28,29],tint=panels.get(host)?.alpha??.18;
  c.fillStyle=`rgba(${neutral},${tint})`;c.fillRect(0,0,w,h);
  const reflection=c.createLinearGradient(0,0,w*.8,h);reflection.addColorStop(0,'rgba(255,255,255,.16)');reflection.addColorStop(.25,'rgba(255,255,255,.025)');reflection.addColorStop(.7,'rgba(255,255,255,0)');reflection.addColorStop(1,'rgba(10,12,13,.08)');c.fillStyle=reflection;c.fillRect(0,0,w,h);
  entry.bevel=paintMaterialBevel(entry,c,r,radius);entry.radius=radius;entry.role=role;entry.tint=tint;
  entry.key=key;entry.rect={left:r.x,top:r.y,width:w,height:h};entry.amplitude=amplitude;entry.paintMs=performance.now()-start;entry.totalPaintMs=(entry.totalPaintMs||0)+entry.paintMs;entry.draws++;paintCount++;totalPaintMs+=entry.paintMs;
 }
 function paint(entry,r){
  if(!sampler)return;
  const {host,canvas,kind}=entry,style=getComputedStyle(host),radius=Math.min(parseFloat(style.borderTopLeftRadius)||16,r.width/2,r.height/2);
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
  c.save();c.strokeStyle=(()=>{const light=c.createLinearGradient(0,0,w*.72,h);light.addColorStop(0,`rgba(255,255,255,${strength})`);light.addColorStop(.12,`rgba(235,239,237,${strength*.5})`);light.addColorStop(.32,'rgba(240,243,241,0)');light.addColorStop(.7,'rgba(240,243,241,0)');light.addColorStop(.9,`rgba(213,219,216,${strength*.65})`);light.addColorStop(1,`rgba(255,255,255,${strength*.3})`);return light})();c.lineWidth=.75;c.stroke(lip);c.restore();
  entry.key=key;entry.rect={left:r.left,top:r.top,width:w,height:h};entry.rim=rim;entry.amplitude=amp;entry.draws++;paintCount++;totalPaintMs+=performance.now()-start;
 }
 function ensureMaterial(host){
  if(materials.has(host))return;
  const plane=host.querySelector(':scope > .glass-reader-material')||host,canvas=document.createElement('canvas');canvas.className='glass-material-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');plane.prepend(canvas);host.dataset.glassMaterial='true';materials.set(host,{host,plane,canvas,key:'',draws:0});
 }
 function syncGeometry(){
  geometryFrame=0;if(!enabled||!sampler)return;
  if(root.dataset.background==='cozy')for(const host of materialCandidates())ensureMaterial(host);
  const all=[...materials.values(),...entries.values()].filter(e=>e.canvas.isConnected&&e.host.isConnected),ports=new Map();
  // A canvas inside an overflow port must stay with its viewport, not with
  // scrolled text. Read dimensions first, then apply only changed port metrics.
  for(const e of all)if(!ports.has(e.host)){const s=getComputedStyle(e.host),port=(e.plane||e.host)===e.host&&/^(auto|scroll)$/.test(s.overflowY);ports.set(e.host,port?{width:e.host.clientWidth,height:e.host.clientHeight,top:parseFloat(s.paddingTop)||0,left:parseFloat(s.paddingLeft)||0}:null)}
  for(const e of all){const port=ports.get(e.host),key=port?Object.values(port).join('|'):'';if(e.portKey===key)continue;e.portKey=key;
   if(port&&!e.scrollPlane){const plane=document.createElement('div');plane.className='glass-scroll-plane';plane.setAttribute('aria-hidden','true');e.canvas.before(plane);plane.append(e.canvas);e.scrollPlane=plane}
   if(!port&&e.scrollPlane){e.scrollPlane.before(e.canvas);e.scrollPlane.remove();e.scrollPlane=null}
   e.canvas.classList.toggle('glass-scroll-optics',!!port);for(const name of ['width','height','top','left'])if(port)e.canvas.style.setProperty('--glass-scroll-'+name,port[name]+'px');else e.canvas.style.removeProperty('--glass-scroll-'+name)
  }
  // These viewport boxes and body::before use the same coordinate space.
  // Sample the canvas itself: its border/padding offset differs from its owner.
  const boxes=all.map(entry=>({entry,rect:entry.canvas.getBoundingClientRect()}));
  adaptPanels();
  for(const {entry,rect}of boxes){if(rect.width<8||rect.height<8||rect.bottom<=0||rect.right<=0||rect.top>=innerHeight||rect.left>=innerWidth)continue;if(materials.get(entry.host)===entry)paintMaterial(entry,rect);else paint(entry,rect)}
 }
 function scheduleGeometry(){if(!geometryFrame&&!frame)geometryFrame=requestAnimationFrame(syncGeometry)}
 // App renders replace foreground children. Keep an existing decorative scene
 // on a surviving reader owner through that transaction, before another paint.
 function captureReaders(){return [...document.querySelectorAll('.content,.terminal-main')].map(owner=>({owner,plane:owner.querySelector(':scope > .glass-reader-material')})).filter(({plane})=>plane)}
 function restoreReaders(saved){
  if(!enabled||root.dataset.palette!=='glass')return;
  let restored=false;
  for(const {owner,plane}of saved||[])if(owner.isConnected&&!plane.isConnected&&!owner.querySelector(':scope > .glass-reader-material')){owner.prepend(plane);restored=true}
  if(restored){if(geometryFrame){cancelAnimationFrame(geometryFrame);geometryFrame=0}syncGeometry()}
 }
 function update(){
  frame=0;const active=root.dataset.palette==='glass'&&!reduced.matches&&supports;
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
  const possible=candidates(),materialHosts=cozy?materialCandidates():[];
  for(const e of [...materials.values()])if(!cozy||!e.host.isConnected||!e.plane.isConnected||!e.canvas.isConnected||getComputedStyle(e.host).display==='none')removeMaterial(e);
  for(const host of materialHosts)ensureMaterial(host);
  const desired=cozy?possible:[],keep=new Set(desired.map(x=>x.host));for(const entry of [...entries.values()])if(!keep.has(entry.host)||!entry.host.isConnected)remove(entry);
  for(const {host,kind}of desired){let entry=entries.get(host);if(!entry){const canvas=document.createElement('canvas');canvas.className='glass-optics';canvas.setAttribute('aria-hidden','true');canvas.setAttribute('role','presentation');host.prepend(canvas);entry={host,canvas,kind,key:'',draws:0};entries.set(host,entry)}}
  syncGeometry();
 }
 function schedule(){if(!frame)frame=requestAnimationFrame(update)}
 new MutationObserver(schedule).observe(root,{attributes:true,attributeFilter:['data-palette','data-theme','data-background']});
 new MutationObserver(schedule).observe(document.body,{childList:true,subtree:true});
 window.addEventListener('resize',schedule,{passive:true});document.addEventListener('scroll',scheduleGeometry,{passive:true,capture:true});document.addEventListener('focusin',schedule);document.addEventListener('focusout',schedule);
 window.visualViewport?.addEventListener('resize',schedule,{passive:true});window.visualViewport?.addEventListener('scroll',scheduleGeometry,{passive:true});reduced.addEventListener?.('change',schedule);
 window.TaskMeccaGlass=Object.freeze({refresh:schedule,captureReaders,restoreReaders,diagnostics:()=>({mode:root.dataset.glassOptics||'material',photo:imageURL,sample:{generation,theme:sampleTheme,background:sampleBackground,source:sampleSource},loading:!!loading,pending:!!frame||!!geometryFrame,readerBackingSupported,stage:geometry(),sampler:sampler?{width:sampler.width,height:sampler.height}:null,paintCount,totalPaintMs,adaptCount,totalAdaptMs,sampleBytes:samplePixels?.byteLength||0,panels:[...panels.values()],profileCacheBytes:[...materials.values()].reduce((n,e)=>n+(e.profileCache?.bytes||0),0),profileCacheCount:[...materials.values()].filter(e=>e.profileCache).length,materials:[...materials.values()].map(e=>({cacheBytes:e.profileCache?.bytes||0,cachePoints:e.profileCache?.points||0,cacheBuilds:e.profileCacheBuilds||0,cacheHits:e.profileCacheHits||0,cacheBuildMs:e.profileCacheBuildMs||0,host:e.host.className,role:e.role,tint:e.tint,radius:e.radius,bevel:e.bevel,bend:MATERIAL_BEND,profile:"continuous-convex",reflection:MATERIAL_REFLECTION,rect:e.rect,amplitude:e.amplitude,paintMs:e.paintMs,totalPaintMs:e.totalPaintMs,draws:e.draws,pixels:e.canvas.width*e.canvas.height})),lenses:[...entries.values()].map(e=>({kind:e.kind,rect:e.rect,rim:e.rim,amplitude:e.amplitude,draws:e.draws,pixels:e.canvas.width*e.canvas.height}))})});
 if(typeof ResizeObserver!=='undefined'){const resize=new ResizeObserver(schedule);document.querySelectorAll('.topbar,.terminal-header,.sidebar-head').forEach(e=>resize.observe(e))}
 document.fonts?.ready.then(schedule);
 schedule();
})();
