// Actual app assets on a disposable dynamic port; raster evidence stays local.
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),zlib=require('node:zlib');
const fixture=require('./helpers/glass_fixture.cjs');
const {installOpticalOracle}=require('./helpers/glass_optical_oracle.cjs');
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
 if(out)fs.mkdirSync(out,{recursive:true});const report={origin:server.origin,scenes:[],spectrum:[],focus:[],stability:[],fallback:[],errors:[],cleanup:{}};
 try{for(const mobile of [false,true]){
  const ctx=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1440,height:960},screen:mobile?{width:390,height:844}:{width:1440,height:960},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});
  try{
   await ctx.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());
   await ctx.addInitScript(installOpticalOracle);
   await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');window.EventSource=class{addEventListener(){}close(){}}});
   const p=await ctx.newPage();p.on('pageerror',e=>report.errors.push(e.message));await p.goto(server.origin);await p.waitForFunction(()=>window.TaskMeccaGlass?.diagnostics().photo);
   await p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null});render()},JSON.parse(JSON.stringify(fixture)));
   const settle=()=>p.waitForTimeout(180);
   // The same shipped photograph at controlled exposures, selected through the
   // real CSS photo input. No preset/theme-to-tone lookup can pass this test.
   await p.evaluate(async()=>{const img=new Image();img.src=getComputedStyle(document.documentElement).getPropertyValue('--glass-room-image').match(/url\(["']?([^"')]+)/)[1];await img.decode();window.fixtureExposure=kind=>{const c=document.createElement('canvas');c.width=800;c.height=600;const x=c.getContext('2d');x.drawImage(img,0,0,c.width,c.height);if(kind==='mixed'){x.fillStyle='rgba(255,255,255,.8)';x.fillRect(0,0,400,600);x.fillStyle='rgba(0,0,0,.93)';x.fillRect(400,0,400,600)}else{x.fillStyle=kind==='bright'?'rgba(255,255,255,.8)':kind==='dark'?'rgba(0,0,0,.93)':`rgba(160,160,160,.98)`;x.fillRect(0,0,800,600)}document.documentElement.style.setProperty('--glass-room-image',`url("${c.toDataURL()}")`);TaskMeccaGlass.refresh()}});
   for(const theme of ['light','dark'])for(const kind of ['bright','dark','mixed']){
    await p.evaluate(({theme,kind})=>{applyTheme(theme);fixtureExposure(kind)},{theme,kind});await settle();
    const text=await p.locator('.topbar,.content,.preview-identity-banner,.user-attention').evaluateAll(hosts=>hosts.flatMap(host=>{const walk=document.createTreeWalker(host,NodeFilter.SHOW_TEXT),a=[];let n;while(n=walk.nextNode()){const e=n.parentElement;if(!n.textContent.trim()||e.closest('script,style')||!e.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect(),s=getComputedStyle(e);let opacity=1;for(let ancestor=e;ancestor;ancestor=ancestor.parentElement)opacity*=Number(getComputedStyle(ancestor).opacity);if(b.x>2&&b.y>2&&b.right<innerWidth-2&&b.bottom<innerHeight-2&&b.width>5)a.push({opacity,disabled:!!e.closest('[disabled],[aria-disabled="true"]'),text:n.textContent.trim().slice(0,40),rect:b.toJSON(),color:s.color,font:parseFloat(s.fontSize),weight:parseFloat(s.fontWeight)});}return a}));
    assert.ok(text.length>8,'actual app text must be measured');const normal=await p.screenshot({animations:'disabled'}),hide=await p.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'}),background=raster(await p.screenshot({animations:'disabled'}));await hide.evaluate(e=>e.remove());
    const samples=text.map(t=>{const rgba=t.color.match(/[\d.]+/g).map(Number),color=rgba.slice(0,3),alpha=(rgba[3]??1)*t.opacity,r=t.rect,ratios=[];for(const dx of [.15,.5,.85])for(const dy of [.2,.5,.8]){const bg=background.pixel(r.x+r.width*dx,r.y+r.height*dy);ratios.push(contrast(color.map((v,k)=>v*alpha+bg[k]*(1-alpha)),bg));}return{...t,minimum:Math.min(...ratios),required:t.font>=24||(t.font>=18.66&&t.weight>=700)?3:4.5}});
    const bad=samples.filter(s=>!s.disabled&&s.minimum<s.required);report.scenes.push({mobile,theme,kind,samples,panels:await p.evaluate(()=>TaskMeccaGlass.diagnostics().panels)});if(out)fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-${theme}-${kind}.png`),normal);
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
   const positive=await p.screenshot({animations:'disabled'}),box=await p.locator('.topbar .glass-optics').boundingBox();
   await p.locator('.topbar .glass-optics').evaluate(e=>__glassSpectralControl(e));
   const negative=await p.screenshot({animations:'disabled'}),a=raster(positive),b=raster(negative);let rimChanged=0,centerChanged=0,maxDifference=0;
   const radius=await p.locator('.topbar').evaluate(e=>parseFloat(getComputedStyle(e).borderTopLeftRadius));
   // Rounded-rectangle distance classifies curved corners as rim as well.
   for(let y=Math.ceil(box.y);y<box.y+box.height;y++)for(let x=Math.ceil(box.x);x<box.x+box.width;x++){
    const qx=Math.abs(x+.5-box.x-box.width/2)-(box.width/2-radius),qy=Math.abs(y+.5-box.y-box.height/2)-(box.height/2-radius),distance=Math.hypot(Math.max(qx,0),Math.max(qy,0))+Math.min(Math.max(qx,qy),0)-radius;
    const d=a.pixel(x,y).reduce((sum,v,k)=>sum+Math.abs(v-b.pixel(x,y)[k]),0);maxDifference=Math.max(maxDifference,d);
    if(d>3){if(distance < -2)centerChanged++;else rimChanged++;}
   }
   report.spectrum.push({mobile,rimChanged,centerChanged,maxDifference});if(out){fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-positive.png`),positive);fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-spectral-negative.png`),negative)}assert.ok(rimChanged>20,'spectral lip changes actual screenshot pixels');assert.equal(centerChanged,0,'spectral stroke leaves central foreground unchanged');
   await p.locator('.topbar .glass-optics').evaluate(e=>__glassSpectralControl(e,true));
   for(const background of ['cozy','ambient','none'])for(const colorScheme of ['light','dark']){
    await p.evaluate(v=>{applyTheme('system');applyBackground(v)},background);await p.emulateMedia({colorScheme});await settle();
    assert.equal(await p.evaluate(()=>document.documentElement.dataset.theme),colorScheme);
    assert.equal(await p.evaluate(()=>localStorage.getItem('task-mecca-theme')),'system');
   }
   await p.evaluate(()=>{applyTheme('dark');applyBackground('cozy')});await settle();
   // Image decode failure preserves the user's readable theme defaults.
   await p.evaluate(()=>{document.documentElement.style.setProperty('--glass-room-image','url("/fixture-missing.webp")');TaskMeccaGlass.refresh()});await settle();
   const fallback=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone||null,color:getComputedStyle(e).color,mode:TaskMeccaGlass.diagnostics().mode}));report.fallback.push({mobile,...fallback});assert.equal(fallback.tone,null);assert.equal(fallback.mode,'material');assert.equal(fallback.color,'rgb(251, 253, 255)');
   await p.evaluate(()=>applyTheme('light'));await settle();const lightFallback=await p.locator('.topbar').evaluate(e=>({tone:e.dataset.glassTone||null,color:getComputedStyle(e).color}));assert.equal(lightFallback.tone,null);assert.equal(lightFallback.color,'rgb(28, 39, 48)');report.fallback.push({mobile,theme:'light',...lightFallback});
   const before=await p.evaluate(()=>TaskMeccaGlass.diagnostics());await p.waitForTimeout(350);const after=await p.evaluate(()=>TaskMeccaGlass.diagnostics());assert.equal(after.adaptCount,before.adaptCount,'idle has no sampling loop');
  }finally{await ctx.close()}
 }
 assert.deepEqual(report.errors,[]);
 }finally{await browser.close();report.cleanup.browserClosed=true;await server.close();report.cleanup.fixtureClosed=true;if(out)fs.writeFileSync(path.join(out,'adaptive.json'),JSON.stringify(report,null,2))}
});
