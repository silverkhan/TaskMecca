package webui

import "testing"

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
