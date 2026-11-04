#!/usr/bin/env node
// Record independent GitHub Actions command evidence without requiring a
// Task Mecca backlog or native agent identity on the CI runner.
'use strict';
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawn}=require('node:child_process');

function parse(argv){
 const sep=argv.indexOf('--');
 if(sep<0||sep===argv.length-1)throw Error('usage: node scripts/record-ci-test.cjs --output PATH -- PROGRAM [ARGS...]');
 const args=argv.slice(0,sep),cmd=argv.slice(sep+1);
 if(args.length!==2||args[0]!=='--output'||!args[1])throw Error('an explicit --output path is required');
 return {file:args[1],cmd};
}
function buildEvidence(cmd,started,ended,code,signal,environment=process.env){
 const hash=crypto.createHash('sha256').update(JSON.stringify(cmd)).digest('hex');
 const repo=environment.GITHUB_REPOSITORY||'';
 const runID=environment.GITHUB_RUN_ID||'';
 const url=repo&&runID?'https://github.com/'+repo+'/actions/runs/'+encodeURIComponent(runID):null;
 return {
  schema_version:1,
  evidence_kind:'ci_command_execution',
  source:'github_actions',
  task_id:null, // CI has no canonical Task Mecca backlog attribution.
  provider:'github_actions',
  repository:repo||null,
  workflow:environment.GITHUB_WORKFLOW||null,
  job:environment.GITHUB_JOB||null,
  run_id:runID||null,
  attempt:environment.GITHUB_RUN_ATTEMPT||null,
  git_sha:environment.GITHUB_SHA||null,
  run_url:url,
  command:path.basename(cmd[0]), // do not leak potentially secret arguments
  argv_sha256:hash,
  scope:'full_go_regression',
  started_at:started,
  ended_at:ended,
  duration_ms:Math.max(0,Date.parse(ended)-Date.parse(started)),
  exit_code:Number.isInteger(code)?code:null,
  signal:signal||null,
  result:code===0?'passed':signal?'cancelled':'failed',
 };
}
async function run(argv=process.argv.slice(2)){
 const {file,cmd}=parse(argv);
 const started=new Date().toISOString();
 let exitCode=1,signal=null;
 try{
  const child=spawn(cmd[0],cmd.slice(1),{stdio:'inherit',shell:false});
  ({code:exitCode,signal}=await new Promise((resolve,reject)=>{
   child.once('error',reject);
   child.once('exit',(code,signal)=>resolve({code,signal}));
  }));
 }catch(err){
  process.stderr.write('CI command failed to start: '+err.message+'\n');
 }
 const evidence=buildEvidence(cmd,started,new Date().toISOString(),exitCode,signal);
 // Write the execution result even when the command fails. Artifact upload has
 // if: always() so a failed regression still leaves evidence for review.
 fs.mkdirSync(path.dirname(file),{recursive:true});
 fs.writeFileSync(file,JSON.stringify(evidence,null,2)+'\n',{mode:0o600});
 process.stderr.write('CI command evidence: '+file+' ('+evidence.result+', '+evidence.duration_ms+' ms)\n');
 process.exitCode=Number.isInteger(exitCode)?exitCode:1;
}
if(require.main===module){
 run().catch(err=>{process.stderr.write(err.stack+'\n');process.exitCode=2;});
}
module.exports={parse,buildEvidence};
