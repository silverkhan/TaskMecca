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
  const renderedSessionRows = [];
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
 querySelectorAll(selector) {
   return selector === '[data-session-project]' ? renderedSessionRows : [];
 },
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
    globalThis.session = { state, nav, switchProject, closeProjectSession, sessionProjectStatus, renderUserAttention };
  `, context);
  return { ...context.session, renderedSessionRows, sidebarSelector: () => elements.get('#stateNav').children[0], menu: () => elements.get('#stateNav').innerHTML };
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


test('project tiles use theme-muted selection and anchored alert badges', () => {
  const css = fs.readFileSync(path.join(__dirname, '../goassets/template/_task_mecca/framework/web/style.css'), 'utf8');
  const initial = css.match(/\.session-initial\{([^}]+)\}/)?.[1] || '';
  const selected = css.match(/\.session-row\.active \.session-initial\{([^}]+)\}/)?.[1] || '';
  const badge = css.match(/\.session-state-error \.session-initial::after,\.session-state-warning \.session-initial::after\{([^}]+)\}/)?.[1] || '';
  const warning = css.match(/\n\.session-state-warning \.session-initial::after\{([^}]+)\}/)?.[1] || '';
  assert.match(initial, /position:relative/, 'badge must anchor to the initial tile');
  assert.match(selected, /outline:0/, 'selection must not use the old bright 2px outline');
  assert.match(selected, /border-color:color-mix\(in srgb,var\(--accent\)/);
  assert.match(selected, /box-shadow:/);
  assert.match(badge, /position:absolute;top:-5px;right:-5px/);
  assert.match(badge, /width:14px;height:14px/);
  assert.match(badge, /border-radius:50%/);
  assert.match(badge, /background:var\(--danger\);color:var\(--surface\)/);
  assert.match(warning, /background:var\(--warn\);color:var\(--surface\)/);
  assert.match(css, /\.status\.blocked:before,\.status\.stale:before,\.status\.worker_missing:before\{background:var\(--danger\)\}/, 'blocked backlog status must share the error badge color');
  assert.match(css, /\.status\.hold:before,\.status\.quiet:before\{background:var\(--warn\)\}/, 'hold backlog status must share the warning badge color');
  assert.match(css, /\.session-state-error \.session-initial\{background:color-mix\(in srgb,var\(--danger\)/, 'error tile must match badge hue');
  assert.match(css, /\.session-state-warning \.session-initial\{background:color-mix\(in srgb,var\(--warn\)/, 'warning tile must match badge hue');
  assert.doesNotMatch(badge, /transform:translate/, 'badge must not rely on a text glyph offset');
  assert.match(css, /\.session-open:focus-visible\{/, 'keyboard focus must remain independently visible');

  const storage = new Map();
  opened(storage, '/repos/error', '/repos/warning', '/repos/working');
  const app = page(storage, '/repos/error');
  app.state.view = 'backlog';
  app.state.hub = { projects: [
    { path: '/repos/error', name: 'Error', counts: { blocked: 1 } },
    { path: '/repos/warning', name: 'Warning', counts: { attention: 1 } },
    { path: '/repos/working', name: 'Working', counts: { working: 1 } },
  ] };
  app.nav();
  const html = app.menu();
  assert.match(html, /session-row session-state-error active/);
  assert.match(html, /session-row session-state-warning /);
  assert.match(html, /session-row session-state-working /);
  assert.match(html, /aria-label="[^"]*Error \/ Blocked"/);
  assert.match(html, /aria-label="[^"]*Needs attention"/);
  assert.match(html, /aria-label="[^"]*Working"/);
});


test('project status prioritizes canonical user intervention over a hold warning', () => {
 const storage=new Map();
 opened(storage,'/repos/a','/repos/b');
 const app=page(storage,'/repos/a');
 app.state.view='backlog';
 app.state.hub={projects:[
   {path:'/repos/a',name:'A',counts:{hold:1,working:1,blocked:0}},
   {path:'/repos/b',name:'B',counts:{hold:1}},
 ]};
 assert.equal(app.sessionProjectStatus({hold:1},false),'warning');
 assert.equal(app.sessionProjectStatus({hold:1},true),'error');
 assert.equal(app.sessionProjectStatus({blocked:1},false),'error');
 assert.equal(app.sessionProjectStatus({hold:1,needs_user:1},false),'error');
 app.nav();
 assert.match(app.menu(),/session-row session-state-warning active" data-session-project="\/repos\/a"/);
 app.state.commonUserAttention['/repos/a|/backlog']={project:'/repos/a',backlog:'/backlog',observed:1,rows:[{id:'A-50',title:'Requires decision',message:'',action:'Review'}]};
 app.nav();
 assert.match(app.menu(),/session-row session-state-error active" data-session-project="\/repos\/a"/);
 assert.match(app.menu(),/session-row session-state-warning " data-session-project="\/repos\/b"/);
 assert.match(app.menu(),/Error \/ User action \/ Blocked/);
});

test('live attention refresh updates project badge and recovers warning without rebuilding navigation', () => {
 const storage=new Map();
 opened(storage,'/repos/a','/repos/b');
 const app=page(storage,'/repos/a');
 app.state.view='backlog';
 app.state.hub={projects:[
   {path:'/repos/a',name:'A',counts:{hold:1}},
   {path:'/repos/b',name:'B',counts:{hold:1}},
 ]};
 const fakeRow=(project)=>{
   const classes=new Set(['session-row','session-state-warning']);
   const attrs={}, button={title:'',setAttribute(key,value){attrs[key]=value;}};
   return {
     dataset:{sessionProject:project},
     classList:{contains:v=>classes.has(v),add:v=>classes.add(v),remove:v=>classes.delete(v)},
     querySelector(selector){return selector==='.session-open'?button:null;},
     classes,button,attrs,
   };
 };
 const selected=fakeRow('/repos/a'),other=fakeRow('/repos/b');
 app.renderedSessionRows.push(selected,other);
 app.state.commonUserAttention['/repos/a|/backlog']={project:'/repos/a',backlog:'/backlog',observed:1,rows:[{id:'A-50',title:'Requires decision',message:'',action:'Review'}]};
 app.renderUserAttention();
 assert.ok(selected.classes.has('session-state-error'));
 assert.ok(other.classes.has('session-state-warning'));
 assert.match(selected.attrs['aria-label'],/Error \/ User action \/ Blocked/);
 app.state.commonUserAttention['/repos/a|/backlog']={project:'/repos/a',backlog:'/backlog',observed:2,rows:[]};
 app.renderUserAttention();
 assert.ok(selected.classes.has('session-state-warning'));
 assert.ok(!selected.classes.has('session-state-error'));
 assert.match(selected.attrs['aria-label'],/Needs attention/);
});
