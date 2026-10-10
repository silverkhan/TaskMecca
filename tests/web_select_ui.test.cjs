const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/select-ui.js','utf8');
const styles=fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8');
const index=fs.readFileSync('goassets/template/_task_mecca/framework/web/index.html','utf8');

class FakeNode {
 constructor(tag,doc){
  this.tagName=tag.toUpperCase();this.ownerDocument=doc;this.children=[];this.parentNode=null;
  this.className='';this.attrs={};this.dataset={};this._text='';this.listeners={};
  this.style={};this.disabled=false;this.isConnected=true;
  this.classList={add:(cl)=>this.className+=' '+cl,contains:(cl)=>this.className.split(/\s+/).includes(cl)};
 }
 set textContent(v){this._text=String(v)}
 get textContent(){return this._text}
 get parentElement(){return this.parentNode}
 appendChild(n){this.insertBefore(n,null);return n}
 insertBefore(n,at){
  if(n.parentNode)n.parentNode.children=n.parentNode.children.filter(x=>x!==n);
  n.parentNode=this;
  const idx=at?this.children.indexOf(at):-1;
  if(idx<0)this.children.push(n);else this.children.splice(idx,0,n);
  return n;
 }
 setAttribute(k,v){this.attrs[k]=String(v)}
 getAttribute(k){return this.attrs[k]??null}
 removeAttribute(k){delete this.attrs[k]}
 addEventListener(k,fn){(this.listeners[k]||=[]).push(fn)}
 dispatchEvent(e){for(const f of this.listeners[e.type]||[])f(e);return true}
 focus(){this.ownerDocument.activeElement=this}
 remove(){if(this.parentNode)this.parentNode.children=this.parentNode.children.filter(x=>x!==this);this.isConnected=false}
 replaceChildren(){this.children=[]}
 getBoundingClientRect(){return {left:20,width:210,right:230,top:20,bottom:60,height:40}}
 scrollIntoView(){}
 contains(n){return this===n||this.children.some(c=>c.contains(n))}
 closest(cl){for(let cur=this;cur;cur=cur.parentNode){if(cl==='label'&&cur.tagName==='LABEL')return cur;if(cl==='.tm-combo'&&cur.className.includes('tm-combo'))return cur}return null}
 querySelector(selector){return this.querySelectorAll(selector)[0]||null}
 querySelectorAll(selector){
  const matches=(n)=>selector==='select'&&n.tagName==='SELECT'||
   selector==='.tm-combo'&&n.className.split(/\s+/).includes('tm-combo')||
   selector==='.tm-combo-trigger'&&n.className.split(/\s+/).includes('tm-combo-trigger')||
   selector==='button.tm-combo-trigger'&&n.tagName==='BUTTON'&&n.className.includes('tm-combo-trigger')||
   selector==='.tm-combo-items'&&n.className.includes('tm-combo-items')||
   selector==='.active'&&n.className.split(/\s+/).includes('active');
  const output=[];const visit=n=>{for(const c of n.children){if(matches(c))output.push(c);visit(c)}};
  visit(this);return output;
 }
}
function harness(){
 const document={listeners:{},documentElement:{lang:'ko'},activeElement:null,
  createElement(tag){return new FakeNode(tag,this)},
  addEventListener(k,fn){(this.listeners[k]||=[]).push(fn)},
  querySelectorAll(sel){
   if(sel==='select:not([data-tm-enhanced])')return this.body.querySelectorAll('select').filter(n=>!n.dataset.tmEnhanced);
   return this.body.querySelectorAll(sel);
  }
 };
 document.body=document.createElement('body');
 const container=document.createElement('label'),native=document.createElement('select');
 native.options=[{textContent:'empfund',selected:true,disabled:false},{textContent:'TaskMecca',selected:false,disabled:false}];
 native.selectedIndex=0;native.getAttribute=k=>null;
 container.appendChild(native);document.body.appendChild(container);
 let observer;
 class MO{constructor(fn){observer=fn}observe(){}}
 const context=vm.createContext({document,window:{innerWidth:360,innerHeight:700,addEventListener(){}},
  MutationObserver:MO,queueMicrotask:fn=>Promise.resolve().then(fn),Event:class Event{constructor(type,opts){this.type=type;this.bubbles=opts?.bubbles}}});
 vm.runInContext(source,context);
 return {document,container,native,window:context.window,notify:()=>observer?.([])};
}
test('every select is enhanced while native change event continues powering existing handlers',()=>{
 const h=harness(),wrapper=h.container.querySelector('.tm-combo');
 assert.ok(wrapper);
 const button=wrapper.querySelector('.tm-combo-trigger');
 assert.equal(button.getAttribute('role'),'combobox');
 assert.equal(button.textContent,'empfund');
 assert.equal(h.native.tabIndex,-1);
 let changed=0;h.native.addEventListener('change',()=>changed++);
 button.dispatchEvent({type:'click',preventDefault(){}});
 const popup=h.document.body.children.find(x=>x.className==='tm-combo-popup');
 assert.ok(popup,'custom popup replaces OS-native option sheet');
 assert.equal(popup.getAttribute('role'),'listbox');
 assert.equal(popup.querySelectorAll('.tm-combo-items').length,1);
 const options=popup.querySelector('.tm-combo-items').children;
 options[1].dispatchEvent({type:'pointerdown',preventDefault(){}});
 assert.equal(h.native.selectedIndex,1);
 assert.equal(button.textContent,'TaskMecca');
 assert.equal(changed,1,'native select change event must fire once');
 assert.equal(button.getAttribute('aria-expanded'),'false');
});
test('keyboard navigation and escape restore focus without changing native selection',()=>{
 const h=harness(),button=h.container.querySelector('.tm-combo-trigger');
 button.dispatchEvent({type:'keydown',key:'Enter',preventDefault(){}});
 assert.equal(button.getAttribute('aria-expanded'),'true');
 button.dispatchEvent({type:'keydown',key:'ArrowDown',preventDefault(){}});
 button.dispatchEvent({type:'keydown',key:'Enter',preventDefault(){}});
 assert.equal(h.native.selectedIndex,1);
 button.dispatchEvent({type:'keydown',key:'Enter',preventDefault(){}});
 button.dispatchEvent({type:'keydown',key:'Escape',preventDefault(){}});
 assert.equal(button.getAttribute('aria-expanded'),'false');
 assert.equal(h.native.selectedIndex,1);
 assert.equal(h.document.activeElement,button);
});
test('mobile theme and all-selector integration are wired into every page',()=>{
 assert.match(index,/<script src="\/select-ui\.js"><\/script>/);
 assert.match(styles,/\.tm-combo-popup\{position:fixed/);
 assert.match(styles,/\.tm-combo-option\.active/);
 assert.match(styles,/select\.tm-combo-native/);
 assert.match(source,/new MutationObserver\(schedule\)/);
 assert.match(source,/aria-haspopup/);
 assert.match(source,/if\(current&&event.target===searchField\)keys\(event\)/);
});

test('mobile header media query hides picker captions only, not inserted select wrappers',()=>{
 // Regresses dev.165: mobile '.language-picker span' and '.palette-picker span'
 // selectors matched ALL descendant spans, including our new .tm-combo span,
 // leaving both topbar dropdowns visually empty.
 for(const klass of ['language-picker','palette-picker','backlog-picker','project-picker']){
  assert.doesNotMatch(styles,new RegExp('\\.'+klass+' span\\{display:none\\}'));
  assert.match(styles,new RegExp('\\.'+klass+' > span:not\\(\\.tm-combo\\)(?:,|\\{display:none\\})'));
 }
 assert.match(styles,/\.language-picker \.tm-combo,\.palette-picker \.tm-combo\{min-width:72px;flex:1\}/);
 assert.match(styles,/\.tm-combo-trigger\{[^}]*color:var\(--text\)/);
});
test('programmatic native select changes immediately synchronize visible label',()=>{
 const h=harness(),button=h.container.querySelector('.tm-combo-trigger');
 assert.equal(button.textContent,'empfund');
 h.native.selectedIndex=1;
 // Initial preference restoration does not emit a change event.
 assert.equal(button.textContent,'empfund');
 h.window.TaskMeccaSelectUI.sync();
 assert.equal(button.textContent,'TaskMecca');
 assert.equal(button.getAttribute('aria-expanded'),'false');
});
test('language and palette initialization explicitly synchronize themed picker text',()=>{
 const app=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
 assert.match(app,/renderLanguagePicker\(\);\s*window\.TaskMeccaSelectUI\?\.sync\(\)/);
 assert.match(app,/picker\.value=state\.palette;\s*window\.TaskMeccaSelectUI\?\.sync\(\)/);
});
