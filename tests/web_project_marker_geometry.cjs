// Offline-only visual regression: shipped nav DOM + shipped stylesheet, no app startup/server.
// PUPPETEER_MODULE and BROWSER_EXECUTABLE may point to existing local installations.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { pathToFileURL } = require('node:url');
const { page: appFixture } = require('./helpers/project_shortcut_fixture.cjs');
const puppeteer = require(process.env.PUPPETEER_MODULE || 'puppeteer');
const root = path.resolve(__dirname, '..');
const web = path.join(root, 'goassets/template/_task_mecca/framework/web');
const out = path.resolve(process.argv[2] || path.join(root, 'handoff-evidence/a68-isanghaessi/geometry'));
const css = fs.readFileSync(path.join(web, 'style.css'), 'utf8');
const shell = fs.readFileSync(path.join(web, 'index.html'), 'utf8')
  .replace(/<script\b[^>]*>[\s\S]*?<\/script>/g, '')
  .replace(/<link\b[^>]*>/g, '')
  .replace(/<img\b[^>]*>/g, '')
  .replace('</head>', `<style>${css}</style></head>`);
const layouts = [
  ['expanded',1280,''], ['compact',1280,'sidebar-collapsed'],
  ['hover',1280,'sidebar-collapsed sidebar-peek'], ['narrow',720,''],
  ['mobile',390,'sidebar-collapsed mobile-nav-open'], ['mobile-small',320,'mobile-nav-open'],
];
(async () => {
  fs.mkdirSync(out, {recursive:true});
  const browser = await puppeteer.launch({headless:true, executablePath:process.env.BROWSER_EXECUTABLE || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    args:['--disable-background-networking','--disable-component-update','--no-first-run']});
  const results = [];
  const requests = [];
  try {
    const page = await browser.newPage();
    await page.setRequestInterception(true);
    page.on('request', request => {
      requests.push(request.url());
      if (request.url().startsWith('file:') || request.url().startsWith('data:')) request.continue();
      else request.abort();
    });
    for (const palette of ['mecca','slate']) for (const theme of ['light','dark']) for (const language of ['ko','en']) for (const [layout,width,classes] of layouts) {
      const id = `${palette}-${theme}-${language}-${layout}`;
      const app = appFixture(10);
      app.state.language = language;
      app.state.hub.projects.forEach((p,i) => {
        p.name = language==='ko' ? `한글 프로젝트 ${i+1} 긴 이름과 상태 확인` : `Project ${i+1} long name and status`;
        p.counts = i%2 ? {hold:1,ready:1} : {blocked:1,working:1};
      });
      app.nav();
      app.translateChrome();
      const html = shell.replace('<html lang="ko">', `<html lang="${language}" data-theme="${theme}" data-palette="${palette}">`)
        .replace('class="app-shell"', `class="app-shell ${classes}"`)
        .replace('<nav id="stateNav" class="nav"></nav>', `<nav id="stateNav" class="nav">${app.menu()}</nav>`)
        .replace(/<div class="shortcut-bar" id="shortcutBar">[\s\S]*?<\/div>/, `<div class="shortcut-bar" id="shortcutBar">${app.html('#shortcutBar')}</div>`);
      const file = path.join(out, `${id}.html`);
      fs.writeFileSync(file, html);
      await page.setViewport({width,height:1000,deviceScaleFactor:1});
      await page.goto(pathToFileURL(file).href, {waitUntil:'load'});
      const rows = await page.evaluate(async () => {
        const rect = r => ({x:r.x,y:r.y,width:r.width,height:r.height,right:r.right,bottom:r.bottom});
        const entries = [];
        for (const row of document.querySelectorAll('.session-row')) {
          row.scrollIntoView({block:'nearest'});
          const marker = row.querySelector('.session-marker');
          const initial = row.querySelector('.session-initial');
          const number = row.querySelector('.session-number');
          const button = row.querySelector('.session-open');
          const statusStyle = getComputedStyle(initial,'::after');
          let cb = initial;
          while (getComputedStyle(cb).position==='static') cb=cb.parentElement;
          const cbr=cb.getBoundingClientRect(), cbs=getComputedStyle(cb);
          const w=parseFloat(statusStyle.width), h=parseFloat(statusStyle.height);
          // Absolutely positioned pseudo-elements use the containing padding box.
          const x=cbr.x+parseFloat(cbs.borderLeftWidth)+cb.clientWidth-parseFloat(statusStyle.right)-w;
          const y=cbr.y+parseFloat(cbs.borderTopWidth)+cb.clientHeight-parseFloat(statusStyle.bottom)-h;
          const status={x,y,width:w,height:h,right:x+w,bottom:y+h};
          function glyph(node) {const range=document.createRange();range.selectNodeContents(node);return rect(range.getBoundingClientRect());}
          const canvas=document.createElement('canvas'),ctx=canvas.getContext('2d');
          ctx.font=`${statusStyle.fontWeight} ${statusStyle.fontSize} ${statusStyle.fontFamily}`;
          const metrics=ctx.measureText('!');
          const clipping=[];
          for(let node=marker.parentElement;node;node=node.parentElement){
            const s=getComputedStyle(node);
            if (/(hidden|clip|auto|scroll)/.test(s.overflowX+' '+s.overflowY))clipping.push({rect:rect(node.getBoundingClientRect()),overflowX:s.overflowX,overflowY:s.overflowY});
          }
          entries.push({project:row.dataset.sessionProject, classes:row.className, marker:rect(marker.getBoundingClientRect()),initial:rect(initial.getBoundingClientRect()),initialGlyph:glyph(initial),number:number?rect(number.getBoundingClientRect()):null,numberGlyph:number?glyph(number):null,numberText:number?.textContent,button:rect(button.getBoundingClientRect()),status,statusContent:statusStyle.content,statusContainingBlock:cb.className,statusColor:statusStyle.color,statusBackground:statusStyle.backgroundColor,statusGlyph:{width:metrics.width,height:metrics.actualBoundingBoxAscent+metrics.actualBoundingBoxDescent},clipping});
        }
        return entries;
      });
      const overlaps=(a,b)=>a.x<b.right-.1&&a.right>b.x+.1&&a.y<b.bottom-.1&&a.bottom>b.y+.1;
      const inside=(a,b)=>a.x>=b.x-.1&&a.y>=b.y-.1&&a.right<=b.right+.1&&a.bottom<=b.bottom+.1;
      assert.equal(rows.length,10,id);
      for(const [i,row] of rows.entries()) {
        assert.equal(row.statusContent,'"!"',id);
        assert.equal(row.statusContainingBlock,'session-marker',id);
        assert.ok(!overlaps(row.initial,row.status),`${id}: initial/status`);
        assert.ok(inside(row.initialGlyph,row.initial),`${id}: initial glyph clipping`);
        assert.ok(inside(row.initial,row.marker)&&inside(row.status,row.marker),`${id}: marker sizing`);
        assert.ok(inside(row.marker,row.button),`${id}: button clipping`);
        assert.ok(row.statusGlyph.width<=12 && row.statusGlyph.height<=12,`${id}: status glyph clipping`);
        for(const clip of row.clipping)assert.ok(inside(row.marker,clip.rect),`${id}: scroll/ancestor clipping`);
        if(i<9){
          assert.equal(row.numberText,String(i+1));
          assert.ok(!overlaps(row.number,row.status)&&!overlaps(row.number,row.initial),`${id}: numeric overlap`);
          assert.ok(inside(row.number,row.marker)&&inside(row.numberGlyph,row.number),`${id}: number clipping`);
          assert.ok(row.status.x-row.number.right>=4-.1,`${id}: status/number gap`);
        }else assert.equal(row.number,null,`${id}: tenth project must have no number`);
      }
      await page.evaluate(()=>document.querySelector('.session-row').scrollIntoView({block:'nearest'}));
      const shortcutHelp=await page.evaluate(()=>[...document.querySelectorAll('#shortcutBar > span')].filter(e=>getComputedStyle(e).display!=='none').map(e=>({text:e.textContent,x:e.getBoundingClientRect().x,right:e.getBoundingClientRect().right})));
      for(const item of shortcutHelp)assert.ok(item.x>=-.1&&item.right<=width+.1,`${id}: shortcut help overflow`);
      const screenshot=path.join(out,`${id}.png`);
      // Full viewport capture preserves media-query dimensions in Chromium.
      await page.screenshot({path:screenshot,captureBeyondViewport:false});
      if(layout==='mobile-small') {
        await page.evaluate(()=>document.querySelectorAll('.session-row')[9].scrollIntoView({block:'nearest'}));
        await page.screenshot({path:path.join(out,`${id}-tail.png`),captureBeyondViewport:false});
      }
      results.push({id,width,theme,palette,language,layout,screenshot,shortcutHelp,rows});
    }
    assert.equal(requests.filter(url=>!url.startsWith('file:')&&!url.startsWith('data:')).length,0,'No network requests permitted');
    fs.writeFileSync(path.join(out,'geometry.json'),JSON.stringify({states:results.length,requests,results},null,2));
    console.log(`PASS ${results.length} offline states, ${results.length*10} rows: initial/number/pseudo-status separation, glyph bounds, clipping, first-nine mapping; network requests=0`);
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
