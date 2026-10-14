package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/silverkhan/TaskMecca/internal/preview"
)

func TestPreviewMutationRequiresSameOriginAndLocalClient(t *testing.T){
 base:=httptest.NewRequest(http.MethodPost,"http://127.0.0.1:18765/api/preview/start",nil)
 base.RemoteAddr="127.0.0.1:51000"
 base.Header.Set("Origin","http://127.0.0.1:18765")
 base.Header.Set("X-Task-Mecca-Action","1")
 if !previewLocalMutation(base){t.Fatal("trusted local action unexpectedly denied")}
 for name,mutate:=range map[string]func(*http.Request){
  "remote":func(r *http.Request){r.RemoteAddr="100.100.218.126:51000"},
  "proxied":func(r *http.Request){r.Host="aiden-macbookpro.tailnet.ts.net:18765";r.Header.Set("Origin","https://aiden-macbookpro.tailnet.ts.net:18765")},
  "bad_origin":func(r *http.Request){r.Header.Set("Origin","https://evil.example")},
  "missing_origin":func(r *http.Request){r.Header.Del("Origin")},
  "no_header":func(r *http.Request){r.Header.Del("X-Task-Mecca-Action")},
  "cross_site":func(r *http.Request){r.Header.Set("Sec-Fetch-Site","cross-site")},
  "GET":func(r *http.Request){r.Method=http.MethodGet},
 }{
  t.Run(name,func(t *testing.T){
   r:=base.Clone(base.Context())
   r.Header=base.Header.Clone()
   mutate(r)
   if previewLocalMutation(r){t.Fatal("unsafe Preview launch action accepted")}
  })
 }
}

func TestPreviewInstancesReadAndDevOnlyAPI(t *testing.T){
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 t.Setenv("TASK_MECCA_PREVIEW_MODE","")
 old:=previewManagerFactory
 defer func(){previewManagerFactory=old}()
 calls:=0
 previewManagerFactory=func()*preview.Manager{calls++;return preview.NewManager(t.TempDir())}
 mux:=http.NewServeMux()
 writer:=func(w http.ResponseWriter,obj any,status int){
  w.Header().Set("Content-Type","application/json")
  w.WriteHeader(status)
  _=json.NewEncoder(w).Encode(obj)
 }
 registerPreviewRoutes(mux,writer)
 r:=httptest.NewRequest("GET","/api/preview/instances",nil)
 w:=httptest.NewRecorder()
 mux.ServeHTTP(w,r)
 if w.Code!=200||calls!=1{t.Fatalf("Dev instances status %d factory=%d",w.Code,calls)}

 w=httptest.NewRecorder()
 mux.ServeHTTP(w,httptest.NewRequest("POST","/api/preview/start",nil))
 if w.Code!=403{t.Fatalf("cross-origin mutation status %d",w.Code)}

 local:=httptest.NewRequest("POST","http://127.0.0.1:18765/api/preview/start",nil)
 local.Host="127.0.0.1:18765"
 local.RemoteAddr="127.0.0.1:5000"
 local.Header.Set("X-Task-Mecca-Action","1")
 local.Header.Set("Origin","http://127.0.0.1:18765")
 w=httptest.NewRecorder()
 mux.ServeHTTP(w,local)
 if w.Code!=400 {t.Fatalf("bad request status %d",w.Code)}

 t.Setenv("TASK_MECCA_CHANNEL","stable")
 w=httptest.NewRecorder()
 mux.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/instances",nil))
 if w.Code!=403{t.Fatalf("stable API leaked status %d",w.Code)}
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 t.Setenv("TASK_MECCA_PREVIEW_MODE","1")
 w=httptest.NewRecorder()
 mux.ServeHTTP(w,httptest.NewRequest("GET","/api/preview/instances",nil))
 if w.Code!=403{t.Fatalf("child API leaked status %d",w.Code)}
}
