const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const root='goassets/template/_task_mecca/framework/web/';
const js=fs.readFileSync(root+'terminal.js','utf8');
const html=fs.readFileSync(root+'terminal.html','utf8');
const app=fs.readFileSync(root+'app.js','utf8');
test('Web restart and PTY restart are separate controls',()=>{
 assert.match(html,/id="terminalWebRestartBtn"/);
 assert.match(html,/id="terminalRestartBtn"/);
 assert.match(js,/terminalWebRestartBtn.*addEventListener\('click',restartWebService\)/);
});
test('Web restart requests detached service restart, not a shell command',()=>{
 assert.match(js,/fetch\('\/api\/admin\/restart'/);
 assert.match(js,/X-Task-Mecca-Action/);
 assert.doesNotMatch(js,/command:'task-mecca web restart'/);
 assert.match(js,/webRestartUseButton/);
});
test('Web restart completes only on a new boot ID, not an HTTP 200',()=>{
 assert.match(js,/after\.boot_id!==before\.boot_id/);
 assert.match(js,/window\.location\.reload\(\)/);
 assert.match(js,/webRestartFailure/);
 assert.match(app,/health\.boot_id\|\|health\.instance_id/);
});
