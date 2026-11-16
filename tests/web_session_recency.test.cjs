const assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {test}=require('node:test');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
function app(){
 const c=vm.createContext({URLSearchParams,location:{search:'?project=/backlog'},navigator:{language:'en'},document:{querySelector:()=>null,querySelectorAll:()=>[]},localStorage:{getItem:()=>null,setItem(){}},requestAnimationFrame:fn=>fn(),queueMicrotask});
 vm.runInContext(source.slice(0,source.indexOf('\ntranslateChrome();'))+'\nglobalThis.api={state,workloadSessionPanel,workloadSessionOpen,workloadSessionKey,loadWorkloadSessions,loadWorkloadSessionDetail,selectWorkloadSessionProject};render=()=>{};preserveViewportAndFocus=fn=>fn();',c);c.api.state.hub={projects:[]};return {c,...c.api};
}
function root(id,group='active'){return {root_session_id:id,provider_session_id:id,display_name:'Fixture '+id,provider:'codex',status:'terminal',usage_group:group,last_used_at:'2026-11-16T00:00:00Z',usage_source:'observed_hook_activity'};}
function payload(project='/sessions',latest='a'){return {project,active:{items:[root('a'),root('b')],page:1,total_pages:3,total:25},historical:{items:[root('old','historical')],page:1,total_pages:2000,total:20000},latest_session_id:latest,unknown_usage:1};}
function deferred(){let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve};}
function response(body){return {ok:true,json:async()=>body};}
test('scope independent in both directions; historical and all its sessions default closed; deterministic latest only',()=>{
 const a=app();a.state.project='/backlog';a.state.hub={projects:[{path:'/sessions',name:'Sessions'},{path:'/other',name:'Other'}]};a.state.workloadSessionProject='/sessions';a.state.workloadSessions=payload();
 let html=a.workloadSessionPanel();assert.match(html,/Session project/);assert.match(html,/Current session scope: \/sessions/);assert.equal((html.match(/data-runtime-root-key=/g)||[]).length,2);assert.doesNotMatch(html,/id="workloadHistorical" open/);assert.equal(a.workloadSessionOpen(root('a')),true);assert.equal(a.workloadSessionOpen(root('b')),false);assert.equal(a.workloadSessionOpen(root('old','historical')),false);
 a.state.project='/other';assert.equal(a.state.workloadSessionProject,'/sessions');a.state.workloadSessionProject='/other';assert.equal(a.state.project,'/other');
});
test('deliberate toggles survive newest replacement, migration, refresh and project switches',()=>{
 const a=app();a.state.workloadSessionProject='/sessions';a.state.workloadSessions=payload();a.state.workloadSessionDisclosure[a.workloadSessionKey('a')]=false;a.state.workloadSessionDisclosure[a.workloadSessionKey('old')]=true;
 a.state.workloadSessions=payload('/sessions','old');assert.equal(a.workloadSessionOpen(root('a')),false);assert.equal(a.workloadSessionOpen(root('old')),true);assert.equal(a.workloadSessionOpen(root('b')),false);
 a.state.workloadSessionProject='/other';a.state.workloadSessions=payload('/other','a');assert.equal(a.workloadSessionOpen(root('a')),true);a.state.workloadSessionProject='/sessions';a.state.workloadSessions=payload('/sessions','old');assert.equal(a.workloadSessionOpen(root('a')),false);assert.equal(a.workloadSessionOpen(root('old')),true);
});
test('full counts/page controls remain reachable with bounded rendered sessions in KO/EN',()=>{
 const a=app();a.state.workloadSessionProject='/sessions';a.state.workloadSessions=payload();
 for(const language of ['ko','en']){a.state.language=language;a.state.workloadHistoricalDisclosure['/sessions']=true;const html=a.workloadSessionPanel();assert.equal((html.match(/data-runtime-root-key=/g)||[]).length,3);assert.match(html,/20000/);assert.match(html,/data-session-page="historical"/);assert.match(html,/data-session-page="active"/);assert.ok(html.length<30000);assert.match(html,language==='ko'?/활성 세션/:/Active sessions/);assert.match(html,language==='ko'?/48시간/:/48 hours/);}
});
test('late project list, detail and errors cannot cross accepted scope; latest refresh wins',async()=>{
 const a=app(),requests=[];a.c.fetch=(url)=>{const d=deferred();requests.push({url,...d});return d.promise;};a.state.workloadSessionProject='/a';const old=a.loadWorkloadSessions();a.state.workloadSessionProject='/b';a.state.workloadSessionEpoch++;const current=a.loadWorkloadSessions();
 requests[1].resolve(response({...payload('/b'),active:{items:[],total:0},historical:{items:[],total:0}}));await current;
 requests[0].resolve(response(payload('/a')));await old;assert.equal(a.state.workloadSessions.project,'/b');assert.equal(a.state.project,'/backlog');
 const detail=a.loadWorkloadSessionDetail('a');a.state.workloadSessionProject='/c';a.state.workloadSessionEpoch++;a.state.workloadSessionDetails={};requests[2].resolve(response({project:'/b',root_session:root('a'),attempts:[{attempt_id:'leak'}]}));await detail;assert.equal(Object.keys(a.state.workloadSessionDetails).length,0);
 a.state.workloadSessionProject='/d';const first=a.loadWorkloadSessions(),second=a.loadWorkloadSessions();requests[4].resolve(response({project:'/d',active:{items:[],total:0},historical:{items:[],total:0},marker:'new'}));await second;requests[3].resolve({ok:false,json:async()=>({error:'old error'})});await first;assert.equal(a.state.workloadSessions.marker,'new');assert.equal(a.state.workloadSessionsError,'');
});
test('distinct no-project, no-session, loading and error states; escape supplied names',()=>{
 const a=app();a.state.hub=null;assert.match(a.workloadSessionPanel(),/Loading sessions/);a.state.workloadSessionProjectsError='inventory failed';assert.match(a.workloadSessionPanel(),/Registered projects could not be loaded/);a.state.workloadSessionProjectsError='';a.state.hub={projects:[]};assert.match(a.workloadSessionPanel(),/No registered projects/);a.state.workloadSessionProject='/p';assert.match(a.workloadSessionPanel(),/Loading sessions/);a.state.workloadSessions={active:{total:0},historical:{total:0}};assert.match(a.workloadSessionPanel(),/No recorded sessions/);a.state.workloadSessionsError='failure';assert.match(a.workloadSessionPanel(),/could not be loaded/);
 a.state.workloadSessionsError='';a.state.workloadSessions=payload('/p');a.state.workloadSessions.active.items[0].display_name='<script>bad</script>';assert.match(a.workloadSessionPanel(),/&lt;script&gt;bad/);
});
module.exports={app,root,payload};

test('an explicit empty project choice stays empty and never fetches another project',async()=>{
 const a=app();a.state.hub={projects:[{path:'/first',name:'First'}]};a.state.project='/backlog';a.c.fetch=()=>{throw Error('unexpected project request');};
 await a.selectWorkloadSessionProject('');assert.equal(a.state.workloadSessionProject,'');assert.equal(a.state.project,'/backlog');assert.equal(a.state.workloadSessions,null);assert.match(a.workloadSessionPanel(),/Select a project/);
 await a.loadWorkloadSessions();assert.equal(a.state.workloadSessionProject,'');
});
