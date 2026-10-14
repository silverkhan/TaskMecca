package backlog

import (
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestBacklogInitialPageAndFastDetailTrackFileChanges(t *testing.T) {
 project:=t.TempDir()
 root:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(root,0755);err!=nil{t.Fatal(err)}
 for i:=1;i<=60;i++ {
  path:=filepath.Join(root,fmt.Sprintf("%06d.B-%d.test.done.md",i,i))
  if err:=os.WriteFile(path,[]byte(fmt.Sprintf("# B-%d Initial\n- Agent: -\n",i)),0644);err!=nil{t.Fatal(err)}
 }
 first,err:=BacklogInitialPage(project,root,1,20,nil,"","id_desc")
 if err!=nil{t.Fatal(err)}
 if first["pending_reconciliation"]!=true{t.Fatal("initial page was marked authoritative")}
 if first["total"]!=60||first["page_size"]!=20{t.Fatalf("invalid pagination: %+v",first)}
 items:=first["items"].([]map[string]any)
 if len(items)!=20||items[0]["id"]!="B-60"||items[0]["state_provisional"]!=true{t.Fatalf("initial page incorrect: %+v",items)}
 row,ok,err:=CachedTaskRecord(project,root,"B-59")
 if err!=nil||!ok||row.ID!="B-59"{t.Fatalf("missing indexed task: %+v %v %v",row,ok,err)}

 changed:=filepath.Join(root,"000059.B-59.test.done.md")
 if err:=os.WriteFile(changed,[]byte("# B-59 Changed body longer than before\n- Agent: -\n"),0644);err!=nil{t.Fatal(err)}
 detail,err:=TaskContent(project,root,"B-59")
 if err!=nil{t.Fatal(err)}
 if !strings.Contains(detail["raw_markdown"].(string),"Changed body"){t.Fatalf("cached detail is stale: %+v",detail)}
 if detail["content_only"]!=true {t.Fatal("content endpoint performed control reconciliation")}

 moved:=filepath.Join(root,"000059.B-59.test.hold.md")
 if err:=os.Rename(changed,moved);err!=nil{t.Fatal(err)}
 detail,err=TaskContent(project,root,"B-59")
 if err!=nil{t.Fatal(err)}
 if detail["file_state"]!="hold"{t.Fatalf("rename not observed: %+v",detail)}
 if err:=os.Remove(moved);err!=nil{t.Fatal(err)}
 if _,err:=TaskContent(project,root,"B-59");err==nil {t.Fatal("deleted task returned from stale cache")}
}

func TestBacklogInitialPageNeverClaimsReadyOrAttention(t *testing.T) {
 project:=t.TempDir()
 root:=filepath.Join(project,"_task_mecca","data","backlog")
 if err:=os.MkdirAll(root,0755);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(root,"000001.B-1.todo.todo.md"),[]byte("# B-1 Pending\n- 선행: B-2\n"),0644);err!=nil{t.Fatal(err)}
 page,err:=BacklogInitialPage(project,root,1,20,nil,"","id_desc")
 if err!=nil {t.Fatal(err)}
 item:=page["items"].([]map[string]any)[0]
 if item["state"]!="todo"||item["file_state"]!="todo"||item["state_provisional"]!=true {
  t.Fatalf("provisional status was promoted without control evidence: %+v",item)
 }
 for _,key:=range []string{"notification_events","attention_items","signal_revision"} {
  if _,exists:=page[key];exists {t.Fatalf("initial page must not provide canonical %s",key)}
 }
}
