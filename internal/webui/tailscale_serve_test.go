package webui

import "testing"

func TestServeConfigStateMatchesTaskMeccaProxy(t *testing.T) {
    status:=tailscaleServeStatus{
        TCP:map[string]tailscaleServeTCPHandler{
            "443":{HTTPS:true},
        },
        Web:map[string]tailscaleServeWebServer{
            "aiden.tailnet.ts.net:443":{
                Handlers:map[string]tailscaleServeHTTPHandler{
                    "/":{Proxy:"http://127.0.0.1:18765"},
                },
            },
        },
    }
    matching,occupied:=serveConfigState(status,"aiden.tailnet.ts.net",443,18765)
    if !matching || !occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}

func TestServeConfigStateDetectsConflict(t *testing.T) {
    status:=tailscaleServeStatus{
        TCP:map[string]tailscaleServeTCPHandler{
            "443":{HTTPS:true},
        },
        Web:map[string]tailscaleServeWebServer{
            "aiden.tailnet.ts.net:443":{
                Handlers:map[string]tailscaleServeHTTPHandler{
                    "/":{Proxy:"http://127.0.0.1:9999"},
                },
            },
        },
    }
    matching,occupied:=serveConfigState(status,"aiden.tailnet.ts.net",443,18765)
    if matching || !occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}

func TestServeConfigStateEmpty(t *testing.T) {
    matching,occupied:=serveConfigState(tailscaleServeStatus{},"aiden.tailnet.ts.net",443,18765)
    if matching || occupied {
        t.Fatalf("matching=%v occupied=%v",matching,occupied)
    }
}

func TestTailscaleServeURLUsesDefaultHTTPSWithoutPort(t *testing.T) {
    got:=tailscaleServeURL("aiden.tailnet.ts.net",443)
    if got!="https://aiden.tailnet.ts.net/" {
        t.Fatalf("got %q",got)
    }
}
