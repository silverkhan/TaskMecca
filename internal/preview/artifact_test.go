package preview

import (
    "archive/zip"
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func testArchive(t *testing.T,pr int,sha string,run int64,corruptChecksum,wrongManifest,unsafePath bool)[]byte {
    t.Helper()
    var raw bytes.Buffer
    writer:=zip.NewWriter(&raw)
    m:=manifest{Repository:Repository,PRNumber:pr,HeadSHA:sha,RunID:run,Workflow:workflow,SchemaVersion:1}
    if wrongManifest{m.HeadSHA=strings.Repeat("d",40)}
    manifestBytes,_:=json.Marshal(m)
    fileName:="task-mecca-linux-amd64"
    if unsafePath{fileName="../task-mecca-linux-amd64"}
    binary:=[]byte("fake standalone preview executable fixture")
    checksum:=sha256.Sum256(binary)
    if corruptChecksum{checksum[0]^=0xff}
    entries:=map[string][]byte{
        "preview-build.json":manifestBytes,
        "SHA256SUMS.txt":[]byte(hex.EncodeToString(checksum[:])+"  task-mecca-linux-amd64\n"),
        fileName:binary,
    }
    for _,key:=range []string{"preview-build.json","SHA256SUMS.txt",fileName}{
        f,err:=writer.Create(key)
        if err!=nil{t.Fatal(err)}
        if _,err= f.Write(entries[key]);err!=nil{t.Fatal(err)}
    }
    if err:=writer.Close();err!=nil{t.Fatal(err)}
    return raw.Bytes()
}

func TestDownloadVerifiedChecksRunManifestChecksumAndCache(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    artifact:=testArchive(t,207,sha,9001,false,false,false)
    calls:=0
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        calls++
        if r.URL.Path!="/repos/silverkhan/TaskMecca/actions/artifacts/300/zip"{
            http.Error(w,"incorrect artifact",404);return
        }
        _,_ = w.Write(artifact)
    }))
    defer server.Close()
    client:=Client{HTTP:server.Client(),APIURL:server.URL}
    build:=Build{Number:207,HeadSHA:sha,RunID:9001,BuildStatus:"success",
        ArtifactID:300,ArtifactName:"task-mecca-preview-pr-207-"+sha}
    cached,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
    if err!=nil{t.Fatal(err)}
    data,err:=os.ReadFile(cached)
    if err!=nil||string(data)!="fake standalone preview executable fixture"{t.Fatalf("downloaded data: %q %v",data,err)}
    if filepath.Base(cached)!="task-mecca-linux-amd64"{t.Fatalf("invalid target path %q",cached)}
    stat,err:=os.Stat(cached)
    if err!=nil{t.Fatal(err)}
    if stat.Mode().Perm()&0111!=0 {t.Fatal("verified bytes must be cached non-executable until sandbox policy is ready")}
    again,err:=client.DownloadVerified(context.Background(),build,filepath.Dir(filepath.Dir(filepath.Dir(cached))),"linux","amd64")
    if err!=nil||again!=cached {t.Fatalf("cache reuse failed %q %v",again,err)}
    if calls!=2 {t.Fatalf("unexpected download calls %d",calls)}
}

func TestDownloadVerifiedRejectsSpoofingAndZIPTraversal(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    for _,mode:=range []string{"checksum","manifest","traversal"} {
        t.Run(mode,func(t *testing.T){
            archive:=testArchive(t,207,sha,9001,mode=="checksum",mode=="manifest",mode=="traversal")
            server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){_,_=w.Write(archive)}))
            defer server.Close()
            client:=Client{HTTP:server.Client(),APIURL:server.URL}
            root:=t.TempDir()
            build:=Build{Number:207,HeadSHA:sha,RunID:9001,BuildStatus:"success",
                ArtifactID:300,ArtifactName:"task-mecca-preview-pr-207-"+sha}
            if _,err:=client.DownloadVerified(context.Background(),build,root,"linux","amd64");err==nil{
                t.Fatal("untrusted artifact accepted")
            }
            if entries,err:=os.ReadDir(root);err!=nil||len(entries)>0{
                t.Fatalf("untrusted artifact wrote cache: %v %v",entries,err)
            }
        })
    }
}

func TestDownloadVerifiedRejectsUnknownOrMismatchedBuild(t *testing.T) {
    sha:=strings.Repeat("a",40)
    c:=Client{APIURL:"https://api.github.com"}
    for _,build:=range []Build{
        {Number:12,HeadSHA:sha,ArtifactID:1,RunID:22,BuildStatus:"in_progress",ArtifactName:"task-mecca-preview-pr-12-"+sha},
        {Number:12,HeadSHA:sha,ArtifactID:1,RunID:22,BuildStatus:"success",ArtifactName:"task-mecca-preview-pr-13-"+sha},
        {Number:12,HeadSHA:"invalid",ArtifactID:1,RunID:22,BuildStatus:"success",ArtifactName:"task-mecca-preview-pr-12-invalid"},
        {Number:12,HeadSHA:sha,ArtifactID:0,RunID:22,BuildStatus:"success",ArtifactName:"task-mecca-preview-pr-12-"+sha},
    }{
        _,err:=c.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
        if err==nil {t.Fatalf("accepted mismatched identity %+v",build)}
    }
    _,err:=assetForPlatform("linux","arm64")
    if err==nil{t.Fatal("unsupported platform was accepted")}
    if _,err:=io.Copy(io.Discard,strings.NewReader("test"));err!=nil{t.Fatal(err)}
}
