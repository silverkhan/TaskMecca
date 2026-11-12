package orchestration

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSessionNamingDirectiveResumeInvariance(t *testing.T) {
	g := Group{GroupID: "D1112_01", DisplayPrefix: "D1112_01_"}
	base := NativeNameTarget{Role: "worker", Worker: "pikachyu", Generation: 1, Provider: "codex", SessionID: "native-1", RuntimeAgentID: "agent-1", AttemptID: "attempt-1", WorkerPath: "/root/controller/pikachyu", T3ThreadID: "t3-1", Execution: "newSession"}
	first, err := SessionNamingDirective(g, base)
	if err != nil {
		t.Fatal(err)
	}
	base.Execution = "existingSession"
	again, err := SessionNamingDirective(g, base)
	if err != nil {
		t.Fatal(err)
	}
	if first.DisplayName != again.DisplayName || again.Target.Generation != 1 || again.Target.SessionID != "native-1" || again.Target.AttemptID != "attempt-1" || again.Target.WorkerPath != "/root/controller/pikachyu" {
		t.Fatalf("identity mutated: %+v", again)
	}
	if again.Verified || again.IdentityAuthoritative || again.Status != "pending_native_verification" || again.DelegateTitle != "D1112_01_Worker_pikachyu_G1" {
		t.Fatalf("unverified directive promoted: %+v", again)
	}
	b, _ := json.Marshal(again)
	for _, need := range []string{"before_dispatch", "after_dispatch", "on_resume", "t3_thread_read", "session-name verify", "Caller JSON"} {
		if !strings.Contains(string(b), need) {
			t.Fatalf("missing %s", need)
		}
	}
	base.SessionID = ""
	if _, err = SessionNamingDirective(g, base); err == nil {
		t.Fatal("resume without exact identity accepted")
	}
}
func TestSessionNamingRecognizesGeneratedNames(t *testing.T) {
	g := Group{GroupID: "D1112_01", DisplayPrefix: "D1112_01_"}
	for _, role := range []string{"root", "registrar", "controller", "worker"} {
		n, err := SessionName(g, role, "pikachyu", 1)
		if err != nil || !IsManagedSessionName(n) {
			t.Fatalf("%q %v", n, err)
		}
	}
	for _, s := range []string{"A-69", "/root/controller/pikachyu", "D1112_01_Worker_pikachyu_G0", "D1112_01_Worker_pikachyu_G1\nforeign"} {
		if IsManagedSessionName(s) {
			t.Fatalf("non-generated accepted: %q", s)
		}
	}
}
