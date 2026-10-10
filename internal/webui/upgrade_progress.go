package webui

import (
    "sync"
    "time"
)

// upgradeProgressTracker is scoped to a single Web service instance. It is
// intentionally not a persisted authority across process restarts.
type upgradeProgressTracker struct {
    mu sync.Mutex
    state upgradeProgressState
}

type upgradeProgressState struct {
    ID string `json:"id,omitempty"`
    Phase string `json:"phase"`
    Active bool `json:"active"`
    StartedAt string `json:"started_at,omitempty"`
    Error string `json:"error,omitempty"`
}

func (u *upgradeProgressTracker) snapshot() upgradeProgressState {
    u.mu.Lock()
    defer u.mu.Unlock()
    if u.state.Phase=="" { return upgradeProgressState{Phase:"idle"} }
    return u.state
}
func (u *upgradeProgressTracker) begin() bool {
    u.mu.Lock()
    defer u.mu.Unlock()
    if u.state.Active { return false }
    at:=time.Now().UTC().Format(time.RFC3339Nano)
    u.state=upgradeProgressState{ID:at,Phase:"checking",Active:true,StartedAt:at}
    return true
}
func (u *upgradeProgressTracker) step(phase string) {
    switch phase { case "checking","downloading","verifying","installing":
    default: return }
    u.mu.Lock()
    defer u.mu.Unlock()
    if u.state.Active { u.state.Phase=phase }
}
func (u *upgradeProgressTracker) finish(phase string,err error) {
    u.mu.Lock()
    defer u.mu.Unlock()
    if phase=="restarting" {
        // Lock out duplicate requests until the current Web process exits.
        u.state.Active=true
        u.state.Phase="restarting"
        return
    }
    u.state.Active=false
    if err!=nil {
        u.state.Phase="failed"
        u.state.Error=err.Error()
    }else{
        u.state.Phase="completed"
        u.state.Error=""
    }
}
