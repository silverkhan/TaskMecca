// Actual embedded application assets; every API response and visible record is synthetic.
const http=require('node:http'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const project='/fixtures/atlas',backlog=project+'/_task_mecca/data/backlog';
const stamp='2026-11-04T10:00:00Z';
const items=Array.from({length:36},(_,i)=>({id:'DEMO-'+(100+i),sort_key:String(100+i).padStart(6,'0'),title:['월간 결산 자료 검증','고객 요청 처리 현황','배치 작업과 의존성 확인'][i%3]+' · '+(i+1),state:['doing','ready','hold','done'][i%4],file_state:['doing','todo','hold','done'][i%4],agent:i%4===0?'worker-pine':'',scope:'가상 검증 데이터',tags:['area:backend'],updated_at:stamp,mtime:stamp,document:{schema:'v2',contract_kind:'simple',summary:{purpose:'운영 화면의 상태와 가독성을 확인하는 가상 자료',change:'테마·레이아웃 검증',status_result:'검증용 fixture',follow_up:'실제 업무 데이터가 아닙니다.'},requirements:{goal:'가상 작업의 처리 흐름 확인',acceptance:'- [ ] 결과와 근거를 확인한다.'}},fields:{},lifecycle:{events:[],active_seconds:128,lead_seconds:600}}));
const detail={...items[0],content_only:false,activity:{health:'healthy'},raw_markdown:'# DEMO-100 가상 결산 자료 검증\n\n실제 업무와 무관한 브라우저 검증 데이터입니다.\n\n## 작업 정의\n운영 화면의 읽기와 상태 전환을 확인합니다.\n\n| 대상 | 상태 | 근거 |\n| --- | --- | --- |\n'+Array.from({length:45},(_,i)=>'| 가상 항목 '+(i+1)+' | 검증 중 | fixture-only |').join('\n')+'\n\n```text\nworker-pine  completed  00:03:12\nqueue        ready      36 items\n```',document:{...items[0].document,notes:'가상 검증 로그\n\n```text\n[10:03] fixture row checked\n[10:04] projection refreshed\n```',result:'대조 준비 완료',verification:'가상 자료만 사용합니다.'},lifecycle:{events:[{label:'Registered',at:stamp,event_id:'demo-registration',state:'todo',provisional:false},{label:'Started',at:stamp,event_id:'demo-start',state:'doing',provisional:false}],current_state:'doing',current_state_at:stamp,active_seconds:128,lead_seconds:600}};
const list={items,page:1,pages:3,page_size:36,total:108,counts:{all:108,working:18,ready:36,hold:9,done:45},selection:{selected:backlog,candidates:[]},content_revision:'fixture-v1',signal_revision:'fixture-s1',backlog_presence:{present:true}};
const snapshot={all_items:Object.fromEntries(items.map(x=>[x.id,x])),counts:list.counts,access:{status:'full',full_access_confirmed:true},workload:{agents:[{agent:'/fixture/controller/pine',doing:[items[0].id],change_scopes:{[items[0].id]:'가상 결산 데이터 검증'},downstream_blocking:2,ready_continuity:3},{agent:'/fixture/controller/cedar',doing:[items[4].id],change_scopes:{[items[4].id]:'가상 보고서 작성'}}],unassigned_doing:[],released_holds:[]},runtime_observability:{attempts:[],root_sessions:[],hooks:[],counts:{active:2},findings:[]},attention:[],notification_events:[],issues:[]};
const hub={projects:[{path:project,name:'Atlas · 가상 운영 프로젝트',status:'active',initialized:true,monitoring:true,counts:list.counts,backlog_path:backlog,access:{status:'full'}}]};
const activity={tasks_observed:36,tasks_with_measured_time:24,agent_time_ms:5400000,sum_task_observed_wall_ms:3600000,median_agent_ms:180000,p90_agent_ms:600000,by_type_ms:{implementation:3600000,verification:1800000},top_tasks:[],events:[{at:stamp,type:'verification',role:'worker',agent_id:'fixture-pine',round:0,evidence:'가상 검증 로그'}],summary:{agent_time_ms:5400000,segments:[{type:'implementation',role:'worker',agent_id:'fixture-pine',attempt_id:'fixture-run-pine',round:0,started_at:'2026-11-04T09:00:00Z',ended_at:'2026-11-04T10:00:00Z',duration_ms:3600000},{type:'verification',role:'worker',agent_id:'fixture-pine',attempt_id:'fixture-run-pine',round:0,started_at:'2026-11-04T10:00:00Z',ended_at:'2026-11-04T10:30:00Z',duration_ms:1800000}]}};
snapshot.runtime_observability={attempts:[{attempt_id:'fixture-run-pine',provider:'codex',runtime_agent_id:'fixture-pine',agent_path:'/fixture/controller/pine',task_id:items[0].id,current_state:'waiting_user',binding_state:'bound',started_at:'2026-11-04T09:00:00Z',last_activity_at:stamp,elapsed_ms:3600000,active_time_available:true,observed_active_ms:3300000,recent_transitions:[{at:'2026-11-04T09:00:00Z',state:'running',evidence_source:'fixture',observation_quality:'synthetic'},{at:stamp,state:'waiting_user',evidence_source:'fixture',observation_quality:'synthetic'}]}],root_sessions:[{root_session_id:'fixture-root',provider_session_id:'fixture-session',display_name:'가상 검증 Root',provider:'codex',status:'active',created_at:'2026-11-04T08:00:00Z',last_activity_at:stamp,agent_count:1,current_count:1}],attempt_root_sessions:{'fixture-run-pine':'fixture-root'},root_session_counts:{active:1},hooks:[],counts:{active:1},findings:[]};
function api(url,origin){
 const p=url.pathname;
 if(p==='/api/health')return{ok:true,preview_mode:true,preview_pr:348,preview_sha:process.env.GLASS_FIXTURE_SHA||'0000000000000000000000000000000000000348',preview_parent_url:origin,version:'fixture-preview'};
 if(p.includes('version'))return{cli:{version:'fixture',channel:'dev'},web:{version:'fixture'},channel:'dev',update_available:false};
 if(p==='/api/hub')return hub;
 if(p==='/api/backlog/tasks')return list;
 if(p==='/api/snapshot'||p==='/api/workload')return snapshot;
 if(p==='/api/revision')return{revision:'fixture-v1',signal_revision:'fixture-s1',states:items};
 if(p.startsWith('/api/tasks/'))return detail;
 if(p==='/api/storage/logs')return{total_bytes:3145728,reclaimable_bytes:524288,items:[{id:'web-service',label:'가상 Web 로그',size_bytes:1048576,can_clear:true,scope:'global'},{id:'notification-events',label:'가상 알림 기록',size_bytes:2097152,can_clear:false,scope:'project'}]};
 if(p==='/api/runtime/storage')return{total_bytes:4194304,raw:{bytes:2097152,files:12},history:{bytes:2097152,files:3},legacy:{bytes:0,files:0},cleanup:{reclaimable_bytes:1048576,candidate_files:4},retention:{raw_days:7,history_days:90,history_max_attempts:2000},raw_protection:{files:[]}};
 if(p==='/api/terminal/settings')return{access_allowed:true,connection:'local',remote_enabled:false,project,project_name:'Atlas · 가상 프로젝트',cwd:project};
 if(p.includes('/terminal/'))return{ok:true,allowed:true,local:true,remote:false,project,project_name:'Atlas · 가상 프로젝트',shell:'fixture shell',cwd:'/fixtures/atlas',session_id:'fixture-session',running:false};
 if(p.includes('/attention'))return{attention:[],all_items:snapshot.all_items,notification_events:[],controller_reviews:[],content_revision:'fixture-v1'};
 if(p.includes('/activity'))return activity;
 if(p.includes('/notifications'))return{enabled:false,configured:false,active:[],history:[],events:[],items:[],counts:{},projects:hub.projects,capabilities:{},telegram:{configured:false}};
 if(p.includes('/operations'))return{active:[],history:[],projects:hub.projects};
 if(p.includes('/projects'))return hub;
 return {ok:true,items:[],events:[],findings:[],issues:[],projects:[],counts:{}};
}
async function start(root,port=0){
 if(Number(port)===18765)throw Error('Operational port is prohibited');
 const server=http.createServer((req,res)=>{
  const origin='http://127.0.0.1:'+server.address().port,url=new URL(req.url,origin);
  if(url.pathname.startsWith('/api/')){res.setHeader('Content-Type','application/json');res.end(JSON.stringify(api(url,origin)));return;}
  let name=decodeURIComponent(url.pathname);if(name==='/'||name.startsWith('/tasks/'))name='/index.html';if(name==='/terminal')name='/terminal.html';
  const file=path.resolve(root,'.'+name);if(!file.startsWith(path.resolve(root)+path.sep)){res.writeHead(403);res.end();return;}
  const type={'.css':'text/css','.js':'text/javascript','.svg':'image/svg+xml','.webp':'image/webp','.html':'text/html','.json':'application/json','.webmanifest':'application/json'}[path.extname(file)]||'application/octet-stream';
  fs.readFile(file,(err,data)=>{if(err){res.writeHead(404);res.end();return;}res.setHeader('Content-Type',type);res.setHeader('Cache-Control','no-store');if(name.includes('cozy-beige-room-photo-')){const hash=crypto.createHash('sha256').update(data).digest('hex');res.setHeader('ETag','\"'+hash+'\"');if(url.searchParams.get('v')===hash.slice(0,8))res.setHeader('Cache-Control','public, max-age=31536000, immutable');res.setHeader('Content-Length',data.length)}res.end(data);});
 });
 await new Promise(r=>server.listen(port,'127.0.0.1',r));return{origin:'http://127.0.0.1:'+server.address().port,close:()=>new Promise(r=>server.close(r))};
}
module.exports={start,project,backlog,items,detail,list,snapshot,hub,stamp,activity,api};

if(require.main===module){start(path.resolve(process.argv[2]||'goassets/template/_task_mecca/framework/web'),Number(process.env.GLASS_FIXTURE_PORT||0)).then(s=>{console.log(JSON.stringify({origin:s.origin,project,backlog,sha:process.env.GLASS_FIXTURE_SHA||'0000000000000000000000000000000000000348',synthetic:true}));for(const sig of ['SIGINT','SIGTERM'])process.on(sig,async()=>{await s.close();process.exit(0)})})}
