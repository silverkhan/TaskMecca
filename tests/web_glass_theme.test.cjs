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

function initialAppearance(theme,palette,systemDark){
 const rootElement={dataset:{},style:{}};
 const meta={setAttribute(key,value){this[key]=value}};
 const document={documentElement:rootElement,querySelector(){return meta}};
 const window={matchMedia(){return {matches:systemDark}}};
 const storage={'task-mecca-theme':theme,'task-mecca-palette':palette};
 vm.runInNewContext(boot,{document,window,localStorage:{getItem:key=>storage[key]||null}});
 return {rootElement,meta};
}

test('Glass first paint resolves Dark, Light and System modes',()=>{
 for(const [theme,dark,expected,color] of [
  ['dark',false,'dark','#0b1020'],
  ['light',true,'light','#edf2fa'],
  ['system',false,'light','#edf2fa'],
  ['system',true,'dark','#0b1020']
 ]){
  const {rootElement,meta}=initialAppearance(theme,'glass',dark);
  assert.equal(rootElement.dataset.palette,'glass');
  assert.equal(rootElement.dataset.theme,expected);
  assert.equal(rootElement.dataset.themePreference,theme);
  assert.equal(meta.content,color);
 }
});

test('Existing Mecca remains the default and Glass can be selected',()=>{
 assert.equal(initialAppearance('dark','',false).rootElement.dataset.palette,'mecca');
 assert.match(html,/<option value="glass">Glass<\/option>/);
 assert.ok(app.includes("['mecca','slate','glass'].includes("));
 assert.ok(terminal.includes("['mecca', 'slate', 'glass'].includes(storedPalette)"));
});

test('Glass material is distinct and keeps high-density data free of blur',()=>{
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
