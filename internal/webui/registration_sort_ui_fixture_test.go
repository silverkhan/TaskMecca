package webui

import (
	stdcontext "context"
	"encoding/json"
	"github.com/silverkhan/TaskMecca/goassets"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Evaluate shipped preference/query/cache/fallback functions without a DOM,
// network, native runtime or listener. Node is a selected fixture dependency.
func TestA95RegistrationSortUIFixture(t *testing.T) {
	app, err := goassets.Template.ReadFile(embeddedRoot + "/web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("selected fixture requires node:", err)
	}
	input, err := json.Marshal(map[string]string{"app": string(app), "fixture": a95RegistrationUIAssertions})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "-e", a95RegistrationUIHarness)
	cmd.Stdin = strings.NewReader(string(input))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registration UI fixture: %v\n%s", err, output)
	}
}

const a95RegistrationUIHarness = `
const fs=require('fs'),assert=require('assert');
const input=JSON.parse(fs.readFileSync(0,'utf8')),source=input.app;
function section(start,end){const a=source.indexOf(start),b=source.indexOf(end,a+start.length);assert(a>=0&&b>a,start);return source.slice(a,b);}
const helpers=source.slice(0,source.indexOf('const state ='));
const fallback=section('function allRowsForView()', 'function effectiveListPageSize()');
const query=section('function listQueryString()', '// Cache rows only.');
const contextKey=section('function listContextKey(', 'function listSourceKey(');
const preference=/listSort: ([^\n]+),/.exec(source)[1];
const translations=section('const I18N =', 'function t(')+section('function t(', '// Use the same project name');
const controls=section('function listControls(info)', 'function listView()');
const setup='const state={statusFilters:["all"],tagFilters:[],query:"",listData:null,listPage:1,backlog:"",project:"fixture"};'+
 'const localStorage={getItem:()=>stored};let stored=null;function initialSort(){return '+preference+';}'+
 'function effectiveListPageSize(){return 2;}function updatedAt(task){return task.updated_at;}function matchesStatusFilter(task,key){return task.state===key;}'+
 'function esc(value){return String(value);}function currentProjectData(){return {};}function backlogListPicker(){return "";}function statusFilterBar(){return "";}function tagFilterBar(){return "";}function backlogListTools(){return "";}';
new Function('assert',helpers+fallback+query+contextKey+translations+controls+setup+input.fixture)(assert);
`

const a95RegistrationUIAssertions = `
// Read actual dictionaries and render the shipped selector; source presence
// alone cannot detect duplicate keys overriding KO or missing EN translations.
for(const [language,label] of [['ko','최근 등록순'],['en','Recently registered']]){
 assert.equal(I18N[language].registeredNewest,label);
 state.language=language;state.listSort='recently_registered';state.listPageSize=20;
 assert.equal(t('registeredNewest'),label);
 const html=listControls({page:1,pages:3,total:6,pageSize:20});
 const selector=/<select id="listSort">([\s\S]*?)<\/select>/.exec(html);
 assert(selector,'rendered sort selector');
 assert(selector[1].includes('value="recently_registered" selected>'+label+'</option>'));
 for(const key of ['id_desc','id_asc','updated_desc','updated_asc'])assert(selector[1].includes('value="'+key+'"'));
}
for(const at of ['2026-02-31T00:00:00Z','2025-02-29T00:00:00.000000100Z','1900-02-29T00:00:00Z','2026-04-31T00:00:00+09:00','2026-00-01T00:00:00Z','2026-13-01T00:00:00Z','2026-01-00T00:00:00Z','2026-01-01T24:00:00Z','2026-01-01T00:60:00Z','2026-01-01T00:00:60Z','2026-01-01T00:00:00+24:00','2026-01-01T00:00:00+00:60']){
 assert.equal(normalizedRegistrationTimestamp(at),'',at);
 assert.equal(registrationSortKey({registration_sort_key:at,lifecycle:{events:[{kind:'registered',source:'durable_lifecycle',at}]}}),'',at);
}
const leap='2024-02-29T23:59:59.000000123Z';
assert.equal(normalizedRegistrationTimestamp(leap),leap);
assert.equal(normalizedRegistrationTimestamp('2024-03-01T08:59:59.000000123+09:00'),leap);
assert.equal(normalizedRegistrationTimestamp('2000-02-29T00:00:00Z'),'2000-02-29T00:00:00.000000000Z');
assert(normalizedRegistrationTimestamp('2024-02-29T23:59:59.000000124Z')>leap);
const event=(at,source='durable_lifecycle',kind='registered')=>({at,source,kind});
const tasks=[
 {id:'A-900',sort_key:'000900',updated_at:'2026-10-02',lifecycle:{events:[event('2026-10-01T00:00:00.000000100Z'),event('2026-10-04T00:00:00Z')]}},
 {id:'B-1',sort_key:'000001',updated_at:'2026-10-01',registration_sort_key:'2026-10-01T00:00:00.000000300Z'},
 {id:'A-2',sort_key:'000002',updated_at:'2026-10-01',lifecycle:{events:[event('2026-10-01T09:00:00.000000200+09:00')]}},
 {id:'C-2',sort_key:'000003',updated_at:'2026-10-01',lifecycle:{events:[event('2026-10-01T00:00:00.000000200Z')]}},
 {id:'D-8',sort_key:'000008',updated_at:'2026-10-01',created_at:'2099-01-01',lifecycle:{events:[event('2099-01-01T00:00:00Z','file_observation'),event('2099-01-01T00:00:00Z','durable_lifecycle','assigned')]}},
 {id:'E-3',sort_key:'000009',updated_at:'2026-10-01',registration_sort_key:'invalid',lifecycle:{events:[event('invalid')]}},
];
state.snapshot={all_items:Object.fromEntries(tasks.map(task=>[task.id,task]))};
for(stored of [null,'','invalid','recently_registered']){
 state.listSort=initialSort();assert.equal(state.listSort,'recently_registered');
 assert.deepEqual(allRowsForView().map(task=>task.id),['B-1','A-2','C-2','A-900','D-8','E-3']);
 assert.equal(new URLSearchParams(listQueryString()).get('sort'),'recently_registered');
}
const registrationContext=listContextKey();
for(const [key,first] of Object.entries({id_desc:'A-900',id_asc:'B-1',updated_desc:'A-900',updated_asc:'B-1'})){
 stored=key;state.listSort=initialSort();assert.equal(state.listSort,key);
 assert.equal(allRowsForView()[0].id,first);
 assert.equal(new URLSearchParams(listQueryString()).get('sort'),key);
 assert.notEqual(listContextKey(),registrationContext);
}
state.listSort='invalid';assert.equal(new URLSearchParams(listQueryString()).get('sort'),'recently_registered');
assert.deepEqual(allRowsForView().map(task=>task.id),['B-1','A-2','C-2','A-900','D-8','E-3']);
assert.equal(registrationSortKey(tasks[2]),registrationSortKey(tasks[3]));
assert(registrationSortKey(tasks[1])>registrationSortKey(tasks[2]));
state.query='C-2';assert.deepEqual(allRowsForView().map(task=>task.id),['C-2']);
state.query='';state.statusFilters=['done'];tasks[1].state='done';assert.deepEqual(allRowsForView().map(task=>task.id),['B-1']);
state.listData={items:[tasks[2],tasks[1]]};assert.deepEqual(allRowsForView().map(task=>task.id),['A-2','B-1']);
`
