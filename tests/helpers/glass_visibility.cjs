// Test-only geometric visibility. A covered or overflow-clipped point is not a
// rendered foreground/background pair, regardless of that pair's contrast.
function installGlassVisibility(){
 window.__glassPointVisibility=(element,{x,y})=>{
  const view=visualViewport;
  if(x<view.offsetLeft||y<view.offsetTop||x>=view.offsetLeft+view.width||y>=view.offsetTop+view.height)return{visible:false,reason:'outside visual viewport',occluder:'visualViewport'};
  for(let parent=element.parentElement;parent;parent=parent.parentElement){
   const style=getComputedStyle(parent),rect=parent.getBoundingClientRect(),left=rect.left+parent.clientLeft,top=rect.top+parent.clientTop;
   if(/^(auto|scroll|hidden|clip)$/.test(style.overflowX)&&(x<left||x>=left+parent.clientWidth)||/^(auto|scroll|hidden|clip)$/.test(style.overflowY)&&(y<top||y>=top+parent.clientHeight))return{visible:false,reason:'clipped by overflow',occluder:parent.id||parent.className||parent.tagName};
  }
  const hit=document.elementFromPoint(x,y),visible=!!hit&&(hit===element||element.contains(hit)||hit.contains(element));
  return{visible,reason:visible?'visible':hit?'occluded':'outside hit-test viewport',occluder:visible?null:hit?.id||hit?.className||hit?.tagName||null};
 };
}
module.exports={installGlassVisibility};
