package backlog

import (
    "encoding/json"
    "regexp"
    "sort"
    "strings"
)

var searchWord = regexp.MustCompile(`[0-9A-Za-z가-힣_]+`)

type SearchResult struct {
    ID string `json:"id"`
    State string `json:"state"`
    Location string `json:"location"`
    Title string `json:"title"`
    Description string `json:"description"`
    Path string `json:"path"`
    Score int `json:"score"`
}

type SearchReport struct {
    Query string `json:"query"`
    Results []SearchResult `json:"results"`
    Warning string `json:"warning"`
}

func normalizeWords(value string) []string {
    matches:=searchWord.FindAllString(strings.ToLower(value),-1)
    out:=[]string{}
    for _,word:=range matches {
        if len([]rune(word))>1 { out=append(out,word) }
    }
    return out
}

func Search(project,root,query string,limit int) (SearchReport,error) {
    rows,err:=Catalog(project,root)
    if err!=nil { return SearchReport{},err }
    words:=normalizeWords(query)
    results:=[]SearchResult{}
    q:=strings.ToLower(query)
    for _,row:=range rows {
        description:=row.Fields["설명"]
        document,_:=json.Marshal(row.Document)
        fields,_:=json.Marshal(row.Fields)
        haystack:=strings.ToLower(strings.Join([]string{row.ID,row.Title,description,string(document),string(fields)}," "))
        score:=0
        titleLower:=strings.ToLower(row.Title)
        for _,word:=range words {
            if strings.Contains(haystack,word) {
                if strings.Contains(titleLower,word) { score+=6 } else { score+=2 }
            }
        }
        if strings.Contains(haystack,q) { score+=8 }
        if score==0 { continue }
        if len(description)>500 { description=description[:500] }
        results=append(results,SearchResult{
            ID:row.ID,State:row.State,Location:row.Location,Title:row.Title,
            Description:description,Path:row.Path,Score:score,
        })
    }
    sort.Slice(results,func(i,j int)bool {
        if results[i].Score!=results[j].Score { return results[i].Score>results[j].Score }
        return results[i].ID<results[j].ID
    })
    if limit<1 { limit=1 }
    if len(results)>limit { results=results[:limit] }
    return SearchReport{
        Query:query,Results:results,
        Warning:"검색 점수는 읽을 후보를 좁힐 뿐 의미상 중복을 확정하지 않는다.",
    },nil
}
