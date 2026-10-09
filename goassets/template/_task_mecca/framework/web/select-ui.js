/* Task Mecca themed select enhancement.
   Native select remains authoritative for all existing change handlers and is
   available as the no-JavaScript fallback. Popup is a single viewport-aware
   listbox, so mobile browsers never open an unrelated OS-native sheet. */
(function(){
 'use strict';
 if(!document?.body||typeof MutationObserver==='undefined')return;
 const SELECT_SELECTOR='select:not([data-tm-enhanced])';
 let current=null,popup=null,activeIndex=-1,searchField=null,filter='',serial=0,ticking=false;
 const visibleOptions=()=>current?[...current.select.options].filter(o=>!o.hidden):[];
 const isDisabled=o=>Boolean(o.disabled||o.parentElement?.disabled);
 const make=(tag,cls,text)=>{
  const el=document.createElement(tag);
  if(cls)el.className=cls;if(text!=null)el.textContent=String(text);
  return el;
 };
 function close(restore=false){
  const previous=current;
  if(popup){popup.remove();popup=null;}
  if(previous){
   previous.button.setAttribute('aria-expanded','false');
   previous.button.removeAttribute('aria-activedescendant');
   if(restore&&previous.button.isConnected)previous.button.focus({preventScroll:true});
  }
  current=null;searchField=null;activeIndex=-1;filter='';
 }
 function labelFor(select){
  const index=select.selectedIndex;
  return index>=0?(select.options[index]?.textContent||'').trim()||'—':'—';
 }
 function update(wrapper){
  if(!wrapper?.isConnected)return;
  const select=wrapper.querySelector('select'),button=wrapper.querySelector('button.tm-combo-trigger');
  if(!select||!button)return;
  button.textContent=labelFor(select);
  button.disabled=select.disabled;
  button.setAttribute('aria-label',select.getAttribute('aria-label')||select.closest('label')?.querySelector('span')?.textContent?.trim()||select.id||'Select');
  const names=select.options.length;
  if(current?.wrapper===wrapper && popup && popup.dataset.optionCount!==String(names))drawOptions();
 }
 function position(){
  if(!current||!popup)return;
  if(!current.wrapper.isConnected){close();return;}
  const r=current.button.getBoundingClientRect();
  const gap=5,pad=9,available=Math.max(180,window.innerWidth-2*pad);
  const width=Math.min(Math.max(r.width,200),available);
  const left=Math.max(pad,Math.min(r.left,window.innerWidth-width-pad));
  const below=window.innerHeight-r.bottom-gap,above=r.top-gap;
  const onTop=below<Math.min(220,window.innerHeight*.35)&&above>below;
  const height=Math.min(320,Math.floor((onTop?above:below)-pad),Math.floor(window.innerHeight*.50));
  popup.style.width=width+'px';
  popup.style.left=left+'px';
  popup.style.maxHeight=Math.max(120,height)+'px';
  popup.style.top=onTop?'auto':Math.max(pad,r.bottom+gap)+'px';
  popup.style.bottom=onTop?Math.max(pad,window.innerHeight-r.top+gap)+'px':'auto';
 }
 function navigate(delta){
  const items=filtered();
  if(!items.length)return;
  const enabled=items.filter(row=>!isDisabled(row.option));
  if(!enabled.length)return;
  let idx=enabled.findIndex(row=>row.index===activeIndex);
  idx=(idx+delta+enabled.length)%enabled.length;
  activeIndex=enabled[idx].index;
  drawOptions(true);
 }
 function filtered(){
  const q=filter.toLocaleLowerCase();
  return visibleOptions().map((option,index)=>({option,index}))
   .filter(row=>!q||row.option.textContent.toLocaleLowerCase().includes(q));
 }
 function commit(index){
  if(!current)return;
  const select=current.select,option=select.options[index];
  if(!option||isDisabled(option))return;
  const changed=select.selectedIndex!==index;
  select.selectedIndex=index;
  const button=current.button;
  update(current.wrapper);
  close(true);
  if(changed)select.dispatchEvent(new Event('change',{bubbles:true}));
  if(!button.isConnected)return;
 }
 function drawOptions(preserve=false){
  if(!popup||!current)return;
  const list=popup.querySelector('.tm-combo-items');
  if(!list)return;
  list.replaceChildren();
  const options=filtered();
  if(!preserve && !options.some(row=>row.index===activeIndex))activeIndex=options.find(row=>row.option.selected&&!isDisabled(row.option))?.index??options.find(row=>!isDisabled(row.option))?.index??-1;
  if(preserve && !options.some(row=>row.index===activeIndex))activeIndex=options.find(row=>!isDisabled(row.option))?.index??-1;
  popup.dataset.optionCount=String(current.select.options.length);
  if(!options.length){list.appendChild(make('p','tm-combo-empty',current.select.ownerDocument.documentElement.lang==='ko'?'검색 결과가 없습니다':'No matches'));return;}
  for(const row of options){
   const item=make('div','tm-combo-option',row.option.textContent||'');
   const disabled=isDisabled(row.option);
   item.id='tm-combo-option-'+row.index;
   item.setAttribute('role','option');item.setAttribute('aria-selected',String(row.index===current.select.selectedIndex));
   if(disabled)item.setAttribute('aria-disabled','true');
   if(row.index===activeIndex)item.classList.add('active');
   if(row.index===current.select.selectedIndex)item.classList.add('selected');
   if(!disabled)item.addEventListener('pointerdown',event=>{event.preventDefault();commit(row.index);});
   if(!disabled)item.addEventListener('click',()=>commit(row.index));
   list.appendChild(item);
  }
  current.button.setAttribute('aria-activedescendant','tm-combo-option-'+activeIndex);
  const highlighted=list.querySelector('.active');
  if(highlighted)highlighted.scrollIntoView({block:'nearest'});
 }
 function show(wrapper){
  const select=wrapper.querySelector('select'),button=wrapper.querySelector('.tm-combo-trigger');
  if(!select||select.disabled||!button)return;
  if(current?.wrapper===wrapper){close(true);return;}
  close();
  current={wrapper,select,button};
  popup=make('div','tm-combo-popup');
  popup.id='tm-combo-popup';
  popup.setAttribute('role','listbox');popup.setAttribute('aria-label',button.getAttribute('aria-label')||'Options');
  const opts=make('div','tm-combo-items');
  if(select.options.length>10){
   searchField=make('input','tm-combo-search');
   searchField.type='search';searchField.autocomplete='off';
   searchField.placeholder=document.documentElement.lang==='ko'?'항목 검색':'Search options';
   searchField.setAttribute('aria-label',searchField.placeholder);
   searchField.addEventListener('input',()=>{filter=searchField.value;drawOptions();});
   popup.appendChild(searchField);
  }
  popup.appendChild(opts);
  document.body.appendChild(popup);
  button.setAttribute('aria-expanded','true');
  button.setAttribute('aria-controls','tm-combo-popup');
  activeIndex=select.selectedIndex;
  position();drawOptions();
  if(searchField)searchField.focus({preventScroll:true});
 }
 function keys(event){
  if(!current)return;
  if(!current.wrapper.isConnected){close();return;}
  if(event.key==='Escape'){event.preventDefault();close(true);return;}
  if(event.key==='Tab'){close();return;}
  if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();navigate(event.key==='ArrowDown'?1:-1);return;}
  if(event.key==='Home'||event.key==='End'){
   event.preventDefault();const entries=filtered().filter(row=>!isDisabled(row.option));
   if(entries.length){activeIndex=entries[event.key==='Home'?0:entries.length-1].index;drawOptions(true);}
   return;
  }
  if(event.key==='Enter'){
   event.preventDefault();if(activeIndex>=0)commit(activeIndex);return;
  }
  if(event.key===' '&&event.target!==searchField){
   event.preventDefault();if(activeIndex>=0)commit(activeIndex);return;
  }
  if(event.target!==searchField&&event.key.length===1&&!event.ctrlKey&&!event.metaKey&&!event.altKey){
   filter+=event.key;drawOptions();event.preventDefault();
  }
 }
 function enhance(select){
  if(select.dataset.tmEnhanced||select.closest('.tm-combo'))return;
  const parent=select.parentNode;
  if(!parent)return;
  const wrapper=make('span','tm-combo'),button=make('button','tm-combo-trigger');
  button.type='button';button.setAttribute('role','combobox');
  button.setAttribute('aria-haspopup','listbox');button.setAttribute('aria-expanded','false');
  wrapper.appendChild(button);
  parent.insertBefore(wrapper,select);
  wrapper.insertBefore(select,button);
  select.dataset.tmEnhanced='1';select.classList.add('tm-combo-native');
  select.tabIndex=-1;
  select.addEventListener('change',()=>update(wrapper));
  button.addEventListener('click',e=>{e.preventDefault();show(wrapper);});
  button.addEventListener('keydown',event=>{
   if(current?.wrapper===wrapper){keys(event);return;}
   if(['ArrowDown','ArrowUp','Enter',' '].includes(event.key)){event.preventDefault();show(wrapper);}
  });
  update(wrapper);
 }
 function scan(){
  ticking=false;
  for(const select of document.querySelectorAll(SELECT_SELECTOR))enhance(select);
  for(const wrapper of document.querySelectorAll('.tm-combo'))update(wrapper);
  if(current&&!current.wrapper.isConnected)close();
 }
 function schedule(){if(ticking)return;ticking=true;queueMicrotask(scan);}
 document.addEventListener('keydown',event=>{
  if(current&&(event.target===searchField||event.target===current.button))keys(event);
 },true);
 document.addEventListener('pointerdown',event=>{
  if(current&&!current.wrapper.contains(event.target)&&!popup?.contains(event.target))close();
 },true);
 window.addEventListener('resize',()=>{if(current)position();});
 document.addEventListener('scroll',event=>{
  if(current&&popup&&!popup.contains(event.target))close();
 },true);
 const observer=new MutationObserver(schedule);
 observer.observe(document.body,{childList:true,subtree:true});
 scan();
})();
