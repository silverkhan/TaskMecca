package notify

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
)

func TestTelegramWebSetupAndDurableDelivery(t *testing.T){
 project:=t.TempDir()
 sent:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  method:=r.URL.Path[strings.LastIndex(r.URL.Path,"/")+1:]
  w.Header().Set("Content-Type","application/json")
  switch method {
  case "getMe": json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"username":"task_mecca_test_bot"}})
  case "getUpdates": json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":[]any{map[string]any{"update_id":1,"message":map[string]any{"text":"/start","chat":map[string]any{"id":12345,"type":"private"}}}}})
  case "sendMessage": sent++;json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":sent}})
  default: http.Error(w,"unknown",404)
  }
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()

 st,err:=ConfigureTelegram(project,"token",nil);if err!=nil{t.Fatal(err)}
 if !st.Configured||st.Connected||st.BotUsername!="task_mecca_test_bot"{t.Fatalf("configured=%+v",st)}
 st,err=DiscoverTelegramChat(project);if err!=nil{t.Fatal(err)}
 if !st.Connected||!st.Enabled||st.ChatID!="12345"{t.Fatalf("connected=%+v",st)}
 event:=Event{ID:"event-1",TaskID:"AID-39",Kind:"stalled",Title:"Runtime sensing",Message:"no activity",At:"2026-10-03T00:00:00Z"}
 if errs:=Deliver(project,[]Event{event});len(errs)>0{t.Fatal(errs)}
 if errs:=Deliver(project,[]Event{event});len(errs)>0{t.Fatal(errs)}
 if sent!=1{t.Fatalf("send count=%d want 1",sent)}
 if err:=TestTelegram(project);err!=nil{t.Fatal(err)}
 if sent!=2{t.Fatalf("send count after test=%d want 2",sent)}
 st,err=TelegramStatusFor(project);if err!=nil{t.Fatal(err)}
 if !st.Connected||st.BotUsername!="task_mecca_test_bot"{t.Fatalf("status=%+v",st)}
}

func TestTelegramStatusDoesNotExposeToken(t *testing.T){
 project:=t.TempDir()
 if err:=saveTelegram(project,TelegramConfig{Token:"secret-token",ChatID:7,Enabled:true,Kinds:defaultKinds()});err!=nil{t.Fatal(err)}
 st,err:=TelegramStatusFor(project);if err!=nil{t.Fatal(err)}
 data,_:=json.Marshal(st)
 if strings.Contains(string(data),"secret-token"){t.Fatalf("status leaked token: %s",data)}
}
