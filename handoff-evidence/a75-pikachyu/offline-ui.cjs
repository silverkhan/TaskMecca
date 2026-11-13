// Bounded offline rendering of the actual Root renderers; no browser or server.
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const code=source.slice(source.indexOf('function runtimeRootStatusLabel('),source.indexOf('function runtimeRootListPanel('));
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
for(const language of [0,1]) {
  const labels={};
  for(const key of ['runtimeRootT3Title','runtimeRootDefaultName','runtimeRootNameUnavailable','runtimeRootFallbackHint']) {
    const values=[...source.matchAll(new RegExp(key+":'([^']*)'",'g'))];
    assert.equal(values.length,2,key+' has Korean and English copy');labels[key]=values[language][1];
  }
  const context=vm.createContext({state:{runtimeRootDisclosure:{}},t:k=>labels[k]||k,esc,dateTimeLabel:()=>'-',ago:()=>'-',fmtBytes:()=>'-'});
  vm.runInContext(code,context,{timeout:1000});
  for(const [display_name_source,display_name_status,label] of [['t3_title','verified','runtimeRootT3Title'],['task_mecca_fallback','empty','runtimeRootDefaultName'],['task_mecca_fallback','unavailable','runtimeRootNameUnavailable'],['provider_name','verified',null]]) {
    const root={display_name:'🌱 <img src=x onerror=alert(1)> '+('긴 이름 '.repeat(80)),display_name_source,display_name_status,root_session_id:'root',provider_session_id:'native',provider:'codex',status:'active'};
    for(const html of [context.runtimeRootCard(root,[],{},{}),context.runtimeRootListRow(root)]) {
      assert(!html.includes('<img'));assert(html.includes('&lt;img'));assert(html.includes('긴 이름'));
      if(label) assert(html.includes(labels[label]));else assert(!html.includes(labels.runtimeRootT3Title));
      assert(!html.includes('display_name_source'));assert(!html.includes('undefined'));
    }
  }
}
console.log('PASS: Korean/English card and list labels, unknown/empty/native/T3 states, long Korean names, escaped markup; offline only.');
