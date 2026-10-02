package webui

import (
    "crypto/subtle"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
    "strings"
    "sync"
    "time"

    ptylib "github.com/aymanbagabas/go-pty"
)

const (
    terminalHistoryLimit = 512 * 1024
    terminalDetachedTTL = 5 * time.Minute
    terminalAbsoluteTTL = 8 * time.Hour
    terminalClosedRetention = 10 * time.Minute
)

type terminalSecuritySettings struct {
    RemoteEnabled bool   `json:"remote_enabled"`
    UpdatedAt     string `json:"updated_at,omitempty"`
}

func terminalSecurityPath() string {
    return filepath.Join(webServiceDir(), "terminal-security.json")
}

func readTerminalSecuritySettings() terminalSecuritySettings {
    data, err := os.ReadFile(terminalSecurityPath())
    if err != nil {
        return terminalSecuritySettings{}
    }
    var settings terminalSecuritySettings
    if json.Unmarshal(data, &settings) != nil {
        return terminalSecuritySettings{}
    }
    return settings
}

func writeTerminalSecuritySettings(settings terminalSecuritySettings) error {
    if err := os.MkdirAll(webServiceDir(), 0755); err != nil {
        return err
    }
    settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
    data, err := json.MarshalIndent(settings, "", "  ")
    if err != nil {
        return err
    }
    path := terminalSecurityPath()
    tmp := path + ".tmp"
    if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
        return err
    }
    return os.Rename(tmp, path)
}

func terminalRequestIP(r *http.Request) net.IP {
    host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
    if err != nil {
        host = strings.TrimSpace(r.RemoteAddr)
    }
    return net.ParseIP(strings.Trim(host, "[]"))
}

func terminalRequestHost(r *http.Request) string {
    raw := strings.TrimSpace(r.Host)
    if host, _, err := net.SplitHostPort(raw); err == nil {
        raw = host
    }
    return strings.Trim(strings.ToLower(raw), "[]")
}

func hasTailscaleIdentityHeader(r *http.Request) bool {
    for _, name := range []string{"Tailscale-User-Login", "Tailscale-User-Name", "Tailscale-User-Profile-Pic"} {
        if strings.TrimSpace(r.Header.Get(name)) != "" {
            return true
        }
    }
    return false
}

func isLocalTerminalRequest(r *http.Request) bool {
    ip := terminalRequestIP(r)
    if ip == nil || !ip.IsLoopback() || hasTailscaleIdentityHeader(r) {
        return false
    }
    host := terminalRequestHost(r)
    if host == "localhost" {
        return true
    }
    parsed := net.ParseIP(host)
    return parsed != nil && parsed.IsLoopback()
}

func isTailscaleTerminalRequest(r *http.Request) bool {
    ip := terminalRequestIP(r)
    if ip == nil {
        return false
    }
    if r.TLS != nil && isTailscaleIPv4(ip) {
        return true
    }
    host := terminalRequestHost(r)
    return ip.IsLoopback() && strings.HasSuffix(host, ".ts.net") && hasTailscaleIdentityHeader(r)
}

func terminalOriginAllowed(r *http.Request) bool {
    if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
        return false
    }
    origin := strings.TrimSpace(r.Header.Get("Origin"))
    if origin == "" {
        return isLocalTerminalRequest(r)
    }
    parsed, err := url.Parse(origin)
    if err != nil || !strings.EqualFold(parsed.Host, r.Host) {
        return false
    }
    if isLocalTerminalRequest(r) {
        return parsed.Scheme == "http" || parsed.Scheme == "https"
    }
    return parsed.Scheme == "https"
}

func terminalConnectionKind(r *http.Request) string {
    if isLocalTerminalRequest(r) {
        return "local"
    }
    if isTailscaleTerminalRequest(r) {
        return "tailscale"
    }
    return "blocked"
}

func terminalAccessAllowed(r *http.Request, settings terminalSecuritySettings) bool {
    switch terminalConnectionKind(r) {
    case "local":
        return true
    case "tailscale":
        return settings.RemoteEnabled
    default:
        return false
    }
}

func clampTerminalSize(cols, rows int) (int, int) {
    if cols < 40 {
        cols = 40
    }
    if cols > 300 {
        cols = 300
    }
    if rows < 10 {
        rows = 10
    }
    if rows > 120 {
        rows = 120
    }
    return cols, rows
}

func defaultTerminalShell() (string, []string, error) {
    if runtime.GOOS == "windows" {
        for _, candidate := range []struct {
            name string
            args []string
        }{
            {"pwsh.exe", []string{"-NoLogo"}},
            {"powershell.exe", []string{"-NoLogo"}},
            {"cmd.exe", nil},
        } {
            if path, err := exec.LookPath(candidate.name); err == nil {
                return path, candidate.args, nil
            }
        }
        return "", nil, errors.New("no supported Windows shell found")
    }

    candidates := []string{strings.TrimSpace(os.Getenv("SHELL"))}
    if runtime.GOOS == "darwin" {
        candidates = append(candidates, "/bin/zsh")
    }
    candidates = append(candidates, "/bin/bash", "/bin/sh")
    seen := map[string]bool{}
    for _, candidate := range candidates {
        if candidate == "" || seen[candidate] {
            continue
        }
        seen[candidate] = true
        path, err := exec.LookPath(candidate)
        if err == nil {
            return path, nil, nil
        }
    }
    return "", nil, errors.New("no supported shell found")
}

func terminalEnvironment() []string {
    env := append([]string(nil), os.Environ()...)
    set := func(key, value string) {
        prefix := key + "="
        for i, item := range env {
            if strings.EqualFold(strings.SplitN(item, "=", 2)[0], key) {
                env[i] = prefix + value
                return
            }
        }
        env = append(env, prefix+value)
    }
    set("TERM", "xterm-256color")
    set("COLORTERM", "truecolor")
    set("TASK_MECCA_TERMINAL", "1")
    return env
}

type terminalSession struct {
    id        string
    token     string
    cwd       string
    shell     string
    shellArgs []string
    pty       ptylib.Pty
    cmd       *ptylib.Cmd
    createdAt time.Time

    mu           sync.Mutex
    history      []byte
    subscribers  map[chan []byte]struct{}
    lastDetached time.Time
    closed       bool
    closedAt     time.Time
    exitError    string
    done         chan struct{}
    doneOnce     sync.Once
}

func newTerminalSession(project string, cols, rows int) (*terminalSession, error) {
    project = filepath.Clean(project)
    info, err := os.Stat(project)
    if err != nil {
        return nil, fmt.Errorf("terminal project: %w", err)
    }
    if !info.IsDir() {
        return nil, fmt.Errorf("terminal project is not a directory: %s", project)
    }

    shell, args, err := defaultTerminalShell()
    if err != nil {
        return nil, err
    }
    p, err := ptylib.New()
    if err != nil {
        return nil, fmt.Errorf("create PTY: %w", err)
    }
    cols, rows = clampTerminalSize(cols, rows)
    if err := p.Resize(cols, rows); err != nil {
        _ = p.Close()
        return nil, fmt.Errorf("resize PTY: %w", err)
    }

    id, err := randomHex(12)
    if err != nil {
        _ = p.Close()
        return nil, err
    }
    token, err := randomHex(24)
    if err != nil {
        _ = p.Close()
        return nil, err
    }

    cmd := p.Command(shell, args...)
    cmd.Dir = project
    cmd.Env = terminalEnvironment()
    if err := cmd.Start(); err != nil {
        _ = p.Close()
        return nil, fmt.Errorf("start terminal shell: %w", err)
    }

    session := &terminalSession{
        id: id, token: token, cwd: project, shell: shell, shellArgs: args,
        pty: p, cmd: cmd, createdAt: time.Now(), subscribers: map[chan []byte]struct{}{},
        lastDetached: time.Now(), done: make(chan struct{}),
    }
    go session.readLoop()
    go session.waitLoop()
    return session, nil
}

func (s *terminalSession) readLoop() {
    buffer := make([]byte, 16*1024)
    for {
        n, err := s.pty.Read(buffer)
        if n > 0 {
            s.publish(buffer[:n])
        }
        if err != nil {
            return
        }
    }
}

func (s *terminalSession) waitLoop() {
    err := s.cmd.Wait()
    s.finish(err)
}

func (s *terminalSession) publish(data []byte) {
    chunk := append([]byte(nil), data...)
    s.mu.Lock()
    s.history = append(s.history, chunk...)
    if len(s.history) > terminalHistoryLimit {
        s.history = append([]byte(nil), s.history[len(s.history)-terminalHistoryLimit:]...)
    }
    for ch := range s.subscribers {
        select {
        case ch <- chunk:
        default:
            close(ch)
            delete(s.subscribers, ch)
        }
    }
    if len(s.subscribers) == 0 && s.lastDetached.IsZero() {
        s.lastDetached = time.Now()
    }
    s.mu.Unlock()
}

func (s *terminalSession) finish(err error) {
    s.mu.Lock()
    if !s.closed {
        s.closed = true
        s.closedAt = time.Now()
        if err != nil {
            s.exitError = err.Error()
        }
    }
    s.mu.Unlock()
    s.doneOnce.Do(func() { close(s.done) })
    _ = s.pty.Close()
}

func (s *terminalSession) close() {
    s.mu.Lock()
    alreadyClosed := s.closed
    s.mu.Unlock()
    if alreadyClosed {
        return
    }
    _ = s.pty.Close()
    if s.cmd != nil && s.cmd.Process != nil {
        _ = s.cmd.Process.Kill()
    }
    s.finish(errors.New("terminal session closed"))
}

func (s *terminalSession) write(data []byte) error {
    s.mu.Lock()
    closed := s.closed
    s.mu.Unlock()
    if closed {
        return errors.New("terminal session has ended")
    }
    _, err := s.pty.Write(data)
    return err
}

func (s *terminalSession) resize(cols, rows int) error {
    cols, rows = clampTerminalSize(cols, rows)
    s.mu.Lock()
    closed := s.closed
    s.mu.Unlock()
    if closed {
        return errors.New("terminal session has ended")
    }
    return s.pty.Resize(cols, rows)
}

func (s *terminalSession) subscribe() ([]byte, <-chan []byte, func()) {
    ch := make(chan []byte, 64)
    s.mu.Lock()
    snapshot := append([]byte(nil), s.history...)
    if s.closed {
        close(ch)
        s.mu.Unlock()
        return snapshot, ch, func() {}
    }
    s.subscribers[ch] = struct{}{}
    s.lastDetached = time.Time{}
    s.mu.Unlock()

    cancel := func() {
        s.mu.Lock()
        if _, ok := s.subscribers[ch]; ok {
            delete(s.subscribers, ch)
            close(ch)
            if len(s.subscribers) == 0 {
                s.lastDetached = time.Now()
            }
        }
        s.mu.Unlock()
    }
    return snapshot, ch, cancel
}

func (s *terminalSession) payload(includeToken bool) map[string]any {
    s.mu.Lock()
    defer s.mu.Unlock()
    result := map[string]any{
        "id": s.id,
        "cwd": s.cwd,
        "shell": s.shell,
        "created_at": s.createdAt.UTC().Format(time.RFC3339),
        "closed": s.closed,
        "exit_error": s.exitError,
    }
    if !s.closedAt.IsZero() {
        result["closed_at"] = s.closedAt.UTC().Format(time.RFC3339)
    }
    if includeToken {
        result["token"] = s.token
    }
    return result
}

func (s *terminalSession) matchesToken(token string) bool {
    if len(token) != len(s.token) {
        return false
    }
    return subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) == 1
}

type terminalHub struct {
    mu       sync.Mutex
    sessions map[string]*terminalSession
}

func newTerminalHub() *terminalHub {
    hub := &terminalHub{sessions: map[string]*terminalSession{}}
    go hub.sweep()
    return hub
}

func (h *terminalHub) sweep() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()
    for now := range ticker.C {
        var closeList []*terminalSession
        h.mu.Lock()
        for id, session := range h.sessions {
            session.mu.Lock()
            closed := session.closed
            closedAt := session.closedAt
            detached := session.lastDetached
            created := session.createdAt
            subscriberCount := len(session.subscribers)
            session.mu.Unlock()

            if closed {
                if !closedAt.IsZero() && now.Sub(closedAt) > terminalClosedRetention {
                    delete(h.sessions, id)
                }
                continue
            }
            if now.Sub(created) > terminalAbsoluteTTL || (subscriberCount == 0 && !detached.IsZero() && now.Sub(detached) > terminalDetachedTTL) {
                closeList = append(closeList, session)
            }
        }
        h.mu.Unlock()
        for _, session := range closeList {
            session.close()
        }
    }
}

func (h *terminalHub) create(project string, cols, rows int) (*terminalSession, error) {
    session, err := newTerminalSession(project, cols, rows)
    if err != nil {
        return nil, err
    }
    var replaced []*terminalSession
    h.mu.Lock()
    for id, existing := range h.sessions {
        existing.mu.Lock()
        sameProject := filepath.Clean(existing.cwd) == filepath.Clean(project) && !existing.closed
        existing.mu.Unlock()
        if sameProject {
            replaced = append(replaced, existing)
            delete(h.sessions, id)
        }
    }
    h.sessions[session.id] = session
    h.mu.Unlock()
    for _, existing := range replaced {
        existing.close()
    }
    return session, nil
}

func (h *terminalHub) closeAll() {
    h.mu.Lock()
    sessions := make([]*terminalSession, 0, len(h.sessions))
    for _, session := range h.sessions {
        sessions = append(sessions, session)
    }
    h.mu.Unlock()
    for _, session := range sessions {
        session.close()
    }
}

func (h *terminalHub) get(id, token string) (*terminalSession, bool) {
    h.mu.Lock()
    session := h.sessions[id]
    h.mu.Unlock()
    if session == nil || !session.matchesToken(token) {
        return nil, false
    }
    return session, true
}

func terminalToken(r *http.Request) string {
    return strings.TrimSpace(r.Header.Get("X-Task-Mecca-Terminal"))
}

func registerTerminalRoutes(
    mux *http.ServeMux,
    projectFor func(*http.Request) string,
    writeJSON func(http.ResponseWriter, any, int),
) {
    hub := newTerminalHub()

    settingsPayload := func(r *http.Request) map[string]any {
        settings := readTerminalSecuritySettings()
        kind := terminalConnectionKind(r)
        port := terminalPortFromHost(r.Host)
        return map[string]any{
            "remote_enabled": settings.RemoteEnabled,
            "connection": kind,
            "local": kind == "local",
            "tailscale": kind == "tailscale",
            "access_allowed": terminalAccessAllowed(r, settings),
            "can_enable_remote": kind == "local",
            "can_disable_remote": kind == "local" || (kind == "tailscale" && settings.RemoteEnabled),
            "updated_at": settings.UpdatedAt,
            "project": projectFor(r),
            "local_terminal_url": fmt.Sprintf("http://127.0.0.1:%d/terminal", port),
        }
    }

    mux.HandleFunc("/api/terminal/settings", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            writeJSON(w, settingsPayload(r), http.StatusOK)
            return
        }
        if r.Method != http.MethodPost {
            writeJSON(w, map[string]any{"error": "GET or POST required"}, http.StatusMethodNotAllowed)
            return
        }
        if r.Header.Get("X-Task-Mecca-Action") != "1" || !terminalOriginAllowed(r) {
            writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
            return
        }
        var body struct {
            RemoteEnabled bool `json:"remote_enabled"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, 4096)
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            writeJSON(w, map[string]any{"error": "invalid JSON"}, http.StatusBadRequest)
            return
        }

        current := readTerminalSecuritySettings()
        kind := terminalConnectionKind(r)
        if body.RemoteEnabled && kind != "local" {
            writeJSON(w, map[string]any{"error": "remote terminal can only be enabled from localhost"}, http.StatusForbidden)
            return
        }
        if !body.RemoteEnabled && kind != "local" && !(kind == "tailscale" && current.RemoteEnabled) {
            writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
            return
        }
        current.RemoteEnabled = body.RemoteEnabled
        if err := writeTerminalSecuritySettings(current); err != nil {
            writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
            return
        }
        if !body.RemoteEnabled {
            hub.closeAll()
        }
        writeJSON(w, settingsPayload(r), http.StatusOK)
    })

    mux.HandleFunc("/api/terminal/sessions", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            writeJSON(w, map[string]any{"error": "POST required"}, http.StatusMethodNotAllowed)
            return
        }
        settings := readTerminalSecuritySettings()
        if !terminalAccessAllowed(r, settings) || r.Header.Get("X-Task-Mecca-Action") != "1" || !terminalOriginAllowed(r) {
            writeJSON(w, map[string]any{"error": "terminal access is not allowed on this connection"}, http.StatusForbidden)
            return
        }
        var body struct {
            Cols int `json:"cols"`
            Rows int `json:"rows"`
        }
        r.Body = http.MaxBytesReader(w, r.Body, 4096)
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
            writeJSON(w, map[string]any{"error": "invalid JSON"}, http.StatusBadRequest)
            return
        }
        project := projectFor(r)
        if abs, err := filepath.Abs(project); err == nil {
            project = abs
        }
        session, err := hub.create(project, body.Cols, body.Rows)
        if err != nil {
            writeJSON(w, map[string]any{"error": err.Error()}, http.StatusInternalServerError)
            return
        }
        writeJSON(w, session.payload(true), http.StatusCreated)
    })

    mux.HandleFunc("/api/terminal/sessions/", func(w http.ResponseWriter, r *http.Request) {
        settings := readTerminalSecuritySettings()
        if !terminalAccessAllowed(r, settings) {
            writeJSON(w, map[string]any{"error": "terminal access is not allowed on this connection"}, http.StatusForbidden)
            return
        }
        rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/terminal/sessions/"), "/")
        parts := strings.Split(rest, "/")
        if len(parts) < 1 || parts[0] == "" || len(parts) > 2 {
            http.NotFound(w, r)
            return
        }
        session, ok := hub.get(parts[0], terminalToken(r))
        if !ok {
            writeJSON(w, map[string]any{"error": "terminal session not found"}, http.StatusNotFound)
            return
        }

        action := ""
        if len(parts) == 2 {
            action = parts[1]
        }

        if action == "" && r.Method == http.MethodGet {
            writeJSON(w, session.payload(false), http.StatusOK)
            return
        }
        if action == "" && r.Method == http.MethodDelete {
            if r.Header.Get("X-Task-Mecca-Action") != "1" || !terminalOriginAllowed(r) {
                writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
                return
            }
            session.close()
            writeJSON(w, map[string]any{"ok": true}, http.StatusOK)
            return
        }

        switch action {
        case "stream":
            if r.Method != http.MethodGet {
                writeJSON(w, map[string]any{"error": "GET required"}, http.StatusMethodNotAllowed)
                return
            }
            if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !terminalOriginAllowed(r) {
                writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
                return
            }
            flusher, ok := w.(http.Flusher)
            if !ok {
                writeJSON(w, map[string]any{"error": "streaming unsupported"}, http.StatusInternalServerError)
                return
            }
            w.Header().Set("Content-Type", "application/octet-stream")
            w.Header().Set("Cache-Control", "no-store")
            w.Header().Set("X-Content-Type-Options", "nosniff")
            w.Header().Set("X-Accel-Buffering", "no")
            snapshot, chunks, cancel := session.subscribe()
            defer cancel()
            if len(snapshot) > 0 {
                if _, err := w.Write(snapshot); err != nil {
                    return
                }
                flusher.Flush()
            }
            for {
                select {
                case <-r.Context().Done():
                    return
                case <-session.done:
                    return
                case chunk, open := <-chunks:
                    if !open {
                        return
                    }
                    if _, err := w.Write(chunk); err != nil {
                        return
                    }
                    flusher.Flush()
                }
            }
        case "input":
            if r.Method != http.MethodPost {
                writeJSON(w, map[string]any{"error": "POST required"}, http.StatusMethodNotAllowed)
                return
            }
            if r.Header.Get("X-Task-Mecca-Action") != "1" || !terminalOriginAllowed(r) {
                writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
                return
            }
            r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
            data, err := io.ReadAll(r.Body)
            if err != nil {
                writeJSON(w, map[string]any{"error": err.Error()}, http.StatusBadRequest)
                return
            }
            if len(data) > 0 {
                if err := session.write(data); err != nil {
                    writeJSON(w, map[string]any{"error": err.Error()}, http.StatusConflict)
                    return
                }
            }
            w.WriteHeader(http.StatusNoContent)
        case "resize":
            if r.Method != http.MethodPost {
                writeJSON(w, map[string]any{"error": "POST required"}, http.StatusMethodNotAllowed)
                return
            }
            if r.Header.Get("X-Task-Mecca-Action") != "1" || !terminalOriginAllowed(r) {
                writeJSON(w, map[string]any{"error": "forbidden"}, http.StatusForbidden)
                return
            }
            var body struct {
                Cols int `json:"cols"`
                Rows int `json:"rows"`
            }
            r.Body = http.MaxBytesReader(w, r.Body, 4096)
            if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
                writeJSON(w, map[string]any{"error": "invalid JSON"}, http.StatusBadRequest)
                return
            }
            if err := session.resize(body.Cols, body.Rows); err != nil {
                writeJSON(w, map[string]any{"error": err.Error()}, http.StatusConflict)
                return
            }
            cols, rows := clampTerminalSize(body.Cols, body.Rows)
            writeJSON(w, map[string]any{"ok": true, "cols": cols, "rows": rows}, http.StatusOK)
        default:
            http.NotFound(w, r)
        }
    })
}

func terminalPortFromHost(host string) int {
    _, port, err := net.SplitHostPort(strings.TrimSpace(host))
    if err != nil {
        return DefaultPort
    }
    value, err := strconv.Atoi(port)
    if err != nil || value <= 0 {
        return DefaultPort
    }
    return value
}
