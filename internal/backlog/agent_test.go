package backlog

import "testing"

func TestAgentPolicy(t *testing.T) {
    cases:=[]struct{path string; newWorker bool; valid bool; kind string}{
        {"/root",false,true,"root"},
        {"/root/registrar",false,true,"subagent"},
        {"/root/controller/pairi",true,true,"worker"},
        {"/root/controller/charmander",false,true,"worker"},
        {"/root/controller/charmander",true,false,"worker"},
        {"/root/controller/unknown",false,false,"worker"},
        {"/root/Bad",false,false,"invalid"},
    }
    for _,test:=range cases {
        got:=AgentReport(test.path,test.newWorker)
        if got["ok"]!=test.valid || got["kind"]!=test.kind { t.Errorf("%s new=%v: %+v",test.path,test.newWorker,got) }
    }
}
