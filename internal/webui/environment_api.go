package webui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/preview"
)

// This is a release replacement, not another child Preview process.
// Mutations are localhost-only; a remote browser cannot replace the host
// executable merely by possessing the public Web URL.
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
		writeJSON(w,map[string]any{"target":target,"transition":pending},200)
	})
	mutate:=func(w http.ResponseWriter,r *http.Request,enter bool){
		if PreviewMode() || maintenance.CurrentChannel()!="dev" || restartCh==nil {
			writeJSON(w,map[string]any{"error":"PR switching requires the managed Dev Web"},403)
			return
		}
		if !previewLocalMutation(r){
			writeJSON(w,map[string]any{"error":"PR version switching requires a local same-origin action"},403)
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
		ctx,cancel:=context.WithTimeout(r.Context(),115*time.Second)
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
			path,downloadErr:=client.DownloadVerified(ctx,match,filepath.Join(cache,"task-mecca","environment-switch"),runtime.GOOS,runtime.GOARCH)
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
			select {case restartCh<-result:default:
				// Should be unreachable for the normal exclusive server
				// restart handoff. Persisted checkpoint prevents false
				// success if a channel consumer is unexpectedly absent.
				_ = errors.New("restart handoff unavailable")
			}
		}
	}
	mux.HandleFunc("/api/environment/enter-pr",func(w http.ResponseWriter,r *http.Request){mutate(w,r,true)})
	mux.HandleFunc("/api/environment/return-dev",func(w http.ResponseWriter,r *http.Request){mutate(w,r,false)})
}
