package backlog

import (
    "errors"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "regexp"
    "sort"
    "strconv"
    "strings"
    "time"
)

var itemName = regexp.MustCompile(`^(\d{4}|\d{6})\.([A-Za-z]+-\d+)\.([a-z0-9-]+)\.(todo|doing|hold|done)\.md$`)
var prefixName = regexp.MustCompile(`^[A-Z]+$`)
var archiveMonthName = regexp.MustCompile(`^\d{4}-\d{2}$`)
var skip = map[string]bool{".git": true, ".venv": true, "venv": true, "node_modules": true, "__pycache__": true, ".mypy_cache": true, ".pytest_cache": true, "framework": true, ".runtime": true, "backups": true}

type Candidate struct {
    Path string `json:"path"`
    Name string `json:"name"`
    Canonical bool `json:"canonical"`
    UnderData bool `json:"under_data"`
    BacklogNamed bool `json:"backlog_named"`
    RecordCount int `json:"record_count"`
    LatestModified *string `json:"latest_modified"`
    LatestModifiedEpoch float64 `json:"latest_modified_epoch"`
}

type Record struct {
    ID string `json:"id"`
    SortKey string `json:"sort_key"`
    Slug string `json:"slug"`
    State string `json:"state"`
    Path string `json:"path"`
    Folder string `json:"folder"`
    Location string `json:"location"`
    ArchiveMonth string `json:"archive_month"`
    Title string `json:"title"`
    Fields map[string]string `json:"fields"`
    Document map[string]any `json:"document"`
    RawMarkdown string `json:"raw_markdown"`
    RuntimeMetadata map[string]any `json:"runtime_metadata"`
    Mtime string `json:"mtime"`
    Ctime string `json:"ctime"`
}

func named(path string) bool { return strings.HasPrefix(strings.ToLower(filepath.Base(path)), "backlog") }

func History(folder string) ([]string, error) {
    paths := []string{}
    entries, err := os.ReadDir(folder)
    if err != nil { return nil, err }
    for _, entry := range entries {
        if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") { paths = append(paths, filepath.Join(folder, entry.Name())) }
    }
    for _, sub := range []string{"archive", "_complete"} {
        dir := filepath.Join(folder, sub)
        if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) { continue } else if err != nil { return nil, err }
        err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
            if err != nil { return err }
            if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") { paths = append(paths, path) }
            return nil
        })
        if err != nil { return nil, err }
    }
    sort.Strings(paths)
    return paths, nil
}

func Discover(project, root string) ([]Candidate, error) {
    framework := filepath.Join(project, "_task_mecca")
    base := root
    if base == "" || filepath.Clean(base) == filepath.Join(framework, "framework") { base = framework }
    if named(base) { base = filepath.Dir(base) }
    base, err := filepath.Abs(base)
    if err != nil { return nil, err }
    depthLimit := 4
    if raw := os.Getenv("TASK_MECCA_BACKLOG_SCAN_DEPTH"); raw != "" { if n, parseErr := strconv.Atoi(raw); parseErr == nil { depthLimit = n } }
    out := []Candidate{}
    err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
        if errors.Is(err, os.ErrNotExist) { return nil }
        if err != nil { return err }
        if !entry.IsDir() { return nil }
        rel, err := filepath.Rel(base, path)
        if err != nil { return err }
        depth := 0
        if rel != "." { depth = len(strings.Split(rel, string(filepath.Separator))) }
        if path != base && (skip[entry.Name()] || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "archive" || entry.Name() == "_complete") { return filepath.SkipDir }
        if depth > depthLimit { return filepath.SkipDir }
        if !named(path) { return nil }
        paths, err := History(path)
        if err != nil { return err }
        latest := time.Time{}
        if stat, statErr := os.Stat(path); statErr == nil { latest = stat.ModTime() }
        count := 0
        for _, task := range paths {
            if !itemName.MatchString(filepath.Base(task)) { continue }
            count++
            if stat, statErr := os.Stat(task); statErr == nil && stat.ModTime().After(latest) { latest = stat.ModTime() }
        }
        abs, err := filepath.Abs(path)
        if err != nil { return err }
        dataRoot := filepath.Join(framework, "data")
        under := abs == dataRoot || strings.HasPrefix(abs, dataRoot+string(filepath.Separator))
        candidate := Candidate{Path: abs, Name: entry.Name(), Canonical: abs == filepath.Join(dataRoot, "backlog"), UnderData: under, BacklogNamed: true, RecordCount: count, LatestModifiedEpoch: float64(latest.UnixNano()) / 1e9}
        if !latest.IsZero() { formatted := latest.Local().Format("2006-01-02T15:04:05-07:00"); candidate.LatestModified = &formatted }
        out = append(out, candidate)
        return nil
    })
    if err != nil { return nil, err }
    sort.Slice(out, func(i,j int) bool {
        a,b := out[i],out[j]
        if (a.RecordCount>0)!=(b.RecordCount>0) { return a.RecordCount>0 }
        if a.Canonical!=b.Canonical { return a.Canonical }
        if a.UnderData!=b.UnderData { return a.UnderData }
        if a.LatestModifiedEpoch!=b.LatestModifiedEpoch { return a.LatestModifiedEpoch>b.LatestModifiedEpoch }
        return a.Path>b.Path
    })
    return out,nil
}

func Select(project, root string) (string,error) {
    if root!="" && named(root) {
        abs,err:=filepath.Abs(root)
        return abs,err
    }
    candidates,err:=Discover(project,root)
    if err!=nil { return "",err }
    if len(candidates)==0 { return "",nil }
    return candidates[0].Path,nil
}

func Ensure(project, root string) (map[string]any,error) {
    selected,err:=Select(project,root)
    if err!=nil { return nil,err }
    created:=false
    if selected=="" {
        selected=filepath.Join(project,"_task_mecca","data","backlog")
        if root!="" && named(root) { selected=root }
        if err=os.MkdirAll(selected,0755); err!=nil { return nil,err }
        created=true
    }
    selected,err=filepath.Abs(selected)
    if err!=nil { return nil,err }
    return map[string]any{"ok":true,"created":created,"path":selected,"canonical":selected==filepath.Join(project,"_task_mecca","data","backlog")},nil
}

func Catalog(project, root string) ([]Record,error) {
    folder,err:=Select(project,root)
    if err!=nil { return nil,err }
    rows:=[]Record{}
    if folder=="" { return rows,nil }
    paths,err:=History(folder)
    if err!=nil { return nil,err }
    for _,path:=range paths {
        match:=itemName.FindStringSubmatch(filepath.Base(path))
        if match==nil { continue }
        data,err:=os.ReadFile(path)
        if err!=nil { return nil,err }
        text:=strings.TrimPrefix(string(data),"\ufeff")
        fields:=parseFields(text)
        document:=documentModel(text,fields)
        if fields["결과"]=="" { if value,ok:=document["result"].(string); ok { fields["결과"]=value } }
        if fields["검증"]=="" { if value,ok:=document["verification"].(string); ok { fields["검증"]=value } }
        rel,_:=filepath.Rel(folder,path)
        location:="active"
        archiveMonth:=""
        if strings.Contains(rel,string(filepath.Separator)) {
            location="legacy"
            parts:=strings.Split(rel,string(filepath.Separator))
            if len(parts)>=3 && parts[0]=="archive" {
                location="archive"
                if archiveMonthName.MatchString(parts[1]) { archiveMonth=parts[1] }
            }
        }
        stat,statErr:=os.Stat(path)
        mtime,ctime:="",""
        if statErr==nil {
            mtime=stat.ModTime().UTC().Format(time.RFC3339Nano)
            ctime=mtime
        }
        rows=append(rows,Record{
            ID:strings.ToUpper(match[2]),SortKey:match[1],Slug:match[3],State:match[4],
            Path:path,Folder:filepath.Base(folder),Location:location,ArchiveMonth:archiveMonth,
            Title:titleOf(text),Fields:fields,Document:document,RawMarkdown:text,
            RuntimeMetadata:runtimeFromFields(fields),Mtime:mtime,Ctime:ctime,
        })
    }
    sort.Slice(rows,func(i,j int)bool { if rows[i].ID!=rows[j].ID { return rows[i].ID<rows[j].ID }; return rows[i].Path<rows[j].Path })
    return rows,nil
}

func NextID(project, root, prefix string) (map[string]any,error) {
    prefix=strings.ToUpper(strings.TrimSpace(prefix))
    if !prefixName.MatchString(prefix) { return nil,errors.New("접두사는 영문자만 사용할 수 있다.") }
    rows,err:=Catalog(project,root)
    if err!=nil { return nil,err }
    maximum:=0
    for _,row:=range rows {
        if !strings.HasPrefix(row.ID,prefix+"-") { continue }
        n,err:=strconv.Atoi(strings.TrimPrefix(row.ID,prefix+"-"))
        if err==nil && n>maximum { maximum=n }
    }
    number:=maximum+1
    if number>999999 { return nil,errors.New("6자리 정렬키 한계(999999)를 넘었다.") }
    return map[string]any{"prefix":prefix,"number":number,"id":fmt.Sprintf("%s-%d",prefix,number),"sort_key":fmt.Sprintf("%06d",number),"reservation":false},nil
}
