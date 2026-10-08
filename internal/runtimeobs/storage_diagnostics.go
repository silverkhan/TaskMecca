package runtimeobs

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RawFileCondition describes a dated execution file as it exists at report
// time. Conditions can overlap. No data in this report authorizes deletion.
type RawFileCondition struct {
	File string `json:"file"`
	Bytes int64 `json:"bytes"`
	EventCount int `json:"event_count"`
	LastObservedAt string `json:"last_observed_at,omitempty"`
	RetentionEligibleAfter string `json:"retention_eligible_after,omitempty"`
	ReasonCodes []string `json:"reason_codes"`
	UnfinishedAttempts int `json:"unfinished_attempts"`
	MissingEvidenceAttempts int `json:"missing_evidence_attempts"`
	BlockingAttemptIDs []string `json:"blocking_attempt_ids,omitempty"`
	SafeNow bool `json:"safe_now"`
	ConditionalBytes int64 `json:"conditional_bytes"`
}

// RawProtectionAnalysis is a read-only diagnosis, NOT a cleanup plan.
// ConditionalBytes is a hypothetical upper bound for CURRENT file sizes,
// once every condition has been satisfied and independently rechecked.
// It excludes SafeNowBytes and UnverifiableBytes, so no double counting.
type RawProtectionAnalysis struct {
	SafeNowBytes int64 `json:"safe_now_bytes"`
	ConditionalBytes int64 `json:"conditional_bytes"`
	UnverifiableBytes int64 `json:"unverifiable_bytes"`
	ConditionalFiles int `json:"conditional_files"`
	Files []RawFileCondition `json:"files"`
	GeneratedAt string `json:"generated_at"`
}

func readRawFileConditions(path string) (map[string]bool,time.Time,int,error) {
	f,err:=os.Open(path)
	if err!=nil { return nil,time.Time{},0,err }
	defer f.Close()
	scanner:=bufio.NewScanner(f)
	scanner.Buffer(make([]byte,64*1024),2*1024*1024)
	ids:=make(map[string]bool)
	latest:=time.Time{}
	count:=0
	for scanner.Scan() {
		var event ExecutionEvent
		if err:=json.Unmarshal(scanner.Bytes(),&event);err!=nil { return nil,time.Time{},0,err }
		at:=eventObservedTime(event.ObservedAt,time.Time{})
		if event.AttemptID=="" || at.IsZero() { return nil,time.Time{},0,errors.New("event has no verifiable identity or time") }
		ids[event.AttemptID]=true
		if at.After(latest) {latest=at}
		count++
	}
	if err:=scanner.Err();err!=nil {return nil,time.Time{},0,err}
	if count==0 {return nil,time.Time{},0,errors.New("empty raw event file")}
	return ids,latest,count,nil
}

// AnalyzeRawProtection uses the SAME retention cutoff and terminal-evidence
// conditions as buildCleanupPlan, but refuses to estimate future reclaim
// values for unreadable files, symlinks, or malformed event streams.
func AnalyzeRawProtection(project string, ledger Ledger, now time.Time) (RawProtectionAnalysis,error) {
	report:=RawProtectionAnalysis{Files:[]RawFileCondition{},GeneratedAt:now.UTC().Format(time.RFC3339Nano)}
	entries,err:=os.ReadDir(ExecutionRawDir(project))
	if errors.Is(err,os.ErrNotExist) {return report,nil}
	if err!=nil {return report,err}

	detailed,err:=readDetailedHistory(project)
	if err!=nil {return report,err}
	terminalKnown:=map[string]bool{}
	active:=map[string]bool{}
	for id:=range detailed {terminalKnown[id]=true}
	for _,attempt:=range ledger.Attempts {
		if attempt.Terminal {terminalKnown[attempt.AttemptID]=true} else {active[attempt.AttemptID]=true}
	}
	cutoff:=now.UTC().AddDate(0,0,-rawRetentionDays)
	for _,entry:=range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(),".jsonl") {continue}
		info,statErr:=entry.Info()
		if statErr!=nil {return report,statErr}
		row:=RawFileCondition{File:entry.Name(),Bytes:info.Size(),ReasonCodes:[]string{}}
		if entry.Type()&os.ModeSymlink!=0 || !info.Mode().IsRegular() {
			row.ReasonCodes=append(row.ReasonCodes,"unverifiable")
			report.UnverifiableBytes+=row.Bytes
			report.Files=append(report.Files,row)
			continue
		}
		ids,latest,count,scanErr:=readRawFileConditions(filepath.Join(ExecutionRawDir(project),entry.Name()))
		if scanErr!=nil {
			row.ReasonCodes=append(row.ReasonCodes,"unverifiable")
			report.UnverifiableBytes+=row.Bytes
			report.Files=append(report.Files,row)
			continue
		}
		row.EventCount=count
		row.LastObservedAt=latest.UTC().Format(time.RFC3339Nano)
		row.RetentionEligibleAfter=latest.AddDate(0,0,rawRetentionDays).UTC().Format(time.RFC3339Nano)
		if !latest.Before(cutoff) {row.ReasonCodes=append(row.ReasonCodes,"retention")}
		var blocking []string
		for id:=range ids {
			if active[id] {
				row.UnfinishedAttempts++
				blocking=append(blocking,id)
			} else if !terminalKnown[id] {
				row.MissingEvidenceAttempts++
				blocking=append(blocking,id)
			}
		}
		sort.Strings(blocking)
		if len(blocking)>3 {blocking=blocking[:3]}
		row.BlockingAttemptIDs=blocking
		if row.UnfinishedAttempts>0 {row.ReasonCodes=append(row.ReasonCodes,"unfinished")}
		if row.MissingEvidenceAttempts>0 {row.ReasonCodes=append(row.ReasonCodes,"missing_terminal")}
		if len(row.ReasonCodes)==0 {
			row.SafeNow=true
			report.SafeNowBytes+=row.Bytes
		} else {
			// A hypothetical upper bound, not a promise that the pending
			// attempts will ever be confirmed finished.
			row.ConditionalBytes=row.Bytes
			report.ConditionalBytes+=row.Bytes
			report.ConditionalFiles++
		}
		report.Files=append(report.Files,row)
	}
	sort.Slice(report.Files,func(i,j int)bool {return report.Files[i].File>report.Files[j].File})
	return report,nil
}
