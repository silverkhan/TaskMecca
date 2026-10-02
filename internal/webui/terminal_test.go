package webui

import (
    "crypto/tls"
    "net/http/httptest"
    "path/filepath"
    "testing"
)

func TestTerminalConnectionClassification(t *testing.T) {
    local := httptest.NewRequest("GET", "http://127.0.0.1:18765/api/terminal/settings", nil)
    local.Host = "127.0.0.1:18765"
    local.RemoteAddr = "127.0.0.1:49152"
    if got := terminalConnectionKind(local); got != "local" {
        t.Fatalf("local connection=%q", got)
    }

    direct := httptest.NewRequest("GET", "https://node.tailnet.ts.net:18765/api/terminal/settings", nil)
    direct.Host = "node.tailnet.ts.net:18765"
    direct.RemoteAddr = "100.100.218.126:49152"
    direct.TLS = &tls.ConnectionState{}
    if got := terminalConnectionKind(direct); got != "tailscale" {
        t.Fatalf("direct tailscale connection=%q", got)
    }

    served := httptest.NewRequest("GET", "http://node.tailnet.ts.net:18765/api/terminal/settings", nil)
    served.Host = "node.tailnet.ts.net:18765"
    served.RemoteAddr = "127.0.0.1:49152"
    served.Header.Set("Tailscale-User-Login", "user@example.com")
    if got := terminalConnectionKind(served); got != "tailscale" {
        t.Fatalf("tailscale serve connection=%q", got)
    }

    lan := httptest.NewRequest("GET", "http://192.168.0.10:18765/api/terminal/settings", nil)
    lan.Host = "192.168.0.10:18765"
    lan.RemoteAddr = "192.168.0.25:49152"
    if got := terminalConnectionKind(lan); got != "blocked" {
        t.Fatalf("lan connection=%q", got)
    }
}

func TestTerminalRemoteAccessDefaultsToTailscale(t *testing.T) {
    home := t.TempDir()
    t.Setenv("TASK_MECCA_HOME", home)

    settings := readTerminalSecuritySettings()
    if !settings.RemoteEnabled {
        t.Fatal("remote terminal should default to enabled when no explicit setting exists")
    }

    req := httptest.NewRequest("GET", "https://node.tailnet.ts.net:18765/api/terminal/settings", nil)
    req.Host = "node.tailnet.ts.net:18765"
    req.RemoteAddr = "100.100.218.126:49152"
    req.TLS = &tls.ConnectionState{}
    if !terminalAccessAllowed(req, settings) {
        t.Fatal("default Tailscale HTTPS connection should be allowed")
    }

    if terminalAccessAllowed(req, terminalSecuritySettings{RemoteEnabled: false}) {
        t.Fatal("explicitly disabled remote terminal should remain blocked")
    }
}

func TestTerminalOriginRules(t *testing.T) {
    local := httptest.NewRequest("POST", "http://127.0.0.1:18765/api/terminal/settings", nil)
    local.Host = "127.0.0.1:18765"
    local.RemoteAddr = "127.0.0.1:49152"
    if !terminalOriginAllowed(local) {
        t.Fatal("localhost request without Origin should be allowed")
    }

    remote := httptest.NewRequest("POST", "https://node.tailnet.ts.net:18765/api/terminal/sessions", nil)
    remote.Host = "node.tailnet.ts.net:18765"
    remote.RemoteAddr = "100.100.218.126:49152"
    remote.TLS = &tls.ConnectionState{}
    remote.Header.Set("Origin", "https://node.tailnet.ts.net:18765")
    if !terminalOriginAllowed(remote) {
        t.Fatal("same-origin Tailscale request should be allowed")
    }

    remote.Header.Set("Origin", "https://evil.example")
    if terminalOriginAllowed(remote) {
        t.Fatal("cross-origin request must be rejected")
    }
}

func TestTerminalSecuritySettingsRoundTrip(t *testing.T) {
    home := t.TempDir()
    t.Setenv("TASK_MECCA_HOME", home)
    want := terminalSecuritySettings{RemoteEnabled: true}
    if err := writeTerminalSecuritySettings(want); err != nil {
        t.Fatal(err)
    }
    got := readTerminalSecuritySettings()
    if !got.RemoteEnabled || got.UpdatedAt == "" {
        t.Fatalf("settings=%+v", got)
    }
    if filepath.Dir(terminalSecurityPath()) != filepath.Join(home, "web") {
        t.Fatalf("security path=%q", terminalSecurityPath())
    }
}

func TestClampTerminalSize(t *testing.T) {
    cols, rows := clampTerminalSize(1, 999)
    if cols != 40 || rows != 120 {
        t.Fatalf("clamped=%dx%d", cols, rows)
    }
    cols, rows = clampTerminalSize(120, 40)
    if cols != 120 || rows != 40 {
        t.Fatalf("ordinary=%dx%d", cols, rows)
    }
}


func TestUnixTerminalShellArgs(t *testing.T) {
    for _, shell := range []string{"/bin/zsh", "/bin/bash", "/bin/sh", "/usr/local/bin/fish"} {
        args := unixTerminalShellArgs(shell)
        if len(args) != 1 || args[0] != "-l" {
            t.Fatalf("login shell args for %q = %#v", shell, args)
        }
    }
    if args := unixTerminalShellArgs("/custom/nonstandard-shell"); args != nil {
        t.Fatalf("unknown shell args = %#v, want nil", args)
    }
}
