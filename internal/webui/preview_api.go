package webui

import (
 stdcontext "context"
 "encoding/json"
 "net"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "time"

 "github.com/silverkhan/TaskMecca/internal/maintenance"
 "github.com/silverkhan/TaskMecca/internal/preview"
)

var previewManagerFactory = func() *preview.Manager {
 home,err:=os.UserCacheDir()
 if err!=nil||home=="" {home=os.TempDir()}
 return preview.NewManager(filepath.Join(home,"task-mecca","preview-builds"))
}

// Preview lifecycle controls are intentionally host-local. In particular a
// remote Tailscale browser cannot trigger docker on the host from this API.
func previewLocalMutation(r *http.Request) bool {
 if r.Method!=http.MethodPost || r.Header.Get("X-Task-Mecca-Action")!="1" {return false}
 host,_,err:=net.SplitHostPort(r.RemoteAddr)
 if err!=nil||!net.ParseIP(host).IsLoopback(){return false}
 // Reverse proxies (including Tailscale Serve) may forward a remote
 // browser via 127.0.0.1. The requested origin itself must be local too.
 requestedHost:=r.Host
 if host,_,err:=net.SplitHostPort(requestedHost);err==nil{requestedHost=host}
 address:=net.ParseIP(requestedHost)
 if requestedHost!="localhost"&&(address==nil||!address.IsLoopback()){return false}
 origin:=r.Header.Get("Origin")
 if origin==""{return false}
 parsed,err:=url.Parse(origin)
 if err!=nil||parsed.Host!=r.Host||(parsed.Scheme!="http"&&parsed.Scheme!="https") {return false}
 if r.Header.Get("Sec-Fetch-Site")=="cross-site"{return false}
 return true
}

func registerPreviewRoutes(mux *http.ServeMux,writeJSON func(http.ResponseWriter,any,int)){
 var manager *preview.Manager
 getManager:=func()*preview.Manager{
  if manager==nil {manager=previewManagerFactory()}
  return manager
 }
 permitted:=func(w http.ResponseWriter,r *http.Request)bool{
  if maintenance.CurrentChannel()!="dev" || PreviewMode(){
   writeJSON(w,map[string]any{"error":"Preview Builds is exclusive to the Dev parent Web"},403)
   return false
  }
  return true
 }
 mux.HandleFunc("/api/preview/instances",func(w http.ResponseWriter,r *http.Request){
  if !permitted(w,r){return}
  if r.Method!=http.MethodGet {w.Header().Set("Allow","GET");writeJSON(w,map[string]any{"error":"method not allowed"},405);return}
  writeJSON(w,map[string]any{"instances":getManager().List()},200)
 })
 action:=func(w http.ResponseWriter,r *http.Request,start bool){
  if !permitted(w,r){return}
  if !previewLocalMutation(r){
   writeJSON(w,map[string]any{"error":"Preview launch and stop require a local same-origin authenticated action"},403);return
  }
  var request struct {Number int `json:"number"`}
  dec:=json.NewDecoder(http.MaxBytesReader(w,r.Body,2048))
  dec.DisallowUnknownFields()
  if err:=dec.Decode(&request);err!=nil||request.Number<1 {
   writeJSON(w,map[string]any{"error":"invalid PR number"},400);return
  }
  timeout:=75*time.Second
  if start{timeout=120*time.Second} // artifact download + Preview health readiness
  ctx,cancel:=stdcontext.WithTimeout(r.Context(),timeout)
  defer cancel()
  if start{
   instance,err:=getManager().Start(ctx,request.Number,r.Header.Get("Origin"))
   if err!=nil{writeJSON(w,map[string]any{"error":err.Error(),"instance":instance},409);return}
   writeJSON(w,map[string]any{"instance":instance,"open_url":instance.URL},200)
  }else{
   if err:=getManager().Stop(ctx,request.Number);err!=nil{
    writeJSON(w,map[string]any{"error":err.Error()},409);return
   }
   // Return the authoritative post-stop instance so the browser can
   // immediately switch its row from Open/Stop to Run, without waiting for
   // the GitHub build catalog or a potentially already-active list request.
   var stopped *preview.Instance
   for _,instance:=range getManager().List(){
    if instance.PRNumber==request.Number{
     copy:=instance
     stopped=&copy
     break
    }
   }
   writeJSON(w,map[string]any{"ok":true,"instance":stopped},200)
  }
 }
 mux.HandleFunc("/api/preview/start",func(w http.ResponseWriter,r *http.Request){action(w,r,true)})
 mux.HandleFunc("/api/preview/stop",func(w http.ResponseWriter,r *http.Request){action(w,r,false)})
}

