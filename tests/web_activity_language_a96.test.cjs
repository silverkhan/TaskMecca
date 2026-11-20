// Runs the shipped app.js renderer in the existing offline B3 VM; no browser/server/network.
const {test}=require('node:test'),assert=require('node:assert/strict');
const {load,fixture}=require('./web_activity_brief_b3.test.cjs');
const escape=s=>s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;').replace(/'/g,'&#39;');
test('A96 actual shared renderer switches KO/EN fixed source values without promoting evidence',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 for(const code of ['claimed_handoff','t3_structured']){
  const marker={kind:'fact',value:{id:'fixed-source',kind:'handoff_claimed',at:p.workflow.axis.end,source:code,reason:code,attempt_id:'exact-native/T3/attempt',assignment_id:'exact-assignment'}};
  const raw=JSON.stringify(marker);
  for(const locale of ['ko','en','ko']){a.state.language=locale;const html=a.workflowFloatingContent(row,null,marker,locale==='ko');
   assert.ok(html.includes(locale==='ko'?(code==='claimed_handoff'?'인계 수신이 보고되었습니다.':'구조화된 T3 관측 근거입니다.'):(code==='claimed_handoff'?'A handoff claim was reported':'Structured T3 observation')));
   assert.ok(html.includes(locale==='ko'?(code==='claimed_handoff'?'검수 수용이나 테스트 통과의 근거는 아닙니다.':'테스트 통과나 독립 검수 수용을 뜻하지 않습니다.'):'does not establish'));
   assert.ok(html.includes('exact-native/T3/attempt'));assert.ok(html.includes(code),'raw exact code retained');
   assert.equal(JSON.stringify(marker),raw,'locale switch cannot mutate source');
  }
 }
});
test('A96 authored KO/EN, missing, long, hostile and free evidence remain exact and escaped',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 for(const [language,text] of [['ko','변경 대상의 검증을 수행합니다.'],['en','Performing targeted validation.'],['ko','<img src=x onerror="boom"> & /tmp/exact '+ '긴 문장 '.repeat(500)]]){
  const interval={...row.intervals[0],explanation:text,explanation_language:language,evidence:'Fresh English gate independently verified /raw/path native:T3',source:'unknown_source'};
  const before=JSON.stringify(interval);
  for(const locale of ['ko','en','ko']){a.state.language=locale;const html=a.workflowFloatingContent(row,interval,null,locale==='ko');
   assert.equal(a.workflowBrief(interval,interval.kind,locale==='ko').text,text);
   assert.ok(html.includes(escape(text)));assert.ok(html.includes(interval.evidence));assert.ok(html.includes('unknown_source'));
   assert.doesNotMatch(html,/<img|<script/);assert.equal(JSON.stringify(interval),before);
  }
 }
 for(const locale of ['ko','en']){a.state.language=locale;const interval={...row.intervals[0],type:'implementation',explanation:'',evidence:'unknown free prose says test passed'};
  const before=JSON.stringify(interval);const brief=a.workflowBrief(interval,interval.kind,locale==='ko');
  assert.ok(brief.text.startsWith(locale==='ko'?'작업 설명 미기록':'Work description not recorded'));assert.ok(brief.text.includes(locale==='ko'?'구현':'Implementation'));
  assert.ok(!brief.text.includes('test passed'),'free prose never classified');assert.equal(brief.evidence,interval.evidence);
  assert.ok(a.workflowFloatingContent(row,interval,null,locale==='ko').includes(interval.evidence));assert.equal(JSON.stringify(interval),before);
 }
});
test('A96 full shipped detail retains authored explanation and raw code across locale changes',()=>{
 const a=load(),p=fixture(),i=p.workflow.rows[0].intervals[0];Object.assign(i,{explanation:'작업 변경 사항을 구현합니다.',explanation_language:'ko',evidence:'claimed_handoff'});
 const raw=JSON.stringify(p),geometry=JSON.stringify(a.workflowActivityLayout(p.workflow.rows[0],p.workflow.axis.end),(_,v)=>typeof v==='bigint'?String(v):v);
 for(const locale of ['ko','en','ko']){a.state.language=locale;const html=a.activityMetricsBody(p,'FLOW-1');assert.ok(html.includes('claimed_handoff'));assert.ok(html.includes('explanation_language'));assert.ok(html.includes(i.explanation));assert.equal(JSON.stringify(p),raw);assert.equal(JSON.stringify(a.workflowActivityLayout(p.workflow.rows[0],p.workflow.axis.end),(_,v)=>typeof v==='bigint'?String(v):v),geometry);}
});

test('A96 actual CLI output is consumed unchanged by the shipped renderer', {skip:!process.env.A96_FIXTURE_OUTPUT},()=>{
 const fs=require('node:fs'),path=require('node:path'),a=load();
 for(const name of ['preferred-ko','preferred-en','conversation-fallback','unsupported-fallback','legacy']){
  const bytes=fs.readFileSync(path.join(process.env.A96_FIXTURE_OUTPUT,name+'.json'),'utf8'),p=JSON.parse(bytes),before=JSON.stringify(p),row=p.workflow.rows[0];
  const generated=row.intervals.find(i=>i.id==='corrected');assert.ok(generated,'actual corrected CLI source interval');
  const expected=name==='preferred-en'||name==='unsupported-fallback'?'Implementing the task changes.':name==='legacy'?undefined:'작업 변경 사항을 구현합니다.';
  assert.equal(generated.explanation,expected);
  for(const locale of ['ko','en','ko']){a.state.language=locale;const html=a.workflowFloatingContent(row,generated,null,locale==='ko');assert.ok(html.includes(expected||(locale==='ko'?'작업 설명 미기록':'Work description not recorded')));assert.ok(html.includes('raw &lt;/pre&gt;&lt;script&gt; exact /path native:T3'));assert.equal(JSON.stringify(p),before);}
  assert.equal(fs.readFileSync(path.join(process.env.A96_FIXTURE_OUTPUT,name+'.json'),'utf8'),bytes);
 }
});

test('A96 unknown special evidence names stay verbatim rather than inheriting dictionary properties',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 for(const locale of ['ko','en'])for(const evidence of ['constructor','__proto__','toString','unknown <source> & /path']){
  a.state.language=locale;const interval={...row.intervals[0],explanation:'',evidence};const brief=a.workflowBrief(interval,interval.kind,locale==='ko');
  assert.equal(brief.evidence,evidence);assert.ok(a.workflowFloatingContent(row,interval,null,locale==='ko').includes(escape(evidence)));
 }
});

test('A96 repair1 free evidence and reasons appear only inside closed raw details across locales',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 const free='Fresh ordinal3 strict gate independently verified /raw/path native:T3';
 const reason='Original free reason <source> & "exact"';
 const cases=[
  {interval:{...row.intervals[0],explanation:'',evidence:free,reason}},
  {interval:{...row.intervals[0],explanation:'사람이 작성한 설명 / authored description',evidence:free,reason}},
  {marker:{kind:'fact',value:{kind:'assigned',at:p.workflow.axis.end,source:'claimed_handoff',reason:free,evidence:[{source:'t3_structured',reason}]}}},
  {marker:{kind:'gap',value:{start:row.intervals[0].start,end:row.intervals[0].end,status:'unmeasured',source:'t3_structured',reason:free,evidence:[{source:'claimed_handoff',reason}]}}},
 ];
 for(const sample of cases){const value=sample.interval||sample.marker.value,before=JSON.stringify(value);
  for(const locale of ['ko','en','ko']){a.state.language=locale;const ko=locale==='ko',html=a.workflowFloatingContent(row,sample.interval||null,sample.marker||null,ko),firstDetails=html.indexOf('<details');
   assert.ok(firstDetails>=0);const primary=html.slice(0,firstDetails),raw=html.slice(firstDetails);
   assert.ok(!primary.includes(free),'free prose hidden from visible surface');assert.ok(!primary.includes(escape(reason)),'free reasons hidden from visible surface');
   assert.match(raw,/^<details class="operation-raw-fields">/,'raw disclosure starts closed');
   assert.ok(raw.includes(free));assert.ok(raw.includes('&lt;source&gt; &amp; \\&quot;exact\\&quot;'),'raw JSON keeps escaped exact reason');
   if(sample.interval?.explanation)assert.ok(primary.includes(sample.interval.explanation),'authored description remains visible');
   assert.equal(JSON.stringify(value),before,'locale switch never changes raw object');
   const brief=a.workflowBrief(value,sample.interval?.kind||sample.marker.kind,ko);
   assert.ok(brief.evidence.includes(free),'legacy workflowBrief.evidence API retains original');
  }
 }
});

test('A96 repair1 visible evidence uses exact individual known fields without splitting unknown prose',()=>{
 const a=load(),p=fixture(),row=p.workflow.rows[0];
 for(const locale of ['ko','en']){a.state.language=locale;const ko=locale==='ko';
  for(const evidence of ['claimed_handoff · t3_structured','claimed_handoff extra prose',' claimed_handoff','future_source','constructor','__proto__']){
   const value={...row.intervals[0],explanation:'',evidence},before=JSON.stringify(value),html=a.workflowFloatingContent(row,value,null,ko),primary=html.slice(0,html.indexOf('<details'));
   assert.ok(!primary.includes('workflow-brief-evidence'),'unknown or joined input is never promoted to visible code evidence');
   assert.ok(html.slice(html.indexOf('<details')).includes(evidence));assert.equal(JSON.stringify(value),before);
  }
  const value={...row.intervals[0],explanation:'',source:'t3_structured',evidence:'claimed_handoff',reason:'free reason with claimed_handoff keyword'},before=JSON.stringify(value),html=a.workflowFloatingContent(row,value,null,ko),primary=html.slice(0,html.indexOf('<details'));
  const paragraph=primary.match(/<p class="workflow-brief-evidence">([\s\S]*?)<\/p>/)?.[1];assert.ok(paragraph);
  assert.ok(paragraph.includes(ko?'인계 수신이 보고되었습니다.':'A handoff claim was reported'));
  assert.ok(paragraph.includes(ko?'구조화된 T3 관측 근거입니다.':'Structured T3 observation'));
  assert.ok(!paragraph.includes(value.reason));assert.ok(html.slice(html.indexOf('<details')).includes(value.reason));assert.equal(JSON.stringify(value),before);
 }
});
