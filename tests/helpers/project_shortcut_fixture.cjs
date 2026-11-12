const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const web = path.join(__dirname, '../../goassets/template/_task_mecca/framework/web');
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
  return { ...app, context, document, routes, calls, projects,
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

module.exports = { page, element };
