// Optional renderer scheduling stress. No product source or CI policy change.
const path=require('node:path'),fixture=require(path.resolve('tests/helpers/workflow_native.cjs')),nativePage=fixture.page;
fixture.page=async function(context,server){const page=await nativePage(context,server),session=await context.newCDPSession(page);await session.send('Emulation.setCPUThrottlingRate',{rate:4});return page;};
