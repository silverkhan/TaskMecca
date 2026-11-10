// Real raster review of all six Glass appearance/background combinations.
// APIs remain synthetic; this never contacts or alters the operational Web.
const fs=require('node:fs'),path=require('node:path'),zlib=require('node:zlib'),assert=require('node:assert/strict'),fixture=require('./glass_fixture.cjs');
const {openGlassPreferences}=require('./glass_state.cjs');
const {installSearchReader,assertSearchFrame,checkSearchControls}=require('./glass_search.cjs');
function rasterStats(png){
 let width,height,channels,offset=8,blocks=[];
 while(offset<png.length){const length=png.readUInt32BE(offset),kind=png.toString('ascii',offset+4,offset+8),data=png.subarray(offset+8,offset+8+length);offset+=length+12;if(kind==='IHDR'){width=data.readUInt32BE(0);height=data.readUInt32BE(4);assert.equal(data[8],8);channels={2:3,6:4}[data[9]];assert.ok(channels)}if(kind==='IDAT')blocks.push(data)}
 const bytes=zlib.inflateSync(Buffer.concat(blocks)),stride=width*channels;let previous=Buffer.alloc(stride),at=0,sum=[0,0,0],squares=[0,0,0],min=[255,255,255],max=[0,0,0],n=0;
 for(let y=0;y<height;y++){
  const filter=bytes[at++],row=Buffer.from(bytes.subarray(at,at+stride));at+=stride;
  for(let x=0;x<stride;x++){const a=x>=channels?row[x-channels]:0,b=previous[x],c=x>=channels?previous[x-channels]:0,p=a+b-c,ds=[Math.abs(p-a),Math.abs(p-b),Math.abs(p-c)];let add=filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?[a,b,c][ds.indexOf(Math.min(...ds))]:0;row[x]=(row[x]+add)&255}
  if(y%24===12)for(let x=12;x<width;x+=24){for(let k=0;k<3;k++){const v=row[x*channels+k];sum[k]+=v;squares[k]+=v*v;min[k]=Math.min(min[k],v);max[k]=Math.max(max[k],v)}n++}previous=row;
 }
 return{mean:sum.map(v=>v/n),stddev:squares.map((v,i)=>Math.sqrt(Math.max(0,v/n-(sum[i]/n)**2))),min,max,span:max.map((v,i)=>v-min[i]),samples:n};
}
async function capture({browser,root,out,verify=true}){
 const server=await fixture.start(root),data=JSON.parse(JSON.stringify(fixture)),report={root,origin:server.origin,browser:browser.version(),scenes:[],textSamples:[],searchControls:[]};if(out)fs.mkdirSync(out,{recursive:true});
 try{for(const mobile of [false,true]){
  const c=await browser.newContext({viewport:mobile?{width:390,height:844}:{width:1550,height:1080},screen:mobile?{width:390,height:844}:{width:1550,height:1080},isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});
  try{await c.route('**/*',r=>r.request().url().startsWith(server.origin+'/')?r.continue():r.abort());await c.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','dark');localStorage.setItem('task-mecca-language','ko');localStorage.setItem('task-mecca-sidebar-mode','expanded');window.EventSource=class{addEventListener(){}close(){}}});const p=await c.newPage();await p.goto(server.origin);await p.waitForTimeout(250);
   const seed=()=>p.evaluate(f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,commonUserAttention:{fixture:{project:f.project,backlog:f.backlog,rows:[{id:f.detail.id,title:'가상 검증 결과 확인',message:'실제 요청이 아닌 fixture 안내입니다.',action:'검증 결과와 근거 확인'}]}},detail:null,detailTask:null,raw:false});render()},data);
   for(const theme of ['dark','light'])for(const background of ['cozy','ambient','none']){
    await p.evaluate(({theme,background})=>{applyTheme(theme);applyBackground(background);scrollTo(0,0)},{theme,background});await seed();await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));await p.mouse.move(1540,1070);
    const name=`${mobile?'mobile':'desktop'}-${theme}-${background}`,scene={mobile,theme,background,name};if(out)await p.screenshot({path:path.join(out,name+'.png'),animations:'disabled'});
    const hidden=await p.addStyleTag({content:'.app-shell{visibility:hidden!important}'});const png=await p.screenshot({animations:'disabled'});await hidden.evaluate(e=>e.remove());scene.raster=rasterStats(png);if(out)fs.writeFileSync(path.join(out,name+'-background-only.png'),png);
    scene.optics=await p.evaluate(()=>window.TaskMeccaGlass?.diagnostics());if(verify){await p.evaluate(installSearchReader);scene.search=await p.evaluate(()=>__glassReadSearch());assertSearchFrame(scene.search);if(theme==='dark'&&background==='cozy')report.searchControls.push({mobile,...await checkSearchControls(p)});assert.equal(await p.locator('.search-wrap .glass-optics,.sidebar .glass-optics,.glass-reader-material .glass-optics').count(),0);assert.ok((scene.optics?.lenses.length||0)<=2);if(background!=='cozy')assert.equal(await p.locator('.glass-optics').count(),0)}
    const samples=await p.evaluate(()=>{const w=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT),a=[];let n;while(n=w.nextNode()){if(!n.textContent.trim()||!n.parentElement||n.parentElement.closest('script,style'))continue;const r=document.createRange();r.selectNodeContents(n);const b=r.getBoundingClientRect(),s=getComputedStyle(n.parentElement);if(b.width>8&&b.height>5&&b.x>=0&&b.right<innerWidth&&b.y>=0&&b.bottom<innerHeight&&n.parentElement.checkVisibility({checkOpacity:true,checkVisibilityCSS:true}))a.push({text:n.textContent.trim().slice(0,60),rect:{x:b.x,y:b.y,width:b.width,height:b.height},color:s.color,fontSize:s.fontSize,fontWeight:s.fontWeight})}return a});
    if(out){const hideText=await p.addStyleTag({content:'*{-webkit-text-fill-color:transparent!important;text-shadow:none!important}'});await p.screenshot({path:path.join(out,name+'-background-samples.png')});await hideText.evaluate(e=>e.remove())}report.textSamples.push({mobile,theme,background,samples});
    if(background==='cozy')await openGlassPreferences(p);if(background==='cozy')for(const menu of ['language','palette']){await p.locator('.'+menu+'-picker .tm-combo-trigger').click();await p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));if(out)await p.screenshot({path:path.join(out,name+'-'+menu+'-menu.png'),animations:'disabled'});await p.keyboard.press('Escape')}
    report.scenes.push(scene);
   }
  }finally{await c.close()}
 }
 for(const mobile of [false,true])for(const theme of ['dark','light']){const ambient=report.scenes.find(s=>s.mobile===mobile&&s.theme===theme&&s.background==='ambient').raster,none=report.scenes.find(s=>s.mobile===mobile&&s.theme===theme&&s.background==='none').raster;if(verify){assert.ok(Math.max(...ambient.span)>24,'Atmospheric has a visible broad light range');assert.ok(Math.max(...none.span)<=1,'None stays flat');assert.ok(ambient.mean.some((v,i)=>Math.hypot(v-none.mean[i],ambient.stddev[i])>8),'Atmospheric differs from None raster')}}
 }finally{if(out)fs.writeFileSync(path.join(out,'background-scenes.json'),JSON.stringify(report,null,2));await server.close()}
 return report;
}
module.exports={capture,rasterStats};
if(require.main===module)(async()=>{const {chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE,headless:true});try{await capture({browser,root:path.resolve(process.env.GLASS_ASSET_ROOT||'goassets/template/_task_mecca/framework/web'),out:path.resolve(process.env.GLASS_EVIDENCE_DIR||'glass-background-scenes'),verify:process.env.GLASS_REVIEW_BASELINE!=='1'})}finally{await browser.close()}})().catch(e=>{console.error(e);process.exitCode=1});
