package maintenance

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "testing"
)

func TestRollingReleaseURLPreservesParametersAndReplacesGeneration(t *testing.T){
    raw:="https://github.com/silverkhan/TaskMecca/releases/download/release-dev/VERSION.txt?keep=yes&_tm=old"
    updated:=releaseCacheURL(raw,"0.3.1-dev.31")
    parsed,err:=url.Parse(updated)
    if err!=nil {t.Fatal(err)}
    if parsed.Query().Get("keep")!="yes"||parsed.Query().Get("_tm")!="0.3.1-dev.31"{
        t.Fatalf("cache query lost or duplicated: %s",updated)
    }
    if len(parsed.Query()["_tm"])!=1 {t.Fatal("duplicate cache-generation marker")}
    if got:=releaseCacheURL(updated,"v32");strings.Count(got,"_tm=")!=1 {t.Fatalf("duplicate marker %q",got)}
    if got:=releaseCacheURL(":bad","version");got!=":bad" {t.Fatal("malformed URL should be unchanged")}
}

func TestRollingReleaseDownloadPairsBinaryWithChecksum(t *testing.T){
    asset,err:=assetName()
    if err!=nil {t.Skipf("unsupported test platform: %v",err)}
    const expectedVersion="0.3.1-dev.32"
    binary:=[]byte("temporary test binary bytes")
    hash:=sha256.Sum256(binary)
    assetMarker:=""
    manifestMarker:=""
    verifyMarker:=""
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        switch {
        case strings.HasSuffix(r.URL.Path,"/"+asset):
            assetMarker=r.URL.Query().Get("_tm")
            _,_=w.Write(binary)
        case strings.HasSuffix(r.URL.Path,"/SHA256SUMS.txt"):
            manifestMarker=r.URL.Query().Get("_tm")
            fmt.Fprintf(w,"%s  %s\n",hex.EncodeToString(hash[:]),asset)
        case strings.HasSuffix(r.URL.Path,"/VERSION.txt"):
            verifyMarker=r.URL.Query().Get("_tm")
            fmt.Fprint(w,expectedVersion)
        default:
            http.Error(w,"not found",404)
        }
    }))
    defer server.Close()
    data,err:=fetchReleaseBinaryWithProgress(server.URL+"/release-dev",nil,expectedVersion)
    if err!=nil{t.Fatal(err)}
    if string(data)!=string(binary){t.Fatal("binary altered")}
    if assetMarker!=expectedVersion||manifestMarker!=expectedVersion {
        t.Fatalf("generation mismatch binary=%q checksum=%q",assetMarker,manifestMarker)
    }
    if verifyMarker==""||verifyMarker==expectedVersion{
        t.Fatalf("VERSION check must bypass mutable cache independently: %q",verifyMarker)
    }
}

func TestRollingReleaseRejectsStaleButMatchingBinaryAndChecksum(t *testing.T){
    asset,err:=assetName()
    if err!=nil {t.Skipf("unsupported test platform: %v",err)}
    binary:=[]byte("old generation")
    hash:=sha256.Sum256(binary)
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        switch {
        case strings.HasSuffix(r.URL.Path,"/"+asset): _,_=w.Write(binary)
        case strings.HasSuffix(r.URL.Path,"/SHA256SUMS.txt"): fmt.Fprintf(w,"%s  %s\n",hex.EncodeToString(hash[:]),asset)
        case strings.HasSuffix(r.URL.Path,"/VERSION.txt"): fmt.Fprint(w,"0.3.1-dev.30")
        default: http.Error(w,"not found",404)
        }
    }))
    defer server.Close()
    _,err=fetchReleaseBinaryWithProgress(server.URL,nil,"0.3.1-dev.31")
    if err==nil||!strings.Contains(err.Error(),"rolling release changed"){
        t.Fatalf("stale release generation not rejected: %v",err)
    }
}

func TestRollingReleaseRejectsChecksumFromDifferentGeneration(t *testing.T){
    asset,err:=assetName()
    if err!=nil {t.Skipf("unsupported test platform: %v",err)}
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        switch {
        case strings.HasSuffix(r.URL.Path,"/"+asset): fmt.Fprint(w,"binary v32")
        case strings.HasSuffix(r.URL.Path,"/SHA256SUMS.txt"):
            hash:=sha256.Sum256([]byte("binary v31"))
            fmt.Fprintf(w,"%s  %s\n",hex.EncodeToString(hash[:]),asset)
        case strings.HasSuffix(r.URL.Path,"/VERSION.txt"): fmt.Fprint(w,"0.3.1-dev.32")
        default: http.Error(w,"not found",404)
        }
    }))
    defer server.Close()
    _,err=fetchReleaseBinaryWithProgress(server.URL,nil,"0.3.1-dev.32")
    if err==nil||!strings.Contains(err.Error(),"SHA-256 verification failed"){
        t.Fatalf("mixed artifact generation not rejected: %v",err)
    }
}
