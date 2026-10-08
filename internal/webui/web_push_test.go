package webui

import (
 "bytes"
 "crypto/aes"
 "crypto/rand"
 "crypto/cipher"
 "crypto/ecdh"
 "crypto/ecdsa"
 "crypto/elliptic"
 "crypto/sha256"
 "encoding/base64"
 "encoding/binary"
 "encoding/json"
 "math/big"
 "net/url"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func TestWebPushEncryptionRoundTripRFC8291(t *testing.T){
 userKey,err:=ecdh.P256().GenerateKey(rand.Reader);if err!=nil{t.Fatal(err)}
 auth:=make([]byte,16)
 if _,err:=rand.Read(auth);err!=nil{t.Fatal(err)}
 payload:=[]byte(`{"title":"Task Mecca","body":"done"}`)
 encoded,err:=encryptWebPush(userKey.PublicKey().Bytes(),auth,payload)
 if err!=nil{t.Fatal(err)}
 if len(encoded)<103||encoded[20]!=65||binary.BigEndian.Uint32(encoded[16:20])!=4096{t.Fatalf("invalid aes128gcm record header")}
 serverPublic,err:=ecdh.P256().NewPublicKey(encoded[21:86]);if err!=nil{t.Fatal(err)}
 shared,err:=userKey.ECDH(serverPublic);if err!=nil{t.Fatal(err)}
 info:=append(append([]byte("WebPush: info\x00"),userKey.PublicKey().Bytes()...),encoded[21:86]...)
 ikm:=pushHKDF(auth,shared,info,32)
 secret:=pushHKDF(encoded[:16],ikm,[]byte("Content-Encoding: aes128gcm\x00"),16)
 nonce:=pushHKDF(encoded[:16],ikm,[]byte("Content-Encoding: nonce\x00"),12)
 block,err:=aes.NewCipher(secret);if err!=nil{t.Fatal(err)}
 gcm,err:=cipher.NewGCM(block);if err!=nil{t.Fatal(err)}
 raw,err:=gcm.Open(nil,nonce,encoded[86:],nil);if err!=nil{t.Fatal(err)}
 if len(raw)==0||raw[len(raw)-1]!=2||!bytes.Equal(raw[:len(raw)-1],payload){t.Fatal("encrypted payload did not round-trip")}
}

func TestWebPushVAPIDJWTES256(t *testing.T){
 priv,err:=ecdsa.GenerateKey(elliptic.P256(),rand.Reader);if err!=nil{t.Fatal(err)}
 raw:=priv.D.FillBytes(make([]byte,32))
 public:=base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(),priv.PublicKey.X,priv.PublicKey.Y))
 token,err:=vapidAuthorization(raw,public,"https://fcm.googleapis.com/fcm/send/test")
 if err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(token,"vapid t=")||!strings.HasSuffix(token,", k="+public){t.Fatalf("invalid VAPID authorization header")}
 parts:=strings.Split(strings.TrimPrefix(strings.Split(token,", k=")[0],"vapid t="),".")
 if len(parts)!=3{t.Fatal("JWT must have three parts")}
 claimsRaw,err:=base64.RawURLEncoding.DecodeString(parts[1]);if err!=nil{t.Fatal(err)}
 claims:=map[string]any{};if err:=json.Unmarshal(claimsRaw,&claims);err!=nil{t.Fatal(err)}
 if claims["aud"]!="https://fcm.googleapis.com"{t.Fatalf("wrong audience: %v",claims)}
 sig,err:=base64.RawURLEncoding.DecodeString(parts[2]);if err!=nil{t.Fatal(err)}
 if len(sig)!=64{t.Fatal("JWT must use 64 byte raw ECDSA signature")}
 digest:=sha256.Sum256([]byte(parts[0]+"."+parts[1]))
 if !ecdsa.Verify(&priv.PublicKey,digest[:],new(big.Int).SetBytes(sig[:32]),new(big.Int).SetBytes(sig[32:])){t.Fatal("ES256 JWT verification failed")}
}

func TestPushEndpointValidationAndProjectIsolation(t *testing.T){
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 first:=t.TempDir();second:=t.TempDir()
 for _,value:=range []string{"https://localhost/push","http://fcm.googleapis.com","https://10.0.0.1/push","https://fcm.googleapis.com.attacker.example","https://fcm.googleapis.com:443/","https://fcm.googleapis.com@evil.example/path"}{
  if validPushEndpoint(value){t.Fatalf("SSRF-prone push endpoint accepted: %s",value)}
 }
 for _,value:=range []string{"https://fcm.googleapis.com/fcm/send/a","https://web.push.apple.com/a","https://updates.push.services.mozilla.com/wpush/a","https://wns2-by3p.notify.windows.com/w/?a"}{
  if !validPushEndpoint(value){t.Fatalf("legitimate Push provider rejected: %s",value)}
 }
 one,err:=webPushPublicKey(first);if err!=nil{t.Fatal(err)}
 two,err:=webPushPublicKey(second);if err!=nil{t.Fatal(err)}
 if one==""||one!=two{t.Fatalf("VAPID must be installation-wide, not per-project")}
 ua,err:=ecdh.P256().GenerateKey(rand.Reader);if err!=nil{t.Fatal(err)}
 auth:=base64.RawURLEncoding.EncodeToString(make([]byte,16))
 request:=pushSubscriptionRequest{Action:"subscribe",Endpoint:"https://fcm.googleapis.com/fcm/send/fixture",P256DH:base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()),Auth:auth,Client:"safari"}
 _,err=webPushSubscription(first,request);if err!=nil{t.Fatal(err)}
 registry,err:=pushRegistrySnapshot(first);if err!=nil{t.Fatal(err)}
 if len(registry.Subscriptions)!=1{t.Fatalf("subscription not saved")}
 registry2,err:=pushRegistrySnapshot(second);if err!=nil{t.Fatal(err)}
 if len(registry2.Subscriptions)!=0{t.Fatalf("subscription leaked to another project")}
 _,err=webPushSubscription(first,pushSubscriptionRequest{Action:"unsubscribe",Endpoint:request.Endpoint});if err!=nil{t.Fatal(err)}
 registry,err=pushRegistrySnapshot(first);if err!=nil||len(registry.Subscriptions)!=0{t.Fatalf("unsubscribe failed: %v",err)}
 info,err:=os.Stat(filepath.Join(first,"_task_mecca",".runtime","notifications","push_subscriptions.json"))
 if err!=nil{t.Fatal(err)}
 if info.Mode().Perm()&0077!=0{t.Fatalf("subscription secrets are not private: %v",info.Mode())}
 if _,err:=url.Parse("https://fcm.googleapis.com");err!=nil{t.Fatal(err)}
}

func TestWebPushProviderAckSharesDeliveryLedger(t *testing.T){
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 project:=webDeliveryFixture(t)
 webDeliveryTestCutover(t,project)
 claim,err:=applyWebDelivery(project,"claim","new-complete","","push")
 if err!=nil||!claim.Granted{t.Fatalf("push claim %+v %v",claim,err)}
 ack,err:=applyWebDelivery(project,"push_ack","new-complete",claim.Token,"push")
 if err!=nil||ack.State!="push_accepted"{t.Fatalf("provider ack %+v %v",ack,err)}
 browser,err:=applyWebDelivery(project,"claim","new-complete","","chrome")
 if err!=nil||browser.Granted||browser.State!="push_accepted"{t.Fatalf("duplicate foreground alert %+v %v",browser,err)}
 if _,err:=time.Parse(time.RFC3339Nano,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{t.Fatal(err)}
}

func TestPushChannelAndKindFilters(t *testing.T){
 if !allowedPushKind("finalize"){t.Fatal("Web Push must support same finalize type as Telegram")}
 disabled:=false
 sub:=pushSubscription{Enabled:&disabled,Kinds:map[string]bool{"completed":true}}
 if pushKindEnabled(sub,"completed"){t.Fatal("disabled web channel must not deliver Push")}
 enabled:=true;sub.Enabled=&enabled
 if !pushKindEnabled(sub,"completed"){t.Fatal("enabled channel should deliver configured type")}
 sub.Kinds["completed"]=false
 if pushKindEnabled(sub,"completed"){t.Fatal("disabled type must not deliver Push")}
 if !pushKindEnabled(sub,"finalize"){t.Fatal("newly supported type defaults to true")}
}
