package webui

import (
 "crypto/rand"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "sync"
 "time"

 "github.com/silverkhan/TaskMecca/internal/backlog"
 "github.com/silverkhan/TaskMecca/internal/projectguard"
)

// projectguard serializes other processes; this mutex also serializes goroutines
// in this server process (OS file locks alone are not goroutine locks).
var webDeliveryMu sync.Mutex

const webDeliveryTTL = 5 * time.Minute
const webDeliveryLease = 45 * time.Second
const webDeliveryMaxAttempts = 3

// Browsers are competing display surfaces for one project/user logical
// recipient. Browser localStorage is not an authoritative delivery ledger.
type webDeliveryEntry struct {
 EventAt string `json:"event_at,omitempty"`
 State string `json:"state"`
 Token string `json:"token,omitempty"`
 ClaimedAt string `json:"claimed_at,omitempty"`
 ExpiresAt string `json:"expires_at,omitempty"`
 SentAt string `json:"sent_at,omitempty"`
 Client string `json:"client,omitempty"`
 Attempts int `json:"attempts"`
}
type webDeliveryLedger struct {
 Version int `json:"version"`
 ActivatedAt string `json:"activated_at"`
 Entries map[string]webDeliveryEntry `json:"entries"`
}
type webDeliveryDecision struct {
 State string `json:"state"`
 Granted bool `json:"granted"`
 Token string `json:"token,omitempty"`
}
func webDeliveryPath(project string) string {
 return filepath.Join(project,"_task_mecca",".runtime","notifications","web_delivery.json")
}
func readWebDelivery(project string) (webDeliveryLedger,bool,error) {
 data,err:=os.ReadFile(webDeliveryPath(project))
 if errors.Is(err,os.ErrNotExist) {
  return webDeliveryLedger{Version:1,Entries:map[string]webDeliveryEntry{}},false,nil
 }
 if err!=nil{return webDeliveryLedger{},false,err}
 var ledger webDeliveryLedger
 if err:=json.Unmarshal(data,&ledger);err!=nil{return webDeliveryLedger{},false,fmt.Errorf("web delivery ledger corrupt (no replay): %w",err)}
 if ledger.Version!=1||ledger.ActivatedAt==""{return webDeliveryLedger{},false,errors.New("invalid web delivery ledger (no replay)")}
 if ledger.Entries==nil{ledger.Entries=map[string]webDeliveryEntry{}}
 return ledger,true,nil
}
func saveWebDelivery(project string,ledger webDeliveryLedger)error{
 path:=webDeliveryPath(project)
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 data,err:=json.MarshalIndent(ledger,"","  ");if err!=nil{return err}
 temp:=path+".tmp"
 if err:=os.WriteFile(temp,append(data,'\n'),0600);err!=nil{return err}
 if err:=os.Rename(temp,path);err!=nil{return err}
 return os.Chmod(path,0600)
}
func initWebDelivery(project string)error{
 webDeliveryMu.Lock();defer webDeliveryMu.Unlock()
 unlock,err:=projectguard.AcquireWrite(project);if err!=nil{return err};defer unlock()
 ledger,exists,err:=readWebDelivery(project);if err!=nil||exists{return err}
 // Installing the new protocol must never replay any pre-existing events.
 ledger.ActivatedAt=time.Now().UTC().Format(time.RFC3339Nano)
 return saveWebDelivery(project,ledger)
}
func webDeliveryNonce()(string,error){
 var token [16]byte
 if _,err:=rand.Read(token[:]);err!=nil{return "",err}
 return hex.EncodeToString(token[:]),nil
}
func webClient(value string)string{
 switch strings.ToLower(strings.TrimSpace(value)){
 case "chrome","safari","edge","samsung","firefox","other":return strings.ToLower(strings.TrimSpace(value))
 default:return "other"
 }
}
// applyWebDelivery is serialized across processes by projectguard. An ack
// proves only showNotification/new Notification was requested successfully,
// not receipt or reading by the operating system.
func applyWebDelivery(project,action,eventID,token,client string)(webDeliveryDecision,error){
 webDeliveryMu.Lock();defer webDeliveryMu.Unlock()
 unlock,err:=projectguard.AcquireWrite(project);if err!=nil{return webDeliveryDecision{},err};defer unlock()
 ledger,exists,err:=readWebDelivery(project);if err!=nil{return webDeliveryDecision{},err}
 now:=time.Now().UTC()
 if !exists{
  ledger.ActivatedAt=now.Format(time.RFC3339Nano)
  if err:=saveWebDelivery(project,ledger);err!=nil{return webDeliveryDecision{},err}
 }
 if action=="init"{return webDeliveryDecision{State:"ready"},nil}
 if eventID==""||len(eventID)>128{return webDeliveryDecision{},errors.New("invalid event id")}
 entry,wasSeen:=ledger.Entries[eventID]
 if action=="ack"||action=="fail"{
  if !wasSeen||token==""||entry.Token!=token||entry.State!="claimed"{return webDeliveryDecision{State:"not_claimed"},nil}
  if action=="ack"{entry.State="display_requested";entry.SentAt=now.Format(time.RFC3339Nano)}else{entry.State="failed"}
  entry.Token=""
  ledger.Entries[eventID]=entry
  if err:=saveWebDelivery(project,ledger);err!=nil{return webDeliveryDecision{},err}
  return webDeliveryDecision{State:entry.State},nil
 }
 if action!="claim"{return webDeliveryDecision{},errors.New("invalid action")}
 if wasSeen {
  if entry.State=="display_requested"||entry.State=="uncertain"{return webDeliveryDecision{State:entry.State},nil}
  if entry.State=="claimed"{
   expires,err:=time.Parse(time.RFC3339Nano,entry.ExpiresAt)
   if err==nil&&now.Before(expires){return webDeliveryDecision{State:"claimed"},nil}
   // A lost ack could follow a successful OS display: avoid duplicates.
   entry.State="uncertain";entry.Token="";ledger.Entries[eventID]=entry
   if err:=saveWebDelivery(project,ledger);err!=nil{return webDeliveryDecision{},err}
   return webDeliveryDecision{State:"uncertain"},nil
  }
  if entry.Attempts>=webDeliveryMaxAttempts{return webDeliveryDecision{State:"attempts_exhausted"},nil}
 }
 // Client-provided timestamps and synthetic event IDs are never trusted.
 events,err:=backlog.ReadNotificationEvents(project);if err!=nil{return webDeliveryDecision{},err}
 var at string
 for _,event:=range events{
  if id,_:=event["id"].(string);id==eventID{at,_=event["at"].(string);break}
 }
 eventTime,err:=time.Parse(time.RFC3339Nano,at)
 if err!=nil{return webDeliveryDecision{State:"unknown_event"},nil}
 cutover,err:=time.Parse(time.RFC3339Nano,ledger.ActivatedAt);if err!=nil{return webDeliveryDecision{},err}
 if !eventTime.After(cutover)||eventTime.After(now.Add(30*time.Second))||now.Sub(eventTime)>webDeliveryTTL{
  return webDeliveryDecision{State:"historical"},nil
 }
 nextToken,err:=webDeliveryNonce();if err!=nil{return webDeliveryDecision{},err}
 entry.State="claimed";entry.Token=nextToken;entry.ClaimedAt=now.Format(time.RFC3339Nano)
 entry.ExpiresAt=now.Add(webDeliveryLease).Format(time.RFC3339Nano);entry.EventAt=at
 entry.Attempts++;entry.Client=webClient(client)
 ledger.Entries[eventID]=entry
 if err:=saveWebDelivery(project,ledger);err!=nil{return webDeliveryDecision{},err}
 return webDeliveryDecision{State:"claimed",Granted:true,Token:nextToken},nil
}
func webDeliveryHistory(project string)(map[string]map[string]any,error){
 webDeliveryMu.Lock();defer webDeliveryMu.Unlock()
 unlock,err:=projectguard.AcquireWrite(project);if err!=nil{return nil,err};defer unlock()
 ledger,_,err:=readWebDelivery(project);if err!=nil{return nil,err}
 out:=make(map[string]map[string]any,len(ledger.Entries))
 for id,e:=range ledger.Entries{out[id]=map[string]any{
  "state":e.State,"sent_at":e.SentAt,"attempts":e.Attempts,"client":e.Client,"claimed_at":e.ClaimedAt,
 }}
 return out,nil
}
