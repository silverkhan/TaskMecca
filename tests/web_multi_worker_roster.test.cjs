const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const start=source.indexOf('// Read-only collaborative assignment presentation.');
const end=source.indexOf('function detailView(task)',start);
assert(start>=0&&end>start);
const code=source.slice(start,end);
const ctx=vm.createContext({state:{language:'ko'},esc:v=>String(v).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;')});
vm.runInContext(code,ctx);
const parse=raw=>Array.from(vm.runInContext('backlogWorkerRoster',ctx)(raw));
const render=raw=>vm.runInContext('backlogWorkerRosterSection',ctx)({raw_markdown:raw});
test('legacy single worker has no collaboration section',()=>{
 assert.equal(parse('# Task\n## 개요\nAgent: raichyu').length,0);
 assert.equal(render('# Task\n## 개요\nAgent: raichyu'),'');
});
test('collaboration roles and delegation origin appear without runtime assumptions',()=>{
 const raw='# B-559\n## 작업 참여자\n\n| Worker | 역할 | 담당 범위 | 배정 경위 |\n| --- | --- | --- | --- |\n| raichyu | 주 구현 | API 연결 | Controller |\n| naong | 독립 검수 | 금융 검산 | Controller |\n| metamong | 보조 구현 | 근거 도구 | raichyu 요청 |\n\n## 수용 기준\n- 검산 완료';
 const rows=parse(raw);
 assert.equal(rows.length,3);
 assert.equal(rows[2].source,'raichyu 요청');
 const html=render(raw);
 assert.match(html,/naong/);assert.match(html,/독립 검수/);assert.match(html,/raichyu 요청/);
 assert.match(html,/실제 착수·완료는 Lifecycle/);
 assert.doesNotMatch(html,/data-status-filter/);
});
test('untrusted roster values are escaped',()=>{
 const html=render('## 작업 참여자\n| Worker | Role | Scope | Origin |\n| --- | --- | --- | --- |\n| <script> | lead | task | Controller |');
 assert.doesNotMatch(html,/<script>/);
 assert.match(html,/&lt;script&gt;/);
});
