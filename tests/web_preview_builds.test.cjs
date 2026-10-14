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
    localStorage:{getItem:key=>store.get(key)||null,setItem:(key,val)=>store.set(key,String(val))},
  });
  const body=source.slice(0,source.indexOf('\ntranslateChrome();'));
  vm.runInContext(body+'\nglobalThis.previewTests={state,previewBuildsView,devPreviewAvailable,previewSafeLocalURL};',context);
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
