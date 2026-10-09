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
 return {document,container,native,notify:()=>observer?.([])};
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

test('responsive picker label rules cannot hide the themed select wrapper (mobile regression AID-131)',()=>{
 const labels=['language-picker','palette-picker','project-picker','backlog-picker'];
 for(const name of labels){
  // The old broad ".language-picker span{display:none}" also hid
  // <span class="tm-combo">, leaving only an empty box on Android.
  assert.doesNotMatch(styles,new RegExp('\\.'+name+'\\s+span\\s*\\{\\s*display\\s*:\\s*none\\b'));
  assert.match(styles,new RegExp('\\.'+name+'\\s*>\\s*span:not\\(\\.tm-combo\\)'));
 }
 assert.match(styles,/\.topbar \.language-picker > \.tm-combo/);
 assert.match(styles,/\.topbar \.palette-picker > \.tm-combo/);
 assert.match(styles,/\.topbar \.palette-picker \.tm-combo-trigger.*color:var\(--text\)/);
 assert.match(styles,/@media\(max-width:680px\)[\s\S]*?\.topbar \.palette-picker > \.tm-combo/);
});
test('theme and language controls remain visible while captions are mobile-hidden',()=>{
 const header=index.slice(index.indexOf('<header class="topbar">'),index.indexOf('</header>'));
 assert.match(header,/<label class="language-picker">.*<select id="languagePicker"/);
 assert.match(header,/<label class="palette-picker".*<select id="palettePicker"/);
 assert.match(styles,/@media\(max-width:1050px\)\{\.backlog-picker > span:not\(\.tm-combo\),\.language-picker > span:not\(\.tm-combo\)\{display:none\}/);
 assert.match(styles,/@media\(max-width:900px\)\{\.palette-picker > span:not\(\.tm-combo\)\{display:none\}/);
});
