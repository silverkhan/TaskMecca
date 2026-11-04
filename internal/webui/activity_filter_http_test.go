package webui

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "net/url"
 "testing"
)

func TestActivityFilterAPIReadOnlyAndValidated(t *testing.T){
 project:=t.TempDir()
 registerWebFixture(t,project)
 handler,err:=Handler(project,"","test")
 if err!=nil{t.Fatal(err)}
 base:="/api/activity/project/filter?"+url.Values{"project":{project},"role":{"worker"}}.Encode()
 request:=func(method,path string)*httptest.ResponseRecorder{
  out:=httptest.NewRecorder()
  handler.ServeHTTP(out,httptest.NewRequest(method,path,nil))
  return out
 }
 valid:=request(http.MethodGet,base)
 if valid.Code!=http.StatusOK{t.Fatalf("valid query %d %s",valid.Code,valid.Body.String())}
 var body map[string]any
 if err:=json.Unmarshal(valid.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
 if body["duration_basis"]!="activity_observed_segments"{t.Fatalf("incorrect measurement basis %v",body)}
 if status:=request(http.MethodGet,base+"&round=not-a-number").Code;status!=http.StatusBadRequest{t.Fatalf("invalid round %d",status)}
 if status:=request(http.MethodGet,base+"&role=admin").Code;status!=http.StatusBadRequest{t.Fatalf("invalid role %d",status)}
 if status:=request(http.MethodPost,base).Code;status!=http.StatusMethodNotAllowed{t.Fatalf("mutation allowed %d",status)}
}
