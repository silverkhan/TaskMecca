const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const root = path.join(__dirname, '..');
const note = JSON.parse(fs.readFileSync(path.join(root, 'release-notes/0.3.0-rc.1.json'), 'utf8'));
const embedded = JSON.parse(fs.readFileSync(path.join(root, 'goassets/template/_task_mecca/framework/release-notes/current.json'), 'utf8'));
const source = fs.readFileSync(path.join(root, 'goassets/template/_task_mecca/framework/web/app.js'), 'utf8');

test('release candidate note has identical canonical and embedded contracts', () => {
  assert.deepEqual(embedded, note);
  assert.equal(note.version, '0.3.0-rc.1');
  assert.equal(note.highlights.length, 5);
  assert.equal(note.categories.length, 6);
  assert.equal(note.migration.required, true);
  assert.equal(note.migration.instruction_refresh, true);
  for (const item of [...note.highlights, ...note.after_update]) {
    assert.ok(item.ko && item.en);
  }
  for (const category of note.categories) {
    assert.ok(category.title.ko && category.title.en);
    assert.ok(category.items.length);
  }
});

test('release modal shows highlights, detailed view groups six categories', () => {
  const start = source.indexOf('function releaseLocalized(');
  const end = source.indexOf('function currentReleaseVersion()', start);
  const detailStart = source.indexOf('function releaseNoteDetailMarkup(');
  const detailEnd = source.indexOf('function renderReleaseNoteModal(', detailStart);
  assert.ok(start >= 0 && end > start && detailEnd > detailStart);
  const context = vm.createContext({
    state: {language: 'ko'},
    t: value => value,
    esc: s => String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  });
  vm.runInContext(source.slice(start,end) + source.slice(detailStart,detailEnd),context);
  context.note = note;
  const preview = vm.runInContext('releaseNoteDetailMarkup(note, {compact:true})',context);
  const details = vm.runInContext('releaseNoteDetailMarkup(note)',context);
  assert.equal((preview.match(/<li>/g)||[]).length, 7, 'five highlights and two action items');
  assert.ok(!preview.includes('class="release-category"'));
  assert.equal((details.match(/class="release-category"/g)||[]).length,6);
  assert.match(details, /에이전트 실행 관제/);
  assert.match(preview, /업데이트 후 확인 사항/);
});
