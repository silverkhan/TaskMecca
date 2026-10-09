const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const app = fs.readFileSync(
  path.join(__dirname, '../goassets/template/_task_mecca/framework/web/app.js'),
  'utf8'
);

test('restricted-access guidance uses the supported standalone CLI', () => {
  const banner = app.match(/function accessBanner\(\)\s*\{[\s\S]*?\n\}/)?.[0];
  assert.ok(banner, 'accessBanner function must exist');
  assert.match(banner, /task-mecca preflight --require-full-access --json/);
  assert.doesNotMatch(banner, /(?:uv run|python\s).*collab_tools\.py/);
});
