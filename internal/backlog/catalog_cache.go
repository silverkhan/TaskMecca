package backlog

import (
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
)

type catalogCacheFile struct {
    ModTime int64
    Size int64
    Record Record
}

type catalogCacheEntry struct {
    Files map[string]catalogCacheFile
}

var catalogCache = struct {
    sync.Mutex
    RefreshLocks map[string]*sync.Mutex
    Entries map[string]catalogCacheEntry
}{
    RefreshLocks:map[string]*sync.Mutex{},
    Entries:map[string]catalogCacheEntry{},
}

// Serialize refreshes for the same backlog only. A background sensor reading
// a different project's files must not block the foreground project open.
func catalogRefreshLock(key string) *sync.Mutex {
    catalogCache.Lock()
    defer catalogCache.Unlock()
    lock:=catalogCache.RefreshLocks[key]
    if lock==nil {
        lock=&sync.Mutex{}
        catalogCache.RefreshLocks[key]=lock
    }
    return lock
}

func parseRecordFile(folder,path string) (Record,error) {
    match:=itemName.FindStringSubmatch(filepath.Base(path))
    if match==nil { return Record{},os.ErrInvalid }
    data,err:=os.ReadFile(path)
    if err!=nil { return Record{},err }
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
        mtime=pythonUTCISO(stat.ModTime())
        ctime=pythonUTCISO(fileChangeTime(stat))
    }
    return Record{
        ID:strings.ToUpper(match[2]),SortKey:match[1],Slug:match[3],State:match[4],
        Path:path,Folder:filepath.Base(folder),Location:location,ArchiveMonth:archiveMonth,
        Title:titleOf(text),Fields:fields,Document:document,RawMarkdown:text,
        RuntimeMetadata:runtimeFromFields(fields),Mtime:mtime,Ctime:ctime,
    },nil
}

func CachedCatalog(project,root string) ([]Record,error) {
    folder,err:=Select(project,root)
    if err!=nil { return nil,err }
    if folder=="" { return []Record{},nil }
    key,err:=filepath.Abs(folder)
    if err!=nil { return nil,err }
    refresh:=catalogRefreshLock(key)
    refresh.Lock()
    defer refresh.Unlock()
    paths,err:=History(folder)
    if err!=nil { return nil,err }
    catalogCache.Lock()
    previous:=catalogCache.Entries[key]
    catalogCache.Unlock()
    if previous.Files==nil { previous.Files=map[string]catalogCacheFile{} }
    next:=catalogCacheEntry{Files:map[string]catalogCacheFile{}}
    rows:=make([]Record,0,len(paths))
    for _,path:=range paths {
        if itemName.FindStringSubmatch(filepath.Base(path))==nil { continue }
        stat,statErr:=os.Stat(path)
        if statErr!=nil { continue }
        mod:=stat.ModTime().UnixNano()
        size:=stat.Size()
        if cached,ok:=previous.Files[path]; ok && cached.ModTime==mod && cached.Size==size {
            next.Files[path]=cached
            rows=append(rows,cached.Record)
            continue
        }
        row,parseErr:=parseRecordFile(folder,path)
        if parseErr!=nil { return nil,parseErr }
        cached:=catalogCacheFile{ModTime:mod,Size:size,Record:row}
        next.Files[path]=cached
        rows=append(rows,row)
    }
    sort.Slice(rows,func(i,j int)bool { if rows[i].ID!=rows[j].ID { return rows[i].ID<rows[j].ID }; return rows[i].Path<rows[j].Path })
    catalogCache.Lock()
    catalogCache.Entries[key]=next
    catalogCache.Unlock()
    return rows,nil
}

func InvalidateCachedCatalog(root string) {
    if abs,err:=filepath.Abs(root); err==nil {
        catalogCache.Lock(); delete(catalogCache.Entries,abs); catalogCache.Unlock()
    }
}
