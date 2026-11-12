// Verifies worktree file bytes against the assigned base; no canonical writes.
const fs=require('node:fs'),crypto=require('node:crypto'),{execFileSync}=require('node:child_process');
const base='f3beec75f6b082a2852cb88ddea2c8d5d901e66e';
const roots=['cmd','internal','goassets','src','tests/fixtures','.github/workflows','handoff-evidence/a65-transmissive'];
const tree=execFileSync('git',['ls-tree','-rz',base,'--',...roots]).toString().split('\0').filter(Boolean),groups={},mismatches=[];
for(const entry of tree){const [meta,file]=entry.split('\t'),oid=meta.split(' ')[2],bytes=fs.readFileSync(file),actual=crypto.createHash('sha1').update(Buffer.from('blob '+bytes.length+'\0')).update(bytes).digest('hex');if(oid!==actual)mismatches.push(file);const group=roots.find(root=>file.startsWith(root+'/'));groups[group]??=[];groups[group].push([file,crypto.createHash('sha256').update(bytes).digest('hex')]);}
const proof={base,algorithm:'Each actual file Git blob SHA-1 matches the base object; SHA-256 aggregates cover path + NUL + actual file SHA-256 + newline.',groups:Object.fromEntries(Object.entries(groups).map(([root,files])=>[root,{files:files.length,sha256:crypto.createHash('sha256').update(files.map(([p,s])=>p+'\0'+s+'\n').join('')).digest('hex')} ])),mismatches};
for(const file of ['tests/web_workflow_navigation.test.cjs','tests/web_workflow_gestures.test.cjs','tests/web_workflow_popover.test.cjs','tests/web_workflow_connected.test.cjs']){const old=execFileSync('git',['show',base+':'+file]);if(!old.equals(fs.readFileSync(file)))mismatches.push(file);}
const original=execFileSync('git',['show',base+':tests/web_workflow_browser.test.cjs']);proof.originalBrowserAssertionsPreserved=fs.readFileSync('tests/web_workflow_browser.test.cjs').subarray(0,original.length).equals(original);proof.unchangedOriginalTestFiles=4;
console.log(JSON.stringify(proof,null,2));if(mismatches.length||!proof.originalBrowserAssertionsPreserved)process.exitCode=1;
