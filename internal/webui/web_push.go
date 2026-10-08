package webui

import (
 "bytes"
 "crypto/ecdsa"
 "crypto/elliptic"
 "crypto/rand"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "sync"
 "time"

 "github.com/silverkhan/TaskMecca/internal/projectguard"
)

var pushRegistryMu sync.Mutex

type pushSubscription struct {
 ID string `json:"id"`
 Endpoint string `json:"endpoint"`
 P256DH string `json:"p256dh"`
 Auth string `json:"auth"`
 Client string `json:"client"`
 RegisteredAt string `json:"registered_at"`
 SeenAt string `json:"seen_at"`
 Kinds map[string]bool `json:"kinds,omitempty"`
}
type pushRegistry struct {
 Version int `json:"version"`
 Private string `json:"vapid_private"`
 Public string `json:"vapid_public"`
 Subscriptions map[string]pushSubscription `json:"subscriptions"`
}
type pushSubscriptionRequest struct {
 Action string `json:"action"`
 Endpoint string `json:"endpoint"`
 P256DH string `json:"p256dh"`
 Auth string `json:"auth"`
 Client string `json:"client"`
 Kinds map[string]bool `json:"kinds"`
}
func pushRegistryPath(project string)string{
 return filepath.Join(project,"_task_mecca",".runtime","notifications","push_subscriptions.json")
}
func readPushRegistry(project string)(pushRegistry,error){
 data,err:=os.ReadFile(pushRegistryPath(project))
 if errors.Is(err,os.ErrNotExist){return pushRegistry{Version:1,Subscriptions:map[string]pushSubscription{}},nil}
 if err!=nil{return pushRegistry{},err}
 var registry pushRegistry
 if err:=json.Unmarshal(data,&registry);err!=nil{return pushRegistry{},fmt.Errorf("push registry is corrupt, refusing reset: %w",err)}
 if registry.Version!=1||registry.Private==""||registry.Public=="" {return pushRegistry{},errors.New("invalid push registry, refusing reset")}
 if registry.Subscriptions==nil{registry.Subscriptions=map[string]pushSubscription{}}
 return registry,nil
}
func savePushRegistry(project string,registry pushRegistry)error{
 path:=pushRegistryPath(project)
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
 body,err:=json.MarshalIndent(registry,"","  ");if err!=nil{return err}
 // Atomic replace under projectguard and in-process mutex.
 temp:=path+".tmp"
 if err:=os.WriteFile(temp,append(body,byte(10)),0600);err!=nil{return err}
 if err:=os.Rename(temp,path);err!=nil{return err}
 return os.Chmod(path,0600)
}
type vapidIdentity struct{
 Private string `json:"private"`
 Public string `json:"public"`
}
// One VAPID application server identity is shared by all Task Mecca projects
// on this installation: PushManager subscriptions are origin/scope-wide.
func sharedPushIdentity()(vapidIdentity,error){
 path:=filepath.Join(projectguard.Home(),"webpush-vapid.json")
 data,err:=os.ReadFile(path)
 if err==nil{
  var result vapidIdentity
  if err:=json.Unmarshal(data,&result);err!=nil{return vapidIdentity{},err}
  if len(mustPushDecode(result.Private))!=32||len(mustPushDecode(result.Public))!=65{return vapidIdentity{},errors.New("invalid shared VAPID identity")}
  return result,nil
 }
 if !errors.Is(err,os.ErrNotExist){return vapidIdentity{},err}
 private,err:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if err!=nil{return vapidIdentity{},err}
 result:=vapidIdentity{
  Private:base64.RawURLEncoding.EncodeToString(private.D.FillBytes(make([]byte,32))),
  Public:base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(),private.PublicKey.X,private.PublicKey.Y)),
 }
 if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return vapidIdentity{},err}
 raw,err:=json.Marshal(result);if err!=nil{return vapidIdentity{},err}
 // Write once under the global projectguard file lock. Refuse to regenerate
 // a damaged or inaccessible identity, as old subscriptions would break.
 tmp:=path+".tmp"
 if err:=os.WriteFile(tmp,raw,0600);err!=nil{return vapidIdentity{},err}
 if err:=os.Rename(tmp,path);err!=nil{return vapidIdentity{},err}
 return result,nil
}
func withPushRegistry(project string, f func(*pushRegistry)(bool,error))(pushRegistry,error){
 pushRegistryMu.Lock();defer pushRegistryMu.Unlock()
 unlock,err:=projectguard.AcquireWrite(project);if err!=nil{return pushRegistry{},err};defer unlock()
 reg,err:=readPushRegistry(project);if err!=nil{return pushRegistry{},err}
 dirty:=false
 identity,err:=sharedPushIdentity();if err!=nil{return pushRegistry{},err}
 if reg.Private==""{
  reg.Private=identity.Private;reg.Public=identity.Public;dirty=true
 }else if reg.Private!=identity.Private||reg.Public!=identity.Public{
  return pushRegistry{},errors.New("VAPID identity mismatch: refusing silent key rotation")
 }
 changed,err:=f(&reg);if err!=nil{return pushRegistry{},err}
 if dirty||changed{if err:=savePushRegistry(project,reg);err!=nil{return pushRegistry{},err}}
 return reg,nil
}
func webPushPublicKey(project string)(string,error){
 reg,err:=withPushRegistry(project,func(_ *pushRegistry)(bool,error){return false,nil})
 return reg.Public,err
}
func pushSubscriptionID(endpoint string)string{
 sha:=sha256.Sum256([]byte(endpoint))
 return base64.RawURLEncoding.EncodeToString(sha[:16])
}
func validPushEndpoint(raw string)bool{
 if len(raw)>2048||len(raw)<15{return false}
 u,err:=url.Parse(raw)
 if err!=nil||u.Scheme!="https"||u.User!=nil||u.RawQuery!=""||u.Fragment!=""||u.Port()!=""||u.Hostname()==""{return false}
 host:=strings.ToLower(u.Hostname())
 if net.ParseIP(host)!=nil {return false}
 if host=="fcm.googleapis.com"||host=="updates.push.services.mozilla.com"||host=="web.push.apple.com"||host=="push.services.mozilla.com"{return true}
 for _,suffix:=range []string{".push.apple.com",".notify.windows.com",".push.services.mozilla.com"}{
  if strings.HasSuffix(host,suffix){return true}
 }
 return false
}
func allowedPushKind(k string)bool{
 switch k{case "registered","started","intervention","approval","stalled","interrupted","runtime_unknown","completed":return true}
 return false
}
func webPushSubscription(project string,request pushSubscriptionRequest)(map[string]any,error){
 if !validPushEndpoint(request.Endpoint){return nil,errors.New("untrusted push endpoint")}
 id:=pushSubscriptionID(request.Endpoint)
 if request.Action=="subscribe"{
  key,err:=base64.RawURLEncoding.DecodeString(request.P256DH)
  if err!=nil||len(key)!=65||key[0]!=4{return nil,errors.New("invalid push p256dh key")}
  x,y:=elliptic.Unmarshal(elliptic.P256(),key)
  if x==nil||y==nil{return nil,errors.New("invalid push public key")}
  auth,err:=base64.RawURLEncoding.DecodeString(request.Auth)
  if err!=nil||len(auth)!=16{return nil,errors.New("invalid push auth secret")}
 }
 now:=time.Now().UTC().Format(time.RFC3339Nano)
 reg,err:=withPushRegistry(project,func(reg *pushRegistry)(bool,error){
  old,exists:=reg.Subscriptions[id]
  switch request.Action{
  case "subscribe":
   kinds:=map[string]bool{}
   for kind,on:=range request.Kinds{if allowedPushKind(kind){kinds[kind]=on}}
   if len(reg.Subscriptions)>=24&&!exists{return false,errors.New("maximum push subscriptions reached")}
   registeredAt:=now;if exists{registeredAt=old.RegisteredAt}
   reg.Subscriptions[id]=pushSubscription{ID:id,Endpoint:request.Endpoint,P256DH:request.P256DH,Auth:request.Auth,Client:webClient(request.Client),RegisteredAt:registeredAt,SeenAt:now,Kinds:kinds}
   return true,nil
  case "heartbeat":
   if !exists{return false,errors.New("subscription not registered")}
   // Avoid frequent fsync/journal writes for every UI polling event.
   last,err:=time.Parse(time.RFC3339Nano,old.SeenAt)
   if err==nil&&time.Since(last)<30*time.Second{return false,nil}
   old.SeenAt=now;reg.Subscriptions[id]=old;return true,nil
  case "unsubscribe":
   if exists{delete(reg.Subscriptions,id);return true,nil}
   return false,nil
  default:return false,errors.New("invalid push action")
  }
 })
 if err!=nil{return nil,err}
 return map[string]any{"ok":true,"registered":request.Action!="unsubscribe","count":len(reg.Subscriptions)},nil
}
func pushRegistrySnapshot(project string)(pushRegistry,error){
 return withPushRegistry(project,func(_ *pushRegistry)(bool,error){return false,nil})
}
func removeInvalidPushEndpoint(project,endpoint string){
 _,_ = withPushRegistry(project,func(reg *pushRegistry)(bool,error){
  id:=pushSubscriptionID(endpoint)
  existing,ok:=reg.Subscriptions[id]
  if !ok||existing.Endpoint!=endpoint{return false,nil}
  delete(reg.Subscriptions,id);return true,nil
 })
}
func pushKindEnabled(sub pushSubscription,kind string)bool{
 if len(sub.Kinds)==0{return true}
 on,ok:=sub.Kinds[kind]
 if !ok{return true}
 return on
}
func choosePushSubscription(reg pushRegistry,kind string)(pushSubscription,bool){
 eligible:=[]pushSubscription{}
 for _,sub:=range reg.Subscriptions{if pushKindEnabled(sub,kind){eligible=append(eligible,sub)}}
 if len(eligible)==0{return pushSubscription{},false}
 sort.Slice(eligible,func(i,j int)bool{return eligible[i].SeenAt>eligible[j].SeenAt})
 return eligible[0],true
}
func pushEventPayload(project string,event map[string]any)[]byte{
 task,_:=event["task_id"].(string);kind,_:=event["kind"].(string);name:=filepath.Base(project)
 title:=name+" · "+task+" · "+kind
 if kind=="completed"{title=name+" · "+task+" 완료"}
 body,_:=event["title"].(string)
 if body==""{body,_=event["message"].(string)}
 urlParams:=url.Values{"project":[]string{project}}
 link:="/tasks/"+url.PathEscape(task)+"?"+urlParams.Encode()
 raw,_:=json.Marshal(map[string]string{"title":title,"body":body,"tag":"task-mecca:"+project+":"+fmt.Sprint(event["id"]),"url":link})
 return raw
}
func postWebPush(reg pushRegistry,sub pushSubscription,payload []byte)(int,error){
 ua,err:=pushB64.DecodeString(sub.P256DH);if err!=nil{return 0,err}
 auth,err:=pushB64.DecodeString(sub.Auth);if err!=nil{return 0,err}
 data,err:=encryptWebPush(ua,auth,payload);if err!=nil{return 0,err}
 secret,err:=pushB64.DecodeString(reg.Private);if err!=nil{return 0,err}
 vapid,err:=vapidAuthorization(secret,reg.Public,sub.Endpoint);if err!=nil{return 0,err}
 request,err:=http.NewRequest(http.MethodPost,sub.Endpoint,bytes.NewReader(data));if err!=nil{return 0,err}
 request.Header.Set("TTL","60")
 request.Header.Set("Content-Encoding","aes128gcm")
 request.Header.Set("Content-Type","application/octet-stream")
 request.Header.Set("Authorization",vapid)
 request.Header.Set("Urgency","normal")
 response,err:=pushHTTPClient().Do(request);if err!=nil{return 0,err}
 defer response.Body.Close()
 _,_=io.Copy(io.Discard,io.LimitReader(response.Body,1024))
 return response.StatusCode,nil
}

// Called by the existing background attention feed, never by page polling.
// The same AID-119 claim ledger arbitrates foreground and background delivery.
func deliverWebPush(project string,rows []map[string]any,current map[string]map[string]any){
 if len(rows)==0{return}
 // Background monitoring should not generate key material or runtime files
 // for projects whose users never opted in to Web Push.
 if _,err:=os.Stat(pushRegistryPath(project));err!=nil{return}
 reg,err:=pushRegistrySnapshot(project);if err!=nil||len(reg.Subscriptions)==0{return}
 for _,event:=range rows{
  id,_:=event["id"].(string);kind,_:=event["kind"].(string);task,_:=event["task_id"].(string)
  if id==""||task==""||!allowedPushKind(kind){continue}
  if currentTask:=current[task];currentTask!=nil{
   state,_:=currentTask["file_state"].(string)
   if state=="done"&&kind!="completed"{continue}
   if kind=="completed"&&state!="done"{continue}
  }
  sub,ok:=choosePushSubscription(reg,kind);if !ok{continue}
  claim,err:=applyWebDelivery(project,"claim",id,"","push")
  if err!=nil||!claim.Granted{continue}
  status,sendErr:=postWebPush(reg,sub,pushEventPayload(project,event))
  switch {
  case sendErr!=nil:
   _,_=applyWebDelivery(project,"uncertain",id,claim.Token,"push")
  case status==201||status==202:
   _,_=applyWebDelivery(project,"push_ack",id,claim.Token,"push")
  case status==404||status==410:
   removeInvalidPushEndpoint(project,sub.Endpoint)
   _,_=applyWebDelivery(project,"fail",id,claim.Token,"push")
  default:
   // Provider definitively rejected the request: allow bounded retry.
   _,_=applyWebDelivery(project,"fail",id,claim.Token,"push")
  }
 }
}

func testWebPush(project,endpoint string)(int,error){
 reg,err:=pushRegistrySnapshot(project)
 if err!=nil{return 0,err}
 sub,ok:=reg.Subscriptions[pushSubscriptionID(endpoint)]
 if !ok||sub.Endpoint!=endpoint{return 0,errors.New("push subscription not registered")}
 payload,_:=json.Marshal(map[string]string{
  "title":"Task Mecca · 알림 연결 테스트",
  "body":"Web Push 테스트 알림입니다.",
  "url":"/?view=notifications",
  "tag":"task-mecca-push-test",
 })
 status,err:=postWebPush(reg,sub,payload)
 if status==404||status==410{removeInvalidPushEndpoint(project,endpoint)}
 return status,err
}
