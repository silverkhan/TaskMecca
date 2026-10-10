package notify

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
 "unicode/utf8"
)

func TestTelegramMachineNameSanitizesAlias(t *testing.T) {
 t.Setenv("TASK_MECCA_MACHINE_NAME","\n 회사\r\nMac \t#1\x1b")
 if got:=telegramMachineName();got!="회사 Mac #1" {
  t.Fatalf("alias was not normalized: %q",got)
 }
 t.Setenv("TASK_MECCA_MACHINE_NAME",strings.Repeat("가",75))
 if got:=telegramMachineName();utf8.RuneCountInString(got)!=48 {
  t.Fatalf("alias length=%d want 48",utf8.RuneCountInString(got))
 }
 t.Setenv("TASK_MECCA_MACHINE_NAME","\x1b")
 if got:=telegramMachineName();got!="알 수 없는 컴퓨터"{
  t.Fatalf("unprintable alias=%q",got)
 }
 t.Setenv("TASK_MECCA_MACHINE_NAME","")
 hostname,err:=os.Hostname()
 if err==nil && strings.TrimSpace(hostname)!="" {
  if got:=telegramMachineName();got==""||got=="알 수 없는 컴퓨터"{
   t.Fatalf("default machine hostname missing: %q",got)
  }
 }
}

func TestTelegramSameBotIdentifiesMachineWithoutChangingDeliveryIdentity(t *testing.T) {
 var messages []string
 var recipients []float64
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  var req map[string]any
  if err:=json.NewDecoder(r.Body).Decode(&req);err!=nil{t.Error(err)}
  messages=append(messages,req["text"].(string))
  recipients=append(recipients,req["chat_id"].(float64))
  w.Header().Set("Content-Type","application/json")
  json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":len(messages)}})
 }))
 defer server.Close()
 old:=telegramAPIBase
 telegramAPIBase=server.URL
 defer func(){telegramAPIBase=old}()
 now:=time.Now().UTC()
 for i,machine:=range []string{"회사-Mac","집-Windows"}{
  t.Setenv("TASK_MECCA_MACHINE_NAME",machine)
  project:=filepath.Join(t.TempDir(),"프로젝트-동일명")
  if err:=os.MkdirAll(project,0755);err!=nil{t.Fatal(err)}
  if err:=saveTelegram(project,TelegramConfig{Token:"same-bot",ChatID:321,Enabled:true,Kinds:defaultKinds(),ActivatedAt:now.Add(-time.Minute).Format(time.RFC3339Nano)});err!=nil{t.Fatal(err)}
  evt:=Event{ID:"same-event",TaskID:"B-8",Kind:"completed",Title:"B-8 완료 보고",Message:"상태 변경",At:now.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano)}
  if errs:=Deliver(project,[]Event{evt});len(errs)!=0{t.Fatal(errs)}
  if errs:=Deliver(project,[]Event{evt});len(errs)!=0{t.Fatal(errs)}
  if len(messages)!=i+1{t.Fatalf("duplicate or missing machine delivery: %v",messages)}
  expected:="💻 컴퓨터: "+machine+"\n[프로젝트-동일명] B-8 · 완료 보고"
  if !strings.Contains(messages[i],expected)||!strings.HasPrefix(messages[i],"✅ 작업 완료") {
   t.Fatalf("incorrect machine/project/task context: %q",messages[i])
  }
  records,err:=DeliveryRecords(project,"B-8","same-event")
  if err!=nil||len(records)!=1||records[0].State!="sent"||records[0].Attempts!=1 {
   t.Fatalf("delivery ledger affected by name: %+v %v",records,err)
  }
  if err:=TestTelegram(project);err!=nil{t.Fatal(err)}
  testText:=messages[len(messages)-1]
  if !strings.Contains(testText,"💻 컴퓨터: "+machine+"\nTelegram 알림 연결이 정상입니다.") {
   t.Fatalf("test notification missing machine: %q",testText)
  }
  if !strings.Contains(testText,"[프로젝트-동일명] 테스트 알림"){t.Fatalf("test notification lost project: %q",testText)}
  messages=append(messages[:i+1],messages[i+2:]...)
 }
 if len(messages)!=2||len(recipients)!=4{t.Fatalf("unexpected deliveries %d messages, %d recipients",len(messages),len(recipients))}
 for _,recipient:=range recipients{if recipient!=321{t.Fatalf("recipient changed: %v",recipients)}}
}
