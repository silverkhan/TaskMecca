package maintenance

import (
 "encoding/json"
 "os"
 "path/filepath"
 "testing"
 "time"
)

func TestCachedUpgradeTargetSkipsRedundantVersionLookupOnlyForVerifiedFreshVersion(t *testing.T){
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 write:=func(info VersionInfo){
  t.Helper()
  if err:=os.MkdirAll(filepath.Dir(versionCachePath()),0700);err!=nil{t.Fatal(err)}
  raw,err:=json.Marshal(info);if err!=nil{t.Fatal(err)}
  if err=os.WriteFile(versionCachePath(),raw,0600);err!=nil{t.Fatal(err)}
 }
 base:=VersionInfo{Current:"0.3.1-dev.29",Latest:"0.3.1-dev.30",
  Channel:"dev",UpdateAvailable:true,CheckedAt:time.Now().UTC().Format(time.RFC3339)}
 write(base)
 actual,ok:=cachedUpgradeTarget("0.3.1-dev.29","0.3.1-dev.30")
 if !ok||actual.Latest!=base.Latest||!actual.UpdateAvailable {t.Fatalf("fresh matching target unavailable: %+v %v",actual,ok)}
 cases:=[]struct{name string; change func(*VersionInfo); expected string}{
  {"stale",func(x *VersionInfo){x.CheckedAt=time.Now().Add(-7*time.Minute).Format(time.RFC3339)},"0.3.1-dev.30"},
  {"bad_time",func(x *VersionInfo){x.CheckedAt="broken"},"0.3.1-dev.30"},
  {"future",func(x *VersionInfo){x.CheckedAt=time.Now().Add(time.Hour).Format(time.RFC3339)},"0.3.1-dev.30"},
  {"failed_refresh",func(x *VersionInfo){x.Error="upstream unavailable"},"0.3.1-dev.30"},
  {"different_channel",func(x *VersionInfo){x.Channel="stable"},"0.3.1-dev.30"},
  {"different_current",func(x *VersionInfo){x.Current="0.3.1-dev.28"},"0.3.1-dev.30"},
  {"different_latest",func(x *VersionInfo){x.Latest="0.3.1-dev.31"},"0.3.1-dev.30"},
  {"not_newer",func(x *VersionInfo){},"0.3.1-dev.29"},
  {"missing_target",func(x *VersionInfo){},""},
 }
 for _,tc:=range cases{
  t.Run(tc.name,func(t *testing.T){
   candidate:=base;tc.change(&candidate);write(candidate)
   if info,ok:=cachedUpgradeTarget("0.3.1-dev.29",tc.expected);ok{
    t.Fatalf("untrusted update target accepted: %+v",info)
   }
  })
 }
}

func TestCachedUpgradeTargetNeverTrustsTamperedOrMissingCache(t *testing.T){
 t.Setenv("TASK_MECCA_CHANNEL","dev")
 t.Setenv("TASK_MECCA_HOME",t.TempDir())
 if _,ok:=cachedUpgradeTarget("0.3.1-dev.29","0.3.1-dev.30");ok{t.Fatal("missing cache accepted")}
 if err:=os.WriteFile(versionCachePath(),[]byte("{invalid"),0600);err!=nil{t.Fatal(err)}
 if _,ok:=cachedUpgradeTarget("0.3.1-dev.29","0.3.1-dev.30");ok{t.Fatal("invalid cache accepted")}
}
