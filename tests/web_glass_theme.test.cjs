const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const root='goassets/template/_task_mecca/framework/web/';
const boot=fs.readFileSync(root+'theme-boot.js','utf8');
const css=fs.readFileSync(root+'style.css','utf8');
const html=fs.readFileSync(root+'index.html','utf8');
const app=fs.readFileSync(root+'app.js','utf8');
const terminal=fs.readFileSync(root+'terminal.js','utf8');
const image=fs.readFileSync(root+'cozy-beige-room.svg','utf8');

function initialAppearance(theme,palette,systemDark,backdrop=''){
 const rootElement={dataset:{},style:{}};
 const meta={setAttribute(key,value){this[key]=value}};
 const document={documentElement:rootElement,querySelector(){return meta}};
 const window={matchMedia(){return {matches:systemDark}}};
 const storage={'task-mecca-theme':theme,'task-mecca-palette':palette,'task-mecca-background':backdrop};
 vm.runInNewContext(boot,{document,window,localStorage:{getItem:key=>storage[key]||null}});
 return {rootElement,meta};
}

test('Glass boot: cozy beige defaults on Dark, Light and System',()=>{
 for(const [theme,dark,expected,color] of [
  ['dark',false,'dark','#211b1b'],
  ['light',true,'light','#e9dfce'],
  ['system',false,'light','#e9dfce'],
  ['system',true,'dark','#211b1b']
 ]){
  const {rootElement,meta}=initialAppearance(theme,'glass',dark);
  assert.equal(rootElement.dataset.palette,'glass');
  assert.equal(rootElement.dataset.background,'cozy');
  assert.equal(rootElement.dataset.theme,expected);
  assert.equal(rootElement.dataset.themePreference,theme);
  assert.equal(meta.content,color);
 }
});

test('Background preference is restored and invalid values safely default',()=>{
 for(const [choice,expected] of [
  ['cozy','cozy'],['ambient','ambient'],['none','none'],
  ['broken','cozy'],['','cozy']
 ])assert.equal(initialAppearance('dark','glass',false,choice).rootElement.dataset.background,expected);
 assert.equal(initialAppearance('light','glass',false,'ambient').meta.content,'#edf2fa');
 assert.equal(initialAppearance('dark','glass',false,'none').meta.content,'#0b1020');
});

test('Existing palettes remain unchanged',()=>{
 assert.equal(initialAppearance('dark','',false).rootElement.dataset.palette,'mecca');
 assert.equal(initialAppearance('dark','mecca',false).meta.content,'#0a0e1b');
 assert.equal(initialAppearance('light','slate',false).meta.content,'#f7f8fa');
 assert.match(html,/<option value="glass">Glass<\/option>/);
 assert.ok(app.includes("['mecca','slate','glass'].includes("));
 assert.ok(terminal.includes("['mecca', 'slate', 'glass'].includes(storedPalette)"));
});

test('Background picker is wired with accessible options and persistence',()=>{
 for(const choice of ['cozy','ambient','none']){
  assert.ok(html.includes('<option value="'+choice+'">'),choice);
 }
 assert.ok(html.includes('id="backgroundPicker"'));
 assert.ok(html.includes('id="backgroundLabel"'));
 assert.ok(app.includes("$('#backgroundPicker')?.addEventListener('change'"));
 assert.ok(app.includes("localStorage.setItem('task-mecca-background',state.background)"));
 assert.ok(app.includes("backgroundCozy:'코지 베이지 실내'"));
 assert.ok(app.includes("backgroundCozy:'Cozy Beige Room'"));
 const start=app.indexOf('function applyBackground(value) {');
 const end=app.indexOf('function applyPalette(value)',start);
 assert.ok(start>0&&end>start);
 const state={background:'cozy'};
 const attrs={};
 const picker={value:''};
 const saved={};
 let chromeSync=0,selectSync=0;
 const context={state,document:{documentElement:{dataset:attrs}},
   localStorage:{setItem(k,v){saved[k]=v}},
   $:()=>picker,window:{TaskMeccaSelectUI:{sync(){selectSync++}}},
   syncThemeBrowserChrome(){chromeSync++}};
 vm.runInNewContext(app.slice(start,end)+"\napplyBackground('none')",context);
 assert.equal(state.background,'none');
 assert.equal(attrs.background,'none');
 assert.equal(saved['task-mecca-background'],'none');
 assert.equal(picker.value,'none');
 assert.equal(chromeSync,1);
 assert.equal(selectSync,1);
});

test('Cozy room asset and distinct Glass material preserve data readability',()=>{
 assert.ok(image.startsWith('<svg'));
 assert.ok(image.includes('aria-label="Cozy beige'));
 assert.ok(css.includes('url("/cozy-beige-room.svg")'));
 assert.ok(css.includes(':root[data-palette="glass"][data-background="cozy"] body'));
 assert.ok(css.includes(':root[data-palette="glass"][data-background="none"] body'));
 assert.ok(css.includes(':root:not([data-palette="glass"]) .background-picker'));
 for(const token of [
  ':root[data-palette="glass"][data-theme="dark"]',
  ':root[data-palette="glass"][data-theme="light"]',
  '--glass-panel:', '--glass-edge:', '--glass-depth:',
  ':root[data-palette="glass"] .worker-card',
  ':root[data-palette="glass"] .runtime-root-card',
  ':root[data-palette="glass"] .activity-metric',
  ':root[data-palette="glass"] .common-attention-body',
  '@media(prefers-reduced-transparency:reduce)'
 ])assert.ok(css.includes(token),token);
 assert.ok(css.includes('background-attachment:scroll'));
 assert.ok(css.includes('backdrop-filter:none'));
});
