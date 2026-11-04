package webui

import (
    "os"
    "path/filepath"
    "time"
)

// These are read-only source identities, never the output of a reconciliation.
// Never trust an incomplete dependency set: a missing project Git directory,
// alternate backlog root, symlink, unreadable file or excessive tree size
// disables the fast path and forces the canonical full computation.
type attentionSourceStamp struct {
    path string
    info os.FileInfo
}
type attentionSnapshotMemo struct {
    sources []attentionSourceStamp
    at time.Time
    payload map[string]any
}
const attentionSnapshotCheckpoint = 12*time.Second
const attentionSourceLimit = 2500

func attentionSourceInventory(project,root string) ([]attentionSourceStamp,bool) {
    project,err:=filepath.Abs(project);if err!=nil{return nil,false}
    root,err=filepath.Abs(root);if err!=nil{return nil,false}
    canonical:=filepath.Join(project,"_task_mecca","data","backlog")
    if root!=canonical {return nil,false}
    git:=filepath.Join(project,".git")
    info,err:=os.Lstat(git)
    if err!=nil || !info.IsDir(){return nil,false}
    sources:=make([]attentionSourceStamp,0,128)
    appendTree:=func(path string) bool {
        count:=len(sources)
        err:=filepath.WalkDir(path,func(name string,entry os.DirEntry,walkErr error) error{
            if walkErr!=nil{return walkErr}
            if entry.Type()&os.ModeSymlink!=0{return os.ErrInvalid}
            meta,e:=entry.Info()
            if e!=nil || !(meta.IsDir() || meta.Mode().IsRegular()){return os.ErrInvalid}
            sources=append(sources,attentionSourceStamp{path:name,info:meta})
            if len(sources)>attentionSourceLimit{return os.ErrInvalid}
            return nil
        })
        return err==nil && len(sources)>count
    }
    if !appendTree(filepath.Join(project,"_task_mecca")){return nil,false}
    // Track the files that define Git history visible to lifecycleGitLog.
    // Never traverse .git/objects, since it can contain millions of entries.
    for _,name:=range []string{"HEAD","packed-refs","index","config","logs/HEAD","refs"}{
        path:=filepath.Join(git,name)
        info,err:=os.Lstat(path)
        if os.IsNotExist(err){continue}
        if err!=nil || info.Mode()&os.ModeSymlink!=0{return nil,false}
        if info.IsDir(){
            if !appendTree(path){return nil,false}
        } else if info.Mode().IsRegular(){
            sources=append(sources,attentionSourceStamp{path:path,info:info})
        } else {return nil,false}
    }
    return sources,true
}

func sameAttentionSources(a,b []attentionSourceStamp) bool {
    if len(a)!=len(b){return false}
    for i:=range a{
        left,right:=a[i],b[i]
        if left.path!=right.path || left.info.Size()!=right.info.Size() ||
            !left.info.ModTime().Equal(right.info.ModTime()) ||
            left.info.Mode()!=right.info.Mode() ||
            !os.SameFile(left.info,right.info){return false}
    }
    return true
}

func reuseAttentionSnapshot(memo *attentionSnapshotMemo, sources []attentionSourceStamp, valid bool,now time.Time) (map[string]any,bool) {
    if memo==nil || !valid || now.Before(memo.at) ||
        now.Sub(memo.at)>=attentionSnapshotCheckpoint ||
        !sameAttentionSources(memo.sources,sources){return nil,false}
    // Snapshot timestamps refer to this request, while the canonical payload
    // and notification IDs are unchanged. Only copy the outer map; nested
    // evidence is immutable until a source revision forces recomputation.
    payload:=make(map[string]any,len(memo.payload))
    for k,v:=range memo.payload{payload[k]=v}
    payload["snapshot_at"]=now.Format(time.RFC3339Nano)
    return payload,true
}
