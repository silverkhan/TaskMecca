const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const {test}=require('node:test');

const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function setup(){
  const store=new Map();
  const ctx=vm.createContext({
    URLSearchParams,
    location:{search:'?view=preview',pathname:'/',reload(){}},
    navigator:{language:'ko'},
    document:{readyState:'loading',addEventListener(){},querySelector(){return null}},
    localStorage:{getItem:key=>store.get(key)||null,setItem:(key,val)=>store.set(key,String(val))}
  });
  const body=source.slice(0,source.indexOf('\ntranslateChrome();'));
  vm.runInContext(body+'\nglobalThis.harness={state,previewBuildsView,devPreviewAvailable,previewIdentityFromHealth};',ctx);
  return ctx.harness;
}

test('PR environments accessible only from Dev and not an active PR',()=>{
  const h=setup();
  h.state.versionInfo={cli:{channel:'stable'}};
  assert.equal(h.devPreviewAvailable(),false);
  assert.match(h.previewBuildsView(),/Dev에서만/);
  h.state.versionInfo={cli:{channel:'dev'}};
  assert.equal(h.devPreviewAvailable(),true);
  assert.match(h.previewBuildsView(),/PR 실행 환경/);
  h.state.executionTarget={kind:'pr',pr_number:217,head_sha:'a'.repeat(40)};
  assert.equal(h.devPreviewAvailable(),false);
  assert.match(source,/id="previewNavBtn"/);
});

test('PR list uses SHA-pinned environment switch, not child processes',()=>{
  const h=setup();
  h.state.versionInfo={cli:{channel:'dev'}};
  h.state.previewLoaded=true;
  h.state.previewBuilds=[
    {number:217,title:'환경 모델',head_sha:'a'.repeat(40),branch:'feat-a',build_status:'success'},
    {number:216,title:'legacy',head_sha:'b'.repeat(40),branch:'feat-b',build_status:'failure'},
    {number:215,title:'진행',head_sha:'c'.repeat(40),branch:'feat-c',build_status:'in_progress'}
  ];
  const html=h.previewBuildsView();
  assert.match(html,/data-pr-switch="217"/);
  assert.doesNotMatch(html,/data-preview-action="start"/);
  assert.match(html,/PR #217/);
  assert.match(html,/빌드 중/);
  assert.ok(html.indexOf('PR #217')<html.indexOf('PR #216'));
  h.state.previewFilter='success';
  assert.doesNotMatch(h.previewBuildsView(),/PR #216/);
  h.state.previewFilter='all';h.state.previewSearch='환경';
  assert.match(h.previewBuildsView(),/PR #217/);
  assert.doesNotMatch(h.previewBuildsView(),/PR #216/);
});

test('Preview child identity never exposes environment manager',()=>{
  const h=setup();
  const identity={preview_mode:true,preview_pr:214,preview_sha:'a'.repeat(40),
     preview_parent_url:'http://127.0.0.1:18765'};
  assert.equal(h.previewIdentityFromHealth(identity).number,214);
  assert.equal(h.previewIdentityFromHealth({...identity,preview_sha:'wrong'}),null);
  h.state.previewIdentity={number:214,sha:'a'.repeat(40)};
  h.state.versionInfo={cli:{channel:'dev'}};
  assert.equal(h.devPreviewAvailable(),false);
});
test('execution switch uses approved local API and health identity',()=>{
  assert.match(source,/\/api\/environment\/enter-pr/);
  assert.match(source,/\/api\/environment\/return-dev/);
  assert.match(source,/body:JSON\.stringify\(\{number,sha:item\.head_sha,approve:true\}\)/);
  assert.match(source,/body\.boot_id!==previousBoot/);
  assert.match(source,/target\.head_sha===sha/);
});

test('a Dev PR with missing or expired build offers a SHA-pinned rebuild, never runs a child Preview',()=>{
 const h=setup();
 h.state.versionInfo={cli:{channel:'dev'}};
 h.state.previewLoaded=true;
 h.state.previewBuilds=[
  {number:155,title:'Missing artifact',head_sha:'a'.repeat(40),branch:'fix',build_status:'none'},
  {number:156,title:'Expired artifact',head_sha:'b'.repeat(40),branch:'fix',build_status:'expired'},
  {number:157,title:'Unsafe fork',head_sha:'c'.repeat(40),branch:'outside',build_status:'blocked'}
 ];
 const html=h.previewBuildsView();
 assert.match(html,/data-pr-rebuild="155"/);
 assert.match(html,/data-pr-rebuild="156"/);
 assert.doesNotMatch(html,/data-pr-rebuild="157"/);
 assert.match(html,/GitHub Actions에서 직접 빌드/);
 assert.match(source,/\/api\/preview\/rebuild/);
 assert.match(source,/body:JSON\.stringify\(\{number,sha:item\.head_sha\}\)/);
 assert.doesNotMatch(html,/data-preview-action="start"/);
});
