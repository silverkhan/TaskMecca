package webui

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"
)

func TestActivityAPIsReadOnlyEmptyEvidence(t *testing.T){
    project:=t.TempDir()
    registerWebFixture(t,project)
    handler,err:=Handler(project,"","test")
    if err!=nil{t.Fatal(err)}
    paths:=[]string{
        "/api/activity?"+url.Values{"project":{project},"task":{"B-1"}}.Encode(),
        "/api/activity/project?"+url.Values{"project":{project}}.Encode(),
    }
    for _,path:=range paths{
        response:=httptest.NewRecorder()
        handler.ServeHTTP(response,httptest.NewRequest(http.MethodGet,path,nil))
        if response.Code!=http.StatusOK{t.Fatalf("%s: %d %s",path,response.Code,response.Body.String())}
        var body map[string]any
        if err:=json.Unmarshal(response.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
        if _,ok:=body["error"];ok{t.Fatalf("%s error: %+v",path,body)}
    }
    response:=httptest.NewRecorder()
    handler.ServeHTTP(response,httptest.NewRequest(http.MethodPost,paths[0],nil))
    if response.Code!=http.StatusMethodNotAllowed{t.Fatalf("mutation incorrectly accepted: %d",response.Code)}
}
