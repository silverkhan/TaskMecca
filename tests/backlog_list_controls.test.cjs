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


// AID-186: the auto page sizer must not observe its own content height and
// turn a page-size change into repeated refresh / scroll / width oscillation.
{
 const start=source.indexOf('let autoPageMeasureRaf = 0;');
 const end=source.indexOf('function applySidebarState()',start);
 assert(start>=0&&end>start,'auto list layout controller must be present');
 let rowHeight=80,refreshCount=0,rafID=0;
 const observed=[],frameQueue=[];
 let layoutCallback=null;
 const fakeNodes={
  '.task-list':{getBoundingClientRect:()=>({top:330})},
  '.shortcut-bar':{getBoundingClientRect:()=>({height:30})},
  '.topbar':{selector:'.topbar'},
  '#userAttention':{selector:'#userAttention'},
  '.main':{selector:'.main'},
  '#content':{selector:'#content'},
 };
 class FakeResizeObserver {
  constructor(fn){layoutCallback=fn;}
  observe(node){observed.push(node.selector);}
 }
 const listState={
  project:'/project',backlog:'/backlog',view:'backlog',detail:null,
  listPageMode:'auto',autoListPageSize:8,listPage:1,selectedIndex:1,
  listData:{items:[{id:'A-1'}],pending_reconciliation:true},
  listRevalidating:true,pendingContentUpdate:false,
  statusFilters:['all'],tagFilters:[],query:'',listSort:'id_desc'
 };
 const layoutContext=vm.createContext({
  state:listState,window:{scrollY:0,innerHeight:880,innerWidth:1100},
  document:{readyState:'complete',querySelector:selector=>fakeNodes[selector]||null,
    querySelectorAll:selector=>selector==='.task-list .task-row'
      ?Array.from({length:5},()=>({getBoundingClientRect:()=>({height:rowHeight})})) : []},
  ResizeObserver:FakeResizeObserver,
  requestAnimationFrame:fn=>{frameQueue.push(fn);return ++rafID;},
  cancelAnimationFrame:()=>{},
  refreshList:()=>{refreshCount++;},
 });
 vm.runInContext(source.slice(start,end),layoutContext);
 assert.deepEqual(observed,['.topbar','#userAttention'],
  'list container and main may NOT be observed: they resize when page_size changes');
 const runFrames=()=>{
  let turns=0;
  while(frameQueue.length){
   assert(++turns<10,'auto sizer re-entered a layout feedback loop');
   frameQueue.shift()();
  }
 };
 // Multiple initial file snapshots and toolbar notifications cannot resize
 // pagination while the authoritative control-tower response is pending.
 for(let i=0;i<3;i++){
  layoutCallback();runFrames();
 }
 assert.equal(refreshCount,0,'provisional view must not trigger a second list request');
 assert.equal(listState.autoListPageSize,8);
 listState.listData.pending_reconciliation=false;
 listState.listRevalidating=false;
 vm.runInContext('scheduleAutoListPageSize(true)',layoutContext);runFrames();
 assert.equal(listState.autoListPageSize,6);
 assert.equal(refreshCount,1,'first stable canonical list may resize once');
 rowHeight=55;
 for(let i=0;i<3;i++){
  vm.runInContext('scheduleAutoListPageSize(true)',layoutContext);runFrames();
 }
 assert.equal(listState.autoListPageSize,6,'shorter replacement rows cannot grow the page-size again');
 assert.equal(refreshCount,1,'same viewport must not oscillate between page sizes');
 rowHeight=105;
 vm.runInContext('scheduleAutoListPageSize(true)',layoutContext);runFrames();
 assert.equal(listState.autoListPageSize,4);
 assert.equal(refreshCount,2,'taller rows can conservatively reduce the page size once');
 rowHeight=55;
 vm.runInContext('scheduleAutoListPageSize(true)',layoutContext);runFrames();
 assert.equal(listState.autoListPageSize,4,'same context retains the safer measured row-height budget');
 assert.equal(refreshCount,2);
 // A real window resize, unlike a list-height change, may recalculate capacity.
 layoutContext.window.innerWidth=920;
 vm.runInContext('scheduleAutoListPageSize(true)',layoutContext);runFrames();
 assert.equal(listState.autoListPageSize,9);
 assert.equal(refreshCount,3);
 assert.match(source,/if\(state\.view==='backlog'\)scheduleAutoListPageSize\(true\)/,
  'canonical render must request a measurement after its rows are painted');
 assert.match(fs.readFileSync(path.join(root,'goassets/template/_task_mecca/framework/web/style.css'),'utf8'),/html\{scrollbar-gutter:stable\}/,
  'content width must stay stable when the scrollbar appears or disappears');
 console.log('AID-186: provisional render, ResizeObserver feedback isolation, stable row budget, and viewport resize PASS');
}
