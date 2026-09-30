package backlog

import "testing"

func TestSourceFromFieldsParsesMarkdownLink(t *testing.T) {
    source:=sourceFromFields(map[string]string{"출처":"[Linear · ENG-123](https://linear.app/acme/issue/ENG-123/example)"})
    if source["provider"]!="Linear" || source["reference"]!="ENG-123" {
        t.Fatalf("unexpected source identity: %+v",source)
    }
    if source["url"]!="https://linear.app/acme/issue/ENG-123/example" {
        t.Fatalf("unexpected source url: %+v",source)
    }
}

func TestSourceFromFieldsKeepsPlainSource(t *testing.T) {
    source:=sourceFromFields(map[string]string{"출처":"GitHub · silverkhan/TaskMecca#84"})
    if source["label"]!="GitHub · silverkhan/TaskMecca#84" || source["url"]!="" {
        t.Fatalf("unexpected plain source: %+v",source)
    }
}
