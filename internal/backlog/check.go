package backlog

import (
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

var markdownLink = regexp.MustCompile("\\]\\(([^)]+)\\)")

func DanglingLinks(protocol string) ([]string,error) {
    data,err:=os.ReadFile(protocol)
    if err!=nil { return nil,err }
    text:=strings.TrimPrefix(string(data),"\ufeff")
    missing:=[]string{}
    for _,match:=range markdownLink.FindAllStringSubmatch(text,-1) {
        target:=match[1]
        if strings.HasPrefix(target,"http://") || strings.HasPrefix(target,"https://") || strings.HasPrefix(target,"#") { continue }
        relative:=strings.TrimSpace(strings.SplitN(target,"#",2)[0])
        if relative=="" { continue }
        if _,err:=os.Stat(filepath.Join(filepath.Dir(protocol),relative)); err!=nil {
            if os.IsNotExist(err) { missing=append(missing,target); continue }
            return nil,err
        }
    }
    return missing,nil
}

func Check(project,root,protocol string) ([]string,error) {
    if protocol=="" { protocol=filepath.Join(project,"_task_mecca","framework","collab.md") }
    if !filepath.IsAbs(protocol) {
        absolute,err:=filepath.Abs(protocol)
        if err==nil { protocol=absolute }
    }
    if stat,err:=os.Stat(protocol); err!=nil || stat.IsDir() {
        if err!=nil && !os.IsNotExist(err) { return nil,err }
        return []string{"__missing_protocol__:"+protocol},nil
    }
    problems:=[]string{}
    links,err:=DanglingLinks(protocol)
    if err!=nil { return nil,err }
    for _,target:=range links { problems=append(problems,"없는 경로: "+target) }
    rows,err:=Catalog(project,filepath.Dir(protocol))
    if err!=nil { return nil,err }
    deps:=DependencyReport(rows)
    if missing,ok:=deps["missing"].([]map[string]string); ok {
        for _,row:=range missing { problems=append(problems,"없는 선행: "+row["id"]+" -> "+row["depends_on"]) }
    }
    if cycles,ok:=deps["cycles"].([][]string); ok {
        for _,cycle:=range cycles { problems=append(problems,"순환 선행: "+strings.Join(cycle," -> ")) }
    }
    return problems,nil
}
