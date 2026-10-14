const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function setup(){
  const store=new Map();
  const context=vm.createContext({
    URLSearchParams,
    location:{search:'?view=preview',pathname:'/'},
    navigator:{language:'ko'},
    document:{readyState:'loading',addEventListener(){},querySelector(){return null}},
    localStorage:{getItem:key=>store.get(key)||null,setItem:(key,val)=>store.set(key,String(val))},
  });
  const body=source.slice(0,source.indexOf('\ntranslateChrome();'));
  vm.runInContext(body+'\nglobalThis.previewTests={state,previewBuildsView,devPreviewAvailable,previewSafeLocalURL,previewIdentityFromHealth};',context);
  return context.previewTests;
}
test('Preview Builds is visible only with Dev channel from version API',()=>{
 const p=setup();
 p.state.versionInfo={cli:{channel:'stable'}};
 assert.equal(p.devPreviewAvailable(),false);
 assert.match(p.previewBuildsView(),/Dev 채널에서만/);
 p.state.versionInfo={cli:{channel:'dev'}};
 assert.equal(p.devPreviewAvailable(),true);
 assert.match(p.previewBuildsView(),/Preview Builds/);
 assert.match(source,/id="previewNavBtn"/);
});
test('list shows different PR build statuses and run/open/stop controls',()=>{
 const p=setup();p.state.versionInfo={cli:{channel:'dev'}};
 p.state.previewLoaded=true;
 p.state.previewBuilds=[
  {number:207,title:'안전한 실행',head_sha:'aaaaaaaaaa',branch:'feature-a',build_status:'success'},
  {number:206,title:'알림 아웃라인',head_sha:'bbbbbbbbbb',branch:'feature-b',build_status:'success'},
  {number:205,title:'빌드 미완료',head_sha:'cccccccccc',branch:'feature-c',build_status:'in_progress'}
 ];
 p.state.previewInstances=[{pr_number:206,status:'running',port:18766,url:'http://127.0.0.1:18766/',head_sha:'bbbbbbbbbb'}];
 const html=p.previewBuildsView();
 assert.match(html,/data-preview-action="start" data-preview-number="207"/);
 assert.match(html,/data-preview-action="open" data-preview-number="206"/);
 assert.match(html,/data-preview-action="stop" data-preview-number="206"/);
 assert.match(html,/빌드 중/);
 assert.ok(html.indexOf('PR #207')<html.indexOf('PR #206'));
 p.state.previewFilter='running';
 const running=p.previewBuildsView();
 assert.doesNotMatch(running,/PR #207/);
 assert.match(running,/PR #206/);
 p.state.previewFilter='all';p.state.previewSearch='안전';
 const searched=p.previewBuildsView();
 assert.match(searched,/PR #207/);
 assert.doesNotMatch(searched,/PR #206/);
});
test('only local preview URLs may open and actions use backend routes',()=>{
 const p=setup();
 for(const url of ['http://127.0.0.1:18766/','http://127.0.0.1:18767/'])
   assert.equal(p.previewSafeLocalURL(url),url);
 for(const url of ['https://evil.example','file:///etc/passwd','http://0.0.0.0:18766/','http://127.0.0.1:18766/abc'])
   assert.equal(p.previewSafeLocalURL(url),'');
 assert.match(source,/'\/api\/preview\/'\+\(action==='stop'\?'stop':'start'\)/);
});

test('Preview identity accepts only exact PR, SHA, verified local Dev origin',()=>{
 const p=setup();
 const valid={preview_mode:true,preview_pr:214,preview_sha:'a'.repeat(40),
   preview_parent_url:'http://127.0.0.1:18765'};
 assert.equal(p.previewIdentityFromHealth(valid).number,214);
 assert.equal(p.previewIdentityFromHealth(valid).parent,'http://127.0.0.1:18765');
 assert.equal(p.previewIdentityFromHealth({...valid,preview_mode:false}),null);
 assert.equal(p.previewIdentityFromHealth({...valid,preview_sha:'abcd'}),null);
 assert.equal(p.previewIdentityFromHealth({...valid,preview_parent_url:'https://evil.example'}),null);
 assert.equal(p.previewIdentityFromHealth({...valid,preview_parent_url:'http://127.0.0.1:99999'}),null);
 p.state.previewIdentity={number:214,sha:'a'.repeat(40)};
 p.state.versionInfo={cli:{channel:'dev'}};
 assert.equal(p.devPreviewAvailable(),false,'nested child must never expose Preview manager menu');
});

function previewActionHarness(mockFetch){
 const cache=new Map();
 const context=vm.createContext({
  URLSearchParams,
  location:{search:'?view=preview',pathname:'/'},
  navigator:{language:'ko'},
  document:{readyState:'loading',addEventListener(){},querySelector(){return null}},
  localStorage:{getItem:key=>cache.get(key)||null,setItem:(key,val)=>cache.set(key,String(val))},
  fetch:mockFetch,window:{confirm(){return true}},
 });
 const body=source.slice(0,source.indexOf('\ntranslateChrome();'));
 vm.runInContext(body+`
render=()=>{};
globalThis.previewHarness={state,previewAction,refreshPreviewData,refreshPreviewInstances,previewBuildsView};
`,context);
 const h=context.previewHarness;
 h.state.versionInfo={cli:{channel:'dev'}};
 h.state.view='preview';
 h.state.language='ko';
 h.state.previewLoaded=true;
 h.state.previewBuilds=[{number:213,title:'Preview',head_sha:'a'.repeat(40),build_status:'success'}];
 h.state.previewInstances=[{pr_number:213,status:'running',port:18766,url:'http://127.0.0.1:18766/'}];
 return h;
}
function response(value){return {ok:true,json:async()=>value};}

test('successful Preview stop immediately replaces Open/Stop with Run',async()=>{
 const calls=[];
 const h=previewActionHarness(async (url,options)=>{
  calls.push({url,method:options?.method||'GET'});
  if(url==='/api/preview/stop')return response({ok:true,instance:{pr_number:213,status:'stopped',port:0}});
  if(url==='/api/preview/instances')return response({instances:[{pr_number:213,status:'stopped',port:0}]});
  throw Error('unexpected endpoint '+url);
 });
 assert.match(h.previewBuildsView(),/data-preview-action="stop" data-preview-number="213"/);
 await h.previewAction(213,'stop');
 const html=h.previewBuildsView();
 assert.match(html,/data-preview-action="start" data-preview-number="213"/);
 assert.doesNotMatch(html,/data-preview-action="open" data-preview-number="213"/);
 assert.doesNotMatch(html,/data-preview-action="stop" data-preview-number="213"/);
 assert.equal(h.state.previewInstances[0].status,'stopped');
 assert.deepEqual(calls.map(c=>c.url),['/api/preview/stop','/api/preview/instances']);
});

test('old GitHub/instance refresh cannot resurrect a stopped Preview',async()=>{
 let resolveOldInstances;
 let first=true;
 const h=previewActionHarness(async(url)=>{
  if(url==='/api/preview/builds?limit=30')return response({builds:[{number:213,title:'Preview',head_sha:'a'.repeat(40),build_status:'success'}]});
  if(url==='/api/preview/instances'){
   if(first){
    first=false;
    return new Promise(resolve=>{resolveOldInstances=resolve});
   }
   return response({instances:[{pr_number:213,status:'stopped',port:0}]});
  }
  if(url==='/api/preview/stop')return response({ok:true,instance:{pr_number:213,status:'stopped',port:0}});
  throw Error('unexpected '+url);
 });
 const slowRefresh=h.refreshPreviewData();
 assert.equal(typeof resolveOldInstances,'function','the original request must have started');
 await h.previewAction(213,'stop');
 resolveOldInstances(response({instances:[{pr_number:213,status:'running',port:18766}]}));
 await slowRefresh;
 assert.equal(h.state.previewInstances[0].status,'stopped','stale request must not resurrect running');
 assert.match(h.previewBuildsView(),/data-preview-action="start" data-preview-number="213"/);
});

test('polling local instances reflects a stop from another tab',async()=>{
 const h=previewActionHarness(async url=>{
  assert.equal(url,'/api/preview/instances');
  return response({instances:[{pr_number:213,status:'stopped',port:0}]});
 });
 await h.refreshPreviewInstances();
 assert.equal(h.state.previewInstances[0].status,'stopped');
 assert.match(h.previewBuildsView(),/data-preview-action="start" data-preview-number="213"/);
 assert.match(source,/if\(state.view==='preview'&&document.visibilityState!=='hidden'\)/);
});
