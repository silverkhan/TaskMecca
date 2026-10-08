package maintenance

import (
    "bytes"
    "crypto/sha256"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync/atomic"
    "testing"
    "time"
)

// The binary downloader must not inherit metadata's short full-body timeout.
// Flush the headers immediately and delay the remainder of a response.
func TestReleaseDownloadAllowsSlowCompleteBody(t *testing.T) {
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
        _,_=w.Write([]byte("part-"))
        w.(http.Flusher).Flush()
        time.Sleep(90*time.Millisecond)
        _,_=w.Write([]byte("complete"))
    }))
    defer server.Close()
    if _,err:=httpGetWithPolicy(server.URL,30*time.Millisecond,1024,1);err==nil {
        t.Fatal("short total-body deadline should fail on slow response")
    }
    data,err:=httpGetWithPolicy(server.URL,1*time.Second,1024,1)
    if err!=nil { t.Fatalf("longer binary deadline should succeed: %v",err) }
    if string(data)!="part-complete" { t.Fatalf("truncated body: %q",data) }
}

func TestReleaseDownloadRejectsOversizedBodies(t *testing.T) {
    for _,announce:=range []bool{true,false} {
        t.Run(fmt.Sprint("announce=",announce),func(t *testing.T) {
            server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
                if !announce { w.Header().Set("Transfer-Encoding","chunked") }
                _,_=w.Write(bytes.Repeat([]byte("x"),64))
            }))
            defer server.Close()
            _,err:=httpGetWithPolicy(server.URL,1*time.Second,16,1)
            if err==nil || !strings.Contains(err.Error(),"maximum download size") {
                t.Fatalf("expected size limit error, got %v",err)
            }
        })
    }
}

func TestReleaseDownloadDoesNotRetryPermanent404(t *testing.T) {
    var requests int32
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
        atomic.AddInt32(&requests,1)
        http.NotFound(w,r)
    }))
    defer server.Close()
    _,err:=httpGetWithPolicy(server.URL,1*time.Second,1024,3)
    if err==nil || !strings.Contains(err.Error(),"HTTP 404") { t.Fatalf("expected HTTP 404, got %v",err) }
    if got:=atomic.LoadInt32(&requests);got!=1 { t.Fatalf("404 should fail fast, requests=%d",got) }
}

func TestReleaseDownloadRetriesTruncatedBodies(t *testing.T) {
    var requests int32
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
        if atomic.AddInt32(&requests,1)==1 {
            w.Header().Set("Content-Length","15")
            _,_=w.Write([]byte("partial"))
            return
        }
        _,_=w.Write([]byte("complete"))
    }))
    defer server.Close()
    data,err:=httpGetWithPolicy(server.URL,1*time.Second,1024,2)
    if err!=nil { t.Fatalf("retry failed: %v",err) }
    if string(data)!="complete" { t.Fatalf("body from wrong attempt: %q",data) }
    if got:=atomic.LoadInt32(&requests);got!=2 { t.Fatalf("want two attempts, got %d",got) }
}

func TestReleaseBinaryRequiresMatchingSHA256BeforeInstall(t *testing.T) {
    asset,err:=assetName()
    if err!=nil { t.Skipf("unsupported platform: %v",err) }
    binary:=[]byte("test executable payload, no real replacement")
    digest:=sha256.Sum256(binary)
    wrong:=false
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request) {
        switch r.URL.Path {
        case "/"+asset:
            _,_=w.Write(binary)
        case "/SHA256SUMS.txt":
            if wrong {
                _,_=w.Write([]byte(fmt.Sprintf("%064x  %s\n",0,asset)))
            } else {
                _,_=w.Write([]byte(fmt.Sprintf("%x  %s\n",digest,asset)))
            }
        default: http.NotFound(w,r)
        }
    }))
    defer server.Close()
    got,err:=fetchReleaseBinary(server.URL)
    if err!=nil || !bytes.Equal(got,binary) { t.Fatalf("valid download failed: %q %v",got,err) }
    wrong=true
    if _,err:=fetchReleaseBinary(server.URL);err==nil || !strings.Contains(err.Error(),"SHA-256") {
        t.Fatalf("checksum mismatch must fail, got %v",err)
    }
}
