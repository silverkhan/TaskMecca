package webui

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/preview"
)

// PR build and installed-environment mutations share the same authorization
// boundary. Remote Tailscale admin access is permitted only under the existing
// terminal identity and origin policy; plain reverse-proxy loopback is unsafe.
func approvedDevPRMutation(r *http.Request) bool {
    if previewLocalMutation(r){return true}
    return r.Method==http.MethodPost &&
        r.Header.Get("X-Task-Mecca-Action")=="1" &&
        strings.TrimSpace(r.Header.Get("Origin"))!="" &&
        terminalAccessAllowed(r,readTerminalSecuritySettings()) &&
        terminalOriginAllowed(r)
}

// This replaces one current executable; it does not launch another Preview.
func registerEnvironmentRoutes(
	mux *http.ServeMux,
	version string,
	restartCh chan<- maintenance.UpgradeResult,
	writeJSON func(http.ResponseWriter,any,int),
) {
	mux.HandleFunc("/api/environment",func(w http.ResponseWriter,r *http.Request){
		if r.Method!=http.MethodGet{writeJSON(w,map[string]any{"error":"GET required"},405);return}
		target,err:=maintenance.InstalledExecutionTarget()
		if err!=nil{writeJSON(w,map[string]any{"error":err.Error()},409);return}
		pending,err:=maintenance.PendingExecutionTransition()
		if err!=nil{writeJSON(w,map[string]any{"error":err.Error()},409);return}
		writeJSON(w,map[string]any{"target":target,"transition":pending,"switch_supported":runtime.GOOS!="windows","os":runtime.GOOS},200)
	})
	mutate:=func(w http.ResponseWriter,r *http.Request,enter bool){
		if PreviewMode() || maintenance.CurrentChannel()!="dev" || restartCh==nil {
			writeJSON(w,map[string]any{"error":"PR switching requires the managed Dev Web"},403)
			return
		}
		if !approvedDevPRMutation(r){
			writeJSON(w,map[string]any{"error":"PR version switching requires local access or an authorized Tailscale administration connection"},403)
			return
		}
		var input struct {
			Number int `json:"number"`
			SHA string `json:"sha"`
			Approve bool `json:"approve"`
		}
		d:=json.NewDecoder(http.MaxBytesReader(w,r.Body,2048))
		d.DisallowUnknownFields()
		if err:=d.Decode(&input);err!=nil||!input.Approve {
			writeJSON(w,map[string]any{"error":"explicit PR execution consent required"},400)
			return
		}
		if !enter&&(input.Number!=0||input.SHA!=""){
			writeJSON(w,map[string]any{"error":"return does not accept a target PR"},400);return
		}
		ctx,cancel:=stdcontext.WithTimeout(r.Context(),115*time.Second)
		defer cancel()
		var result maintenance.UpgradeResult
		var err error
		if enter {
			if input.Number<1 || len(input.SHA)!=40{
				writeJSON(w,map[string]any{"error":"a complete PR number and SHA are required"},400);return
			}
			target,targetErr:=maintenance.InstalledExecutionTarget()
			if targetErr!=nil||target.Kind!="dev"{
				writeJSON(w,map[string]any{"error":"return to Dev before selecting a PR"},409);return
			}
			client:=preview.NewClient()
			builds,fetchErr:=client.ListBuilds(ctx,30)
			if fetchErr!=nil{writeJSON(w,map[string]any{"error":fetchErr.Error()},409);return}
			var match preview.Build
			for _,build:=range builds{
				if build.Number==input.Number&&build.HeadSHA==input.SHA {
					match=build
					break
				}
			}
			if match.BuildStatus!="success" || match.ArtifactID<1 {
				writeJSON(w,map[string]any{"error":"verified current PR build unavailable"},409);return
			}
			cache,cacheErr:=os.UserCacheDir()
			if cacheErr!=nil{writeJSON(w,map[string]any{"error":cacheErr.Error()},500);return}
			path,downloadErr:=client.DownloadVerified(ctx,match,filepath.Join(cache,"task-mecca","environment-switch"),runtime.GOOS,runtime.GOARCH,true)
			if downloadErr!=nil{writeJSON(w,map[string]any{"error":downloadErr.Error()},409);return}
			// Requery immediately before installation; a PR that gained a
			// commit during downloading must not be installed as "current".
			refreshed,checkErr:=client.ListBuilds(ctx,30)
			if checkErr!=nil{writeJSON(w,map[string]any{"error":checkErr.Error()},409);return}
			stillCurrent:=false
			for _,build:=range refreshed {
				if build.Number==input.Number&&build.HeadSHA==input.SHA&&
					build.ArtifactID==match.ArtifactID&&build.BuildStatus=="success"{
					stillCurrent=true
					break
				}
			}
			if !stillCurrent{writeJSON(w,map[string]any{"error":"PR head changed during artifact download; refresh first"},409);return}
			result,err=maintenance.EnterPR(version,input.Number,input.SHA,path)
		}else{
			result,err=maintenance.ReturnFromPR(version)
		}
		if err!=nil {
			writeJSON(w,map[string]any{"error":err.Error()},409)
			return
		}
		writeJSON(w,map[string]any{"ok":true,"result":result,"restarting":true},200)
		if result.RestartRequired {
			select {case restartCh<-result:default:}
		}
	}
	mux.HandleFunc("/api/environment/enter-pr",func(w http.ResponseWriter,r *http.Request){mutate(w,r,true)})
	mux.HandleFunc("/api/environment/return-dev",func(w http.ResponseWriter,r *http.Request){mutate(w,r,false)})
}
