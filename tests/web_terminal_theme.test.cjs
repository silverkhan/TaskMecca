const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const dir='goassets/template/_task_mecca/framework/web/';
const html=fs.readFileSync(dir+'terminal.html','utf8');
const css=fs.readFileSync(dir+'terminal.css','utf8');
const js=fs.readFileSync(dir+'terminal.js','utf8');

test('Terminal initializes shared appearance before first stylesheet paint',()=>{
 const boot=html.indexOf('<script src="/theme-boot.js"></script>');
 const cssLink=html.indexOf('<link rel="stylesheet" href="/style.css" />');
 assert.ok(boot>0&&cssLink>boot);
 assert.ok(html.includes('<meta name="theme-color"'));
});

test('Terminal shell, fallback and emergency commands use semantic colors',()=>{
 for(const token of ['--terminal-console-bg','--terminal-console-text','--terminal-console-toolbar',
   '--terminal-console-border','--terminal-control-bg']){
  assert.ok(css.includes(token),token);
 }
 assert.ok(css.includes('background: var(--terminal-console-bg);'));
 assert.ok(!css.includes('background: #080c12;'));
 assert.ok(!css.includes('background: #101620;'));
 assert.ok(!css.includes('background: #141b26;'));
 assert.ok(js.includes('theme: terminalTheme()'));
 assert.ok(!js.includes('theme: prefersDark'));
});

function simulate(theme,palette,dark) {
 const root={dataset:{},style:{colorScheme:'',removeProperty(){}}};
 const canvas={};
 const meta={setAttribute(k,v){this[k]=v}};
 const term={options:{}};
 const settings={'task-mecca-theme':theme,'task-mecca-palette':palette};
 const document={documentElement:root,querySelector(s){return s==='meta[name="theme-color"]'?meta:null}};
 const window={
  matchMedia(){return {matches:dark}},
  getComputedStyle(el){
   if(el===root)return {getPropertyValue(k){return {'--bg':'#f7f3f8','--accent':'#795b91','--accent-soft':'#eee3f2'}[k]||''}};
   return {backgroundColor:'rgb(247, 243, 248)',color:'rgb(41, 37, 54)'};
  }
 };
 const context={window,document,state:{terminal:term},$:()=>canvas,localStorage:{getItem:k=>settings[k]||null}};
 const from=js.indexOf('  function terminalTheme()'),to=js.indexOf('  function projectQuery()',from);
 assert.ok(from>=0&&to>from);
 vm.runInNewContext(js.slice(from,to)+'\napplyTheme();',context);
 return {root,term,meta};
}

test('Terminal resolves Mecca/Slate and System/Light/Dark without overrides',()=>{
 for(const [theme,palette,dark,expected] of [
  ['light','mecca',true,'light'],['dark','slate',false,'dark'],
  ['system','mecca',false,'light'],['system','slate',true,'dark'],
  ['', '',false,'dark']
 ]){
  const {root}=simulate(theme,palette,dark);
  assert.equal(root.dataset.theme,expected);
  assert.equal(root.dataset.themePreference,theme||'dark');
  assert.equal(root.dataset.palette,palette||'mecca');
 }
});

test('Existing xterm canvas recolors from CSS without a session restart',()=>{
 const {term,meta}=simulate('light','mecca',false);
 assert.equal(meta.content,'#f7f3f8');
 assert.equal(term.options.theme.background,'rgb(247, 243, 248)');
 assert.equal(term.options.theme.foreground,'rgb(41, 37, 54)');
 assert.equal(term.options.theme.cursor,'#795b91');
 assert.equal(term.options.theme.selectionBackground,'#eee3f2');
 assert.ok(js.includes("window.addEventListener('storage', event =>"));
 assert.ok(js.includes("['task-mecca-theme', 'task-mecca-palette'].includes(event.key)"));
 assert.ok(js.includes("addEventListener?.('change', applyTheme)"));
});
