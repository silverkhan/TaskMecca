package main

import (
  "strings"
  "testing"

  "github.com/silverkhan/TaskMecca/internal/maintenance"
)

func TestPRRuntimeUsesPinnedDevReleaseVersion(t *testing.T){
  pr:=maintenance.ExecutionTarget{
    Kind:"pr", Channel:"dev", PRNumber:218,
    HeadSHA:strings.Repeat("a",40),OriginalDevVersion:"0.3.1-dev.20",
  }
  if got:=runtimeVersionForTarget("0.2.51",pr);got!="0.3.1-dev.20"{
    t.Fatalf("PR inherited wrong framework version: %s",got)
  }
  if got:=runtimeVersionForTarget("0.3.1-dev.20",maintenance.ExecutionTarget{Kind:"dev",Channel:"dev"});got!="0.3.1-dev.20"{
    t.Fatalf("normal Dev version changed: %s",got)
  }
  if got:=runtimeVersionForTarget("0.3.1",maintenance.ExecutionTarget{Kind:"stable",Channel:"stable"});got!="0.3.1"{
    t.Fatalf("Stable version changed: %s",got)
  }
}
