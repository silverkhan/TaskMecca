// Same continuous document / sticky desktop index / floating mobile index as
// the incumbent GUI. Anchor navigation never rebuilds or hides document bodies.
import { state } from './state.js';
let dispose = () => {};
let readingIntent = 0;
for(const type of ['wheel','touchstart','pointerdown','keydown'])addEventListener(type,()=>{readingIntent++},{capture:true,passive:true});
export const readingIntentVersion = () => readingIntent;
export function scrollDetailAnchor(id, { updateURL = true, focus = true } = {}) {
  const target=document.getElementById(id);
  if(!target?.closest('.detail-layout'))return false;
  if(target.tagName==='DETAILS')target.open=true;
  target.scrollIntoView({behavior:'auto',block:'start'});
  if(focus){target.tabIndex=-1;target.focus({preventScroll:true})}
  if(updateURL){const url=new URL(location.href);url.hash=id;history.replaceState(history.state,'',url)}
  return true;
}
export function initialDetailAnchor() {
  let hash='';try{hash=decodeURIComponent(location.hash.slice(1))}catch{}
  if(hash&&document.getElementById(hash))return hash;
  return ({lifecycle:'lifecycle',requirements:document.getElementById('requirements')?'requirements':document.querySelector('.detail-body .document-section')?.id,evidence:'verification'})[state.detailTab];
}
export function bindDetailNavigation(root) {
  dispose();const layout=root.querySelector('.detail-layout');if(!layout)return;
  const toc=layout.querySelector('.detail-toc-nav'),toggle=layout.querySelector('.detail-toc-fab'),close=layout.querySelector('.detail-toc-close');
  const sections=[...layout.querySelectorAll('.detail-section[id]')];
  toc.replaceChildren(...sections.map(section=>{const a=document.createElement('a');a.href='#'+section.id;a.dataset.detailTarget=section.id;a.textContent=section.dataset.tocLabel||section.querySelector('h2')?.textContent||section.id;return a}));
  const setOpen=open=>{layout.classList.toggle('toc-open',open);toggle.setAttribute('aria-expanded',String(open))};
  const click=e=>{const anchor=e.target.closest('[data-detail-target]');if(anchor&&e.button===0&&!e.metaKey&&!e.ctrlKey&&!e.shiftKey&&!e.altKey){e.preventDefault();if(scrollDetailAnchor(anchor.dataset.detailTarget)&&matchMedia('(max-width:1280px)').matches)setOpen(false)}else if(e.target.closest('.detail-toc-fab')){setOpen(!layout.classList.contains('toc-open'));if(layout.classList.contains('toc-open'))toc.querySelector('a')?.focus()}else if(e.target.closest('.detail-toc-close,.detail-toc-scrim')){setOpen(false);toggle.focus()}};
  const key=e=>{if(e.key==='Escape'&&layout.classList.contains('toc-open')){e.preventDefault();setOpen(false);toggle.focus()}};
  layout.addEventListener('click',click);layout.addEventListener('keydown',key);
  const active=id=>toc.querySelectorAll('a').forEach(a=>{const current=a.dataset.detailTarget===id;a.classList.toggle('active',current);if(current)a.setAttribute('aria-current','location');else a.removeAttribute('aria-current')});
  active(sections[0]?.id);
  const observer=new IntersectionObserver(entries=>{const visible=entries.filter(e=>e.isIntersecting).sort((a,b)=>a.boundingClientRect.top-b.boundingClientRect.top);if(visible[0])active(visible[0].target.id)},{rootMargin:'-18% 0px -68% 0px',threshold:[0,.01]});sections.forEach(s=>observer.observe(s));
  dispose=()=>{observer.disconnect();layout.removeEventListener('click',click);layout.removeEventListener('keydown',key)};
}
