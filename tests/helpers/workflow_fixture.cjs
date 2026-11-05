// Public synthetic workflow fixtures. No operating Web or user ledger is read.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const project='/fixtures/workflow',backlog=project+'/_task_mecca/data/backlog';
const clone=value=>JSON.parse(JSON.stringify(value));
const fixture=name=>clone(require('../fixtures/workflow/'+name+'.json').workflow);
function payload(kind='continuous'){
 const name=({history:'unmeasured-history',overlap:'overlap',zero:'zero'})[kind]||'continuous-activity';
 const raw=clone(require('../fixtures/workflow/'+name+'.json')),workflow=raw.workflow;
 workflow.control_snapshot={trust:'verified',version:'synthetic-v1',computed_at:workflow.computed_at,validated_at:workflow.computed_at,handoff_checked_at:workflow.computed_at};
 if(kind==='stale')workflow.control_snapshot.trust='stale';
 if(kind==='unknown'){workflow.rows[0].round=null;workflow.rows[0].round_status='unclassified';workflow.current.round=null;workflow.current.round_status='unclassified';workflow.rounds=[];}
 const events=raw.events||[];if(events.length)events[0].evidence='Synthetic evidence <img onerror=alert(1)>';
 return {task_id:'FLOW-1',workflow,events,summary:raw.summary,review_report:{},lifecycle_coverage:workflow.lifecycle};
}
function task(data=payload()){
 const wf=data.workflow,events=[{event_id:'synthetic-register',state:'todo',label:'Registered',kind:'registered',at:wf.axis.start,actor:'/root/registrar',source:'fixture'}];
 for(const [i,at] of (wf.lifecycle.completion_history||[]).entries())events.push({event_id:'synthetic-complete-'+i,state:'done',label:'Completed',kind:'completed',at,actor:'/root/controller',source:'fixture'});
 if(wf.axis.open){const starts=wf.rows.flatMap(r=>r.intervals).filter(i=>i.kind==='execution_report').sort((a,b)=>Date.parse(a.start)-Date.parse(b.start));const at=starts.at(-1)?.start||wf.axis.start;events.push({event_id:'synthetic-resume',state:'doing',label:'Resumed',kind:'resumed',at,actor:wf.current.assigned_worker||'worker',source:'fixture'});}
 else if(!events.some(e=>e.kind==='completed'&&e.at===wf.axis.end))events.push({event_id:'synthetic-complete',state:'done',label:'Completed',kind:'completed',at:wf.axis.end,actor:'/root/controller',source:'fixture'});

 return {id:'FLOW-1',title:'작업 기록과 검토 흐름 확인',state:data.workflow.axis.open?'doing':'done',file_state:data.workflow.axis.open?'doing':'done',agent:'/root/controller/pairi',location:'active',fields:{},tags:[],depends_on:[],related:[],raw_markdown:'# Synthetic workflow fixture\n'+Array.from({length:30},(_,i)=>'## 기록 '+(i+1)+'\n본문과 읽기 위치를 보존하는 가상 자료입니다.').join('\n'),activity:{health:'healthy'},lifecycle:{events,current_state:'done',lead_seconds:(data.workflow.axis.elapsed_ms||0)/1000},snapshot:{trust:'verified',version:'fixture-v1'},document:{schema:'defined-v2',summary:{purpose:'실제 사건과 측정 누락을 구분해 읽는다.',change:'역할별 전체 시간축',status_result:'합성 검증 자료',follow_up:'실제 운영 데이터가 아닙니다.'},notes:'가상 검증 기록입니다.',result:'가상 결과입니다.',verification:'브라우저 fixture 검증',requirements:{goal:'기록과 전체 시간축을 확인한다.',acceptance:'- [ ] 근거와 시간을 구별한다.'}}};
}
async function start(root){
 const state={data:payload(),activityStatus:200,requests:[]};
 const server=http.createServer((req,res)=>{
  const url=new URL(req.url,'http://127.0.0.1');state.requests.push(url.pathname+url.search);
  if(url.pathname.startsWith('/api/')){
   res.setHeader('Content-Type','application/json');const p=url.pathname,d=task(state.data),counts={all:1,done:1,working:0,hold:0,ready:0};let value={};
   if(p==='/api/activity'){res.statusCode=state.activityStatus;value=state.activityStatus===200?state.data:{error:'synthetic refresh error'};}
   else if(p.startsWith('/api/tasks/'))value=d;
   else if(p==='/api/health')value={ok:true,version:'synthetic-fixture'};
   else if(p==='/api/hub'||p.includes('/projects'))value={projects:[{project,name:'Workflow fixture',initialized:true,monitoring:true,status:'active',backlog_path:backlog,counts}]};
   else if(p==='/api/backlog/tasks')value={items:[d],page:1,pages:1,page_size:20,total:1,counts,selection:{selected:backlog,candidates:[]},content_revision:'fixture-v1',signal_revision:'fixture-s1',backlog_presence:{present:true}};
   else if(p==='/api/snapshot'||p==='/api/workload')value={all_items:{'FLOW-1':d},counts,workload:{agents:[],unassigned_doing:[],released_holds:[]},access:{status:'full',full_access_confirmed:true},attention:[],notification_events:[],issues:[]};
   else if(p==='/api/revision')value={revision:'fixture-v1',signal_revision:'fixture-s1',states:[d]};
   else if(p.includes('version'))value={cli:{version:'fixture',channel:'dev'},web:{version:'fixture'},channel:'dev',update_available:false};
   else if(p.includes('/attention'))value={attention:[],all_items:{'FLOW-1':d},notification_events:[],controller_reviews:[]};
   else if(p.includes('/notifications'))value={enabled:false,configured:false,active:[],history:[],events:[],items:[],counts:{},capabilities:{},telegram:{configured:false}};
   else if(p.includes('/operations'))value={active:[],history:[],projects:[]};
   res.end(JSON.stringify(value));return;
  }
  let name=decodeURIComponent(url.pathname);if(name==='/'||name.startsWith('/tasks/'))name='/index.html';const file=path.resolve(root,'.'+name);if(!file.startsWith(path.resolve(root)+path.sep)){res.writeHead(403);res.end();return;}
  fs.readFile(file,(err,bytes)=>{if(err){res.writeHead(404);res.end();return;}res.setHeader('Content-Type',({'.css':'text/css','.js':'text/javascript','.svg':'image/svg+xml','.html':'text/html','.woff2':'font/woff2'})[path.extname(file)]||'application/octet-stream');res.setHeader('Cache-Control','no-store');
   res.end(bytes);
  });
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 return {origin:'http://127.0.0.1:'+server.address().port,state,close:()=>new Promise(resolve=>server.close(resolve))};
}
module.exports={project,backlog,payload,task,start};
