// Restore only the exported helper for the negative-control regression run.
// The repository production modules are served byte-for-byte unchanged.
const path=require('node:path'),fixture=require(path.resolve('tests/helpers/workflow_native.cjs'));
fixture.waitFor=(page,fn,arg)=>page.waitForFunction(async({code,arg})=>(await import('/__test__/workflow-bridge.js')).run(code,arg),{code:fn.toString(),arg});
