const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

const web = path.join(__dirname, '../goassets/template/_task_mecca/framework/web');
const source = fs.readFileSync(path.join(web, 'app.js'), 'utf8');
const startup = source.indexOf('\ntranslateChrome();');
const keyboardStart = source.indexOf('let projectShortcutComposing=');
const keyboardEnd = source.indexOf('\nlet autoPageResizeTimer=', keyboardStart);
assert.ok(startup > 0 && keyboardStart > startup && keyboardEnd > keyboardStart);

// Minimal DOM ancestry, rather than replacing the app's editing guard.
function element(tagName = 'DIV', attrs = {}, parentElement = null) {
  return {
    tagName, parentElement, attrs,
    get isContentEditable() {
      if ('contenteditable' in attrs) return attrs.contenteditable !== 'false';
      return parentElement?.isContentEditable || false;
    },
    closest(selectors) {
      for (let node = this; node; node = node.parentElement) {
        if (selectors.split(',').some(selector => {
          if (selector.startsWith('#')) return node.attrs.id === selector.slice(1);
          if (selector.startsWith('.')) return (node.attrs.class || '').split(' ').includes(selector.slice(1));
          if (selector.startsWith('[contenteditable]')) return 'contenteditable' in node.attrs && node.attrs.contenteditable !== 'false';
          const role = selector.match(/^\[role="(.+)"\]$/);
          return role ? node.attrs.role === role[1] : node.tagName.toLowerCase() === selector;
        })) return node;
      }
      return null;
    },
  };
}

function page(count = 10) {
  const elements = new Map();
  const listeners = new Map();
  const routes = [];
  const calls = [];
  const projects = Array.from({ length: count }, (_, i) => `/repos/project-${i + 1}`);
  const storage = new Map([['task-mecca-open-projects', JSON.stringify(projects)]]);
  const document = {
    activeElement: element('BODY'), documentElement: {},
    createElement() { return { dataset: {}, setAttribute() {}, querySelectorAll() { return []; } }; },
    addEventListener(type, callback) { listeners.set(type, callback); },
    querySelector(selector) {
      if (selector === '.hub-confirm-overlay') return this.confirmOpen ? {} : null;
      if (!elements.has(selector)) elements.set(selector, {
        innerHTML: '', textContent: '', addEventListener() {}, setAttribute() {},
        closest() { return null; }, prepend() {},
        focus() { document.activeElement = element('INPUT'); },
        classList: { toggle() {}, add() {}, remove() {}, contains() { return false; } },
      });
      return elements.get(selector);
    },
    querySelectorAll() { return []; },
  };
  const context = vm.createContext({
    URLSearchParams, location: { search: '' }, navigator: { language: 'en' }, document, window: { matchMedia: () => ({ matches: false }), innerWidth: 1400 },
    history: { pushState(...args) { routes.push(args[2]); } },
    localStorage: {
      getItem(key) { return storage.get(key) ?? null; },
      setItem(key, value) { storage.set(key, String(value)); },
      removeItem(key) { storage.delete(key); },
    }, calls,
  });
  vm.runInContext(source.slice(0, startup) + `
    render = () => nav();
    refresh = refreshList = ensureAttentionStream = refreshVersionInfo = updateDiagnosticNavigation = () => {};
    highlightSelection = n => calls.push(['highlight', n]);
    selectedRowId = () => 'A-1';
    openTask = id => { state.detail = id; calls.push(['open', id]); };
    closeTask = () => { state.detail = null; calls.push(['close']); };
    changeListPage = n => calls.push(['page', n]);
    globalThis.app = { state, nav, switchProject, closeProjectSession, manualView, translateChrome };
  ` + source.slice(keyboardStart, keyboardEnd), context);
  const app = context.app;
  app.state.hub = { projects: projects.map((path, i) => ({ path, name: `Project ${i + 1}`, counts: {} })) };
  app.state.project = projects[0] || '';
  app.state.view = 'backlog';
  app.nav();
  return { ...app, document, routes, calls, projects,
    menu: () => elements.get('#stateNav').innerHTML,
    html: selector => elements.get(selector)?.innerHTML,
    composition(type) { listeners.get(type)(); },
    key(key, options = {}) {
      const event = { key, target: document.activeElement, prevented: false,
        preventDefault() { this.prevented = true; }, ...options };
      listeners.get('keydown')(event);
      return event;
    },
  };
}

test('1–9 directly use sidebar order without reordering sessions, including from details', () => {
  const app = page();
  for (const ordinal of [9, 2, 1, 8, 3, 7, 4, 6, 5]) {
    app.state.detail = 'A-1';
    app.state.detailTask = { id: 'A-1' };
    assert.equal(app.key(String(ordinal)).prevented, true);
    assert.equal(app.state.project, app.projects[ordinal - 1]);
    assert.equal(app.state.detail, null);
    assert.equal(app.state.detailTask, null);
    assert.equal(app.state.view, 'backlog');
    assert.deepEqual(Array.from(app.state.openProjects), app.projects);
    assert.equal(app.routes.at(-1), `/?project=${encodeURIComponent(app.projects[ordinal - 1])}&view=backlog`);
  }
});

test('missing ordinals, zero and out-of-range numbers are no-ops, even in detail', () => {
  for (const count of [0, 2]) {
    const app = page(count);
    app.state.detail = 'A-1';
    for (const key of ['3', '9', '0', '10']) assert.equal(app.key(key).prevented, false);
    assert.equal(app.state.project, app.projects[0] || '');
    assert.equal(app.state.detail, 'A-1');
    assert.equal(app.routes.length, 0);
  }
});

test('input/search/select, editable descendants and terminal descendants keep their numbers', () => {
  const owners = [element('INPUT'), element('TEXTAREA'), element('SELECT'),
    element('DIV', { contenteditable: 'true' }), element('DIV', { contenteditable: 'plaintext-only' }),
    element('DIV', { role: 'textbox' }), element('DIV', { role: 'combobox' }),
    element('DIV', { class: 'xterm' }), element('DIV', { class: 'terminal' })];
  for (const owner of owners) {
    for (const node of [owner, element('SPAN', {}, owner)]) {
      for (const via of ['target', 'focus']) {
        const app = page();
        app.state.detail = 'A-1';
        if (via === 'focus') app.document.activeElement = node;
        assert.equal(app.key('2', via === 'target' ? { target: node } : {}).prevented, false);
        assert.equal(app.state.project, app.projects[0]);
        assert.equal(app.state.detail, 'A-1');
        assert.equal(app.routes.length, 0);
      }
    }
  }
});

test('IME event flags, keyCode 229 and the full composition lifecycle suppress switching', () => {
  const app = page();
  for (const options of [{ isComposing: true }, { keyCode: 229 }]) {
    assert.equal(app.key('2', options).prevented, false);
  }
  app.composition('compositionstart');
  assert.equal(app.key('2').prevented, false);
  assert.equal(app.state.project, app.projects[0]);
  app.composition('compositionend');
  assert.equal(app.key('2').prevented, true);
  assert.equal(app.state.project, app.projects[1]);
});

test('every modifier combination and an already consumed event stay with their owner', () => {
  const app = page();
  const modifiers = ['ctrlKey', 'metaKey', 'altKey', 'shiftKey'];
  for (let mask = 1; mask < 16; mask++) {
    const options = Object.fromEntries(modifiers.map((name, i) => [name, !!(mask & (1 << i))]));
    assert.equal(app.key('2', options).prevented, false);
  }
  assert.equal(app.key('2', { getModifierState: name => name === 'AltGraph' }).prevented, false);
  assert.equal(app.key('2', { defaultPrevented: true }).prevented, false);
  assert.equal(app.routes.length, 0);
  for (const key of ['/', 'ArrowLeft', 'ArrowDown', 'PageDown']) {
    assert.equal(app.key(key, { metaKey: true }).prevented, false);
  }
  assert.equal(app.calls.length, 0);
});

test('first nine labels and accessible names remain in collapsed and expanded sidebar order', () => {
  const app = page();
  for (const collapsed of [false, true]) {
    app.state.sidebarCollapsed = collapsed;
    app.nav();
    const rows = [...app.menu().matchAll(/<div class="session-row[^]*?(?=<div class="session-row|$)/g)].map(match => match[0]);
    assert.equal(rows.length, 10);
    rows.forEach((row, i) => {
      assert.match(row, /session-state-unknown/);
      assert.match(row, /class="session-initial" aria-hidden="true">P/);
      assert.match(row, new RegExp(`aria-label="Project ${i + 1}[^\"]*"`));
      if (i < 9) {
        assert.match(row, new RegExp(`aria-keyshortcuts="${i + 1}"`));
        assert.match(row, new RegExp(`class="session-number" aria-hidden="true">${i + 1}</span>`));
      } else assert.doesNotMatch(row, /session-number|aria-keyshortcuts/);
    });
    assert.equal(app.key('9').prevented, true);
  }
  // The real stylesheet hides names/counts on collapse, while retaining the number.
  const css = fs.readFileSync(path.join(web, 'style.css'), 'utf8');
  for (const [, selectors, rules] of css.matchAll(/([^{}]+)\{([^{}]+)\}/g)) {
    if (/display\s*:\s*none/.test(rules) && selectors.includes('sidebar-collapsed')) {
      assert.doesNotMatch(selectors, /session-number|session-open|session-row|session-list/);
    }
  }
});

test('click switching, explicit open/close and existing backlog/detail keys retain their paths', () => {
  const app = page(2);
  app.switchProject('/repos/extra');
  assert.equal(app.state.openProjects.length, 3);
  app.closeProjectSession('/repos/extra');
  assert.equal(app.state.project, app.projects[0]);
  app.key('ArrowDown');
  app.key('Enter');
  assert.equal(app.state.detail, 'A-1');
  app.key('ArrowLeft');
  app.key('PageDown');
  assert.deepEqual(JSON.parse(JSON.stringify(app.calls)), [['highlight', 1], ['open', 'A-1'], ['close'], ['page', 1]]);
  assert.equal(app.key('/').prevented, true);
  assert.equal(app.document.activeElement.tagName, 'INPUT');
});

test('existing manual and shortcut bar document numeric navigation in both languages', () => {
  const app = page();
  for (const language of ['ko', 'en']) {
    app.state.language = language;
    app.translateChrome();
    assert.match(app.html('#shortcutBar'), /<kbd>1–9<\/kbd>/);
    assert.match(app.manualView(), /<kbd>1–9<\/kbd>/);
    assert.match(app.html('#shortcutBar'), language === 'ko' ? /프로젝트 이동/ : /switch project/);
  }
});


test('dev confirmation overlays and attention focus retain their shortcut boundary', () => {
  const app = page();
  app.document.confirmOpen = true;
  assert.equal(app.key('2').prevented, false);
  assert.equal(app.routes.length, 0);
  app.document.confirmOpen = false;
  app.document.activeElement = element('BUTTON', {}, element('SECTION', { id: 'userAttention' }));
  assert.equal(app.key('2').prevented, false);
  assert.equal(app.routes.length, 0);
  app.document.activeElement = element('BODY');
  assert.equal(app.key('2').prevented, true);
  assert.equal(app.state.project, app.projects[1]);
});
