package notify

import (
 "bytes"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "sync"
 "time"
)

type Event struct { ID,TaskID,Kind,Title,Message,ResumeCondition,At string }
type Channel interface { Name() string; Deliver(Event) error }

type TelegramConfig struct {
 Token string `json:"token,omitempty"`
 ChatID int64 `json:"chat_id,omitempty"`
 BotUsername string `json:"bot_username,omitempty"`
 Enabled bool `json:"enabled"`
 Kinds map[string]bool `json:"kinds,omitempty"`
 Delivered []string `json:"delivered,omitempty"`
}
type TelegramStatus struct {
 Configured bool `json:"configured"`
 Connected bool `json:"connected"`
 Enabled bool `json:"enabled"`
 BotUsername string `json:"bot_username,omitempty"`
 ChatID string `json:"chat_id,omitempty"`
 Kinds map[string]bool `json:"kinds"`
}
type telegramChannel struct{ cfg TelegramConfig }
func (t telegramChannel) Name() string { return "telegram" }
func (t telegramChannel) Deliver(e Event) error {
 if !t.cfg.Enabled||t.cfg.ChatID==0||!t.cfg.Kinds[e.Kind] { return nil }
 head:="[Task Mecca] "+e.TaskID
 if e.Title!="" { head+=" · "+e.Title }
 labels:=map[string]string{
  "registered":"📝 작업 등록","started":"▶️ 작업 착수","intervention":"🙋 사용자 개입 필요","approval":"🔐 승인 필요",
  "stalled":"⏳ 작업 정체 확인 필요","interrupted":"⚠️ 실행 중단/오류","runtime_unknown":"❓ 실행 상태 확인 필요",
  "finalize":"📌 완료 처리 필요","completed":"✅ 작업 완료",
 }
 label:=labels[e.Kind]; if label=="" { label=e.Kind }
 lines:=[]string{label,head}
 if e.Message!="" { lines=append(lines,e.Message) }
 if e.ResumeCondition!="" { lines=append(lines,"➡️ 다음 조치: "+e.ResumeCondition) }
 return telegramCall(t.cfg.Token,"sendMessage",map[string]any{"chat_id":t.cfg.ChatID,"text":strings.Join(lines,"\n")},nil)
}

var telegramMu sync.Mutex
var telegramAPIBase = "https://api.telegram.org"
func telegramPath(project string) string { return filepath.Join(project,"_task_mecca",".runtime","notifications","telegram.json") }
func defaultKinds() map[string]bool { return map[string]bool{
 "registered":true,"started":true,"intervention":true,"approval":true,"stalled":true,
 "interrupted":true,"runtime_unknown":true,"finalize":true,"completed":true,
} }
func mergeDefaultKinds(kinds map[string]bool) map[string]bool {
 out:=defaultKinds(); for kind,enabled:=range kinds { out[kind]=enabled }; return out
}
func loadTelegram(project string)(TelegramConfig,error){
 cfg:=TelegramConfig{Kinds:defaultKinds()}; data,err:=os.ReadFile(telegramPath(project))
 if errors.Is(err,os.ErrNotExist){return cfg,nil}; if err!=nil{return cfg,err}
 if err=json.Unmarshal(data,&cfg);err!=nil{return cfg,err}; cfg.Kinds=mergeDefaultKinds(cfg.Kinds); return cfg,nil
}
func saveTelegram(project string,cfg TelegramConfig)error{
 path:=telegramPath(project); if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 data,err:=json.MarshalIndent(cfg,"","  ");if err!=nil{return err}; tmp:=path+".tmp"
 if err=os.WriteFile(tmp,append(data,'\n'),0600);err!=nil{return err};if err=os.Rename(tmp,path);err!=nil{return err};return os.Chmod(path,0600)
}
func status(cfg TelegramConfig)TelegramStatus{
 chat:="";if cfg.ChatID!=0{chat=fmt.Sprintf("%d",cfg.ChatID)}
 return TelegramStatus{Configured:cfg.Token!="",Connected:cfg.Token!=""&&cfg.ChatID!=0,Enabled:cfg.Enabled,BotUsername:cfg.BotUsername,ChatID:chat,Kinds:cfg.Kinds}
}
func TelegramStatusFor(project string)(TelegramStatus,error){telegramMu.Lock();defer telegramMu.Unlock();cfg,err:=loadTelegram(project);if err!=nil{return TelegramStatus{},err};return status(cfg),nil}

type telegramEnvelope struct{ OK bool `json:"ok"`; Description string `json:"description,omitempty"`; Result json.RawMessage `json:"result,omitempty"` }
func telegramCall(token,method string,body,out any)error{
 token=strings.TrimSpace(token);if token==""{return errors.New("telegram bot token is empty")}
 data,err:=json.Marshal(body);if err!=nil{return err};client:=&http.Client{Timeout:8*time.Second}
 req,err:=http.NewRequest(http.MethodPost,strings.TrimRight(telegramAPIBase,"/")+"/bot"+token+"/"+method,bytes.NewReader(data));if err!=nil{return err};req.Header.Set("Content-Type","application/json")
 resp,err:=client.Do(req);if err!=nil{return err};defer resp.Body.Close();var env telegramEnvelope
 if err=json.NewDecoder(resp.Body).Decode(&env);err!=nil{return err};if !env.OK{if env.Description!=""{return errors.New(env.Description)};return fmt.Errorf("telegram %s failed",method)}
 if out!=nil&&len(env.Result)>0{return json.Unmarshal(env.Result,out)};return nil
}
func ConfigureTelegram(project,token string,kinds map[string]bool)(TelegramStatus,error){
 telegramMu.Lock();defer telegramMu.Unlock();var me struct{Username string `json:"username"`}
 if err:=telegramCall(token,"getMe",map[string]any{},&me);err!=nil{return TelegramStatus{},err};cfg,err:=loadTelegram(project);if err!=nil{return TelegramStatus{},err}
 cfg.Token=strings.TrimSpace(token);cfg.BotUsername=me.Username;cfg.ChatID=0;cfg.Enabled=false;cfg.Delivered=nil;if kinds!=nil{cfg.Kinds=kinds}
 if err=saveTelegram(project,cfg);err!=nil{return TelegramStatus{},err};return status(cfg),nil
}
func DiscoverTelegramChat(project string)(TelegramStatus,error){
 telegramMu.Lock();defer telegramMu.Unlock();cfg,err:=loadTelegram(project);if err!=nil{return TelegramStatus{},err};if cfg.Token==""{return TelegramStatus{},errors.New("telegram bot is not configured")}
 var updates []struct{UpdateID int64 `json:"update_id"`;Message *struct{Text string `json:"text"`;Chat struct{ID int64 `json:"id"`;Type string `json:"type"`} `json:"chat"`} `json:"message"`}
 if err=telegramCall(cfg.Token,"getUpdates",map[string]any{"limit":100,"timeout":0,"allowed_updates":[]string{"message"}},&updates);err!=nil{return TelegramStatus{},err}
 var chatID int64;for i:=len(updates)-1;i>=0;i--{if updates[i].Message!=nil&&updates[i].Message.Chat.Type=="private"{chatID=updates[i].Message.Chat.ID;if strings.HasPrefix(strings.TrimSpace(updates[i].Message.Text),"/start"){break}}}
 if chatID==0{return TelegramStatus{},errors.New("no private Telegram chat found; send /start to the bot first")}
 cfg.ChatID=chatID;cfg.Enabled=true;if err=saveTelegram(project,cfg);err!=nil{return TelegramStatus{},err};return status(cfg),nil
}
func TestTelegram(project string)error{telegramMu.Lock();defer telegramMu.Unlock();cfg,err:=loadTelegram(project);if err!=nil{return err};if cfg.Token==""||cfg.ChatID==0{return errors.New("telegram bot is not connected")};return telegramCall(cfg.Token,"sendMessage",map[string]any{"chat_id":cfg.ChatID,"text":"🔔 [Task Mecca] 테스트 알림\nTelegram 알림 연결이 정상입니다."},nil)}
func DisableTelegram(project string)error{telegramMu.Lock();defer telegramMu.Unlock();err:=os.Remove(telegramPath(project));if errors.Is(err,os.ErrNotExist){return nil};return err}
func UpdateTelegramKinds(project string,kinds map[string]bool)(TelegramStatus,error){telegramMu.Lock();defer telegramMu.Unlock();cfg,err:=loadTelegram(project);if err!=nil{return TelegramStatus{},err};if cfg.Token==""{return TelegramStatus{},errors.New("telegram bot is not configured")};cfg.Kinds=kinds;if err=saveTelegram(project,cfg);err!=nil{return TelegramStatus{},err};return status(cfg),nil}
func Deliver(project string,events []Event)[]error{
 telegramMu.Lock();defer telegramMu.Unlock();cfg,err:=loadTelegram(project);if err!=nil{return []error{err}};if !cfg.Enabled||cfg.ChatID==0{return nil}
 seen:=map[string]bool{};for _,id:=range cfg.Delivered{seen[id]=true};errs:=[]error{};dirty:=false;ch:=telegramChannel{cfg}
 sort.Slice(events,func(i,j int)bool{return events[i].At<events[j].At})
 for _,e:=range events{if e.ID==""||seen[e.ID]||!cfg.Kinds[e.Kind]{continue};if err:=ch.Deliver(e);err!=nil{errs=append(errs,err);continue};seen[e.ID]=true;cfg.Delivered=append(cfg.Delivered,e.ID);dirty=true}
 if len(cfg.Delivered)>250{cfg.Delivered=append([]string{},cfg.Delivered[len(cfg.Delivered)-250:]...);dirty=true};if dirty{if err:=saveTelegram(project,cfg);err!=nil{errs=append(errs,err)}};return errs
}
