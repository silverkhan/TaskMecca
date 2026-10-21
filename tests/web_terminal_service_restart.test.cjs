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

test('Terminal stream failures remain visible and basic-input recovery is accessible',()=>{
 const css=fs.readFileSync(root+'terminal.css','utf8');
 assert.match(html,/id="terminalDiagnostics" role="status" aria-live="polite"/);
 assert.match(html,/id="terminalBasicBtn"/);
 assert.match(css,/\.terminal-diagnostics\[data-kind="error"\]/);
 assert.match(js,/function showTerminalDiagnostic\(kind, message\)/);
 assert.match(js,/showTerminalDiagnostic\('error', t\('inputFailed'\)/);
 assert.match(js,/state\.preferBasicInput = !state\.preferBasicInput/);
 assert.match(js,/info\.output_bytes > 0/);
 assert.match(js,/controller\.signal\.aborted/);
 assert.match(js,/await mountSession\(\);/);
});

test('Terminal status visibility is not inferred from HTTP status alone',()=>{
 assert.match(js,/receivedBytes \+= value\.length/);
 assert.match(js,/t\('noShellOutput'\)/);
 assert.match(js,/t\('outputNotReceived'\)/);
 assert.match(js,/if \(!host\.clientWidth \|\| !host\.clientHeight\)/);
});

test('Terminal can poll bounded PTY history without weakening auth',()=>{
 const go=fs.readFileSync('internal/webui/terminal.go','utf8');
 assert.match(go,/case "output":/);
 assert.match(go,/session\.outputSince\(after\)/);
 assert.match(go,/X-Task-Mecca-Output-Start/);
 assert.match(go,/X-Task-Mecca-Output-End/);
 assert.match(go,/X-Task-Mecca-Session-Closed/);
 assert.match(js,/\/output\?after=/);
 assert.match(js,/X-Task-Mecca-Terminal/);
 assert.match(js,/void pollOutput\(session, controller\)/);
 assert.match(js,/if \(polling\) continue/);
});
