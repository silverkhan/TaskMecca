const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),vm=require('node:vm');
const root='goassets/template/_task_mecca/framework/web/';
const boot=fs.readFileSync(root+'theme-boot.js','utf8');
const app=fs.readFileSync(root+'app.js','utf8');
const css=fs.readFileSync(root+'style.css','utf8');
function appearance({theme='dark',palette='glass',background='',dark=false,coarse=false,width=390,height=844,blocked=false}={}){
 const style={setProperty(k,v){this[k]=v}},element={dataset:{},style},meta={setAttribute(k,v){this[k]=v}};
 const values={'task-mecca-theme':theme,'task-mecca-palette':palette,'task-mecca-background':background};
 vm.runInNewContext(boot,{document:{documentElement:element,querySelector:()=>meta},window:{screen:{width,height},innerWidth:width,innerHeight:height,matchMedia:q=>({matches:q.includes('pointer')?coarse:dark})},localStorage:{getItem(k){if(blocked)throw Error('denied');return values[k]}}});
 return{element,meta};
}
test('boot validates appearance and resolves System before application paint',()=>{
 for(const theme of ['dark','light','system'])for(const dark of [true,false])for(const background of ['cozy','ambient','none']){
  const {element,meta}=appearance({theme,dark,background});
  const expected=theme==='system'?(dark?'dark':'light'):theme;
  assert.equal(element.dataset.theme,expected);assert.equal(element.dataset.themePreference,theme);assert.equal(element.dataset.background,background);
  assert.equal(element.style.backgroundColor,meta.content);
  assert.equal(meta.content,expected==='dark'?'#211d1d':'#ece2d4');
 }
 const invalid=appearance({theme:'broken',palette:'invalid',background:'url(secret)'}).element.dataset;
 assert.deepEqual({...invalid},{themePreference:'dark',theme:'dark',palette:'mecca',background:'cozy'});
 assert.equal(appearance({blocked:true}).element.dataset.palette,'mecca');
});
test('other palette boot colors remain stable',()=>{
 for(const [palette,theme,color] of [['mecca','dark','#0a0e1b'],['mecca','light','#f7f3f8'],['slate','dark','#111318'],['slate','light','#f7f8fa']])assert.equal(appearance({palette,theme}).meta.content,color);
});
test('coarse screen canvas freezes a square before paint, desktop keeps screen dimensions',()=>{
 const portrait=appearance({coarse:true}),landscape=appearance({coarse:true,width:844,height:390});
 for(const result of [portrait,landscape])for(const key of ['--glass-stage-width','--glass-stage-height'])assert.equal(result.element.style[key],'844px');
 assert.equal(appearance({width:1440,height:1000}).element.style['--glass-stage-width'],'1440px');
});
test('background control persists validated selection and updates browser chrome',()=>{
 const start=app.indexOf('function applyBackground('),end=app.indexOf('function applyPalette(',start);
 const state={},attrs={},saved={},picker={};let sync=0;
 const context={state,document:{documentElement:{dataset:attrs}},localStorage:{setItem:(k,v)=>saved[k]=v},syncThemeBrowserChrome(){sync++},$:()=>picker,window:{TaskMeccaSelectUI:{sync(){}}}};
 vm.runInNewContext(app.slice(start,end)+"\napplyBackground('none');applyBackground('bad');",context);
 assert.equal(state.background,'cozy');assert.equal(attrs.background,'cozy');assert.equal(saved['task-mecca-background'],'cozy');assert.equal(picker.value,'cozy');assert.equal(sync,2);
});
test('Glass fallback and accessibility preferences are separate contracts',()=>{
 const glass=css.slice(css.indexOf('/* GLASS MATERIAL SYSTEM'));
 assert.match(glass,/@supports not/);assert.match(glass,/@media\(prefers-reduced-transparency:reduce\)/);assert.match(glass,/@media\(prefers-reduced-motion:reduce\)/);
 const fallback=glass.slice(glass.indexOf('@supports not'),glass.indexOf('@media(prefers-reduced-transparency'));
 assert.ok(!fallback.includes('background-image:none')); // unsupported blur still has a room
 assert.match(glass,/background-image:none!important/); // user preference removes the room
 assert.equal((glass.match(/\/\* GLASS MATERIAL SYSTEM/g)||[]).length,1);
 assert.ok(fs.readFileSync(root+'cozy-beige-room.svg','utf8').startsWith('<svg'));
});
test('photographic derivatives match provenance and explicit byte budgets',()=>{
 const crypto=require('node:crypto'),manifest=JSON.parse(fs.readFileSync('docs/assets/cozy-beige-room-photo.json'));
 assert.deepEqual(manifest.native_generation_size,[1672,941]);assert.equal(manifest.neural_upscale.model,'realesrgan-x4plus');
 for(const [name,budget] of [['cozy-beige-room-photo-1920.webp',500000],['cozy-beige-room-photo-3840.webp',1250000]]){
  const data=fs.readFileSync(root+name),record=manifest.files.find(x=>x.file===name);
  assert.equal(data.length,record.bytes);assert.ok(data.length<=budget);assert.equal(crypto.createHash('sha256').update(data).digest('hex'),record.sha256);
  assert.equal(data.toString('ascii',0,4),'RIFF');assert.equal(data.toString('ascii',8,12),'WEBP');assert.ok(css.includes(name));
 }
});
