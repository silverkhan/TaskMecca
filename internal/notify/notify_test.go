package notify

import (
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
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
 event:=Event{ID:"event-1",TaskID:"AID-39",Kind:"stalled",Title:"Runtime sensing",Message:"no activity",At:time.Now().UTC().Add(time.Second).Format(time.RFC3339Nano)}
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

func TestDefaultLifecycleKindsAreAllEnabled(t *testing.T){
 kinds:=defaultKinds()
 for _,kind:=range []string{"registered","started","intervention","approval","stalled","interrupted","runtime_unknown","finalize","completed"} {
  if !kinds[kind] { t.Fatalf("expected %s enabled by default: %v",kind,kinds) }
 }
}
func TestExistingTelegramConfigGainsNewLifecycleDefaults(t *testing.T){
 project:=t.TempDir()
 if err:=saveTelegram(project,TelegramConfig{Token:"x",Kinds:map[string]bool{"stalled":false}});err!=nil{t.Fatal(err)}
 cfg,err:=loadTelegram(project);if err!=nil{t.Fatal(err)}
 if cfg.Kinds["stalled"] {t.Fatal("explicit disabled kind was overwritten")}
 if !cfg.Kinds["started"]||!cfg.Kinds["finalize"]||!cfg.Kinds["approval"] {t.Fatalf("new defaults missing: %v",cfg.Kinds)}
}


func TestTelegramDeliversLifecycleSequenceExactlyOnce(t *testing.T){
 project:=t.TempDir()
 sent:=[]string{}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  method:=r.URL.Path[strings.LastIndex(r.URL.Path,"/")+1:]
  w.Header().Set("Content-Type","application/json")
  switch method {
  case "sendMessage":
   var body map[string]any
   if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Fatal(err)}
   sent=append(sent,body["text"].(string))
   json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":len(sent)}})
  default: http.Error(w,"unknown",404)
  }
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()
 if err:=saveTelegram(project,TelegramConfig{Token:"token",ChatID:123,Enabled:true,Kinds:defaultKinds(),ActivatedAt:"2026-10-05T09:00:00Z"});err!=nil{t.Fatal(err)}
 events:=[]Event{
  {ID:"reg",TaskID:"B-500",Kind:"registered",Title:"Lifecycle",At:"2026-10-05T10:00:00Z"},
  {ID:"start",TaskID:"B-500",Kind:"started",Title:"Lifecycle",At:"2026-10-05T10:01:00Z"},
  {ID:"done",TaskID:"B-500",Kind:"completed",Title:"Lifecycle",At:"2026-10-05T10:02:00Z"},
 }
 if errs:=Deliver(project,events);len(errs)>0{t.Fatal(errs)}
 if errs:=Deliver(project,events);len(errs)>0{t.Fatal(errs)}
 if len(sent)!=3{t.Fatalf("sent=%d want 3: %v",len(sent),sent)}
 if !strings.HasPrefix(sent[0],"📝 작업 등록")||!strings.HasPrefix(sent[1],"▶️ 작업 착수")||!strings.HasPrefix(sent[2],"✅ 작업 완료"){
  t.Fatalf("unexpected lifecycle messages/order: %v",sent)
 }
}


func TestTelegramAllSupportedKindsHaveUserFacingLabelsAndDedupe(t *testing.T){
 project:=t.TempDir()
 sent:=[]string{}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  var body map[string]any
  if err:=json.NewDecoder(r.Body).Decode(&body);err!=nil{t.Fatal(err)}
  sent=append(sent,body["text"].(string))
  w.Header().Set("Content-Type","application/json")
  json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":len(sent)}})
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()
 if err:=saveTelegram(project,TelegramConfig{Token:"token",ChatID:123,Enabled:true,Kinds:defaultKinds(),ActivatedAt:"2026-10-05T11:00:00Z"});err!=nil{t.Fatal(err)}
 expected:=[]struct{kind,prefix string}{
  {"registered","📝 작업 등록"},{"started","▶️ 작업 착수"},{"intervention","🙋 사용자 개입 필요"},
  {"approval","🔐 승인 필요"},{"stalled","⏳ 작업 정체 확인 필요"},{"interrupted","⚠️ 실행 중단/오류"},
  {"runtime_unknown","❓ 실행 상태 확인 필요"},{"finalize","📌 완료 처리 필요"},{"completed","✅ 작업 완료"},
 }
 events:=make([]Event,0,len(expected))
 for i,row:=range expected {
  events=append(events,Event{ID:fmt.Sprintf("all-%d",i),TaskID:"B-700",Kind:row.kind,Title:"All kinds",At:fmt.Sprintf("2026-10-05T12:%02d:00Z",i)})
 }
 if errs:=Deliver(project,events);len(errs)>0{t.Fatal(errs)}
 if errs:=Deliver(project,events);len(errs)>0{t.Fatal(errs)}
 if len(sent)!=len(expected){t.Fatalf("sent=%d want=%d",len(sent),len(expected))}
 for i,row:=range expected {
  if !strings.HasPrefix(sent[i],row.prefix){t.Fatalf("kind=%s message=%q",row.kind,sent[i])}
 }
}


func TestTelegramLegacyConfigBaselinesExistingEvents(t *testing.T){
 project:=t.TempDir()
 sent:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  sent++
  w.Header().Set("Content-Type","application/json")
  json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":sent}})
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()

 if err:=saveTelegram(project,TelegramConfig{Token:"token",ChatID:123,Enabled:true,Kinds:defaultKinds()});err!=nil{t.Fatal(err)}
 oldEvent:=Event{ID:"old-1",TaskID:"B-900",Kind:"started",Title:"Old",At:"2026-10-05T10:00:00Z"}
 if errs:=Deliver(project,[]Event{oldEvent});len(errs)>0{t.Fatal(errs)}
 if sent!=0{t.Fatalf("legacy activation must not backfill; sent=%d",sent)}

 cfg,err:=loadTelegram(project);if err!=nil{t.Fatal(err)}
 if cfg.ActivatedAt==""{t.Fatal("activation watermark was not established")}
 if len(cfg.Delivered)!=1||cfg.Delivered[0]!="old-1"{t.Fatalf("historical event was not consumed: %+v",cfg.Delivered)}

 cutover,err:=time.Parse(time.RFC3339Nano,cfg.ActivatedAt);if err!=nil{t.Fatal(err)}
 live:=Event{ID:"live-1",TaskID:"B-900",Kind:"completed",Title:"Live",At:cutover.Add(time.Second).Format(time.RFC3339Nano)}
 if errs:=Deliver(project,[]Event{oldEvent,live});len(errs)>0{t.Fatal(errs)}
 if sent!=1{t.Fatalf("live event after cutover should deliver once; sent=%d",sent)}
}

func TestTelegramDisabledKindIsConsumedWithoutLaterBackfill(t *testing.T){
 project:=t.TempDir()
 sent:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  sent++
  w.Header().Set("Content-Type","application/json")
  json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":sent}})
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()

 kinds:=defaultKinds();kinds["started"]=false
 if err:=saveTelegram(project,TelegramConfig{Token:"token",ChatID:123,Enabled:true,Kinds:kinds,ActivatedAt:"2026-10-05T09:00:00Z"});err!=nil{t.Fatal(err)}
 event:=Event{ID:"disabled-1",TaskID:"B-901",Kind:"started",Title:"Disabled",At:"2026-10-05T10:00:00Z"}
 if errs:=Deliver(project,[]Event{event});len(errs)>0{t.Fatal(errs)}
 if sent!=0{t.Fatalf("disabled kind should not send; sent=%d",sent)}

 cfg,err:=loadTelegram(project);if err!=nil{t.Fatal(err)}
 cfg.Kinds["started"]=true
 if err:=saveTelegram(project,cfg);err!=nil{t.Fatal(err)}
 if errs:=Deliver(project,[]Event{event});len(errs)>0{t.Fatal(errs)}
 if sent!=0{t.Fatalf("re-enabled kind must not backfill previously consumed event; sent=%d",sent)}
}

func TestTelegramDeliveryFailureRetriesSameEligibleEvent(t *testing.T){
 project:=t.TempDir()
 attempts:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  attempts++
  w.Header().Set("Content-Type","application/json")
  if attempts==1{
   json.NewEncoder(w).Encode(map[string]any{"ok":false,"description":"temporary failure"})
   return
  }
  json.NewEncoder(w).Encode(map[string]any{"ok":true,"result":map[string]any{"message_id":1}})
 }))
 defer server.Close()
 old:=telegramAPIBase;telegramAPIBase=server.URL;defer func(){telegramAPIBase=old}()

 if err:=saveTelegram(project,TelegramConfig{Token:"token",ChatID:123,Enabled:true,Kinds:defaultKinds(),ActivatedAt:"2026-10-05T09:00:00Z"});err!=nil{t.Fatal(err)}
 event:=Event{ID:"retry-1",TaskID:"B-902",Kind:"completed",Title:"Retry",At:"2026-10-05T10:00:00Z"}
 if errs:=Deliver(project,[]Event{event});len(errs)!=1{t.Fatalf("first failure errs=%v",errs)}
 cfg,err:=loadTelegram(project);if err!=nil{t.Fatal(err)}
 if len(cfg.Delivered)!=0{t.Fatalf("failed event must remain unconsumed: %+v",cfg.Delivered)}

 if errs:=Deliver(project,[]Event{event});len(errs)>0{t.Fatal(errs)}
 if attempts!=2{t.Fatalf("attempts=%d want 2",attempts)}
 cfg,err=loadTelegram(project);if err!=nil{t.Fatal(err)}
 if len(cfg.Delivered)!=1||cfg.Delivered[0]!="retry-1"{t.Fatalf("successful retry not persisted: %+v",cfg.Delivered)}
}
