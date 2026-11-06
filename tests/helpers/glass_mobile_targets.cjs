const selectors=['#search','.search-wrap','#sidebarToggle','#refreshBtn','#prevPage','#nextPage','#globalVersionRetryBtn'];
function readMobileTargets(){
 const selectors=['#search','.search-wrap','#sidebarToggle','#refreshBtn','#prevPage','#nextPage','#globalVersionRetryBtn'],v=visualViewport;
 const probe=document.createElement('div');probe.style.cssText='position:fixed;visibility:hidden;pointer-events:none;padding:env(safe-area-inset-top) env(safe-area-inset-right) env(safe-area-inset-bottom) env(safe-area-inset-left)';document.body.append(probe);const ps=getComputedStyle(probe),safe={top:parseFloat(ps.paddingTop),right:parseFloat(ps.paddingRight),bottom:parseFloat(ps.paddingBottom),left:parseFloat(ps.paddingLeft)};probe.remove();
 return{viewport:{left:v.offsetLeft,top:v.offsetTop,width:v.width,height:v.height,scale:v.scale,safe},layout:{width:innerWidth,height:innerHeight,clientWidth:document.documentElement.clientWidth,scrollWidth:document.documentElement.scrollWidth,scrollX,scrollY},targets:selectors.map(selector=>{const e=document.querySelector(selector);if(!e)return{selector,missing:true};const s=getComputedStyle(e);return{selector,tag:e.tagName,disabled:!!e.disabled,rect:e.getBoundingClientRect().toJSON(),style:Object.fromEntries(['display','width','height','minWidth','minHeight','paddingTop','paddingRight','paddingBottom','paddingLeft','borderWidth','borderRadius','fontSize','lineHeight','color','backgroundColor','boxShadow'].map(k=>[k,s[k]]))}})};
}
function targetViolations(row,{active=[]}={}){
 const failures=[],v=row.viewport;
 for(const t of row.targets){if(t.missing){failures.push(t.selector+': missing');continue}if(t.rect.width<44||t.rect.height<44)failures.push(t.selector+': native '+t.rect.width+'x'+t.rect.height+' is below44');if(active.includes(t.selector)&&(t.rect.left<v.left+v.safe.left-.05||t.rect.right>v.left+v.width-v.safe.right+.05||t.rect.top<v.top+v.safe.top-.05||t.rect.bottom>v.top+v.height-v.safe.bottom+.05))failures.push(t.selector+': outside actual visual viewport/safe area')}
 for(let i=0;i<row.targets.length;i++)for(let j=i+1;j<row.targets.length;j++){const a=row.targets[i],b=row.targets[j];if(a.missing||b.missing||[a.selector,b.selector].includes('.search-wrap')&&[a.selector,b.selector].includes('#search'))continue;const width=Math.min(a.rect.right,b.rect.right)-Math.max(a.rect.left,b.rect.left),height=Math.min(a.rect.bottom,b.rect.bottom)-Math.max(a.rect.top,b.rect.top);if(width>.5&&height>.5)failures.push(a.selector+' overlaps '+b.selector)}
 if(row.layout.scrollWidth>row.layout.clientWidth+1)failures.push('new horizontal document overflow: '+row.layout.scrollWidth+'/'+row.layout.clientWidth);
 return failures;
}
module.exports={selectors,readMobileTargets,targetViolations};
