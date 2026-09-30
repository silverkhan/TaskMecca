//go:build !darwin

package webui

import (
    "net"
    "strconv"
)

func listenTailscaleIP(ip string,port int) (net.Listener,error) {
    return net.Listen("tcp",net.JoinHostPort(ip,strconv.Itoa(port)))
}
