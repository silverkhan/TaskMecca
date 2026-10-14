package webui

import (
    "net/http"
    "os"
    "strings"
    "strconv"
)

// PreviewMode is a defense-in-depth guard for builds which support it.
// Older PR binaries MUST NOT be assumed to implement this guard: the parent
// Preview manager still needs an independent host/OS isolation boundary.
func PreviewMode() bool {
    return os.Getenv("TASK_MECCA_PREVIEW_MODE")=="1"
}

func previewReadAllowed(r *http.Request) bool {
    if r.Method!=http.MethodGet && r.Method!=http.MethodHead{return false}
    path:=r.URL.Path
    for _,prefix:=range []string{
        "/api/notifications", "/api/runtime", "/api/terminal",
        "/api/preview", "/api/channel", "/api/upgrade",
        "/api/web", "/terminal", "/api/operations/action",
    } {
        if strings.HasPrefix(path,prefix){return false}
    }
    // Deny service worker install; Preview must never become a push receiver.
    if path=="/sw.js"||path=="/manifest.webmanifest"{return false}
    return true
}

func previewGuard(next http.Handler) http.Handler {
    if !PreviewMode() {return next}
    return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        if !previewReadAllowed(r){
            w.Header().Set("Cache-Control","no-store")
            w.Header().Set("Content-Type","application/json; charset=utf-8")
            w.WriteHeader(http.StatusForbidden)
            _,_=w.Write([]byte(`{"error":"operation disabled in isolated Preview mode"}`))
            return
        }
        next.ServeHTTP(w,r)
    })
}

func previewIdentity() (int,string) {
 if !PreviewMode(){return 0,""}
 number,err:=strconv.Atoi(os.Getenv("TASK_MECCA_PREVIEW_PR"))
 sha:=strings.TrimSpace(os.Getenv("TASK_MECCA_PREVIEW_SHA"))
 if err!=nil||number<1||len(sha)!=40{return 0,""}
 for _,r:=range sha{if !((r>='0'&&r<='9')||(r>='a'&&r<='f')){return 0,""}}
 return number,sha
}
