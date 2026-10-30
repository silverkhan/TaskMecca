package contextlife

import (
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "math"
 "strings"
 "time"
)

// ObserveClaudeStatusline consumes the documented Claude Code statusline JSON
// from stdin. A structured provider payload is still not cryptographically
// authenticated: it can suggest a safe checkpoint, never force a rotation.
func ObserveClaudeStatusline(project,role,attemptID string,input io.Reader,now time.Time)(State,error){
 a,err:=attemptFor(project,role,attemptID,now);if err!=nil{return State{},err}
 if a.Provider!="claude"{return State{},errors.New("Claude statusline requires a Claude runtime attempt")}
 var payload struct{
  SessionID string `json:"session_id"`
  ContextWindow struct{
   UsedPercentage *float64 `json:"used_percentage"`
   ContextWindowSize int64 `json:"context_window_size"`
   TotalInputTokens int64 `json:"total_input_tokens"`
  } `json:"context_window"`
 }
 dec:=json.NewDecoder(io.LimitReader(input,128*1024))
 if err=dec.Decode(&payload);err!=nil{return State{},fmt.Errorf("invalid provider statusline JSON: %w",err)}
 if strings.TrimSpace(payload.SessionID)==""||payload.SessionID!=a.SessionID {return State{},errors.New("statusline session_id does not match exact bound runtime session")}
 window:=payload.ContextWindow.ContextWindowSize
 percent:=payload.ContextWindow.UsedPercentage
 if window<=0||percent==nil||math.IsNaN(*percent)||math.IsInf(*percent,0)||*percent<0||*percent>100 {
  return State{},errors.New("provider context metric unavailable or invalid; never interpret null as zero")
 }
 used:=payload.ContextWindow.TotalInputTokens
 if used<=0 {used=int64(math.Round(*percent*float64(window)/100))}
 if used<0||used>window {return State{},errors.New("provider token reading exceeds context window")}
 return withState(project,role,func(s *State)error{
  if s.OwnerAttempt!=""&&s.OwnerAttempt!=attemptID{return errors.New("statusline comes from a retired attempt")}
  val:=*percent
  s.Observation=&Observation{AttemptID:attemptID,Provider:"claude",UsedTokens:used,WindowTokens:window,UsagePercent:&val,Evidence:"claude_statusline_exact_session",Trust:"provider_structured",ObservedAt:now.UTC().Format(time.RFC3339Nano)}
  return nil
 })
}
