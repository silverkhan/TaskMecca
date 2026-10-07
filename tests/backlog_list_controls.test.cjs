const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const source = fs.readFileSync(path.join(root, 'goassets/template/_task_mecca/framework/web/app.js'), 'utf8');
const index = fs.readFileSync(path.join(root, 'goassets/template/_task_mecca/framework/web/index.html'), 'utf8');
assert(!index.includes('id="search"'));
assert(!index.includes('id="backlogPicker"'));
function extract(name, next) {
  const start = source.indexOf(`function ${name}(`);
  assert(start >= 0, name);
  const end = source.indexOf(next, start + 1);
  assert(end > start, next);
  return source.slice(start, end);
}
let refreshes = 0, listRefreshes = 0, callback;
const elements = {};
const makeElement = () => ({listeners:{}, addEventListener(name, fn){assert(!this.listeners[name], 'duplicate listener');this.listeners[name]=fn;}});
const state = {query:'<needle>', backlog:'',view:'backlog',detail:null,listPage:3,selectedIndex:2,listSort:'id_desc',listPageMode:'auto'};
const data = {backlog_selection:{selected:'/fixture/backlog',candidates:[{path:'/fixture/backlog',name:'backlog',record_count:2},{path:'/fixture/backlog_long',name:'backlog_long',record_count:0}]}};
const storage = new Map();
const context = vm.createContext({state, currentProjectData:()=>data, $:id=>elements[id], t:key=>key, esc:value=>String(value).replaceAll('<','&lt;').replaceAll('>','&gt;'), dateLabel:()=>'-', localStorage:{setItem:(k,v)=>storage.set(k,v),removeItem:k=>storage.delete(k)},refresh:()=>refreshes++,refreshList:()=>listRefreshes++,clearTimeout:()=>{callback=null},setTimeout:fn=>{callback=fn;return 1},statusFilterBar:()=>'',tagFilterBar:()=>''});
vm.runInContext(extract('renderBacklogPicker','function backlogUrl')+extract('backlogListTools','function listView')+'let searchRefreshTimer=0;'+extract('bindBacklogListTools',"$('#refreshBtn').onclick"),context);
const html = vm.runInContext('listControls({page:1,pages:1,total:2,pageSize:10})',context);
assert(html.includes('backlog-list-controls'));
assert.equal((html.match(/id="search"/g)||[]).length,1);
assert.equal((html.match(/id="backlogPicker"/g)||[]).length,1);
assert(html.includes('value="&lt;needle&gt;"'));
for(let render=0;render<3;render++){
  elements['#search']=makeElement();elements['#backlogPicker']=makeElement();
  vm.runInContext('bindBacklogListTools()',context);
  assert(elements['#backlogPicker'].innerHTML.includes('backlog_long'));
  assert.equal(elements['#backlogPicker'].disabled,false);
  elements['#search'].listeners.input({target:{value:'needle'}});
  assert.equal(state.query,'needle');assert.equal(state.listPage,1);callback();
}
assert.equal(listRefreshes,3);
elements['#backlogPicker'].listeners.change({target:{value:'/fixture/backlog_long'}});
assert.equal(state.backlog,'/fixture/backlog_long');assert.equal(refreshes,1);
assert.equal(storage.get('task-mecca-backlog-folder'),state.backlog);
for(const view of ['hub','workload','attention','issues','notifications','manual','release-notes','terminal']){
  state.view=view;delete elements['#search'];delete elements['#backlogPicker'];
  vm.runInContext('bindBacklogListTools()',context);
  if(callback)callback();
}
assert.equal(listRefreshes,3);
state.view='backlog';state.detail='A-1';vm.runInContext('bindBacklogListTools()',context);if(callback)callback();assert.equal(listRefreshes,3);
const slash=source.split('\n').find(line=>line.includes("if(e.key==='/'"));
let prevented=0,focused=0;
context.e={key:'/',preventDefault:()=>prevented++};context.editing=false;
vm.runInContext(`function key(){${slash}}`,context);
vm.runInContext('key()',context);assert.equal(prevented,0);
state.detail=null;elements['#search']={focus:()=>focused++};vm.runInContext('key()',context);
assert.equal(prevented,1);assert.equal(focused,1);
context.editing=true;vm.runInContext('key()',context);assert.equal(focused,1);
state.view='manual';context.editing=false;vm.runInContext('key()',context);assert.equal(focused,1);
console.log('Backlog list controls: placement, escaped query, rebind, folder, debounce, scope and slash PASS');
context.URLSearchParams=URLSearchParams;context.effectiveListPageSize=()=>10;
Object.assign(state,{project:'/fixture/project',backlog:'/fixture/backlog',listPage:2,statusFilters:['ready','doing'],tagFilters:['area:frontend'],query:' needle ',listSort:'updated_desc'});
vm.runInContext(extract('listQueryString','async function refreshList'),context);
const params=new URLSearchParams(vm.runInContext('listQueryString()',context));
assert.equal(params.get('project'),'/fixture/project');assert.equal(params.get('backlog'),'/fixture/backlog');
assert.equal(params.get('q'),'needle');assert.equal(params.get('status'),'ready,doing');assert.equal(params.get('tags'),'area:frontend');
assert.equal(params.get('page'),'2');assert.equal(params.get('page_size'),'10');assert.equal(params.get('sort'),'updated_desc');
