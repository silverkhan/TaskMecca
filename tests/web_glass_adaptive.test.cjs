// Actual app assets on a disposable dynamic port; raster evidence stays local.
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),zlib=require('node:zlib');
const fixture=require('./helpers/glass_fixture.cjs');
const {installOpticalOracle}=require('./helpers/glass_optical_oracle.cjs');
const {waitForGlass,captureGlass,installGlassUpdateGate}=require('./helpers/glass_state.cjs');
function raster(png){
 let w,h,n,at=8,blocks=[];while(at<png.length){const l=png.readUInt32BE(at),kind=png.toString('ascii',at+4,at+8),b=png.subarray(at+8,at+8+l);at+=l+12;if(kind==='IHDR'){w=b.readUInt32BE(0);h=b.readUInt32BE(4);n={2:3,6:4}[b[9]];assert.ok(n)}if(kind==='IDAT')blocks.push(b)}
 const raw=zlib.inflateSync(Buffer.concat(blocks)),stride=w*n,pixels=Buffer.alloc(w*h*n);let prev=Buffer.alloc(stride),p=0;
 for(let y=0;y<h;y++){const filter=raw[p++],row=Buffer.from(raw.subarray(p,p+stride));p+=stride;for(let x=0;x<stride;x++){const a=x>=n?row[x-n]:0,b=prev[x],c=x>=n?prev[x-n]:0,v=a+b-c,d=[Math.abs(v-a),Math.abs(v-b),Math.abs(v-c)];row[x]=(row[x]+(filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?[a,b,c][d.indexOf(Math.min(...d))]:0))&255}row.copy(pixels,y*stride);prev=row}
 return{w,h,pixel:(x,y)=>[...pixels.subarray((Math.floor(y)*w+Math.floor(x))*n,(Math.floor(y)*w+Math.floor(x))*n+3)]};
}
const lum=rgb=>rgb.map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
const contrast=(a,b)=>(Math.max(lum(a),lum(b))+.05)/(Math.min(lum(a),lum(b))+.05);
const enabled=!!process.env.GLASS_PLAYWRIGHT_MODULE;
test('Glass adapts actual panel pixels, keeps a stable reading polarity and isolates the spectral lip',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web')),browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE}),out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR,'adaptive');
 if(out)fs.mkdirSync(out,{recursive:true});const report={browser:browser.version(),platform:process.platform,node:process.version,locale:'en-US',origin:server.origin,captures:[],scenes:[],spectrum:[],focus:[],stability:[],fallback:[],errors:[],cleanup:{}};
 try{for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1,locale:'en-US'});
  try{
   await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
   await ctx.addInitScript(installOpticalOracle);
   await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');window.EventSource=class{addEventListener(){}close(){}}});
   const p=await ctx.newPage();p.on('pageerror',e=>report.errors.push(e.message));await p.goto(server.origin);await p.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().photo);
   await p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},JSON.parse(JSON.stringify(fixture)));
   const settle=()=>waitForGlass(p);
   // The same shipped photograph at controlled exposures, selected through the
   // real CSS photo input. No preset/theme-to-tone lookup can pass this test.
   await p.evaluate(async()=>{const img=new Image();img.src=getComputedStyle(document.documentElement).getPropertyValue('--glass-room-image').match(/url\(["']?([^"')]+)/)[1];await img.decode();window.fixtureExposure=kind=>{const c=document.createElement('canvas');c.width=800;c.height=600;const x=c.getContext('2d');x.drawImage(img,0,0,c.width,c.height);if(kind==='mixed'){x.fillStyle='rgba(255,255,255,.8)';x.fillRect(0,0,400,600);x.fillStyle='rgba(0,0,0,.93)';x.fillRect(400,0,400,600)}else{x.fillStyle=kind==='bright'?'rgba(255,255,255,.8)':kind==='dark'?'rgba(0,0,0,.93)':`rgba(160,160,160,.98)`;x.fillRect(0,0,800,600)}document.documentElement.style.setProperty('--glass-room-image',`url("${c.toDataURL()}")`);TaskMeccaGlass.refresh()}});
   for(const theme of ['light','dark'])for(const kind of ['bright','dark','mixed']){
    await p.evaluate(({theme,kind})=>{applyTheme(theme);fixtureExposure(kind)},{theme,kind});await settle();
    const {text,normal,background}=await captureGlass(p,async()=>{
     const text=await p.locator('.topbar,.content,.preview-identity-banner,.user-attention').evaluateAll(hosts=>hosts.flatMap(host=>{const walk=document.createTreeWalker(host,NodeFilter.SHOW_TEXT),a=[];let n;while(n=walk.nextNode()){const e=n.parentElement;if(!n.textContent.trim()||e.closest('script,style')||!e.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect(),s=getComputedStyle(e);let opacity=1;for(let ancestor=e;ancestor;ancestor=ancestor.parentElement)opacity*=Number(getComputedStyle(ancestor).opacity);if(b.x>2&&b.y>2&&b.right<innerWidth-2&&b.bottom<innerHeight-2&&b.width>5)a.push({opacity,disabled:!!e.closest('[disabled],[aria-disabled="true"]'),text:n.textContent.trim().slice(0,40),rect:b.toJSON(),color:s.color,font:parseFloat(s.fontSize),weight:parseFloat(s.fontWeight)});}return a}));
    text.push(...await p.locator('.content input:not([type=checkbox]):not([type=radio]):not([type=range])').evaluateAll(inputs=>inputs.flatMap(e=>{const r=e.getBoundingClientRect();if(!e.checkVisibility()||r.x<2||r.y<2||r.right>=innerWidth-2||r.bottom>=innerHeight-2||!e.value&&!e.placeholder)return[];const s=getComputedStyle(e,e.value?null:'::placeholder');return[{opacity:1,disabled:e.disabled,text:'form: '+(e.value||e.placeholder),rect:r.toJSON(),color:s.color,font:parseFloat(s.fontSize),weight:parseFloat(s.fontWeight)}]})));
    const normal=await p.screenshot({animations:'disabled'}),hide=await p.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'}),background=raster(await p.screenshot({animations:'disabled'}));await hide.evaluate(e=>e.remove());
     return{text,normal,background};
    },report.captures);
    assert.ok(text.length>8,'actual app text must be measured');
    const samples=text.map(t=>{const rgba=t.color.match(/[\d.]+/g).map(Number),color=rgba.slice(0,3),alpha=(rgba[3]??1)*t.opacity,r=t.rect,ratios=[];for(const dx of [.15,.5,.85])for(const dy of [.2,.5,.8]){const bg=background.pixel(r.x+r.width*dx,r.y+r.height*dy);ratios.push(contrast(color.map((v,k)=>v*alpha+bg[k]*(1-alpha)),bg));}return{...t,minimum:Math.min(...ratios),required:t.font>=24||(t.font>=18.66&&t.weight>=700)?3:4.5}});
    const bad=samples.filter(s=>!s.disabled&&s.minimum<s.required);report.scenes.push({mobile,theme,kind,samples,panels:await p.evaluate(()=>TaskMeccaGlass.diagnostics().panels)});if(out)fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-${theme}-${kind}.png`),normal);
    if(out)fs.writeFileSync(path.join(out,'adaptive.json'),JSON.stringify(report,null,2));
    assert.deepEqual(bad,[],`${theme}/${kind}: representative actual text pixels meet contrast`);
    if(kind!=='mixed'){const actual=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone,color:getComputedStyle(e).color}));assert.equal(actual.tone,kind==='bright'?'light':'dark');}
   }
   // Wide dead band: both directions retain polarity under small repeated moves.
   for(const initial of ['bright','dark']){
    await p.evaluate(v=>fixtureExposure(v),initial);await settle();await p.evaluate(()=>fixtureExposure('threshold'));await settle();const tones=[];
    for(let i=0;i<8;i++){await p.setViewportSize({width:(mobile?390:1440)+(i%2?2:0),height:mobile?844:960});await p.evaluate(v=>{document.body.style.minHeight='2500px';scrollTo(0,v%2?5:0)},i);await settle();tones.push(await p.locator('.topbar').getAttribute('data-glass-tone'))}
    report.stability.push({mobile,initial,tones});assert.ok(tones.every(t=>t===(initial==='bright'?'light':'dark')),'threshold scroll/resize retains panel polarity');
   }
   await p.evaluate(()=>{document.body.style.minHeight='';scrollTo(0,0)});
   for(const kind of ['bright','dark']){
    await p.evaluate(v=>fixtureExposure(v),kind);await settle();await p.keyboard.press('Tab');await p.locator('.palette-picker .tm-combo-trigger').focus();
    const ring=await p.locator('.palette-picker .tm-combo-trigger').evaluate(e=>{const r=e.getBoundingClientRect(),s=getComputedStyle(e);return{rect:r.toJSON(),width:parseFloat(s.outlineWidth),offset:parseFloat(s.outlineOffset),style:s.outlineStyle}}),shot=raster(await p.screenshot({animations:'disabled'}));
    assert.equal(ring.style,'solid');const x=ring.rect.x-ring.offset-ring.width/2,y=ring.rect.y+ring.rect.height/2,ratio=contrast(shot.pixel(x,y),shot.pixel(x-ring.width-2,y));report.focus.push({mobile,kind,ratio,ring});assert.ok(ratio>=3,'actual keyboard focus outline has 3:1 adjacent contrast');
   }
   await p.evaluate(()=>{document.body.style.minHeight='';scrollTo(0,0);fixtureExposure('dark')});await settle();await p.locator('.palette-picker .tm-combo-trigger').click();await settle();
   // Render the real canvas with and without ONLY its spectral stroke. The draw
   // observer supplies an independent replay for the optical displacement test.
   await p.keyboard.press('Escape');
   const {positive,negative,box,copyControl,rimMask}=await captureGlass(p,async()=>{
    const positive=await p.screenshot({animations:'disabled'}),box=await p.locator('.topbar .glass-optics').boundingBox();
    await p.locator('.topbar .glass-optics').evaluate(e=>__glassSpectralControl(e,'copy'));
    const copyControl=await p.screenshot({animations:'disabled'});
    await p.locator('.topbar .glass-optics').evaluate(e=>__glassSpectralControl(e));let negative;
    try{negative=await p.screenshot({animations:'disabled'})}finally{await p.locator('.topbar .glass-optics').evaluate(e=>__glassSpectralControl(e,true))}
    const radius=await p.locator('.topbar').evaluate(e=>getComputedStyle(e).borderTopLeftRadius);
    await p.evaluate(({box,radius})=>{const overlay=document.createElement('div');overlay.id='fixture-rim-reference';overlay.style.cssText='position:fixed;inset:0;background:#000;z-index:2147483647;pointer-events:none';const ring=document.createElement('div');ring.style.cssText=`position:absolute;box-sizing:border-box;left:${box.x}px;top:${box.y}px;width:${box.width}px;height:${box.height}px;border:2px solid #fff;border-radius:${radius}`;overlay.append(ring);document.documentElement.append(overlay)},{box,radius});
    let rimMask;try{rimMask=await p.screenshot({animations:'disabled'})}finally{await p.evaluate(()=>document.getElementById('fixture-rim-reference').remove())}
    return{positive,negative,box,copyControl,rimMask};
   },report.captures);
   const a=raster(positive),b=raster(negative),mask=raster(rimMask);let rimChanged=0,centerChanged=0,maxDifference=0,maskArea=0;
   const radius=await p.locator('.topbar').evaluate(e=>parseFloat(getComputedStyle(e).borderTopLeftRadius));
   // Rounded-rectangle distance classifies curved corners as rim as well.
   for(let y=Math.ceil(box.y);y<Math.min(a.h,box.y+box.height);y++)for(let x=Math.ceil(box.x);x<Math.min(a.w,box.x+box.width);x++){
    const qx=Math.abs(x+.5-box.x-box.width/2)-(box.width/2-radius),qy=Math.abs(y+.5-box.y-box.height/2)-(box.height/2-radius),distance=Math.hypot(Math.max(qx,0),Math.max(qy,0))+Math.min(Math.max(qx,qy),0)-radius;
    const d=a.pixel(x,y).reduce((sum,v,k)=>sum+Math.abs(v-b.pixel(x,y)[k]),0);maxDifference=Math.max(maxDifference,d);
    const onRim=mask.pixel(x,y).some(v=>v>0);if(onRim)maskArea++;
    if(d>3){if(!onRim)centerChanged++;else rimChanged++;}
   }
   const copyRaster=raster(copyControl);let copyChanged=0,copyMaximum=0,copyOutsideRim=0;const copyOutsideSurface=[];for(let y=0;y<a.h;y++)for(let x=0;x<a.w;x++){const difference=Math.max(...a.pixel(x,y).map((v,k)=>Math.abs(v-copyRaster.pixel(x,y)[k])));if(!difference)continue;if(x<box.x||x>=box.x+box.width||y<box.y||y>=box.y+box.height){copyOutsideSurface.push({x,y,difference});continue;}copyMaximum=Math.max(copyMaximum,difference);copyChanged++;if(!mask.pixel(x,y).some(v=>v>0))copyOutsideRim++;}
   report.spectrum.push({mobile,box,radius,raster:{width:a.w,height:a.h},copyChanged,copyMaximum,copyOutsideRim,copyOutsideSurface,maskArea,rimChanged,centerChanged,maxDifference});if(out){fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-positive.png`),positive);fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-positive-replay.png`),copyControl);fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-negative.png`),negative);fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-2px-reference.png`),rimMask)}if(out)fs.writeFileSync(path.join(out,'adaptive.json'),JSON.stringify(report,null,2));assert.ok(copyMaximum<=1,'positive draw replay stays within the existing 8-bit oracle quantization bound');assert.equal(copyOutsideRim,0,'positive replay changes no foreground/center pixels');assert.ok(maskArea<box.width*box.height*.25,'independent 2 CSS px rim mask leaves a clear center');assert.ok(rimChanged>20,'spectral lip changes actual screenshot pixels');assert.equal(centerChanged,0,'spectral stroke leaves central foreground unchanged');
   for(const background of ['cozy','ambient','none'])for(const colorScheme of ['light','dark']){
    await p.evaluate(v=>{applyTheme('system');applyBackground(v)},background);await p.emulateMedia({colorScheme});await p.waitForFunction(value=>document.documentElement.dataset.theme===value,colorScheme);await settle();
    assert.equal(await p.evaluate(()=>document.documentElement.dataset.theme),colorScheme);
    assert.equal(await p.evaluate(()=>localStorage.getItem('task-mecca-theme')),'system');
   }
   await p.evaluate(()=>{applyTheme('dark');applyBackground('cozy')});await settle();
   // Image decode failure preserves the user's readable theme defaults.
   await p.evaluate(()=>{document.documentElement.style.setProperty('--glass-room-image','url("/fixture-missing.webp")');TaskMeccaGlass.refresh()});await waitForGlass(p,'material');
   const fallback=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone||null,color:getComputedStyle(e).color,mode:TaskMeccaGlass.diagnostics().mode}));report.fallback.push({mobile,...fallback});assert.equal(fallback.tone,null);assert.equal(fallback.mode,'material');assert.equal(fallback.color,'rgb(251, 253, 255)');
   await p.evaluate(()=>applyTheme('light'));await waitForGlass(p,'material');const lightFallback=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone||null,color:getComputedStyle(e).color}));assert.equal(lightFallback.tone,null);assert.equal(lightFallback.color,'rgb(28, 39, 48)');report.fallback.push({mobile,theme:'light',...lightFallback});
   const before=await p.evaluate(()=>TaskMeccaGlass.diagnostics());await p.waitForTimeout(350);const after=await p.evaluate(()=>TaskMeccaGlass.diagnostics());assert.equal(after.adaptCount,before.adaptCount,'idle has no sampling loop');
  }finally{await ctx.close()}
 }
 assert.deepEqual(report.errors,[]);
 }finally{await browser.close();report.cleanup.browserClosed=true;await server.close();report.cleanup.fixtureClosed=true;if(out)fs.writeFileSync(path.join(out,'adaptive.json'),JSON.stringify(report,null,2))}
});

test('Glass ambient and None sample the actual fixed CSS background raster and adapt to position/color',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web')),browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE}),out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR,'ambient');
 if(out)fs.mkdirSync(out,{recursive:true});const report={browser:browser.version(),platform:process.platform,node:process.version,locale:'en-US',origin:server.origin,captures:[],scenes:[],controls:[],errors:[],cleanup:{}};
 try{for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1,locale:'en-US'});
  try{
   await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
   // Test-only observation of the real production sampler readback. The independent
   // reference is the browser's CSS screenshot, never a second Canvas gradient.
   await ctx.addInitScript(()=>{const read=CanvasRenderingContext2D.prototype.getImageData;CanvasRenderingContext2D.prototype.getImageData=function(x,y,w,h,...rest){const pixels=read.call(this,x,y,w,h,...rest);if(!this.canvas.className&&x===0&&y===0&&w===this.canvas.width&&h===this.canvas.height&&w>100&&h>100)window.__fixtureSampler={width:w,height:h,pixels:pixels.data,source:getComputedStyle(document.body,'::before').backgroundImage,backgroundColor:getComputedStyle(document.body,'::before').backgroundColor,theme:document.documentElement.dataset.theme,background:document.documentElement.dataset.background};return pixels};localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');window.EventSource=class{addEventListener(){}close(){}}});
   const p=await ctx.newPage();p.on('pageerror',e=>report.errors.push(e.message));await p.goto(server.origin);await p.waitForTimeout(250);
   await p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},JSON.parse(JSON.stringify(fixture)));
   const checkRaster=async(name)=>{
    await waitForGlass(p);
    const observation=await p.evaluate(()=>{const style=getComputedStyle(document.body,'::before'),s=window.__fixtureSampler;return{computed:style.backgroundImage,mode:TaskMeccaGlass.diagnostics().mode,panels:TaskMeccaGlass.diagnostics().panels,stage:TaskMeccaGlass.diagnostics().stage,sampler:!!s,samplerMatches:s?.source===style.backgroundImage&&s?.backgroundColor===style.backgroundColor&&s?.theme===document.documentElement.dataset.theme&&s?.background===document.documentElement.dataset.background}});
    report.readiness??=[];report.readiness.push({mobile,name,...observation,state:await p.evaluate(()=>window.__glassReadState())});if(out)fs.writeFileSync(path.join(out,'ambient.json'),JSON.stringify(report,null,2));
    assert.equal(observation.mode,'active',`${name}: material fallback cannot stand in for active adaptation`);assert.ok(observation.panels.length>=2);assert.equal(observation.sampler,true);assert.equal(observation.samplerMatches,true,'observed sampler belongs to the actual current CSS generation');
    const hide=await p.addStyleTag({content:'.app-shell{visibility:hidden!important}'}),png=await p.screenshot({animations:'disabled'}),reference=raster(png);await hide.evaluate(e=>e.remove());
    const points=[];for(const fy of [.03,.12,.3,.5,.7,.83,.97])for(const fx of [.03,.16,.35,.5,.7,.88,.97])points.push({x:Math.floor(reference.w*fx),y:Math.floor(reference.h*fy)});
    const sampled=await p.evaluate(points=>{const s=window.__fixtureSampler,g=TaskMeccaGlass.diagnostics().stage;return points.map(({x,y})=>{const sx=Math.min(s.width-1,Math.floor((x+.5)/g.width*s.width)),sy=Math.min(s.height-1,Math.floor((y+.5)/g.height*s.height)),at=(sy*s.width+sx)*4;return[...s.pixels.slice(at,at+3)]})},points);
    const comparisons=points.map((v,i)=>{const css=reference.pixel(v.x,v.y),canvas=sampled[i];return{...v,css,canvas,maximumChannelError:Math.max(...css.map((n,k)=>Math.abs(n-canvas[k]))),luminanceError:Math.abs(lum(css)-lum(canvas))}}),maximumChannelError=Math.max(...comparisons.map(v=>v.maximumChannelError)),meanChannelError=comparisons.reduce((sum,v)=>sum+v.css.reduce((a,n,k)=>a+Math.abs(n-v.canvas[k]),0),0)/(comparisons.length*3),maximumLuminanceError=Math.max(...comparisons.map(v=>v.luminanceError));
    const scene={mobile,name,...observation,comparisons,maximumChannelError,meanChannelError,maximumLuminanceError};report.scenes.push(scene);if(out){fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-${name}-css-background.png`),png);fs.writeFileSync(path.join(out,'ambient.json'),JSON.stringify(report,null,2))}
    assert.ok(maximumChannelError<=3,`${name}: sampler matches independent CSS raster within 3 channel levels (actual ${maximumChannelError})`);assert.ok(meanChannelError<=1,`${name}: mean channel error <= 1`);assert.ok(maximumLuminanceError<=.012,`${name}: sampled relative luminance tracks actual rendered CSS`);
    const texts=await p.locator('.topbar,.content,.preview-identity-banner,.user-attention').evaluateAll(hosts=>hosts.flatMap(e=>{const w=document.createTreeWalker(e,NodeFilter.SHOW_TEXT),a=[];let n;while(n=w.nextNode()){const el=n.parentElement;if(!n.textContent.trim()||!el.checkVisibility())continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect(),s=getComputedStyle(el);let opacity=1;for(let t=el;t;t=t.parentElement)opacity*=Number(getComputedStyle(t).opacity);if(b.x>=0&&b.y>=0&&b.right<innerWidth&&b.bottom<innerHeight)a.push({text:n.textContent.trim(),rect:b.toJSON(),color:s.color,opacity,disabled:!!el.closest('[disabled],[aria-disabled="true"]')})}return a}));
    const hiddenText=await p.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'}),bg=raster(await p.screenshot({animations:'disabled'}));await hiddenText.evaluate(e=>e.remove());
    scene.textSamples=texts.filter(t=>!t.disabled).map(t=>{const c=t.color.match(/[\d.]+/g).map(Number),alpha=(c[3]??1)*t.opacity,ratios=[];for(const dx of [.15,.5,.85]){const b=bg.pixel(t.rect.x+t.rect.width*dx,t.rect.y+t.rect.height*.5);ratios.push(contrast(c.slice(0,3).map((v,k)=>v*alpha+b[k]*(1-alpha)),b))}return{text:t.text,minimumContrast:Math.min(...ratios)}});assert.ok(scene.textSamples.length>5);assert.ok(scene.textSamples.every(t=>t.minimumContrast>=4.5),'active ambient/None rendered text reaches 4.5:1');
    if(out)await p.screenshot({path:path.join(out,`${mobile?'mobile':'desktop'}-${name}-ui.png`),animations:'disabled'});
    return scene;
   };
   for(const theme of ['light','dark'])for(const background of ['ambient','none']){
    await p.evaluate(v=>{applyTheme(v.theme);applyBackground(v.background);TaskMeccaGlass.refresh()},{theme,background});await checkRaster(theme+'-'+background);
    if(background==='ambient'){await p.evaluate(()=>{document.body.style.minHeight='2500px';scrollTo(0,180)});await p.setViewportSize({width:mobile?390:1440,height:mobile?640:700});await checkRaster(theme+'-ambient-fixed-stage');await p.setViewportSize({width:mobile?390:1440,height:mobile?844:960});await p.evaluate(()=>{document.body.style.minHeight='';scrollTo(0,0)});}
   }
   await p.evaluate(()=>{applyTheme('light');applyBackground('ambient')});
   // Same theme and surface; moving only the authored radial position must move
   // both browser pixels and the sampled panel's decision. A centered substitute fails.
   let override;
   for(const at of ['0%','100%']){
    if(override)await override.evaluate(e=>e.remove());override=await p.addStyleTag({content:`:root[data-palette="glass"][data-background="ambient"] body::before{background-image:radial-gradient(ellipse 45% 150% at ${at} 50%,#fff 50%,transparent 65%),linear-gradient(0deg,#080c10,#080c10)!important}`});await p.evaluate(()=>{TaskMeccaGlass.refresh()});const scene=await checkRaster('position-'+at.replace('%',''));const panel=await p.locator('.sidebar').evaluate(e=>({tone:e.dataset.glassTone,color:getComputedStyle(e).color}));report.controls.push({mobile,kind:'position',at,panel,scene:scene.name});assert.equal(panel.tone,at==='0%'?'light':'dark');
   }
   for(const color of ['#f7f8fa','#080c10']){
    await override.evaluate(e=>e.remove());override=await p.addStyleTag({content:`:root[data-palette="glass"][data-background="ambient"] body::before{background-image:linear-gradient(0deg,${color},${color})!important}`});await p.evaluate(()=>{TaskMeccaGlass.refresh()});const scene=await checkRaster('color-'+color.slice(1)),panel=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone,color:getComputedStyle(e).color}));report.controls.push({mobile,kind:'color',color,panel,scene:scene.name});assert.equal(panel.tone,color==='#f7f8fa'?'light':'dark');
   }
   await override.evaluate(e=>e.remove());override=await p.addStyleTag({content:':root[data-palette="glass"][data-background="ambient"] body::before{background-image:conic-gradient(#fff,#000)!important}'});await p.evaluate(()=>TaskMeccaGlass.refresh());await p.waitForTimeout(150);const unsupported=await p.evaluate(()=>({mode:TaskMeccaGlass.diagnostics().mode,panels:TaskMeccaGlass.diagnostics().panels.length}));assert.equal(unsupported.mode,'material');assert.equal(unsupported.panels,0);report.controls.push({mobile,kind:'unsupported-negative',...unsupported});await override.evaluate(e=>e.remove());
   await p.evaluate(()=>TaskMeccaGlass.refresh());await p.waitForTimeout(200);const before=await p.evaluate(()=>TaskMeccaGlass.diagnostics());await p.waitForTimeout(250);const after=await p.evaluate(()=>TaskMeccaGlass.diagnostics());assert.equal(after.adaptCount,before.adaptCount);assert.equal(after.paintCount,before.paintCount);
  }finally{await ctx.close()}
 }
 assert.deepEqual(report.errors,[]);
 }finally{await browser.close();report.cleanup.browserClosed=true;await server.close();report.cleanup.fixtureClosed=true;if(out)fs.writeFileSync(path.join(out,'ambient.json'),JSON.stringify(report,null,2))}
});

test('Glass keeps a real fallback during render and changes reading polarity atomically',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve(process.env.GLASS_TRANSITION_ASSET_ROOT||'goassets/template/_task_mecca/framework/web')),browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE}),out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR,'adaptive');
 const report={browser:browser.version(),node:process.version,platform:process.platform,locale:'en-US',origin:server.origin,frames:[],backingFrames:[],cleanup:{}};if(out)fs.mkdirSync(out,{recursive:true});
 try{for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},deviceScaleFactor:1,locale:'en-US',isMobile:mobile,hasTouch:mobile});
  try{await ctx.addInitScript(installGlassUpdateGate);await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','light');window.EventSource=class{addEventListener(){}close(){}}});const p=await ctx.newPage();await p.goto(server.origin);await p.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().photo);
   await p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},JSON.parse(JSON.stringify(fixture)));
   await p.evaluate(async()=>{const image=new Image();image.src=TaskMeccaGlass.diagnostics().photo;await image.decode();window.__transitionPhotos={};for(const kind of ['bright','dark']){const c=document.createElement('canvas');c.width=800;c.height=600;const x=c.getContext('2d');x.drawImage(image,0,0,800,600);x.fillStyle=kind==='bright'?'rgba(255,255,255,.8)':'rgba(0,0,0,.93)';x.fillRect(0,0,800,600);window.__transitionPhotos[kind]=c.toDataURL()}window.__setTransitionPhoto=kind=>{document.documentElement.style.setProperty('--glass-room-image',`url("${window.__transitionPhotos[kind]}")`);TaskMeccaGlass.refresh()}});
   for(const [initial,target]of [['dark','bright'],['bright','dark']]){
    await p.evaluate(v=>__setTransitionPhoto(v),initial);await p.waitForFunction(kind=>TaskMeccaGlass.diagnostics().photo===__transitionPhotos[kind]&&!!document.querySelector('.content > .glass-reader-material')&&!document.getAnimations().some(a=>a.playState==='running'||a.pending),initial);
    const frames=await p.evaluate(async kind=>{__setTransitionPhoto(kind);const rows=[];for(let frame=0;frame<14;frame++){await new Promise(r=>requestAnimationFrame(r));const owner=document.querySelector('.content'),plane=owner.querySelector(':scope > .glass-reader-material'),d=TaskMeccaGlass.diagnostics();rows.push({frame,kind,sampledMatches:d.sample?.source===__transitionPhotos[kind],generation:d.sample?.generation??null,decodedMatches:d.photo===__transitionPhotos[kind],tone:owner.dataset.glassTone,fill:getComputedStyle(plane||owner,plane?null:'::before').backgroundColor,pseudo:getComputedStyle(owner,'::before').content,plane:!!plane,colors:[...owner.querySelectorAll('.status-filter-btn')].map(e=>({text:e.textContent,color:getComputedStyle(e).color,colorTransitions:e.getAnimations().filter(a=>a.transitionProperty==='color').length}))})}return rows},target);report.frames.push({mobile,initial,target,frames});
   }
   const backing=await p.evaluate(async()=>{const rows=[];for(let i=0;i<3;i++){await new Promise(r=>requestAnimationFrame(r));render();const e=document.querySelector('.content');rows.push({frame:i,plane:!!e.querySelector('.glass-reader-material'),pseudo:getComputedStyle(e,'::before').content})}return rows});report.backingFrames.push({mobile,frames:backing});
   // Delay only the optical update callback, emulating a backing unavailable
   // across a paint. The product's independent CSS fallback must still render.
   await waitForGlass(p);
   const faultStart=await p.evaluate(()=>{TaskMeccaGlass.refresh();const gate=window.__glassUpdateGate,queuedBefore=gate.state().queued;if(!queuedBefore.length)throw Error('Fault setup: no actual production update was queued');gate.activate();render();document.querySelector('.content > .glass-reader-material')?.remove();return{queuedBefore,state:gate.state()}});
   await p.waitForFunction(ids=>ids.every(id=>window.__glassUpdateGate.state().held.includes(id)),faultStart.queuedBefore);
   const owner=await p.locator('.content').evaluate(e=>({plane:!!e.querySelector('.glass-reader-material'),pseudo:getComputedStyle(e,'::before').content,rect:e.getBoundingClientRect().toJSON(),gate:window.__glassUpdateGate.state()}));let hidden,negativeStyle,positive,negative,released,afterRelease;
   try{hidden=await p.addStyleTag({content:'.content>*{visibility:hidden!important}'});positive=await p.screenshot({animations:'disabled'});negativeStyle=await p.addStyleTag({content:'.content::before{content:none!important}'});negative=await p.screenshot({animations:'disabled'});report.faultHeldDuringPixels??=[];report.faultHeldDuringPixels.push({mobile,state:await p.evaluate(()=>window.__glassUpdateGate.state())});}
   finally{if(negativeStyle)await negativeStyle.evaluate(e=>e.remove());if(hidden)await hidden.evaluate(e=>e.remove());released=await p.evaluate(()=>window.__glassUpdateGate.release());await waitForGlass(p);afterRelease=await p.evaluate(()=>({gate:window.__glassUpdateGate.state(),plane:!!document.querySelector('.content > .glass-reader-material'),pending:TaskMeccaGlass.diagnostics().pending,material:!!document.querySelector('.content > .glass-reader-material > .glass-material-optics')?.isConnected}));}
   report.faultSynchronization??=[];report.faultSynchronization.push({mobile,faultStart,released,afterRelease});
   const a=raster(positive),b=raster(negative),y=Math.min(a.h-20,Math.max(180,owner.rect.y+80)),deltas=[.25,.5,.75].map(k=>{const x=owner.rect.x+owner.rect.width*k;return a.pixel(x,y).reduce((sum,v,i)=>sum+Math.abs(v-b.pixel(x,y)[i]),0)/3});report.backingFrames.push({mobile,faultWindow:owner,rasterDelta:deltas});if(out){fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-fallback-present.png`),positive);fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-fallback-negative.png`),negative)}
   // Assert after preserving both polarity and backing evidence, including failures.
   if(out)fs.writeFileSync(path.join(out,'transitions.json'),JSON.stringify(report,null,2));
   assert.ok(report.frames.filter(v=>v.mobile===mobile).every(v=>v.frames.every(f=>f.colors.length&&f.colors.every(c=>c.color===(f.tone==='light'?'rgb(28, 39, 48)':'rgb(251, 253, 255)')&&c.colorTransitions===0))),'foreground changes atomically with its actual reading material');
   assert.ok(backing.every(f=>f.plane||!['none','normal'].includes(f.pseudo)),'every render frame retains a reader or its fallback');assert.ok(owner.gate.active&&faultStart.queuedBefore.every(id=>owner.gate.held.includes(id)),'fault held the actual callback queued before activation');assert.ok(released.length&&released.every(id=>afterRelease.gate.events.some(event=>event.phase==='released-executed'&&event.id===id&&event.plane===true&&event.pending===false)),'release executed each actual held callback');assert.equal(afterRelease.material,true,'reader DOM replacement reconnects its actual continuous material canvas');assert.equal(owner.plane,false);assert.ok(!['none','normal'].includes(owner.pseudo));assert.ok(deltas.every(d=>d>5),'actual fallback pixels differ from a deliberately absent reading surface');
   if(!mobile){
    await ctx.addInitScript(()=>{const supports=CSS.supports.bind(CSS);CSS.supports=(...args)=>args[0]==='selector(:has(*))'?false:supports(...args)});await p.reload();await waitForGlass(p);
    const fallback=await p.locator('.content').evaluate(e=>({supported:TaskMeccaGlass.diagnostics().readerBackingSupported,plane:!!e.querySelector('.glass-reader-material'),pseudo:getComputedStyle(e,'::before').content,filter:getComputedStyle(e,'::before').backdropFilter}));report.selectorFallback=fallback;assert.equal(fallback.supported,false);assert.equal(fallback.plane,false);assert.ok(!['none','normal'].includes(fallback.pseudo));assert.ok(fallback.filter.includes('blur('));
   }
  }finally{await ctx.close()}
 }
 }finally{await browser.close();await server.close();report.cleanup={browserClosed:true,fixtureClosed:true};if(out){fs.writeFileSync(path.join(out,'transitions.json'),JSON.stringify(report,null,2));const target=path.join(out,'adaptive.json'),existing=fs.existsSync(target)?JSON.parse(fs.readFileSync(target,'utf8')):{};existing.transitions=report;fs.writeFileSync(target,JSON.stringify(existing,null,2))}}
});

test('Glass visual equivalence ignores node identity and rejects actual text/color/opacity/geometry changes',{skip:!enabled,timeout:120000},async()=>{
 const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),server=await fixture.start(path.resolve('goassets/template/_task_mecca/framework/web')),browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE}),out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR,'adaptive');
 const report={origin:server.origin,browser:browser.version(),node:process.version,locale:'en-US',negativeControls:[],cleanup:{}};if(out)fs.mkdirSync(out,{recursive:true});
 try{const ctx=await browser.newContext({viewport:{width:1440,height:960},screen:{width:1440,height:960},deviceScaleFactor:1,locale:'en-US'});
  try{await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','light');window.EventSource=class{addEventListener(){}close(){}}});const p=await ctx.newPage();await p.goto(server.origin);await waitForGlass(p);
   await p.evaluate(()=>{const marker=document.createElement('div');marker.id='fixture-equivalence';marker.style.cssText='position:fixed;left:310px;top:280px;width:220px;height:24px;z-index:20000;color:rgb(18,52,86);background:white;opacity:1;font:16px/24px Arial;transition:none';marker.innerHTML='<span>Stable visual probe</span>';document.body.append(marker)});
   const before=await waitForGlass(p),clip=await p.locator('#fixture-equivalence').boundingBox(),positive=await p.screenshot({clip,animations:'disabled'});await p.evaluate(()=>{const span=document.querySelector('#fixture-equivalence span');span.firstChild.replaceWith(document.createTextNode(span.textContent))});const after=await waitForGlass(p),equivalent=await p.screenshot({clip,animations:'disabled'}),a=raster(positive),b=raster(equivalent);let pixelChanges=0;for(let y=0;y<a.h;y++)for(let x=0;x<a.w;x++)if(a.pixel(x,y).some((v,k)=>v!==b.pixel(x,y)[k]))pixelChanges++;
   const beforeText=before.texts.find(v=>v[1]==='Stable visual probe'),afterText=after.texts.find(v=>v[1]==='Stable visual probe');report.positive={before,after,beforeText,afterText,pixelChanges,visuallyEqual:before.fingerprint===after.fingerprint};if(out){fs.writeFileSync(path.join(out,'identity-positive.png'),positive);fs.writeFileSync(path.join(out,'identity-equivalent.png'),equivalent);fs.writeFileSync(path.join(out,'observation-controls.json'),JSON.stringify(report,null,2))}
   assert.notEqual(beforeText[0],afterText[0],'diagnostics retain actual changed node identity');assert.deepEqual(beforeText.slice(1),afterText.slice(1));assert.equal(before.fingerprint,after.fingerprint,'same measured visual state remains equivalent');assert.equal(pixelChanges,0,'actual probe pixels are unchanged by text-node replacement');
   for(const kind of ['text','color','opacity','geometry']){
    const base=await waitForGlass(p);await p.evaluate(kind=>{const e=document.getElementById('fixture-equivalence');if(kind==='text')e.querySelector('span').textContent='Changed visual probe';if(kind==='color')e.style.color='rgb(86,52,18)';if(kind==='opacity')e.style.opacity='.6';if(kind==='geometry')e.style.left='313px'},kind);const changed=await waitForGlass(p),negative=await p.screenshot({clip,animations:'disabled'}),negativeRaster=raster(negative);let changedPixels=0;for(let y=0;y<a.h;y++)for(let x=0;x<a.w;x++)if(a.pixel(x,y).some((v,k)=>v!==negativeRaster.pixel(x,y)[k]))changedPixels++;report.negativeControls.push({kind,before:base,after:changed,rejected:base.fingerprint!==changed.fingerprint,changedPixels});if(out)fs.writeFileSync(path.join(out,`identity-negative-${kind}.png`),negative);if(out)fs.writeFileSync(path.join(out,'observation-controls.json'),JSON.stringify(report,null,2));assert.notEqual(base.fingerprint,changed.fingerprint,`${kind} changes remain visually different`);assert.ok(changedPixels>0,`${kind} control changes actual probe pixels`);
    await p.evaluate(()=>{const e=document.getElementById('fixture-equivalence');e.querySelector('span').textContent='Stable visual probe';e.style.color='rgb(18,52,86)';e.style.opacity='1';e.style.left='310px'});const restored=await waitForGlass(p);assert.equal(base.fingerprint,restored.fingerprint,`${kind} reset restores visual equivalence despite new IDs`);
   }
  }finally{await ctx.close()}
 }finally{await browser.close();await server.close();report.cleanup={browserClosed:true,fixtureClosed:true};if(out){fs.writeFileSync(path.join(out,'observation-controls.json'),JSON.stringify(report,null,2));const target=path.join(out,'adaptive.json'),existing=fs.existsSync(target)?JSON.parse(fs.readFileSync(target,'utf8')):{};existing.visualEquivalenceControls=report;fs.writeFileSync(target,JSON.stringify(existing,null,2))}}
});
