package runtimeobs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRawProtectionConditionsAndNonOverlappingConditionalBytes(t *testing.T) {
	project:=t.TempDir()
	now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
	old:=now.AddDate(0,0,-12)
	recent:=now.AddDate(0,0,-2)
	appendEventsForHistoryTest(t,project,terminalFixture("old-finished","codex",old,old.Add(time.Minute)))
	appendEventsForHistoryTest(t,project,terminalFixture("new-finished","codex",recent,recent.Add(time.Minute)))
	appendEventsForHistoryTest(t,project,[]ExecutionEvent{{
		EventKind:"state",AttemptID:"old-live",Provider:"codex",
		ObservedAt:old.Add(48*time.Hour).Format(time.RFC3339Nano),
		State:StateRunning,EvidenceSource:EvidenceHook,ObservationQuality:QualityObserved,
	}})
	ledger,err:=BuildLedger(project,10,now)
	if err!=nil {t.Fatal(err)}
	report,err:=AnalyzeRawProtection(project,ledger,now)
	if err!=nil {t.Fatal(err)}
	if len(report.Files)!=3 {t.Fatalf("expected three dated raw files: %+v",report)}
	var safe,retention,unfinished bool
	var expectedConditional,expectedSafe int64
	for _,file:=range report.Files {
		switch {
		case file.SafeNow:
			safe=true
			expectedSafe+=file.Bytes
		case file.UnfinishedAttempts>0:
			unfinished=true
			expectedConditional+=file.Bytes
			if !hasCode(file.ReasonCodes,"unfinished"){t.Errorf("missing unfinished reason: %+v",file)}
		case hasCode(file.ReasonCodes,"retention"):
			retention=true
			expectedConditional+=file.Bytes
			if file.RetentionEligibleAfter=="" {t.Errorf("missing future retention date")}
		default:
			t.Errorf("unexpected raw condition %+v",file)
		}
	}
	if !safe||!retention||!unfinished {t.Fatalf("missing one status: %+v",report)}
	if report.SafeNowBytes!=expectedSafe||report.ConditionalBytes!=expectedConditional {
		t.Fatalf("double counting: %+v want now %d conditional %d",report,expectedSafe,expectedConditional)
	}
	if report.ConditionalFiles!=2 {t.Fatalf("conditional files %+v",report)}
}
func hasCode(codes []string, code string)bool{
	for _,v:=range codes {if v==code{return true}}
	return false
}
func TestRawProtectionUnknownEvidenceAndCorruptFilesNeverCountAsSafe(t *testing.T) {
	project:=t.TempDir()
	now:=time.Date(2026,10,20,12,0,0,0,time.UTC)
	old:=now.AddDate(0,0,-11)
	appendEventsForHistoryTest(t,project,terminalFixture("not-in-ledger","claude",old,old.Add(time.Minute)))
	// Explicitly use a partial ledger; missing terminal evidence must never
	// be interpreted as permission to reclaim the file.
	report,err:=AnalyzeRawProtection(project,Ledger{},now)
	if err!=nil {t.Fatal(err)}
	if len(report.Files)!=1||report.Files[0].MissingEvidenceAttempts!=1 ||
		!hasCode(report.Files[0].ReasonCodes,"missing_terminal") ||
		report.SafeNowBytes!=0 || report.ConditionalBytes<=0 {
		t.Fatalf("missing terminal proof incorrectly classified: %+v",report)
	}
	invalid:=filepath.Join(ExecutionRawDir(project),"broken.jsonl")
	if err:=os.WriteFile(invalid,[]byte("{not-json}\n"),0600);err!=nil{t.Fatal(err)}
	report,err=AnalyzeRawProtection(project,Ledger{},now)
	if err!=nil {t.Fatal(err)}
	if report.UnverifiableBytes!=int64(len("{not-json}\n")) {
		t.Fatalf("unverifiable bytes must be excluded: %+v",report)
	}
	for _,item:=range report.Files {
		if item.File=="broken.jsonl"&&(item.ConditionalBytes!=0||item.SafeNow||!hasCode(item.ReasonCodes,"unverifiable")){
			t.Fatalf("corrupt file improperly advertised as reclaimable: %+v",item)
		}
	}
}
func TestRawProtectionNeverFollowsSymlink(t *testing.T) {
	project:=t.TempDir()
	dir:=ExecutionRawDir(project)
	if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
	target:=filepath.Join(t.TempDir(),"outside.jsonl")
	if err:=os.WriteFile(target,[]byte("private outside raw"),0600);err!=nil{t.Fatal(err)}
	link:=filepath.Join(dir,"linked.jsonl")
	if err:=os.Symlink(target,link);err!=nil{t.Skipf("symlink not supported: %v",err)}
	report,err:=AnalyzeRawProtection(project,Ledger{},time.Now())
	if err!=nil{t.Fatal(err)}
	if len(report.Files)!=1||!hasCode(report.Files[0].ReasonCodes,"unverifiable") ||
		report.SafeNowBytes!=0||report.ConditionalBytes!=0 {
		t.Fatalf("symlink must not be scanned or estimated: %+v",report)
	}
}
