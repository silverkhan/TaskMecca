package backlog

import (
    "path/filepath"
    "strings"
)

func Audit(project,root string) ([]map[string]string,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    findings:=[]map[string]string{}
    for _,row:=range rows {
        required:=[]string{}
        if row.Location=="active" && row.State=="doing" {
            required=[]string{"Agent","변경범위"}
        } else if row.Location=="active" && (row.State=="todo" || row.State=="hold") {
            for _,name:=range []string{"Agent","변경범위"} {
                value:=strings.TrimSpace(row.Fields[name])
                if value!="" && value!="-" {
                    findings=append(findings,map[string]string{
                        "file":filepath.Base(row.Path),
                        "state":row.State,
                        "field":name,
                        "value":value,
                        "problem":"미배정 상태는 현재 Agent/변경범위를 가질 수 없다.",
                    })
                }
            }
        } else if row.State=="done" {
            required=[]string{"Agent","결과","검증"}
        }
        for _,name:=range required {
            if strings.TrimSpace(row.Fields[name])=="" {
                findings=append(findings,map[string]string{
                    "file":filepath.Base(row.Path),
                    "state":row.State,
                    "field":name,
                    "value":"<missing>",
                    "problem":"상태에 필요한 필드가 비어 있다.",
                })
            }
        }
    }
    return findings,nil
}
