package activity

import "testing"

func TestActivityDiagnosticBoundedAndSampled(t *testing.T){
    l:=Ledger{Project:t.TempDir()}
    for i:=0;i<6;i++{
        e:=validEvent()
        e.EventID="diagnostic-"+fmtIndex(i)
        if _,err:=l.Append(e);err!=nil{t.Fatal(err)}
    }
    small,err:=l.Diagnose(3)
    if err!=nil{t.Fatal(err)}
    if small.EventFilesObserved!=3||!small.Truncated||small.SampledFiles!=3||small.SampledBytes==0{
        t.Fatalf("bounded diagnostic %+v",small)
    }
    full,err:=l.Diagnose(100)
    if err!=nil{t.Fatal(err)}
    if full.Truncated||full.EventFilesObserved!=6||full.SampledFiles!=6{
        t.Fatalf("complete diagnostic %+v",full)
    }
}

func TestActivityDiagnosticEmptyProject(t *testing.T){
    l:=Ledger{Project:t.TempDir()}
    stats,err:=l.Diagnose(10)
    if err!=nil || stats.EventFilesObserved!=0 || stats.Truncated{t.Fatalf("empty %+v %v",stats,err)}
}
