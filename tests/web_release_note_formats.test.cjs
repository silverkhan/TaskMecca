const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'../goassets/template/_task_mecca/framework/web/app.js'),'utf8');

function renderer(language='ko') {
  const begin=source.indexOf('function releaseLocalized(');
  const end=source.indexOf('function currentReleaseVersion()',begin);
  const detail=source.indexOf('function releaseNoteDetailMarkup(');
  const next=source.indexOf('function renderReleaseNoteModal()',detail);
  assert.ok(begin>=0&&end>begin&&detail>=0&&next>detail);
  const context=vm.createContext({
    state:{language},t:key=>key,
    esc:value=>String(value??'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
  });
  vm.runInContext(source.slice(begin,end)+source.slice(detail,next),context);
  return note=>{context.note=note;return {
    preview:vm.runInContext('releaseNoteDetailMarkup(note,{compact:true})',context),
    detail:vm.runInContext('releaseNoteDetailMarkup(note)',context)
  }};
}

const editorial={
  version:'0.3.1',date:'2026-10-16',
  summary:{ko:'새로운 기능',en:'New features'},
  migration:{required:false,instruction_refresh:false},
  highlights:[
    {ko:'첫 번째 개선',en:'First improvement'},
    {ko:'두 번째 개선',en:'Second improvement'}
  ],
  categories:[
    {title:{ko:'성능·안정성',en:'Performance and reliability'},items:[
      {ko:'백로그 조회 안정화',en:'Reliable backlog navigation'}
    ]},
    {title:{ko:'업데이트',en:'Updates'},items:[
      {ko:'&lt;script> 안전',en:'Escaped <script>'}
    ]}
  ],
  after_update:[{ko:'프로젝트 확인',en:'Check your projects'}]
};

test('Stable structured notes retain highlights in upgrade preview and full categories in history',()=>{
  for(const lang of ['ko','en']){
    const {preview,detail}=renderer(lang)(editorial);
    assert.equal((preview.match(/<li>/g)||[]).length,3);
    assert.equal((detail.match(/class="release-category"/g)||[]).length,2);
    assert.ok(!preview.includes('class="release-category"'));
    assert.match(preview,/class="release-highlights"/);
    assert.match(preview,lang==='ko'?/업데이트 후 확인 사항/:/After updating/);
    assert.match(detail,lang==='ko'?/성능·안정성/:/Performance and reliability/);
    assert.match(detail,/release-next-actions/);
    assert.ok(!detail.includes('<script>'));
  }
});

test('legacy section-only release notes keep their original renderer',()=>{
  const note={version:'0.2.51',date:'2026-10-02',
    summary:{ko:'기존 형식',en:'Legacy'},sections:[
      {type:'improved',items:[{ko:'원래 내용',en:'Existing content'}]}
    ]};
  for(const lang of ['ko','en']){
    const {preview,detail}=renderer(lang)(note);
    assert.match(preview,/release-sections/);
    assert.match(detail,/release-sections/);
    assert.ok(!detail.includes('release-categories'));
  }
});


test('Stable 0.3.1 keeps contributor-only Dev features exclusively in the last detailed category',()=>{
  const root=path.join(__dirname,'..');
  const note=JSON.parse(fs.readFileSync(path.join(root,'release-notes/0.3.1.json'),'utf8'));
  const embedded=JSON.parse(fs.readFileSync(path.join(root,'goassets/template/_task_mecca/framework/release-notes/current.json'),'utf8'));
  const index=JSON.parse(fs.readFileSync(path.join(root,'release-notes/index.json'),'utf8'));
  assert.deepEqual(note,embedded);
  assert.equal(index.releases[0].version,'0.3.1');
  assert.deepEqual(index.releases[0].title,note.title);
  assert.deepEqual(index.releases[0].summary,note.summary);
  assert.equal(note.categories.at(-1).id,'preview');
  assert.match(note.categories.at(-1).title.ko,/기여자/);
  assert.match(note.categories.at(-1).title.en,/Contributor/);
  for(const lang of ['ko','en']){
    const publicContents=[note.title[lang],note.summary[lang],
      ...note.highlights.map(h=>h[lang]),
      ...note.after_update.map(item=>item[lang]),
      ...(note.sections||[]).flatMap(s=>s.items.map(item=>item[lang]))].join(' ');
    assert.doesNotMatch(publicContents,/\bdev\b|preview|docker|PR\/SHA|기여자|개발 환경/i);
    const rendered=renderer(lang)(note);
    assert.doesNotMatch(rendered.preview,/\bdev\b|preview|docker|PR\/SHA|기여자|개발 환경/i);
    assert.match(rendered.detail,/PR Preview/);
    const marker=lang==='ko'?'기여자를 위한 개발 기능':'Contributor Features';
    const lastCategory=note.categories.at(-2).title[lang];
    assert.ok(rendered.detail.indexOf(marker)>rendered.detail.indexOf(lastCategory));
  }
});
