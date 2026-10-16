package preview

import (
    "archive/zip"
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func testArchive(t *testing.T,pr int,sha string,run int64,corruptChecksum,wrongManifest,unsafePath bool)[]byte {
    return testArchiveWithChecksumPath(t,pr,sha,run,corruptChecksum,wrongManifest,unsafePath,"task-mecca-linux-amd64")
}
func testArchiveWithChecksumPath(t *testing.T,pr int,sha string,run int64,corruptChecksum,wrongManifest,unsafePath bool,checksumPath string,protocol ...int)[]byte {
    t.Helper()
    var raw bytes.Buffer
    writer:=zip.NewWriter(&raw)
    m:=manifest{Repository:Repository,PRNumber:pr,HeadSHA:sha,RunID:run,Workflow:workflow,SchemaVersion:1}
    if len(protocol)>0 {m.ExecutionSwitchProtocol=protocol[0]}
    if wrongManifest{m.HeadSHA=strings.Repeat("d",40)}
    manifestBytes,_:=json.Marshal(m)
    fileName:="task-mecca-linux-amd64"
    if unsafePath{fileName="../task-mecca-linux-amd64"}
    binary:=[]byte("fake standalone preview executable fixture")
    checksum:=sha256.Sum256(binary)
    if corruptChecksum{checksum[0]^=0xff}
    entries:=map[string][]byte{
        "preview-build.json":manifestBytes,
        "SHA256SUMS.txt":[]byte(hex.EncodeToString(checksum[:])+"  "+checksumPath+"\n"),
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
}

func TestPreviewArtifact401UsesExistingGitHubCLILogin(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    archive:=testArchive(t,213,sha,500,false,false,false)
    attempts:=0
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        attempts++
        if r.Header.Get("Authorization")!="Bearer existing-gh-token"{
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        w.Header().Set("Content-Type","application/zip")
        _,_=w.Write(archive)
    }))
    defer server.Close()
    fallbackCalls:=0
    client:=Client{HTTP:server.Client(),APIURL:server.URL,
        TokenFallback:func(ctx context.Context)(string,error){
            fallbackCalls++
            return "existing-gh-token",nil
        },
    }
    build:=Build{Number:213,HeadSHA:sha,RunID:500,BuildStatus:"success",
        ArtifactID:300,ArtifactName:"task-mecca-preview-pr-213-"+sha}
    path,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
    if err!=nil{t.Fatal(err)}
    data,err:=os.ReadFile(path)
    if err!=nil||string(data)!="fake standalone preview executable fixture"{
        t.Fatalf("authenticated download content: %v %q",err,data)
    }
    if attempts!=2||fallbackCalls!=1{
        t.Fatalf("expected one anonymous 401 then authenticated retry: attempts=%d fallback=%d",attempts,fallbackCalls)
    }
}

func TestPreviewArtifact401ExplainsCredentialSetupWithoutToken(t *testing.T){
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        w.WriteHeader(http.StatusUnauthorized)
    }))
    defer server.Close()
    build:=Build{Number:213,HeadSHA:sha,RunID:500,BuildStatus:"success",
        ArtifactID:300,ArtifactName:"task-mecca-preview-pr-213-"+sha}
    client:=Client{HTTP:server.Client(),APIURL:server.URL,
        TokenFallback:func(context.Context)(string,error){return "",errors.New("not signed in")},
    }
    _,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
    if err==nil||!strings.Contains(err.Error(),"Actions:read")||
        !strings.Contains(err.Error(),"TASK_MECCA_GITHUB_TOKEN")||
        !strings.Contains(err.Error(),"gh auth login"){
        t.Fatalf("missing credential must be actionable; got %v",err)
    }
    client.Token="invalid"
    fallbackCalls:=0
    client.TokenFallback=func(context.Context)(string,error){fallbackCalls++;return "different-token",nil}
    _,err=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
    if err==nil||!strings.Contains(err.Error(),"401")||!strings.Contains(err.Error(),"Actions:read")||fallbackCalls!=0{
        t.Fatalf("invalid explicit token must fail without retrying another credential; got %v",err)
    }
}

func TestPreviewArtifactPermissionAndExpirationErrors(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    for _,tc:=range []struct{code int;term string}{
        {http.StatusForbidden,"Actions:read"},
        {http.StatusGone,"expired"},
    } {
        t.Run(fmt.Sprintf("%d",tc.code),func(t *testing.T){
            server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
                w.WriteHeader(tc.code)
            }))
            defer server.Close()
            client:=Client{HTTP:server.Client(),APIURL:server.URL}
            build:=Build{Number:213,HeadSHA:sha,RunID:500,BuildStatus:"success",
                ArtifactID:300,ArtifactName:"task-mecca-preview-pr-213-"+sha}
            _,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
            if err==nil||!strings.Contains(err.Error(),tc.term){t.Fatalf("wrong status error: %v",err)}
        })
    }
}

func TestChecksumEntryAcceptsOnlyExactRootOrLegacyDistPaths(t *testing.T) {
    hash:=strings.Repeat("a",64)
    for _,platform:=range []struct{os,arch string}{
        {"darwin","arm64"},{"darwin","amd64"},{"windows","amd64"},{"linux","amd64"},
    }{
        name,err:=assetForPlatform(platform.os,platform.arch)
        if err!=nil{t.Fatal(err)}
        for _,entry:=range []string{name,"dist/"+name}{
            sums:=[]byte(hash+"  "+entry+"\n")
            got,err:=checksumEntry(sums,name)
            if err!=nil||got!=hash{t.Fatalf("platform=%s/%s entry=%s: %q %v",platform.os,platform.arch,entry,got,err)}
        }
        for _,bad:=range []string{
            hash+"  ../dist/"+name+"\n",
            hash+"  other/"+name+"\n",
            hash+"  dist/../"+name+"\n",
            hash+"  "+name+".old\n",
        }{
            if _,err=checksumEntry([]byte(bad),name);err==nil{
                t.Fatalf("ambiguous checksum path accepted: %q",bad)
            }
        }
        duplicate:=hash+"  "+name+"\n"+hash+"  dist/"+name+"\n"
        if _,err=checksumEntry([]byte(duplicate),name);err==nil{
            t.Fatalf("duplicate platform entry accepted for %s",name)
        }
        if _,err=checksumEntry([]byte("bad-hash  dist/"+name+"\n"),name);err==nil{
            t.Fatalf("invalid SHA accepted for %s",name)
        }
    }
}

func TestDownloadVerifiedLegacyDistChecksumWorksWithoutRebuild(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    archive:=testArchiveWithChecksumPath(t,213,sha,500,false,false,false,"dist/task-mecca-linux-amd64")
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        w.Header().Set("Content-Type","application/zip")
        _,_=w.Write(archive)
    }))
    defer server.Close()
    client:=Client{HTTP:server.Client(),APIURL:server.URL}
    build:=Build{Number:213,HeadSHA:sha,RunID:500,BuildStatus:"success",
        ArtifactID:300,ArtifactName:"task-mecca-preview-pr-213-"+sha}
    file,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64")
    if err!=nil{t.Fatalf("PR #213 style artifact rejected: %v",err)}
    actual,err:=os.ReadFile(file)
    if err!=nil||string(actual)!="fake standalone preview executable fixture"{
        t.Fatalf("bad verified binary: %v %q",err,actual)
    }
}

func TestDownloadVerifiedLegacyDistChecksumStillRejectsTampering(t *testing.T) {
    const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    artifact:=testArchiveWithChecksumPath(t,213,sha,500,true,false,false,"dist/task-mecca-linux-amd64")
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        _,_=w.Write(artifact)
    }))
    defer server.Close()
    client:=Client{HTTP:server.Client(),APIURL:server.URL}
    build:=Build{Number:213,HeadSHA:sha,RunID:500,BuildStatus:"success",
        ArtifactID:300,ArtifactName:"task-mecca-preview-pr-213-"+sha}
    cache:=t.TempDir()
    if _,err:=client.DownloadVerified(context.Background(),build,cache,"linux","amd64");err==nil||
       !strings.Contains(err.Error(),"checksum mismatch"){
        t.Fatalf("legacy checksum permitted altered binary: %v",err)
    }
    if entries,err:=os.ReadDir(cache);err!=nil||len(entries)!=0{
        t.Fatalf("rejected artifact created cached data: %v %v",entries,err)
    }
}

func TestDownloadVerifiedRequiresPRReturnProtocol(t *testing.T){
  const sha="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  build:=Build{Number:217,HeadSHA:sha,RunID:500,BuildStatus:"success",
    ArtifactID:300,ArtifactName:"task-mecca-preview-pr-217-"+sha}
  for _,tc:=range []struct{protocol int;ok bool}{{0,false},{1,true}}{
    t.Run(fmt.Sprintf("protocol-%d",tc.protocol),func(t *testing.T){
      archive:=testArchiveWithChecksumPath(t,217,sha,500,false,false,false,
        "task-mecca-linux-amd64",tc.protocol)
      server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        _,_=w.Write(archive)
      }))
      defer server.Close()
      client:=Client{HTTP:server.Client(),APIURL:server.URL}
      _,err:=client.DownloadVerified(context.Background(),build,t.TempDir(),"linux","amd64",true)
      if (err==nil)!=tc.ok{t.Fatalf("protocol=%d error=%v",tc.protocol,err)}
    })
  }
}
