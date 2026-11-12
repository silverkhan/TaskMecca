const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

const web = path.join(__dirname, '../goassets/template/_task_mecca/framework/web');
const { page, element } = require('./helpers/project_shortcut_fixture.cjs');

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

test('input/search/select, editable descendants and terminals keep project shortcut keys', () => {
  const owners = [element('INPUT'), element('TEXTAREA'), element('SELECT'),
    element('DIV', { contenteditable: 'true' }), element('DIV', { contenteditable: 'plaintext-only' }),
    element('DIV', { role: 'textbox' }), element('DIV', { role: 'combobox' }), element('DIV', { role: 'spinbutton' }),
    element('DIV', { class: 'xterm' }), element('DIV', { class: 'terminal' })];
  for (const owner of owners) {
    for (const node of [owner, element('SPAN', {}, owner)]) {
      for (const via of ['target', 'focus']) {
        for (const key of ['2', '[', ']']) {
          const app = page();
          app.state.detail = 'A-1';
          if (via === 'focus') app.document.activeElement = node;
          assert.equal(app.key(key, via === 'target' ? { target: node } : {}).prevented, false);
          assert.equal(app.state.project, app.projects[0]);
          assert.equal(app.state.detail, 'A-1');
          assert.equal(app.routes.length, 0);
        }
      }
    }
  }
});

test('IME event flags, keyCode 229 and the full composition lifecycle suppress switching', () => {
  const app = page();
  for (const key of ['2', '[', ']']) {
    for (const options of [{ isComposing: true }, { keyCode: 229 }]) {
      assert.equal(app.key(key, options).prevented, false);
    }
  }
  app.composition('compositionstart');
  for (const key of ['2', '[', ']']) assert.equal(app.key(key).prevented, false);
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
    for (const key of ['2', '[', ']']) assert.equal(app.key(key, options).prevented, false);
  }
  for (const key of ['2', '[', ']']) assert.equal(app.key(key, { getModifierState: name => name === 'AltGraph' }).prevented, false);
  for (const key of ['2', '[', ']']) assert.equal(app.key(key, { defaultPrevented: true }).prevented, false);
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
    for (const html of [app.html('#shortcutBar'), app.manualView()]) {
      assert.match(html, /<kbd>\[<\/kbd>\/<kbd>\]<\/kbd>/);
      assert.match(html, language === 'ko' ? /이전\/다음/ : /previous\/next/);
    }
    assert.match(app.html('#shortcutBar'), language === 'ko' ? /프로젝트 이동/ : /switch project/);
  }
});


test('dev confirmation overlays and attention focus retain their shortcut boundary', () => {
  const app = page();
  app.document.confirmOpen = true;
  for (const key of ['2', '[', ']']) assert.equal(app.key(key).prevented, false);
  assert.equal(app.routes.length, 0);
  app.document.confirmOpen = false;
  app.document.activeElement = element('BUTTON', {}, element('SECTION', { id: 'userAttention' }));
  for (const key of ['2', '[', ']']) assert.equal(app.key(key).prevented, false);
  assert.equal(app.routes.length, 0);
  app.document.activeElement = element('BODY');
  assert.equal(app.key('2').prevented, true);
  assert.equal(app.state.project, app.projects[1]);
});


test('brackets follow stable OPEN sidebar order in both directions, wrap, and leave details', () => {
  const app = page();
  // Closed hub projects are never inserted into the shortcut sequence.
  app.state.hub.projects.unshift({path:'/repos/closed',name:'Closed',counts:{}});
  for (const [key, ordinal] of [['[',10], [']',1], [']',2], ['[',1], ['[',10]]) {
    app.state.detail = 'A-1';
    app.state.detailTask = {id:'A-1'};
    assert.equal(app.key(key).prevented, true);
    assert.equal(app.state.project, app.projects[ordinal-1]);
    assert.equal(app.state.detail, null);
    assert.equal(app.state.detailTask, null);
    assert.deepEqual(Array.from(app.state.openProjects), app.projects);
  }
  app.key('9');
  app.key(']');
  assert.equal(app.state.project, app.projects[9]);
  app.key('1');
  assert.equal(app.state.project, app.projects[0]);
});

test('empty and single active sessions preserve detail, filters, routes and focus', () => {
  for (const count of [0,1]) {
    const app = page(count);
    const focus = app.document.activeElement;
    app.state.detail = 'A-1';
    app.state.query = 'retained';
    app.state.listPage = 3;
    for (const key of ['[', ']']) assert.equal(app.key(key).prevented, false);
    assert.equal(app.state.detail, 'A-1');
    assert.equal(app.state.query, 'retained');
    assert.equal(app.state.listPage, 3);
    assert.equal(app.routes.length, 0);
    assert.equal(app.document.activeElement, focus);
  }
});

test('hub or unmatched active project enters first forward, last backward without off-by-one', () => {
  for (const count of [1,3]) for (const view of ['hub','backlog']) for (const key of ['[',']']) {
    const app = page(count);
    app.state.view = view;
    // Hub can retain a matching project path; it still has no active row.
    app.state.project = view === 'hub' ? app.projects[0] : '/repos/missing';
    assert.equal(app.key(key).prevented, true);
    assert.equal(app.state.project, app.projects[key === ']' ? 0 : count-1]);
    assert.equal(app.state.view, 'backlog');
    assert.deepEqual(Array.from(app.state.openProjects), app.projects);
  }
});

test('mobile navigation retains Escape and Tab focus ownership with brackets enabled', () => {
  const app = page();
  app.state.mobileNavOpen = true;
  vm.runInContext("window.matchMedia=()=>({matches:true}); closeMobileNav=()=>{state.mobileNavOpen=false}", app.context);
  const panel = app.document.querySelector('#mobileSidebarPanel');
  const first = {getClientRects:()=>[{}],closest:()=>null,focus:()=>{app.document.activeElement=first}};
  const last = {getClientRects:()=>[{}],closest:()=>null,focus:()=>{app.document.activeElement=last}};
  panel.querySelectorAll=()=>[first,last];
  panel.contains=node=>node===first||node===last;
  app.document.activeElement=last;
  assert.equal(app.key('Tab').prevented,true);
  assert.equal(app.document.activeElement,first);
  assert.equal(app.key('Tab',{shiftKey:true}).prevented,true);
  assert.equal(app.document.activeElement,last);
  assert.equal(app.key('Escape').prevented,true);
  assert.equal(app.state.mobileNavOpen,false);
  assert.equal(app.routes.length,0);
});
