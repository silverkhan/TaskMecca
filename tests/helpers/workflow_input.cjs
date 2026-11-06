// Native range input replacing the removed selection-zoom affordance.
async function zoomActivityRange(page,index=8){
  await page.locator('#workflowFit').click();
  const chart=page.locator('#workflowTimelineViewport');await chart.focus();
  for(const key of ['+','+','ArrowLeft','ArrowLeft'])await page.keyboard.press(key);
  await page.locator('.workflow-section').evaluate(el=>scrollTo({top:el.getBoundingClientRect().top+scrollY-(document.querySelector('.topbar')?.getBoundingClientRect().height||0)-18,behavior:'instant'}));
  const target=await page.locator('[data-workflow-interval]').nth(index).evaluate(el=>({start:+el.dataset.workflowStart,end:+el.dataset.workflowEnd}));
  const r=await page.locator('.workflow-axis-track').boundingBox(),row=await page.locator('.workflow-agent-track').first().boundingBox(),view=await page.locator('.workflow-timeline-canvas').evaluate(el=>({start:+el.dataset.viewStart,end:+el.dataset.viewEnd}));
  const padding=(target.end-target.start)*.1,position=t=>r.x+(t-view.start)/(view.end-view.start)*r.width,y=row.y+row.height/2;
  await page.keyboard.down('Shift');await page.mouse.move(position(target.start-padding),y);await page.mouse.down();await page.mouse.move(position(target.end+padding),y,{steps:6});await page.mouse.up();await page.keyboard.up('Shift');
  await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
}
module.exports={zoomActivityRange};
