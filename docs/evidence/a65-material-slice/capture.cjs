// Actual Go review route; no API replacement. Each round gets a new immutable directory.
const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict'), crypto = require('node:crypto');
const { chromium } = require(process.env.WORKFLOW_PLAYWRIGHT_MODULE || '/tmp/taskmecca-perceptual-runtime/node_modules/playwright');
const url = process.argv[2] || 'http://127.0.0.1:57069/ui/material-review.html';
const out = path.resolve(process.argv[3] || 'docs/evidence/a65-material-slice/round1');
if (fs.existsSync(out) && fs.readdirSync(out).length) throw Error('Refusing to overwrite evidence: ' + out);
fs.mkdirSync(out, { recursive: true });
const report = { url, synthetic: true, response_routing: 'none; actual Go HTTP', source_files: {}, cases: [], errors: [], requests: [] };
for (const name of ['material-review.html','material/components.js','material/material.css','material/proximity.js','material/review.js']) {
  const p = path.resolve('goassets/template/_task_mecca/framework/web/ui', name);
  report.source_files[name] = crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
}
const sleep = ms => new Promise(r => setTimeout(r, ms));
const snapshot = page => page.evaluate(() => ({ width: innerWidth, scrollWidth: document.documentElement.scrollWidth, theme: document.documentElement.dataset.materialTheme, focus: document.activeElement?.outerHTML, inspect: window.taskMeccaMaterialReview.inspect(), surfaces: [...document.querySelectorAll('[data-light-surface]')].map(el => {const r=el.getBoundingClientRect(),s=getComputedStyle(el);return {name:el.id||el.className,x:r.x,y:r.y,width:r.width,height:r.height,radius:s.borderTopLeftRadius,background:s.backgroundImage,shadow:s.boxShadow,strength:el.style.getPropertyValue('--light-strength'),xLight:el.style.getPropertyValue('--light-x'),yLight:el.style.getPropertyValue('--light-y')};}) }));
(async () => {
  const browser = await chromium.launch({headless:true}); report.browser = browser.version();
  try {
    for (const [name,width,height,touch] of [['desktop',1440,1000,false],['mobile',390,844,true]]) for (const theme of ['dark','light']) {
      const context = await browser.newContext({viewport:{width,height},deviceScaleFactor:1,hasTouch:touch,isMobile:touch,colorScheme:theme});
      const page = await context.newPage(); const item = {name,theme,errors:[],checks:{},states:{}}; report.cases.push(item);
      page.on('pageerror',e=>item.errors.push(e.message)); page.on('request',r=>{if(r.url().includes('/api/'))report.requests.push({url:r.url(),method:r.method()});});
      try {
        await page.goto(url+'?theme='+theme+'&rail=expanded'); await page.locator('#material-task-card').waitFor(); await page.locator('#material-card-slot[aria-busy="false"]').waitFor(); await page.evaluate(()=>document.fonts.ready);
        await page.mouse.move(width-2,height-2); await sleep(400);
        assert.equal(await page.locator('.material-project-row').count(),3); item.checks.three_actual_projects = true;
        assert.equal(await page.locator('[data-destination="hub"]').getAttribute('aria-current'),'page');
        assert.match(await page.locator('#material-card-title').textContent(),/SYN-1/);
        item.states.default = await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-default.png`),fullPage:true});
        assert.equal(item.states.default.scrollWidth<=width,true,'document overflow');
        if (!touch) {
          assert.deepEqual(await page.locator('.material-nav-group h2').allTextContents(),['SYSTEM','BACKLOGS','OPERATIONS','HELP']);
          const box=await page.locator('#material-task-card').boundingBox();
          await page.mouse.move(box.x+24,box.y+24); await sleep(350); item.states.near=await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-near.png`),fullPage:true});
          assert.equal(item.states.near.surfaces.find(s=>s.name==='material-task-card').strength,'1.0000');
          await page.mouse.move(box.x+box.width+24,box.y+24); await sleep(350); item.states.outside24=await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-outside24.png`),fullPage:true});
          const strength=Number(item.states.outside24.surfaces.find(s=>s.name==='material-task-card').strength); assert.ok(strength>0&&strength<1); item.checks.distance_attenuates=true;
          await page.mouse.move(width-2,height-2); await sleep(450); item.states.leave=await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-leave.png`),fullPage:true});
          assert.equal(Number(item.states.leave.surfaces.find(s=>s.name==='material-task-card').strength),0);
          const before=await page.evaluate(()=>window.taskMeccaMaterialReview.inspect()); await sleep(900); const after=await page.evaluate(()=>window.taskMeccaMaterialReview.inspect()); assert.equal(after.frames,before.frames);assert.equal(after.writes,before.writes);assert.equal(after.pending,false);item.checks.idle_frames_and_writes_zero=true;
        } else {
          await page.touchscreen.tap(width-35,42); await page.locator('#material-rail-panel:not([hidden])').waitFor(); item.states.menu=await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-menu.png`),fullPage:true});
          assert.deepEqual(await page.locator('.material-nav-group h2').allTextContents(),['SYSTEM','BACKLOGS','OPERATIONS','HELP']);
          await page.keyboard.press('Escape'); assert.equal(await page.locator('.material-mobile-toggle').getAttribute('aria-expanded'),'false');
          assert.equal(await page.locator('.material-mobile-toggle').evaluate(el=>el===document.activeElement),true); item.checks.mobile_escape_focus_return=true;
          const inspect=await page.evaluate(()=>window.taskMeccaMaterialReview.inspect());assert.equal(inspect.enabled,false);assert.equal(inspect.frames,0);item.checks.touch_static=true;
        }
        await page.locator('.material-select-card').click(); assert.equal(await page.locator('.material-select-card').getAttribute('aria-pressed'),'true');
        await page.mouse.move(width-2,height-2); await sleep(350);item.states.selected=await snapshot(page); await page.screenshot({path:path.join(out,`${name}-${theme}-selected.png`),fullPage:true});
        await page.locator('.material-select-card').focus(); await page.keyboard.press('Tab'); await page.keyboard.press('Shift+Tab');
        item.states.keyboard=await snapshot(page); item.states.focusOutline=await page.locator('.material-select-card').evaluate(el=>({style:getComputedStyle(el).outlineStyle,width:getComputedStyle(el).outlineWidth}));
        assert.equal(item.states.focusOutline.style,'solid'); await page.screenshot({path:path.join(out,`${name}-${theme}-keyboard.png`),fullPage:true});item.checks.keyboard_focus_visible=true;
        await page.locator('#material-show-source').click(); await page.waitForFunction(()=>document.querySelector('#material-source').textContent.includes('##'));assert.match(await page.locator('#material-source').textContent(),/합성/);item.checks.actual_source_read=true;
        await page.locator('#material-show-source').click();
        if (touch) await page.locator('.material-mobile-toggle').click();
        const second=page.locator('.material-project-open').nth(1), projectName=await second.locator('.material-nav-label').textContent(); await second.click(); await page.waitForFunction(expected=>document.querySelector('#material-view-title').textContent===expected,projectName);await page.locator('#material-card-slot[aria-busy="false"]').waitFor(); item.checks.direct_project_context=true;
        if (touch) await page.locator('.material-mobile-toggle').click();
        await page.locator('.material-project-row.is-current .material-project-close').click();assert.equal(await page.locator('.material-project-row').count(),2);
        await page.locator('#material-open-project').click();await page.locator('[data-reopen-project]').click();await page.locator('#material-card-slot[aria-busy="false"]').waitFor();assert.equal(await page.locator('.material-project-row').count(),3);item.checks.local_close_reopen=true;
        const href=page.url();await page.locator('[data-destination="workload"]').focus();await page.keyboard.press('Enter');assert.match(await page.locator('#material-feedback').textContent(),/아직 연결/);assert.equal(page.url(),href);item.checks.unconnected_labeled_no_navigation=true;
        if (!touch) {
          await page.locator('[data-rail-mode="compact"]').click();await page.locator('#material-view-title').click();await sleep(300);assert.equal(Math.round((await page.locator('#material-rail').boundingBox()).width),76);
          await page.locator('[data-rail-mode="auto"]').click();await page.locator('#material-view-title').click();await page.mouse.move(width-2,height-2);await sleep(300);assert.equal(Math.round((await page.locator('#material-rail').boundingBox()).width),76);
          await page.locator('#material-rail').hover();await sleep(300);assert.equal(Math.round((await page.locator('#material-rail').boundingBox()).width),252);item.checks.rail_modes=true;
        }
        await page.locator('[data-material-mode="system"]').click(); await page.emulateMedia({colorScheme:'light'}); assert.equal(await page.locator('html').getAttribute('data-material-theme'),'light');await page.emulateMedia({colorScheme:'dark'});assert.equal(await page.locator('html').getAttribute('data-material-theme'),'dark');item.checks.system_follows_preference=true;
        assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-smoke-mode')),null);assert.equal(await page.evaluate(()=>localStorage.getItem('task-mecca-theme')),null);item.checks.original_preferences_untouched=true;
        assert.deepEqual(item.errors,[]);item.passed=true;
      } catch(error) { item.failure=error.stack; item.passed=false; }
      await context.close();
    }
    const reduced=await browser.newContext({viewport:{width:1440,height:1000},reducedMotion:'reduce'});const p=await reduced.newPage();await p.goto(url+'?theme=dark');await p.locator('#material-task-card').waitFor();const b=await p.locator('#material-task-card').boundingBox();await p.mouse.move(b.x+24,b.y+24);await sleep(400);report.reduced_motion=await snapshot(p);assert.equal(report.reduced_motion.inspect.enabled,false);assert.equal(report.reduced_motion.inspect.frames,0);await p.screenshot({path:path.join(out,'desktop-dark-reduced-motion.png'),fullPage:true});await reduced.close();
    report.passed=report.cases.every(c=>c.passed)&&report.requests.every(r=>r.method==='GET');
  } catch(error) {report.failure=error.stack;report.passed=false;}
  finally {await browser.close();fs.writeFileSync(path.join(out,'capture-report.json'),JSON.stringify(report,null,2)+'\n');console.log(JSON.stringify({passed:report.passed,cases:report.cases.map(c=>({name:c.name,theme:c.theme,passed:c.passed,failure:c.failure})),failure:report.failure}));if(!report.passed)process.exitCode=1;}
})();
