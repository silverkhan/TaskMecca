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
 "github.com/silverkhan/TaskMecca/internal/notify"
 "github.com/silverkhan/TaskMecca/internal/runtimeobs"
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
 if action=="ack"||action=="fail"||action=="push_ack"||action=="uncertain"{
  if !wasSeen||token==""||entry.Token!=token||entry.State!="claimed"{return webDeliveryDecision{State:"not_claimed"},nil}
  switch action{case "ack":entry.State="display_requested";entry.SentAt=now.Format(time.RFC3339Nano)
  case "push_ack":entry.State="push_accepted";entry.SentAt=now.Format(time.RFC3339Nano)
  case "uncertain":entry.State="uncertain"
  default:entry.State="failed"}
  entry.Token=""
  ledger.Entries[eventID]=entry
  if err:=saveWebDelivery(project,ledger);err!=nil{return webDeliveryDecision{},err}
  return webDeliveryDecision{State:entry.State},nil
 }
 if action!="claim"{return webDeliveryDecision{},errors.New("invalid action")}
 if wasSeen {
  if entry.State=="display_requested"||entry.State=="push_accepted"||entry.State=="uncertain"{return webDeliveryDecision{State:entry.State},nil}
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
 var selected map[string]any
 for _,event:=range events{
  if id,_:=event["id"].(string);id==eventID{at,_=event["at"].(string);selected=event;break}
 }
 if selected != nil && backlog.HumanActionKind(notificationValue(selected["kind"])) {
  decisions, gateErr := currentNotificationDecisions(project, "")
  if gateErr != nil { return webDeliveryDecision{State:"decision_unavailable"},nil }
  decision, exists := decisions[strings.ToUpper(notificationValue(selected["task_id"]))]
  if !exists || !decision.Match(selected) { return webDeliveryDecision{State:"condition_resolved"},nil }
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

// Both senders and the browser/push claim boundary use the same canonical
// decision. Runtime-derived actions also require a bounded official exact read;
// a cached turn interval is never a liveness or CPU measurement.
func currentNotificationDecisions(project, root string) (map[string]backlog.HumanActionDecision, error) {
	decisions, err := backlog.CurrentHumanActionDecisions(project, root)
	if err != nil {
		return nil, err
	}
	needRuntime := false
	for _, decision := range decisions {
		if strings.HasPrefix(notificationValue(decision.Condition["key"]), "runtime:") {
			needRuntime = true
		}
	}
	if !needRuntime {
		return stableHumanActionDecisions(project, root, decisions)
	}
	now := time.Now().UTC()
	ledger, err := runtimeobs.BuildLedger(project, 3, now)
	if err != nil {
		return nil, err
	}
	evidence := operationExecutionLoader(project, ledger, now, true)
	now = time.Now().UTC()
	for task, decision := range decisions {
		if !strings.HasPrefix(notificationValue(decision.Condition["key"]), "runtime:") {
			continue
		}
		current, ok := evidence[task]
		observed, parseErr := time.Parse(time.RFC3339Nano, current.ObservedAt)
		if !ok || !current.Matched || !current.Fresh || !current.OfficialCurrent || current.FailedStage != "" || parseErr != nil || observed.After(now) || now.Sub(observed) > 2*time.Minute || current.AttemptID != notificationValue(decision.Condition["attempt_id"]) || current.Identity.AssignmentID != notificationValue(decision.Condition["assignment_id"]) || current.Identity.NativeID != notificationValue(decision.Condition["native_id"]) || string(current.State) != notificationValue(decision.Condition["runtime_state"]) {
			delete(decisions, task)
		}
	}
	return stableHumanActionDecisions(project, root, decisions)
}

// Canonical completion, reason or assignment may change during official I/O.
// Preserve only the same current episode after that independent read boundary.
func stableHumanActionDecisions(project, root string, decisions map[string]backlog.HumanActionDecision) (map[string]backlog.HumanActionDecision, error) {
	fresh, err := backlog.CurrentHumanActionDecisions(project, root)
	if err != nil {
		return nil, err
	}
	for task, prior := range decisions {
		current, ok := fresh[task]
		same := ok && prior.Since == current.Since
		for _, field := range []string{"key", "attempt_id", "assignment_id", "native_id", "runtime_state", "message", "resume_condition"} {
			if notificationValue(prior.Condition[field]) != notificationValue(current.Condition[field]) {
				same = false
			}
		}
		if !same {
			delete(decisions, task)
		}
	}
	return preserveHumanActionDeliveryIDs(project, decisions), nil
}

// A shared ID migration must not resend a cause delivered by the old monitor.
// Read its immutable outcome, and alias only the still-unresolved same cause
// from this episode and assignment. Never rewrite source actions or sent rows.
func preserveHumanActionDeliveryIDs(project string, decisions map[string]backlog.HumanActionDecision) map[string]backlog.HumanActionDecision {
	journal, err := readOperationJournal(project)
	if err != nil {
		return decisions
	}
	for task, decision := range decisions {
		since, e := time.Parse(time.RFC3339Nano, decision.Since)
		assigned, aerr := time.Parse(time.RFC3339Nano, notificationValue(decision.Condition["assigned_at"]))
		if e != nil || aerr != nil {
			continue
		}
		for _, action := range journal.Actions {
			detected, err := time.Parse(time.RFC3339Nano, action.DetectedAt)
			if err != nil || detected.Before(since) || detected.Before(assigned) || detected.After(time.Now().UTC()) || action.ResolvedAt != "" || !strings.EqualFold(action.TaskID, task) || action.Reason != notificationValue(decision.Condition["message"]) || action.NextAction != notificationValue(decision.Condition["resume_condition"]) {
				continue
			}
			records, err := notify.DeliveryRecords(project, task, action.ID)
			if err != nil {
				continue
			}
			for _, record := range records {
				if record.State == "sent" || record.State == "consumed_legacy" {
					decision.EventID = action.ID
					decisions[task] = decision
					break
				}
			}
		}
	}
	return decisions
}

func currentBacklogDeliveryEvents(project, root string, rows []map[string]any) []notify.Event {
	decisions, err := currentNotificationDecisions(project, root)
	if err != nil {
		decisions = nil
	} // Fail closed for actions; lifecycle stays separate.
	events := make([]notify.Event, 0, len(rows))
	for _, row := range rows {
		kind := notificationValue(row["kind"])
		id := notificationValue(row["id"])
		if backlog.HumanActionKind(kind) {
			decision, exists := decisions[strings.ToUpper(notificationValue(row["task_id"]))]
			if !exists || !decision.Match(row) {
				continue
			}
			id = decision.EventID
		}
		events = append(events, notify.Event{ID: id, TaskID: notificationValue(row["task_id"]), Kind: kind, Title: notificationValue(row["title"]), Message: notificationValue(row["message"]), ResumeCondition: notificationValue(row["resume_condition"]), At: notificationValue(row["at"])})
	}
	return events
}

func currentOperationDeliveryEvents(project string, events []notify.Event) []notify.Event {
	decisions, err := currentNotificationDecisions(project, "")
	if err != nil {
		return nil
	}
	filtered := make([]notify.Event, 0, len(events))
	for _, event := range events {
		decision, ok := decisions[strings.ToUpper(event.TaskID)]
		if !ok || event.Message != notificationValue(decision.Condition["message"]) || event.ResumeCondition != notificationValue(decision.Condition["resume_condition"]) {
			continue
		}
		event.ID = decision.EventID
		filtered = append(filtered, event)
	}
	return filtered
}
