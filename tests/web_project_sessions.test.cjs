const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

const source = fs.readFileSync(path.join(__dirname, '../goassets/template/_task_mecca/framework/web/app.js'), 'utf8');
// Load the shipped state and functions without starting timers or API requests.
const startup = source.indexOf('\ntranslateChrome();');
assert.ok(startup > 0, 'the browser startup boundary must exist');
const declarations = source.slice(0, startup);
const storageKey = 'task-mecca-open-projects';

function page(storage, project = '') {
  const elements = new Map();
  function element(tagName = 'div') {
 const node = { tagName: tagName.toUpperCase(), innerHTML: '', textContent: '', dataset: {}, children: [], attributes: {}, listeners: {},
 addEventListener(type, listener) { this.listeners[type] = listener; },
 setAttribute(name, value) { this.attributes[name] = value; },
 prepend(child) { this.children.unshift(child); },
 classList: { toggle() {}, add() {}, remove() {} },
 querySelectorAll(selector) {
 if (selector !== '[data-sidebar-mode]') return [];
 return [...this.innerHTML.matchAll(/data-sidebar-mode="([^"]+)"/g)].map(match => {
 const button = element('button'); button.dataset.sidebarMode = match[1]; return button;
 });
 },
 };
 return node;
 }
 const document = {
 createElement: element,
 querySelector(selector) {
 if (!elements.has(selector)) elements.set(selector, element());
 return elements.get(selector);
 },
 querySelectorAll() { return []; },
 };
 const context = vm.createContext({
    URLSearchParams, location: { search: project ? `?project=${encodeURIComponent(project)}` : '' },
    navigator: { language: 'ko' }, document,
    history: { pushState() {} },
    localStorage: {
      getItem(key) { return storage.get(key) ?? null; },
      setItem(key, value) { storage.set(key, String(value)); },
      removeItem(key) { storage.delete(key); },
    },
  });
  vm.runInContext(declarations + `
    render = () => nav();
    refresh = refreshList = ensureAttentionStream = refreshVersionInfo = () => {};
    globalThis.session = { state, nav, switchProject, closeProjectSession };
  `, context);
  return { ...context.session, sidebarSelector: () => elements.get('#stateNav').children[0], menu: () => elements.get('#stateNav').innerHTML };
}

function saved(storage) { return JSON.parse(storage.get(storageKey)); }
function opened(storage, ...projects) { storage.set(storageKey, JSON.stringify(projects)); }

test('first render before hub discovery preserves all saved projects, including inactive ones', () => {
  const storage = new Map();
  opened(storage, '/repos/alpha', '/repos/beta');
  for (const selected of ['', '/repos/alpha']) {
    const app = page(storage, selected);
    app.nav();
    assert.deepEqual(saved(storage), ['/repos/alpha', '/repos/beta']);
    assert.match(app.menu(), /data-session-project="\/repos\/beta"/);
  }
});

test('partial and empty hub refreshes do not close projects and returning metadata renders normally', () => {
  const storage = new Map();
  opened(storage, '/repos/alpha', '/repos/beta');
  const app = page(storage, '/repos/alpha');
  for (const projects of [[{ path: '/repos/alpha', name: 'Alpha' }], [], [{ path: '/repos/beta', name: 'Beta restored' }]]) {
    app.state.hub = { projects };
    app.nav();
    assert.deepEqual(saved(storage), ['/repos/alpha', '/repos/beta']);
    assert.match(app.menu(), /data-session-project="\/repos\/beta"/);
  }
  assert.match(app.menu(), /Beta restored/);
});

test('reload after update restores opened projects using the same browser storage', () => {
  const storage = new Map();
  const app = page(storage);
  app.switchProject('/repos/alpha');
  app.switchProject('/repos/beta');
  app.state.project = '';
  app.state.view = 'hub';
  app.nav();
  const reloaded = page(storage);
  reloaded.nav();
  assert.deepEqual(saved(storage), ['/repos/alpha', '/repos/beta']);
  assert.match(reloaded.menu(), /data-session-project="\/repos\/alpha"/);
});

test('explicit close removes inactive project persistently, even when rediscovered', () => {
  const storage = new Map();
  opened(storage, '/repos/alpha', '/repos/beta');
  const app = page(storage, '/repos/alpha');
  app.closeProjectSession('/repos/beta');
  assert.deepEqual(saved(storage), ['/repos/alpha']);
  const reloaded = page(storage, '/repos/alpha');
  reloaded.state.hub = { projects: [{ path: '/repos/beta', name: 'Beta' }] };
  reloaded.nav();
  assert.deepEqual(saved(storage), ['/repos/alpha']);
  assert.doesNotMatch(reloaded.menu(), /data-session-project="\/repos\/beta"/);
  reloaded.switchProject('/repos/beta');
  assert.deepEqual(saved(storage), ['/repos/alpha', '/repos/beta']);
});

test('closing active project switches to remaining project; closing last project returns to hub', () => {
  const storage = new Map();
  opened(storage, '/repos/alpha', '/repos/beta');
  const app = page(storage, '/repos/alpha');
  app.closeProjectSession('/repos/alpha');
  assert.equal(app.state.project, '/repos/beta');
  assert.deepEqual(saved(storage), ['/repos/beta']);
  app.closeProjectSession('/repos/beta');
  assert.equal(app.state.project, '');
  assert.equal(app.state.view, 'hub');
  assert.deepEqual(saved(storage), []);
  page(storage).nav();
  assert.deepEqual(saved(storage), []);
});

 test('navigation prepends the real sidebar mode selector without removing project sessions', () => {
 const storage = new Map();
 opened(storage, '/repos/alpha', '/repos/beta');
 const app = page(storage, '/repos/alpha');
 app.nav();
 const selector = app.sidebarSelector();
 assert.equal(selector.tagName, 'DIV');
 assert.equal(selector.className, 'sidebar-mode-selector');
 assert.equal(selector.attributes.role, 'group');
 assert.ok(selector.attributes['aria-label']);
 assert.deepEqual(selector.querySelectorAll('[data-sidebar-mode]').map(button => button.dataset.sidebarMode), ['auto', 'expanded', 'compact']);
 assert.match(app.menu(), /data-session-project="\/repos\/beta"/);
 assert.deepEqual(saved(storage), ['/repos/alpha', '/repos/beta']);
 });
