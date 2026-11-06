// Independent pixels from the real shared material, including zero-bevel and
// zero-displacement controls. Fixture routing changes only production constants.
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const fixture=require('./helpers/glass_fixture.cjs'),{waitForGlass}=require('./helpers/glass_state.cjs'),{raster}=require('./helpers/glass_raster.cjs');
const enabled=!!process.env.GLASS_PLAYWRIGHT_MODULE;
test('Reference Glass curves every shared shell within bounded existing canvases without changing center or foreground',{skip:!enabled,timeout:120000},async()=>{
 const root=path.resolve('goassets/template/_task_mecca/framework/web'),source=fs.readFileSync(path.join(root,'glass-optics.js'),'utf8'),server=await fixture.start(root),{chromium}=require(process.env.GLASS_PLAYWRIGHT_MODULE),browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE});
 const out=process.env.GLASS_EVIDENCE_DIR&&path.resolve(process.env.GLASS_EVIDENCE_DIR,'material'),report={browser:browser.version(),source:'production constants changed only in negative fixtures',cases:[],cleanup:{}};if(out)fs.mkdirSync(out,{recursive:true});
 try{for(const mobile of [false,true]){
  const size=mobile?{width:390,height:844}:{width:1448,height:1090},ctx=await browser.newContext({viewport:size,screen:size,isMobile:mobile,hasTouch:mobile,deviceScaleFactor:1});let variant='positive';
  try{await ctx.route('**/*',r=>{const u=new URL(r.request().url());if(u.origin!==server.origin)return r.abort();if(u.pathname==='/glass-optics.js')return r.fulfill({contentType:'text/javascript',body:source.replace('MATERIAL_BEVEL=10',`MATERIAL_BEVEL=${variant==='absent'?0:10}`).replace('MATERIAL_BEND=12',`MATERIAL_BEND=${variant==='flat'?0:12}`)});if(u.pathname==='/api/environment')return r.fulfill({json:{target:{kind:'pr',pr_number:348,head_sha:'0000000000000000000000000000000000000348'},switch_supported:true,os:'fixture'}});if(u.pathname==='/api/health')return r.fulfill({json:{ok:true,preview_mode:false}});return r.continue()});
   await ctx.addInitScript(()=>{localStorage.setItem('task-mecca-palette','glass');localStorage.setItem('task-mecca-theme','light');localStorage.setItem('task-mecca-sidebar-mode','expanded');window.EventSource=class{addEventListener(){}close(){}}});const p=await ctx.newPage(),captures={};
   for(variant of ['positive','absent','flat']){
    await p.goto(server.origin);await p.evaluate(async f=>{Object.assign(state,{project:f.project,backlog:f.backlog,view:'backlog',listData:f.list,snapshot:f.snapshot,hub:f.hub,detail:null,executionTarget:{kind:'pr',pr_number:348,head_sha:'0000000000000000000000000000000000000348'},commonUserAttention:{fixture:{project:f.project,backlog:f.backlog,rows:[{id:f.detail.id,title:'가상 검증 결과 확인',message:'fixture',action:'검증'}]}}});render();await refreshExecutionTarget()},JSON.parse(JSON.stringify(fixture)));
    await waitForGlass(p);
    captures[variant]=await p.evaluate(()=>({diagnostics:TaskMeccaGlass.diagnostics(),foreground:[...document.querySelectorAll('.task-title,.theme-btn,.preview-identity-title')].map(e=>({text:e.textContent,rect:e.getBoundingClientRect().toJSON(),color:getComputedStyle(e).color})),canvases:[...document.querySelectorAll('.glass-material-optics')].map(c=>({host:c.closest('[data-glass-material]').className,width:c.width,height:c.height,rect:c.getBoundingClientRect().toJSON(),radius:Math.min(parseFloat(getComputedStyle(c).borderTopLeftRadius)||28,c.clientWidth/2,c.clientHeight/2),png:c.toDataURL()}))}));
   }
   const positive=captures.positive;assert.ok(positive.canvases.length>=4);assert.ok(positive.diagnostics.lenses.length<=2);assert.ok(positive.diagnostics.lenses.every(l=>l.pixels<=350000));assert.ok(positive.canvases.every(c=>c.width*c.height<=480000));assert.ok(positive.canvases.some(c=>c.host.includes('preview-identity-banner')),'actual PR shell has material');assert.ok(positive.canvases.some(c=>c.host.includes('user-attention')),'actual attention shell has material');assert.ok(positive.canvases.some(c=>c.host==='content'),'reader has material');assert.ok(positive.canvases.some(c=>c.host==='topbar'),'topbar has material');assert.ok(positive.canvases.some(c=>c.host.includes(mobile?'sidebar-head':'sidebar')),'visible navigation shell has material');
   for(const negative of ['absent','flat']){
    assert.deepEqual(captures[negative].foreground,positive.foreground,`${negative}: foreground geometry/color/text unchanged`);assert.equal(captures[negative].canvases.length,positive.canvases.length);
    for(let i=0;i<positive.canvases.length;i++){
     const a=positive.canvases[i],b=captures[negative].canvases[i];assert.equal(a.host,b.host);assert.deepEqual([a.width,a.height,a.rect],[b.width,b.height,b.rect]);const pa=raster(Buffer.from(a.png.split(',')[1],'base64')),pb=raster(Buffer.from(b.png.split(',')[1],'base64')),scale=a.width/a.rect.width,band=Math.min(10,a.rect.width/4,a.rect.height/4);let edgeChanged=0,centerChanged=0,edgeMaximum=0,centerMaximum=0;
     for(let y=0;y<pa.h;y++)for(let x=0;x<pa.w;x++){
      const xx=(x+.5)/scale,yy=(y+.5)/(a.height/a.rect.height),qx=Math.abs(xx-a.rect.width/2)-(a.rect.width/2-a.radius),qy=Math.abs(yy-a.rect.height/2)-(a.rect.height/2-a.radius),distance=Math.hypot(Math.max(qx,0),Math.max(qy,0))+Math.min(Math.max(qx,qy),0)-a.radius;
      const delta=Math.max(...pa.pixel(x,y).map((v,k)=>Math.abs(v-pb.pixel(x,y)[k])));if(distance<-band-3){centerMaximum=Math.max(centerMaximum,delta);if(delta>1)centerChanged++}else if(distance<=0){edgeMaximum=Math.max(edgeMaximum,delta);if(delta>2)edgeChanged++}
     }
     report.cases.push({mobile,negative,host:a.host,width:a.width,height:a.height,band,radius:a.radius,edgeChanged,centerChanged,edgeMaximum,centerMaximum});if(out)fs.writeFileSync(path.join(out,'witness.json'),JSON.stringify(report,null,2));assert.equal(centerChanged,0,`${a.host}/${negative}: curved profile leaves center unchanged`);assert.ok(edgeChanged>20,`${a.host}/${negative}: actual perimeter pixels witness the optical effect`);
     if(out)for(const [label,c]of [['positive',a],[negative,b]])fs.writeFileSync(path.join(out,`${mobile?'mobile':'desktop'}-${i}-${label}.png`),Buffer.from(c.png.split(',')[1],'base64'));
    }
   }
  }finally{await ctx.close()}
 }}finally{await browser.close();await server.close();report.cleanup={browserClosed:true,fixtureClosed:true};if(out)fs.writeFileSync(path.join(out,'witness.json'),JSON.stringify(report,null,2))}
});
