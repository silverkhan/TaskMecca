package maintenance

import (
    "net/url"
    "os"
    "path/filepath"
    "testing"
)

func TestCompareVersionsWithPrerelease(t *testing.T) {
    cases:=[]struct{
        left string
        right string
        want int
    }{
        {"0.2.45-dev.1","0.2.44",1},
        {"0.2.45-dev.2","0.2.45-dev.1",1},
        {"0.2.45-dev.10","0.2.45-dev.2",1},
        {"0.2.45","0.2.45-dev.99",1},
        {"0.2.45-dev.1","0.2.45",-1},
        {"0.2.46-dev.1","0.2.45",1},
        {"0.2.45+build.1","0.2.45+build.2",0},
    }
    for _,tc:=range cases {
        got:=compareVersions(tc.left,tc.right)
        if got<0 { got=-1 } else if got>0 { got=1 }
        if got!=tc.want {
            t.Fatalf("compareVersions(%q,%q)=%d, want %d",tc.left,tc.right,got,tc.want)
        }
    }
}

func TestUpdateChannelPersistenceAndReleaseTag(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    t.Setenv("TASK_MECCA_CHANNEL","")
    t.Setenv("TASK_MECCA_RELEASE_TAG","")

    if got:=CurrentChannel(); got!="stable" {
        t.Fatalf("default channel=%q, want stable",got)
    }
    _,tag:=releaseLocation()
    if tag!="release-stable" {
        t.Fatalf("stable release tag=%q, want release-stable",tag)
    }

    if err:=SetChannel("dev"); err!=nil { t.Fatal(err) }
    if got:=CurrentChannel(); got!="dev" {
        t.Fatalf("persisted channel=%q, want dev",got)
    }
    _,tag=releaseLocation()
    if tag!="release-dev" {
        t.Fatalf("dev release tag=%q, want release-dev",tag)
    }
    if got:=versionCachePath(); got!=filepath.Join(home,"update-check-dev.json") {
        t.Fatalf("dev cache path=%q",got)
    }

    if err:=SetChannel("stable"); err!=nil { t.Fatal(err) }
    if got:=CurrentChannel(); got!="stable" {
        t.Fatalf("restored channel=%q, want stable",got)
    }
}


func TestExplicitReleaseChannelLocation(t *testing.T) {
    t.Setenv("TASK_MECCA_REPO","silverkhan/TaskMecca")
    t.Setenv("TASK_MECCA_RELEASE_TAG","custom-test-tag")

    repoName,tag,err:=releaseLocationForChannel("dev")
    if err!=nil { t.Fatal(err) }
    if repoName!="silverkhan/TaskMecca" || tag!="release-dev" {
        t.Fatalf("explicit dev release=%s %s",repoName,tag)
    }
    _,tag,err=releaseLocationForChannel("stable")
    if err!=nil { t.Fatal(err) }
    if tag!="release-stable" { t.Fatalf("explicit stable tag=%s",tag) }
}

func TestWebChannelSwitchRejectsEnvironmentOverride(t *testing.T) {
    t.Setenv("TASK_MECCA_CHANNEL","stable")
    if _,err:=SwitchChannel("0.2.49","dev"); err==nil {
        t.Fatal("expected environment override to block Web channel switch")
    }
}


func TestFrameworkSyncCandidatesFollowChannel(t *testing.T) {
    home:=t.TempDir()
    t.Setenv("TASK_MECCA_HOME",home)
    t.Setenv("TASK_MECCA_CHANNEL","")

    stableProject:=filepath.Join(t.TempDir(),"stable-project")
    devProject:=filepath.Join(t.TempDir(),"dev-project")
    for _,item:=range []struct{ path,version string }{{stableProject,"0.2.50"},{devProject,"0.2.51-dev.3"}} {
        if err:=os.MkdirAll(filepath.Join(item.path,"_task_mecca"),0755); err!=nil { t.Fatal(err) }
        if err:=os.WriteFile(filepath.Join(item.path,"_task_mecca","VERSION"),[]byte(item.version+"\n"),0644); err!=nil { t.Fatal(err) }
        if err:=RegisterProject(item.path); err!=nil { t.Fatal(err) }
    }

    down:=frameworkSyncCandidates("0.2.51-dev.4","0.2.50","stable")
    if len(down)!=1 || down[0].Path!=devProject {
        t.Fatalf("stable sync candidates=%+v",down)
    }
    up:=frameworkSyncCandidates("0.2.50","0.2.51-dev.4","dev")
    if len(up)!=1 || up[0].Path!=stableProject {
        t.Fatalf("dev sync candidates=%+v",up)
    }
}


func TestCacheBustURLReplacesTokenAndPreservesQuery(t *testing.T) {
    raw:="https://github.com/silverkhan/TaskMecca/releases/download/release-dev/VERSION.txt?existing=1"
    first:=cacheBustURL(raw,"0.2.52-dev.7")
    parsed,err:=url.Parse(first)
    if err!=nil { t.Fatal(err) }
    if got:=parsed.Query().Get("_tm"); got!="0.2.52-dev.7" {
        t.Fatalf("cache token=%q",got)
    }
    if got:=parsed.Query().Get("existing"); got!="1" {
        t.Fatalf("existing query lost: %q",got)
    }

    second:=cacheBustURL(first,"0.2.52-dev.8")
    parsed,err=url.Parse(second)
    if err!=nil { t.Fatal(err) }
    values:=parsed.Query()["_tm"]
    if len(values)!=1 || values[0]!="0.2.52-dev.8" {
        t.Fatalf("cache token should be replaced, got=%v",values)
    }
}
