package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestEnvironmentAPIFailsClosedWithoutDevManagedWeb(t *testing.T) {
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	t.Setenv("TASK_MECCA_CHANNEL","dev")
	t.Setenv("TASK_MECCA_PREVIEW_MODE","")
	mux:=http.NewServeMux()
	writer:=func(w http.ResponseWriter,obj any,status int){
		w.Header().Set("Content-Type","application/json")
		w.WriteHeader(status)
		_ =json.NewEncoder(w).Encode(obj)
	}
	restarts:=make(chan maintenance.UpgradeResult,1)
	registerEnvironmentRoutes(mux,"0.3.1-dev.16",restarts,writer)
	get:=httptest.NewRecorder()
	mux.ServeHTTP(get,httptest.NewRequest("GET","/api/environment",nil))
	if get.Code!=200||!strings.Contains(get.Body.String(),"\"kind\":\"dev\""){
		t.Fatalf("bad Dev environment response %d %s",get.Code,get.Body.String())
	}
	for name,modify:=range map[string]func(*http.Request){
		"no_origin":func(r *http.Request){r.Header.Del("Origin")},
		"cross_site":func(r *http.Request){r.Header.Set("Origin","https://example.org")},
		"missing_header":func(r *http.Request){r.Header.Del("X-Task-Mecca-Action")},
	}{
		t.Run(name,func(t *testing.T){
			r:=httptest.NewRequest("POST","http://127.0.0.1:18765/api/environment/enter-pr",
				strings.NewReader(`{"number":217,"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","approve":true}`))
			r.RemoteAddr="127.0.0.1:50501"
			r.Header.Set("Origin","http://127.0.0.1:18765")
			r.Header.Set("X-Task-Mecca-Action","1")
			modify(r)
			w:=httptest.NewRecorder();mux.ServeHTTP(w,r)
			if w.Code!=403{t.Fatalf("%s status=%d",name,w.Code)}
		})
	}
	t.Setenv("TASK_MECCA_PREVIEW_MODE","1")
	w:=httptest.NewRecorder()
	mux.ServeHTTP(w,httptest.NewRequest("POST","/api/environment/return-dev",nil))
	if w.Code!=403{t.Fatalf("Preview mutation allowed: %d",w.Code)}
	t.Setenv("TASK_MECCA_PREVIEW_MODE","")
	t.Setenv("TASK_MECCA_CHANNEL","stable")
	w=httptest.NewRecorder()
	mux.ServeHTTP(w,httptest.NewRequest("POST","/api/environment/return-dev",nil))
	if w.Code!=403{t.Fatalf("Stable mutation allowed: %d",w.Code)}
}

func TestEnvironmentAPIRejectsUnapprovedSwitchBeforeDownload(t *testing.T){
	t.Setenv("TASK_MECCA_HOME",t.TempDir())
	t.Setenv("TASK_MECCA_CHANNEL","dev")
	t.Setenv("TASK_MECCA_PREVIEW_MODE","")
	mux:=http.NewServeMux()
	writer:=func(w http.ResponseWriter,obj any,status int){
		w.WriteHeader(status);_ =json.NewEncoder(w).Encode(obj)
	}
	registerEnvironmentRoutes(mux,"dev",make(chan maintenance.UpgradeResult,1),writer)
	req:=httptest.NewRequest("POST","http://127.0.0.1:18765/api/environment/enter-pr",
		strings.NewReader(`{"number":217,"sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	req.RemoteAddr="127.0.0.1:50000"
	req.Header.Set("X-Task-Mecca-Action","1")
	req.Header.Set("Origin","http://127.0.0.1:18765")
	w:=httptest.NewRecorder()
	mux.ServeHTTP(w,req)
	if w.Code!=400 {t.Fatalf("unapproved PR accepted: %d",w.Code)}
}
