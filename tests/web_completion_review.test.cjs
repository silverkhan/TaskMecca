const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../goassets/template/_task_mecca/framework/web/app.js'), 'utf8');
const startup = source.indexOf('\ntranslateChrome();');
assert.ok(startup > 0);

function page() {
  const storage = new Map();
  const emitted = [];
  class Notification {
    static permission = 'granted';
    constructor(title, options) { emitted.push({ title, ...options }); }
  }
  const context = vm.createContext({
    URLSearchParams, location: { search: '?project=/repos/demo' }, navigator: { language: 'ko', userAgent: 'test', platform: 'MacIntel' },
    window: { isSecureContext: true, focus() {} }, Notification,
    fetch: async (url, options) => {
      if (!String(url).startsWith('/api/notifications/web')) throw Error('Unexpected URL '+url);
      const request=JSON.parse(options.body);
      return {ok:true,json:async()=>request.action==='claim'?{granted:true,state:'claimed',token:'fixture-lease'}:{state:'display_requested'}};
    },
    localStorage: { getItem: key => storage.get(key) ?? null, setItem: (key, value) => storage.set(key, String(value)) },
  });
  vm.runInContext(source.slice(0, startup) + `
    globalThis.app = { state, processTaskNotifications, stateLabel, healthLabel };
  `, context);
  return { ...context.app, emitted, async notify(snapshot) {
    context.app.processTaskNotifications(snapshot);
    // Wait for the real asynchronous Notification/service-worker path.
    await new Promise(setImmediate);
  } };
}

test('normal Controller review phases and recovery do not emit user intervention or completion', async () => {
  const app = page();
  for (const state of ['controller_review_pending', 'controller_review', 'controller_finalizing', 'controller_recovery']) {
    await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'doing', state, completion_review: { state, audience: 'controller' } } } });
    assert.equal(app.emitted.length, 0);
    assert.match(app.stateLabel(state), /Controller/);
    assert.equal(app.stateLabel(state), app.healthLabel(state));
    assert.doesNotMatch(app.stateLabel(state), /사용자|완료 처리 필요/);
  }
});

test('old completion_pending and Controller recovery attention reasons cannot become user notifications', async () => {
  const app = page();
  for (const reason of [
    { type: 'completion_pending' },
    { type: 'controller_recovery', audience: 'controller' },
    { type: 'runtime_stalled', audience: 'controller' },
  ]) {
    await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'doing', attention_reason: reason } } });
  }
  assert.equal(app.emitted.length, 0);
  assert.equal(app.stateLabel('awaiting_finalize'), 'Controller 검토 대기');
});

test('explicit user intervention is still delivered', async () => {
  const app = page();
  await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'doing', state: 'needs_user', attention_reason: { type: 'user_intervention', title: '사용자 개입 필요' } } }, notification_events:[{id:'user-event',task_id:'A-31',kind:'intervention',reason_type:'user_intervention',at:new Date().toISOString()}] });
  assert.equal(app.emitted.length, 1);
  assert.match(app.emitted[0].tag, /:intervention$/);
});

test('server completion events cannot notify before canonical file state is done', async () => {
  const app = page();
  const task = { id: 'A-31', file_state: 'doing', state: 'controller_finalizing' };
  const event = { id: 'done-event', kind: 'completed', task_id: 'A-31', at: new Date().toISOString() };
  await app.notify({ all_items: { 'A-31': task }, notification_events: [event] });
  assert.equal(app.emitted.length, 0);
  task.file_state = task.state = 'done';
  await app.notify({ all_items: { 'A-31': task }, notification_events: [event] });
  assert.equal(app.emitted.length, 1);
  assert.match(app.emitted[0].tag, /:completed$/);
});

test('file-state completion requires a canonical completed event', async () => {
  const app = page();
  await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'doing', state: 'controller_review' } } });
  assert.equal(app.emitted.length, 0);
  await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'done', state: 'done' } } });
  assert.equal(app.emitted.length, 0); // legacy delta is history-only, never a push
  await app.notify({ all_items: { 'A-31': { id: 'A-31', file_state: 'done', state: 'done' } }, notification_events:[{id:'done-canonical',task_id:'A-31',kind:'completed',at:new Date().toISOString()}] });
  assert.equal(app.emitted.length, 1);
  assert.match(app.emitted[0].tag, /:completed$/);
});
