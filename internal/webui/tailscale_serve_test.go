package webui

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestServeConfigStateMatchesTaskMeccaProxy(t *testing.T) {
    status:=tailscaleServeStatus{
        TCP:map[string]tailscaleServeTCPHandler{
            "18765":{HTTPS:true},
        },
        Web:map[string]tailscaleServeWebServer{
            "aiden.tailnet.ts.net:18765":{
                Handlers:map[string]tailscaleServeHTTPHandler{
                    "/":{Proxy:"http://127.0.0.1:18765"},
                },
            },
        },
    }
    matching,occupied:=serveConfigState(status,"aiden.tailnet.ts.net",18765)
    if !matching || !occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}

func TestServeConfigStateDetectsConflict(t *testing.T) {
    status:=tailscaleServeStatus{
        TCP:map[string]tailscaleServeTCPHandler{
            "18765":{HTTPS:true},
        },
        Web:map[string]tailscaleServeWebServer{
            "aiden.tailnet.ts.net:18765":{
                Handlers:map[string]tailscaleServeHTTPHandler{
                    "/":{Proxy:"http://127.0.0.1:9999"},
                },
            },
        },
    }
    matching,occupied:=serveConfigState(status,"aiden.tailnet.ts.net",18765)
    if matching || !occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}

func TestServeConfigStateEmpty(t *testing.T) {
    matching,occupied:=serveConfigState(tailscaleServeStatus{},"aiden.tailnet.ts.net",18765)
    if matching || occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}


func TestEnsureTailscaleServeLeavesHealthyOwnedRouteUntouched(t *testing.T) {
    dir:=t.TempDir()
    home:=filepath.Join(dir,"home")
    if err:=os.MkdirAll(filepath.Join(home,"web"),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_HOME",home)

    dns:="aiden.tailnet.ts.net"
    port:=18765
    statusPath:=filepath.Join(dir,"status.json")
    logPath:=filepath.Join(dir,"calls.log")
    matchingStatus:=`{"TCP":{"18765":{"HTTPS":true}},"Web":{"aiden.tailnet.ts.net:18765":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:18765"}}}}}`
    if err:=os.WriteFile(statusPath,[]byte(matchingStatus),0644); err!=nil { t.Fatal(err) }

    cli:=filepath.Join(dir,"tailscale")
    script:="#!/bin/sh\n"+
        "echo \"$@\" >> \""+logPath+"\"\n"+
        "if [ \"$1 $2 $3\" = \"serve status --json\" ]; then cat \""+statusPath+"\"; exit 0; fi\n"+
        "exit 1\n"
    if err:=os.WriteFile(cli,[]byte(script),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_TAILSCALE_CLI",cli)

    owned:=tailscaleServeOwnership{DNSName:dns,Port:port,Target:tailscaleServeTarget(port)}
    if err:=writeServeOwnership(owned); err!=nil { t.Fatal(err) }

    url,managed,err:=ensureTailscaleServe(dns,port)
    if err!=nil { t.Fatal(err) }
    if !managed { t.Fatal("expected Task Mecca managed route") }
    if url!="https://aiden.tailnet.ts.net:18765/" { t.Fatalf("url=%q",url) }

    calls,err:=os.ReadFile(logPath)
    if err!=nil { t.Fatal(err) }
    text:=string(calls)
    if strings.Contains(text," off") || strings.Contains(text,"--bg") {
        t.Fatalf("healthy matching route must not be mutated: %s",text)
    }
}

func TestRefreshOwnedTailscaleServePerformsHardResetWhenExplicitlyCalled(t *testing.T) {
    dir:=t.TempDir()
    home:=filepath.Join(dir,"home")
    if err:=os.MkdirAll(filepath.Join(home,"web"),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_HOME",home)

    dns:="aiden.tailnet.ts.net"
    port:=18765
    statusPath:=filepath.Join(dir,"status.json")
    logPath:=filepath.Join(dir,"calls.log")
    matchingStatus:=`{"TCP":{"18765":{"HTTPS":true}},"Web":{"aiden.tailnet.ts.net:18765":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:18765"}}}}}`
    if err:=os.WriteFile(statusPath,[]byte(matchingStatus),0644); err!=nil { t.Fatal(err) }

    cli:=filepath.Join(dir,"tailscale")
    script:="#!/bin/sh\n"+
        "echo \"$@\" >> \""+logPath+"\"\n"+
        "if [ \"$1\" = \"serve\" ] && [ \"$4\" = \"off\" ]; then exit 0; fi\n"+
        "if [ \"$1\" = \"serve\" ] && [ \"$2\" = \"--bg\" ]; then exit 0; fi\n"+
        "exit 1\n"
    if err:=os.WriteFile(cli,[]byte(script),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_TAILSCALE_CLI",cli)

    owned:=tailscaleServeOwnership{DNSName:dns,Port:port,Target:tailscaleServeTarget(port)}
    if err:=writeServeOwnership(owned); err!=nil { t.Fatal(err) }

    if err:=refreshOwnedTailscaleServe(dns,port); err!=nil { t.Fatal(err) }

    calls,err:=os.ReadFile(logPath)
    if err!=nil { t.Fatal(err) }
    text:=string(calls)
    if !strings.Contains(text,"serve --yes --https=18765 off") {
        t.Fatalf("hard reset did not disable owned route: %s",text)
    }
    if !strings.Contains(text,"serve --bg --yes --https=18765 http://127.0.0.1:18765") {
        t.Fatalf("hard reset did not reapply owned route: %s",text)
    }
}

func TestEnsureTailscaleServeLeavesHealthyUnownedRouteUntouched(t *testing.T) {
    dir:=t.TempDir()
    home:=filepath.Join(dir,"home")
    if err:=os.MkdirAll(filepath.Join(home,"web"),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_HOME",home)

    statusPath:=filepath.Join(dir,"status.json")
    logPath:=filepath.Join(dir,"calls.log")
    matchingStatus:=`{"TCP":{"18765":{"HTTPS":true}},"Web":{"aiden.tailnet.ts.net:18765":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:18765"}}}}}`
    if err:=os.WriteFile(statusPath,[]byte(matchingStatus),0644); err!=nil { t.Fatal(err) }

    cli:=filepath.Join(dir,"tailscale")
    script:="#!/bin/sh\n"+
        "echo \"$@\" >> \""+logPath+"\"\n"+
        "if [ \"$1 $2 $3\" = \"serve status --json\" ]; then cat \""+statusPath+"\"; exit 0; fi\n"+
        "exit 1\n"
    if err:=os.WriteFile(cli,[]byte(script),0755); err!=nil { t.Fatal(err) }
    t.Setenv("TASK_MECCA_TAILSCALE_CLI",cli)

    _,managed,err:=ensureTailscaleServe("aiden.tailnet.ts.net",18765)
    if err!=nil { t.Fatal(err) }
    if managed { t.Fatal("matching user route must not become Task Mecca-owned") }

    calls,err:=os.ReadFile(logPath)
    if err!=nil { t.Fatal(err) }
    text:=string(calls)
    if strings.Contains(text," off") || strings.Contains(text,"--bg") {
        t.Fatalf("healthy unowned route must not be mutated: %s",text)
    }
}

