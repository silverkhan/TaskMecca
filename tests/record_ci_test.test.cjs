const {test}=require('node:test');
const assert=require('node:assert/strict');
const {parse,buildEvidence}=require('../scripts/record-ci-test.cjs');

test('CI command parsing requires one explicit output and command',()=>{
 assert.deepEqual(parse(['--output','evidence.json','--','go','test','./...']),{
  file:'evidence.json',cmd:['go','test','./...']});
 assert.throws(()=>parse(['--','go','test']),/output/);
 assert.throws(()=>parse(['--output','x','--']),/usage/);
});
test('CI run evidence has grounded timing, result, SHA and no backlog inference',()=>{
 const env={
  GITHUB_REPOSITORY:'silverkhan/TaskMecca',
  GITHUB_RUN_ID:'456',
  GITHUB_RUN_ATTEMPT:'2',
  GITHUB_SHA:'0123456789abcdef',
  GITHUB_WORKFLOW:'CI',
  GITHUB_JOB:'go-runtime',
 };
 const result=buildEvidence(['go','test','./...'],'2026-11-04T00:00:00Z','2026-11-04T00:01:05Z',0,null,env);
 assert.equal(result.task_id,null);
 assert.equal(result.duration_ms,65000);
 assert.equal(result.result,'passed');
 assert.equal(result.git_sha,env.GITHUB_SHA);
 assert.equal(result.run_url,'https://github.com/silverkhan/TaskMecca/actions/runs/456');
 assert.equal(result.command,'go');
 assert.equal(result.argv_sha256.length,64);
 assert.equal(JSON.stringify(result).includes('./...'),false,'raw args should not be published');
});
test('failed CI command preserves exit code and does not invent wall time',()=>{
 const result=buildEvidence(['go','test','./...'],'2026-11-04T00:00:00Z','2026-11-04T00:00:10Z',7,null,{});
 assert.equal(result.exit_code,7);
 assert.equal(result.result,'failed');
 assert.equal(result.duration_ms,10000);
 assert.equal(result.run_url,null);
});
