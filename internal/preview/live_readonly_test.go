package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

// Explicit opt-in integration probe. Never installs, executes, dispatches or
// starts a Web process; verified artifact bytes remain non-executable cache.
func TestLivePreviewReadOnlyProbe(t *testing.T) {
	if os.Getenv("TASK_MECCA_LIVE_PREVIEW") != "1" {
		t.Skip("explicit read-only GitHub probe not requested")
	}
	number, err := strconv.Atoi(os.Getenv("TASK_MECCA_LIVE_PREVIEW_PR"))
	sha := os.Getenv("TASK_MECCA_LIVE_PREVIEW_SHA")
	if err != nil || number < 1 || !validSHA(sha) {
		t.Fatal("explicit PR number and exact SHA required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	c := NewClient()
	builds, err := c.ListBuilds(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	selectBuild := func(rows []Build) Build {
		for _, b := range rows {
			if b.Number == number && b.HeadSHA == sha {
				return b
			}
		}
		return Build{}
	}
	b := selectBuild(builds)
	if b.BuildStatus != "success" || b.RunID < 1 || b.ArtifactID < 1 {
		t.Fatal("current exact PR artifact was not discovered")
	}
	cache := os.Getenv("TASK_MECCA_LIVE_PREVIEW_CACHE")
	if cache == "" {
		cache = t.TempDir()
	} else {
		cache, err = filepath.Abs(cache)
		if err != nil {
			t.Fatal("invalid probe cache path")
		}
	}
	file, err := c.DownloadVerified(ctx, b, cache, runtime.GOOS, runtime.GOARCH, true)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal("cannot inspect verified cache")
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0111 != 0 {
		t.Fatal("probe cache unexpectedly executable")
	}
	again, err := c.ListBuilds(ctx, 30)
	if err != nil {
		t.Fatal(err)
	}
	current := selectBuild(again)
	if current.BuildStatus != "success" || current.RunID != b.RunID || current.ArtifactID != b.ArtifactID {
		t.Fatal("PR/run/artifact changed during verification")
	}
	digest := sha256.Sum256(data)
	output := map[string]any{"pr": number, "sha": sha, "run_id": b.RunID, "artifact_id": b.ArtifactID, "platform": runtime.GOOS + "/" + runtime.GOARCH, "binary_bytes": len(data), "binary_sha256": hex.EncodeToString(digest[:]), "protocol_verified": true, "metadata_rechecked": true, "installed": false, "executed": false, "transitioned": false}
	safe, _ := json.Marshal(output)
	t.Log(string(safe))
}
