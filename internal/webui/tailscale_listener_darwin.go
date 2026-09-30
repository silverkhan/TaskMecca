//go:build darwin

package webui

import (
    stdcontext "context"
    "fmt"
    "net"
    "strconv"
    "strings"
    "syscall"
)

const ipBoundIf = 25 // IP_BOUND_IF on Darwin.

func tailscaleInterfaceIndex(ip string) (int,error) {
    target:=net.ParseIP(strings.TrimSpace(ip)).To4()
    if target==nil {
        return 0,fmt.Errorf("invalid Tailscale IPv4 address: %s",ip)
    }

    interfaces,err:=net.Interfaces()
    if err!=nil {
        return 0,err
    }
    for _,iface:=range interfaces {
        if iface.Flags&net.FlagUp==0 {
            continue
        }
        addrs,addrErr:=iface.Addrs()
        if addrErr!=nil {
            continue
        }
        for _,addr:=range addrs {
            raw:=addr.String()
            if slash:=strings.IndexByte(raw,'/'); slash>=0 {
                raw=raw[:slash]
            }
            candidate:=net.ParseIP(raw).To4()
            if candidate!=nil && candidate.Equal(target) {
                return iface.Index,nil
            }
        }
    }
    return 0,fmt.Errorf("Tailscale interface for %s was not found",ip)
}

func listenTailscaleIP(ip string,port int) (net.Listener,error) {
    ifIndex,err:=tailscaleInterfaceIndex(ip)
    if err!=nil {
        return nil,err
    }

    lc:=net.ListenConfig{
        Control: func(network,address string,raw syscall.RawConn) error {
            var sockErr error
            if err:=raw.Control(func(fd uintptr) {
                sockErr=syscall.SetsockoptInt(int(fd),syscall.IPPROTO_IP,ipBoundIf,ifIndex)
            }); err!=nil {
                return err
            }
            return sockErr
        },
    }

    // Tailscale itself binds the socket to the utun interface, then listens\n    // on the wildcard address on Darwin. Binding directly to the 100.x address\n    // can create a socket that is not reachable through the Network Extension.\n    return lc.Listen(stdcontext.Background(),"tcp4",net.JoinHostPort("",strconv.Itoa(port)))\n}
