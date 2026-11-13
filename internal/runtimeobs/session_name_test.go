package runtimeobs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The fake executable is this test binary, never a real provider/service.
func TestNameProtocolHelper(t *testing.T) {
	if os.Getenv("A69_METADATA_HELPER") != "1" {
		return
	}
	args := os.Args
	mode := os.Getenv("A69_METADATA_MODE")
	// A valid schema helper may start slowly; this is not a protocol timeout.
	if mode == "slow_schema" && strings.Contains(strings.Join(os.Args, " "), "generate-json-schema") {
		time.Sleep(650 * time.Millisecond)
	}
	if strings.Contains(strings.Join(args, " "), "generate-json-schema") {
		var dir string
		for i, a := range args {
			if a == "--out" {
				dir = args[i+1]
			}
		}
		methods := []string{"thread/read", "thread/name/set"}
		if mode == "schema_error" {
			os.Exit(3)
		}
		if mode == "unsupported" {
			methods = methods[:1]
		}
		entries := []any{}
		for _, m := range methods {
			entries = append(entries, map[string]any{"properties": map[string]any{"method": map[string]any{"enum": []string{m}}}})
		}
		b, _ := json.Marshal(map[string]any{"oneOf": entries})
		_ = os.WriteFile(filepath.Join(dir, "ClientRequest.json"), b, 0600)
		os.Exit(0)
	}
	name := os.Getenv("A69_METADATA_INITIAL")
	reads := 0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req struct {
			ID     int            `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.Unmarshal(scanner.Bytes(), &req)
		if req.ID == 0 {
			continue
		}
		result := any(map[string]any{})
		switch req.Method {
		case "initialize":
		case "thread/name/set":
			if mode == "set_error" {
				fmt.Printf("{\"id\":%d,\"error\":{\"code\":-1,\"message\":\"refused\"}}\n", req.ID)
				continue
			}
			if mode != "mismatch" {
				name = req.Params["name"].(string)
			}
			if mode == "timeout_after_set" {
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}
		case "thread/read":
			reads++
			if req.Params["includeTurns"] != false {
				os.Exit(6)
			}
			if mode == "partial" && reads > 1 {
				os.Exit(0)
			}
			if mode == "timeout" {
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}
			id := req.Params["threadId"].(string)
			if mode == "wrong_id" || (mode == "wrong_id_after" && reads > 1) {
				id = "foreign"
			}
			result = map[string]any{"thread": map[string]any{"id": id, "name": name}}
			if mode == "missing_name" {
				result = map[string]any{"thread": map[string]any{"id": id, "preview": "must not become a display name"}}
			}
			if mode == "null_name" {
				result = map[string]any{"thread": map[string]any{"id": id, "name": nil, "preview": "must not become a display name", "title": "legacy first prompt"}}
			}
		default:
			os.Exit(7) // No turn, resume, start, query or other method allowed.
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"id": req.ID, "result": result})
	}
	os.Exit(0)
}
func mockNameCodex(t *testing.T, mode, initial string) {
	t.Helper()
	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := "#!/bin/sh\nexec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=^TestNameProtocolHelper$ -- \"$@\"\n"
	if err = os.WriteFile(filepath.Join(dir, "codex"), []byte(wrapper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("A69_METADATA_HELPER", "1")
	t.Setenv("A69_METADATA_MODE", mode)
	t.Setenv("A69_METADATA_INITIAL", initial)
}
func TestSessionNameOfficialProtocol(t *testing.T) {
	for _, tt := range []struct {
		mode, initial, status           string
		apply, verified, ack, requested bool
	}{
		{"", "legacy", "verified", true, true, true, false},
		{"", "D1112_01_Controller", "verified", true, true, false, false},
		{"", "legacy", "mismatch", false, false, false, false},
		{"mismatch", "legacy", "mismatch", true, false, true, true},
		{"slow_schema", "legacy", "verified", true, true, true, false},
		{"unsupported", "legacy", "unsupported", true, false, false, false},
		{"schema_error", "legacy", "error", true, false, false, false},
		{"set_error", "legacy", "partial", true, false, false, true},
		{"partial", "legacy", "partial", true, false, true, true},
		{"wrong_id", "legacy", "error", true, false, false, false},
		{"wrong_id_after", "legacy", "partial", true, false, true, true},
		{"timeout", "legacy", "timeout", true, false, false, false},
		{"timeout_after_set", "legacy", "partial", true, false, false, true},
		{"", "D1112_01_Worker_pikachyu_G1", "identity_conflict", true, false, false, false},
	} {
		t.Run(tt.mode+tt.initial+tt.status, func(t *testing.T) {
			mockNameCodex(t, tt.mode, tt.initial)
			// Normal paths use the adapter's bounded production context. Only the
			// intentionally nonresponsive fixtures need a shorter test deadline.
			ctx := context.Background()
			cancel := func() {}
			if tt.mode == "timeout" || tt.mode == "timeout_after_set" {
				ctx, cancel = context.WithTimeout(ctx, 500*time.Millisecond)
			}
			defer cancel()
			r := CodexSessionName(ctx, "exact-native", "D1112_01_Controller", tt.apply)
			if r.Status != tt.status || r.Verified != tt.verified || r.RequestAcknowledged != tt.ack || r.RequestedNotConfirmed != tt.requested {
				t.Fatalf("receipt=%+v", r)
			}
			if r.Verified && (r.ObservedSessionID != "exact-native" || r.ObservedName != "D1112_01_Controller") {
				t.Fatal("wrong readback")
			}
		})
	}
}
func TestSessionNameSharedCodexReaderChecksIdentity(t *testing.T) {
	mockNameCodex(t, "wrong_id", "")
	if _, err := ResolveCodexThreadMetadata(context.Background(), "exact"); err == nil {
		t.Fatal("foreign ID accepted")
	}
}

func TestA75OfficialMetadataSchemaPresence(t *testing.T) {
	for _, mode := range []string{"missing_name", "null_name"} {
		t.Run(mode, func(t *testing.T) {
			mockNameCodex(t, mode, "")
			meta, err := ResolveCodexThreadMetadata(context.Background(), "exact")
			if mode == "missing_name" {
				if err == nil {
					t.Fatal("missing name field treated as verified empty name")
				}
				return
			}
			if err != nil || meta.ID != "exact" || meta.Name != "" {
				t.Fatalf("metadata %+v error %v", meta, err)
			}
		})
	}
}

func TestSessionNameClaudeSDKFixture(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	for _, mode := range []string{"apply", "idempotent", "mismatch", "unsupported", "error", "partial", "identity", "generation"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			pkg := filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk")
			if err := os.MkdirAll(pkg, 0700); err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(filepath.Join(pkg, "package.json"), []byte(`{"type":"module","exports":"./index.js"}`), 0600)
			code := `let name='legacy';export async function getSessionInfo(id,opts){if(!opts.dir)throw Error('dir');return {sessionId:id,customTitle:name};} export async function renameSession(id,title,opts){name=title;}`
			switch mode {
			case "idempotent":
				code = `export async function getSessionInfo(id){return {sessionId:id,customTitle:'D1112_01_Controller'}};export async function renameSession(){throw Error('must not churn')}`
			case "mismatch":
				code = `export async function getSessionInfo(id){return {sessionId:id,customTitle:'legacy'}};export async function renameSession(){}`
			case "unsupported":
				code = `export const unsupported=true;`
			case "error":
				code = `export async function getSessionInfo(){throw Error('read')};export async function renameSession(){}`
			case "partial":
				code = `let n=0;export async function getSessionInfo(id){if(n++)throw Error('readback');return {sessionId:id,customTitle:'legacy'}};export async function renameSession(){}`
			case "identity":
				code = `export async function getSessionInfo(){return {sessionId:'foreign',customTitle:'D1112_01_Controller'}};export async function renameSession(){}`
			case "generation":
				code = `export async function getSessionInfo(id){return {sessionId:id,customTitle:'D1112_01_Worker_pikachyu_G1'}};export async function renameSession(){throw Error('must not rename')}`
			}
			_ = os.WriteFile(filepath.Join(pkg, "index.js"), []byte(code), 0600)
			r := ClaudeSessionName(context.Background(), "exact-native", "D1112_01_Controller", dir, true)
			want := map[string]string{"apply": "verified", "idempotent": "verified", "mismatch": "mismatch", "unsupported": "unsupported", "error": "error", "partial": "partial", "identity": "error", "generation": "identity_conflict"}[mode]
			if r.Status != want || r.Verified != (want == "verified") {
				t.Fatalf("%+v", r)
			}
		})
	}
}
