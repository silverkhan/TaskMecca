package runtimeobs

// Metadata-only official provider adapters: no model turns or operational writes.
import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/orchestration"
)

type codexMetadataClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	seq    int
}

func openCodexMetadata(ctx context.Context) (*codexMetadataClient, error) {
	cmd := codexAppServerCommand(ctx)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		in.Close()
		return nil, err
	}
	c := &codexMetadataClient{cmd: cmd, stdin: in, reader: bufio.NewReader(out)}
	if _, err = c.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "task_mecca", "version": "0.1"}}); err != nil {
		c.close()
		return nil, err
	}
	if err = json.NewEncoder(in).Encode(map[string]any{"method": "initialized"}); err != nil {
		c.close()
		return nil, err
	}
	return c, nil
}
func (c *codexMetadataClient) close() {
	c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}
func (c *codexMetadataClient) call(method string, params any) (json.RawMessage, error) {
	c.seq++
	if err := json.NewEncoder(c.stdin).Encode(map[string]any{"id": c.seq, "method": method, "params": params}); err != nil {
		return nil, err
	}
	msg, err := readCodexRPC(c.reader, float64(c.seq))
	return msg.Result, err
}
func (c *codexMetadataClient) read(id string) (CodexThreadMetadata, error) {
	raw, err := c.call("thread/read", map[string]any{"threadId": id, "includeTurns": false})
	if err != nil {
		return CodexThreadMetadata{}, err
	}
	var result struct {
		Thread struct {
			ID   string          `json:"id"`
			Name json.RawMessage `json:"name"`
		} `json:"thread"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return CodexThreadMetadata{}, err
	}
	if result.Thread.ID != id {
		return CodexThreadMetadata{}, errors.New("official readback session identity mismatch")
	}
	if len(result.Thread.Name) == 0 {
		return CodexThreadMetadata{}, errors.New("official metadata does not expose name")
	}
	var name string
	if string(result.Thread.Name) != "null" {
		if err = json.Unmarshal(result.Thread.Name, &name); err != nil {
			return CodexThreadMetadata{}, err
		}
	}
	return CodexThreadMetadata{ID: result.Thread.ID, Name: name}, nil
}

// Receipts are output only. Verify always does a fresh exact provider read.
type NameReceipt struct {
	Provider              string `json:"provider"`
	SessionID             string `json:"session_id"`
	RequestedName         string `json:"requested_name"`
	ObservedName          string `json:"observed_name,omitempty"`
	ObservedSessionID     string `json:"observed_session_id,omitempty"`
	Status                string `json:"status"`
	Support               string `json:"support"`
	Source                string `json:"source"`
	Applied               bool   `json:"applied"`
	Verified              bool   `json:"verified"`
	RequestAcknowledged   bool   `json:"request_acknowledged"`
	RequestedNotConfirmed bool   `json:"requested_not_confirmed"`
	CheckedAt             string `json:"checked_at"`
	Reason                string `json:"reason,omitempty"`
}

// Inspect the installed official executable's generated protocol schema.
var errNamingUnsupported = errors.New("installed official naming interface unsupported")

func codexNamingSupport(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "task-mecca-name-schema-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err = codexCommand(ctx, "app-server", "generate-json-schema", "--out", dir).Run(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "ClientRequest.json"))
	if err != nil {
		return err
	}
	var schema struct {
		OneOf []struct {
			Properties struct {
				Method struct {
					Enum []string `json:"enum"`
				} `json:"method"`
			} `json:"properties"`
		} `json:"oneOf"`
	}
	if err = json.Unmarshal(data, &schema); err != nil {
		return err
	}
	methods := map[string]bool{}
	for _, entry := range schema.OneOf {
		for _, m := range entry.Properties.Method.Enum {
			methods[m] = true
		}
	}
	if !methods["thread/name/set"] || !methods["thread/read"] {
		return errNamingUnsupported
	}
	return nil
}
func nameFailure(r NameReceipt, ctx context.Context, err error, stage string) NameReceipt {
	r.Reason = stage
	if ctx.Err() != nil || errors.Is(err, context.DeadlineExceeded) {
		r.Status = "timeout"
	} else {
		r.Status = "error"
	}
	if r.RequestedNotConfirmed {
		r.Reason = stage + "; " + r.Status + "; requested_not_confirmed"
		r.Status = "partial"
	}
	return r
}
func CodexSessionName(ctx context.Context, id, name string, apply bool) NameReceipt {
	r := NameReceipt{Provider: "codex", SessionID: id, RequestedName: name, Status: "error", Support: "unknown", Source: "codex_app_server:thread/read", CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		r.Reason = "exact session ID and generated name required"
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := codexNamingSupport(ctx); err != nil {
		r = nameFailure(r, ctx, err, "installed official schema unavailable or naming unsupported")
		if errors.Is(err, errNamingUnsupported) || errors.Is(err, exec.ErrNotFound) {
			r.Status = "unsupported"
			r.Support = "unsupported"
		}
		return r
	}
	r.Support = "supported"
	c, err := openCodexMetadata(ctx)
	if err != nil {
		return nameFailure(r, ctx, err, "initialize_failed")
	}
	defer c.close()
	before, err := c.read(id)
	if err != nil {
		return nameFailure(r, ctx, err, "initial_read_failed")
	}
	r.ObservedSessionID = before.ID
	r.ObservedName = before.Name
	if before.Name != name && orchestration.IsManagedSessionName(before.Name) {
		r.Status = "identity_conflict"
		r.Reason = "managed group/role/worker/generation is immutable on same native session"
		return r
	}
	if before.Name == name {
		r.Status = "verified"
		r.Verified = true
		return r
	}
	if !apply {
		r.Status = "mismatch"
		r.Reason = "official name differs from generated name"
		return r
	}
	// A timeout/error may follow a committed write; never silently retry it.
	r.RequestedNotConfirmed = true
	if _, err = c.call("thread/name/set", map[string]string{"threadId": id, "name": name}); err != nil {
		return nameFailure(r, ctx, err, "name_set_failed")
	}
	r.RequestAcknowledged = true
	after, err := c.read(id)
	if err != nil {
		return nameFailure(r, ctx, err, "readback_failed")
	}
	r.ObservedSessionID = after.ID
	r.ObservedName = after.Name
	if after.Name != name {
		r.Status = "mismatch"
		r.Reason = "acknowledged request but official name readback differs"
		return r
	}
	r.RequestedNotConfirmed = false
	r.Status = "verified"
	r.Verified = true
	r.Applied = true
	return r
}

// Documented SDK metadata exports only. Never invokes query() or a model turn.
const ClaudeMetadataScript = `
const [id,name,dir,apply,managedPattern] = process.argv.slice(1);
let sdk;
try { sdk = await import('@anthropic-ai/claude-agent-sdk'); }
catch { console.log(JSON.stringify({status:'unsupported',reason:'official SDK unavailable'})); process.exit(0); }
if(typeof sdk.renameSession !== 'function' || typeof sdk.getSessionInfo !== 'function') {
 console.log(JSON.stringify({status:'unsupported',reason:'official SDK metadata exports unavailable'})); process.exit(0);
}
if(apply==='support') { console.log(JSON.stringify({status:'supported'})); process.exit(0); }
let requested=false,ack=false;
try {
 const before=await sdk.getSessionInfo(id,{dir});
 if(!before || before.sessionId!==id) throw Error('identity');
 if(before.customTitle!==name && new RegExp(managedPattern).test(before.customTitle||'')) { console.log(JSON.stringify({status:'identity_conflict',session_id:before.sessionId,name:before.customTitle,reason:'managed session identity/generation differs'})); process.exit(0); }
 let after=before;
 if(before.customTitle!==name && apply==='true') {
  requested=true; await sdk.renameSession(id,name,{dir}); ack=true;
  after=await sdk.getSessionInfo(id,{dir});
 }
 if(!after || after.sessionId!==id) throw Error('identity');
 console.log(JSON.stringify({status:after.customTitle===name?'verified':'mismatch',session_id:after.sessionId,name:after.customTitle||'',ack,requested}));
} catch { console.log(JSON.stringify({status:requested?'partial':'error',ack,requested,reason:'official SDK metadata operation/readback failed'})); }
`

func ClaudeSessionName(ctx context.Context, id, name, dir string, apply bool) NameReceipt {
	r := NameReceipt{Provider: "claude", SessionID: id, RequestedName: name, Status: "error", Support: "unknown", Source: "claude_agent_sdk:getSessionInfo.customTitle", CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", ClaudeMetadataScript, id, name, dir, fmt.Sprint(apply), orchestration.ManagedSessionNamePattern)
	cmd.Dir = dir
	data, err := cmd.Output()
	if err != nil {
		// Once the SDK subprocess started an interrupted mutation may have
		// committed. No response is insufficient to assert it was not applied.
		r.RequestedNotConfirmed = apply && cmd.ProcessState != nil
		return nameFailure(r, ctx, err, "official SDK helper unavailable or failed")
	}
	var v struct {
		Status    string `json:"status"`
		ID        string `json:"session_id"`
		Name      string `json:"name"`
		Ack       bool   `json:"ack"`
		Requested bool   `json:"requested"`
		Reason    string `json:"reason"`
	}
	if err = json.Unmarshal(data, &v); err != nil {
		return nameFailure(r, ctx, err, "invalid SDK metadata response")
	}
	r.Status = v.Status
	r.Reason = v.Reason
	r.RequestAcknowledged = v.Ack
	r.RequestedNotConfirmed = v.Requested
	r.Support = "supported"
	if v.Status == "unsupported" {
		r.Support = "unsupported"
		return r
	}
	r.ObservedName = v.Name
	r.ObservedSessionID = v.ID
	if v.Status == "verified" && v.ID == id && v.Name == name {
		r.Verified = true
		r.Applied = v.Requested && v.Ack
		r.RequestedNotConfirmed = false
	} else if v.Status == "verified" {
		r.Status = "mismatch"
	}
	return r
}

// Support check is separate from applying or claiming a verified name.
func SessionNamingSupport(ctx context.Context, provider, dir string) string {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	switch provider {
	case "codex":
		if err := codexNamingSupport(ctx); err != nil {
			if ctx.Err() != nil {
				return "timeout"
			}
			if errors.Is(err, errNamingUnsupported) || errors.Is(err, exec.ErrNotFound) {
				return "unsupported"
			}
			return "error"
		}
		return "supported"
	case "claude":
		cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", ClaudeMetadataScript, "", "", dir, "support")
		cmd.Dir = dir
		data, err := cmd.Output()
		if err != nil {
			if ctx.Err() != nil {
				return "timeout"
			}
			return "unsupported"
		}
		var result struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(data, &result) != nil {
			return "unknown"
		}
		if result.Status == "supported" {
			return "supported"
		}
		return "unsupported"
	default:
		return "unsupported"
	}
}
