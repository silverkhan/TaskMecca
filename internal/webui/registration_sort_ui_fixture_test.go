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
assert(source.includes('value="recently_registered"'));
assert(source.includes("registeredNewest:'최근 등록순'"));
assert(source.includes("registeredNewest:'Recently registered'"));
const setup='const state={statusFilters:["all"],tagFilters:[],query:"",listData:null,listPage:1,backlog:"",project:"fixture"};'+
 'const localStorage={getItem:()=>stored};let stored=null;function initialSort(){return '+preference+';}'+
 'function effectiveListPageSize(){return 2;}function updatedAt(task){return task.updated_at;}function matchesStatusFilter(task,key){return task.state===key;}';
new Function('assert',helpers+fallback+query+contextKey+setup+input.fixture)(assert);
`

const a95RegistrationUIAssertions = `
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
