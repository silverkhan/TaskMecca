const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const root=path.join(__dirname,'..');
const read=file=>fs.readFileSync(path.join(root,file),'utf8');

const workflow=read('.github/workflows/release-stable.yml');
const guard=read('.github/workflows/release-version-guard.yml');

test('Stable release is main-only and never publishes before both native platform gates',()=>{
  assert.match(workflow,/branches: \[main\]/);
  assert.match(workflow,/- "\.github\/workflows\/release-stable\.yml"/);
  assert.doesNotMatch(workflow,/branches: \[(?:main, dev|dev, main)\]/);
  assert.match(workflow,/platform-gate:\s*\n/);
  assert.match(workflow,/os: \[macos-latest, windows-latest\]/);
  assert.match(workflow,/go test \.\/internal\/maintenance \.\/internal\/webui \.\/internal\/install -run "Upgrade\|Rollback\|Migration\|Installed\|BrandAssets"/);
  assert.match(workflow,/Smoke test candidate native executable before publishing/);
  assert.match(workflow,/release:\s*\n\s+needs: platform-gate/);
  assert.ok(workflow.indexOf('Smoke test unpublished Linux release binary and Web health')<
    workflow.indexOf('Update Stable release'), 'unpublished binary must pass Web health before public release');
  assert.match(workflow,/sha256sum -c SHA256SUMS\.txt/);
  assert.match(workflow,/gh release upload "\$tag" dist\/VERSION\.txt --clobber/);
  assert.ok(workflow.indexOf('gh release upload "$tag" dist/VERSION.txt --clobber')>
    workflow.indexOf('dist/SHA256SUMS.txt'), 'VERSION.txt must be the last uploaded public release asset');
  assert.match(workflow,/windows-installer-smoke:\s*\n\s+needs: release/);
});

test('Stable release version, notes and binary verification are version-independent',()=>{
  assert.match(workflow,/current_version="\$\(tr -d/);
  assert.match(workflow,/test "\$current_version" = "\$cli_version"/);
  assert.match(workflow,/test "\$note_version" = "\$current_version"/);
  assert.match(workflow,/test "\$index_version" = "\$current_version"/);
  assert.match(workflow,/diff -u <\(jq -S \. "\$note_path"\) <\(jq -S \. "\$remote_note_path"\)/);
  assert.match(workflow,/expected_version="\$\(tr -d/);
  assert.match(workflow,/task-mecca \$expected_version/);
  assert.match(workflow,/Task Mecca Stable \$\{version\} — \$\{title\}/);
  assert.doesNotMatch(workflow,/task-mecca 0\.3\.0|Task Mecca Stable 0\.3\.0/);
  assert.match(workflow,/tests\/web_release_note_formats\.test\.cjs/);
  assert.doesNotMatch(workflow,/tests\/web_release_notes_030\.test\.cjs/);
});

test('Stable PR guard permits same-version repairs only before publication',()=>{
  assert.match(guard,/branches: \[main\]/);
  assert.match(guard,/head_version.*base_version/);
  assert.match(guard,/published_version=/);
  assert.match(guard,/if head > base:/);
  assert.match(guard,/elif head == base and sys\.argv\[3\]:/);
  assert.match(guard,/published < head/);
  assert.match(guard,/Stable product changes require a version increase/);
  assert.match(guard,/diff -u <\(jq -S \. "\$note_path"\) <\(jq -S \. "\$canonical_note"\)/);
});

test('Previously published Windows command-launch hotfix remains in Dev runtime',()=>{
  const maintenance=read('internal/maintenance/maintenance.go');
  const service=read('internal/webui/service_process_other.go');
  assert.ok(maintenance.includes('exec.Command("cmd.exe","/D","/C","start","","/MIN",script)'));
  assert.ok(maintenance.includes('_=cmd.Process.Release()'));
  assert.ok(service.includes('cmd.Process.Release()'));
  // The newer PowerShell restart helper replaces the old fragile cmd /C start.
  assert.ok(service.includes('exec.Command("powershell.exe"'));
  assert.doesNotMatch(service,/start","\\?"Task Mecca Web Restart/);
});

test('Every static Web and Node test prerequisite referenced by stable prepublish gate exists',()=>{
  const app=read('goassets/template/_task_mecca/framework/web/app.js');
  for(const line of workflow.split('\n')){
    const match=line.match(/grep -(?:F)?q "(function [^"]+)" goassets\/template\/_task_mecca\/framework\/web\/app\.js/);
    if(match)assert.ok(app.includes(match[1]),'missing required Web function: '+match[1]);
  }
  const suite=new Set(workflow.match(/tests\/[a-z_0-9]+\.test\.cjs/g)||[]);
  assert.ok(suite.size>=6,'release smoke must test Web and update behavior');
  for(const entry of suite)assert.ok(fs.existsSync(path.join(root,entry)),'missing test suite: '+entry);
});
