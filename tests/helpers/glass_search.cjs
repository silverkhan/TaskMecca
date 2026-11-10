const assert=require('node:assert/strict');

// Resolve and read the current input in one browser task. A locator handle can
// be detached by an app render before its separate evaluate task executes.
function installSearchReader(){
 window.__glassReadSearch=(input=document.querySelector('.search-wrap input'))=>{
  const frame=input?.closest('.search-wrap'),snapshot=e=>{if(!e)return null;const s=getComputedStyle(e);return{connected:e.isConnected,rect:e.getBoundingClientRect().toJSON(),widths:['Top','Right','Bottom','Left'].map(side=>s['border'+side+'Width']),styles:['Top','Right','Bottom','Left'].map(side=>s['border'+side+'Style']),colors:['Top','Right','Bottom','Left'].map(side=>s['border'+side+'Color']),shadow:s.boxShadow}};
  return{current:!!input&&input===document.querySelector('.search-wrap input'),input:snapshot(input),frame:snapshot(frame)};
 };
}
function assertSearchFrame(row){
 assert.ok(row.current&&row.input?.connected&&row.frame?.connected,'search observation uses the connected current input');
 assert.deepEqual(row.input.widths,Array(4).fill('0px'),'search input has no border on any side');
 assert.equal(row.input.shadow,'none','search input has no inner shadow');
 assert.ok(row.frame.widths.every((width,i)=>parseFloat(width)>0&&!['none','hidden',''].includes(row.frame.styles[i])),'search retains its outer frame on all four sides');
 const a=row.input.rect,b=row.frame.rect;
 assert.ok(a.width>0&&a.height>0&&a.left>=b.left&&a.top>=b.top&&a.right<=b.right&&a.bottom<=b.bottom,'search input occupies the interior of the outer frame');
}
async function checkSearchControls(page){
 const controls=await page.evaluate(()=>{const input=document.querySelector('.search-wrap input'),positive=__glassReadSearch(),detached=__glassReadSearch(input.cloneNode(true)),style=input.getAttribute('style');input.style.setProperty('border-left','1px solid red','important');const border=__glassReadSearch();if(style===null)input.removeAttribute('style');else input.setAttribute('style',style);return{positive,detached,border,restored:__glassReadSearch()}});
 assertSearchFrame(controls.positive);assertSearchFrame(controls.restored);
 assert.throws(()=>assertSearchFrame(controls.detached),/connected current/);
 assert.equal(controls.border.input.widths[3],'1px');assert.throws(()=>assertSearchFrame(controls.border),/no border on any side/);
 return controls;
}
module.exports={installSearchReader,assertSearchFrame,checkSearchControls};
