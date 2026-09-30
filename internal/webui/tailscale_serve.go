package webui

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

type tailscaleServeTCPHandler struct {
    HTTPS bool `json:"HTTPS"`
}

type tailscaleServeHTTPHandler struct {
    Proxy string `json:"Proxy"`
}

type tailscaleServeWebServer struct {
    Handlers map[string]tailscaleServeHTTPHandler `json:"Handlers"`
}

type tailscaleServeStatus struct {
    TCP map[string]tailscaleServeTCPHandler `json:"TCP"`
    Web map[string]tailscaleServeWebServer `json:"Web"`
}

type tailscaleServeOwnership struct {
    DNSName string `json:"dns_name"`
    Port int `json:"port"`
    Target string `json:"target"`
}

func tailscaleServeTarget(port int) string {
    return fmt.Sprintf("http://127.0.0.1:%d",port)
}

func tailscaleServeOwnershipPath() string {
    return filepath.Join(webServiceDir(),"tailscale-serve.json")
}

func readTailscaleServeStatus() (tailscaleServeStatus,error) {
    cmd,err:=tailscaleCommand("serve","status","--json")
    if err!=nil { return tailscaleServeStatus{},err }
    out,err:=cmd.CombinedOutput()
    if err!=nil {
        msg:=strings.TrimSpace(string(out))
        if msg!="" { return tailscaleServeStatus{},fmt.Errorf("tailscale serve status failed: %s",msg) }
        return tailscaleServeStatus{},fmt.Errorf("tailscale serve status failed: %w",err)
    }
    var status tailscaleServeStatus
    if len(strings.TrimSpace(string(out)))==0 { return status,nil }
    if err:=json.Unmarshal(out,&status); err!=nil {
        return tailscaleServeStatus{},fmt.Errorf("invalid tailscale serve status JSON: %w",err)
    }
    return status,nil
}

func serveConfigState(status tailscaleServeStatus,dns string,port int) (matching bool,occupied bool) {
    portKey:=strconv.Itoa(port)
    target:=tailscaleServeTarget(port)
    hostPort:=dns+":"+portKey

    tcp,tcpExists:=status.TCP[portKey]
    web,webExists:=status.Web[hostPort]
    if !tcpExists && !webExists { return false,false }

    if tcpExists && tcp.HTTPS && webExists {
        if root,ok:=web.Handlers["/"]; ok && strings.TrimRight(root.Proxy,"/")==target {
            return true,true
        }
    }
    return false,true
}

func readServeOwnership() (tailscaleServeOwnership,bool) {
    data,err:=os.ReadFile(tailscaleServeOwnershipPath())
    if err!=nil { return tailscaleServeOwnership{},false }
    var owned tailscaleServeOwnership
    if json.Unmarshal(data,&owned)!=nil { return tailscaleServeOwnership{},false }
    return owned,true
}

func writeServeOwnership(owned tailscaleServeOwnership) error {
    if err:=os.MkdirAll(webServiceDir(),0755); err!=nil { return err }
    data,err:=json.MarshalIndent(owned,"","  ")
    if err!=nil { return err }
    tmp:=tailscaleServeOwnershipPath()+".tmp"
    if err:=os.WriteFile(tmp,append(data,'\n'),0600); err!=nil { return err }
    return os.Rename(tmp,tailscaleServeOwnershipPath())
}

func ownershipMatches(dns string,port int) bool {
    owned,ok:=readServeOwnership()
    if !ok { return false }
    return owned.DNSName==dns && owned.Port==port && strings.TrimRight(owned.Target,"/")==tailscaleServeTarget(port)
}

func applyTailscaleServe(port int) (string,error) {
    target:=tailscaleServeTarget(port)
    cmd,err:=tailscaleCommand("serve","--bg","--yes","--https="+strconv.Itoa(port),target)
    if err!=nil { return "",err }
    output,cmdErr:=cmd.CombinedOutput()
    if cmdErr!=nil {
        msg:=strings.TrimSpace(string(output))
        if msg!="" { return msg,fmt.Errorf("unable to configure Tailscale Serve: %s",msg) }
        return msg,fmt.Errorf("unable to configure Tailscale Serve: %w",cmdErr)
    }
    return strings.TrimSpace(string(output)),nil
}

func refreshOwnedTailscaleServe(dns string,port int) error {
    if !ownershipMatches(dns,port) { return nil }
    cmd,err:=tailscaleCommand("serve","--yes","--https="+strconv.Itoa(port),"off")
    if err!=nil { return err }
    output,cmdErr:=cmd.CombinedOutput()
    if cmdErr!=nil {
        msg:=strings.TrimSpace(string(output))
        if msg!="" { return fmt.Errorf("unable to refresh Tailscale Serve: %s",msg) }
        return fmt.Errorf("unable to refresh Tailscale Serve: %w",cmdErr)
    }
    if _,err:=applyTailscaleServe(port); err!=nil { return err }
    return nil
}

func ensureTailscaleServe(dns string,port int) (url string,managed bool,err error) {
    if dns=="" { return "",false,fmt.Errorf("Tailscale DNS name is unavailable") }
    status,err:=readTailscaleServeStatus()
    if err!=nil { return "",false,err }

    matching,occupied:=serveConfigState(status,dns,port)
    if matching {
        managed:=ownershipMatches(dns,port)
        if managed {
            if err:=refreshOwnedTailscaleServe(dns,port); err!=nil {
                return "",false,err
            }
            status,err=readTailscaleServeStatus()
            if err!=nil { return "",false,err }
            matching,_=serveConfigState(status,dns,port)
            if !matching {
                return "",false,fmt.Errorf("Tailscale Serve refresh completed but port %d is no longer registered",port)
            }
        } else {
            // Re-issue the identical Serve command to wake a persisted but stale
            // proxy listener without taking ownership of a user-managed route.
            if _,err:=applyTailscaleServe(port); err!=nil {
                return "",false,err
            }
        }
        return fmt.Sprintf("https://%s:%d/",dns,port),managed,nil
    }
    if occupied {
        return "",false,fmt.Errorf("Tailscale Serve port %d is already configured for another service",port)
    }

    target:=tailscaleServeTarget(port)
    output,applyErr:=applyTailscaleServe(port)
    if applyErr!=nil { return "",false,applyErr }

    status,err=readTailscaleServeStatus()
    if err!=nil { return "",false,err }
    matching,_=serveConfigState(status,dns,port)
    if !matching {
        msg:=strings.TrimSpace(string(output))
        if msg!="" {
            return "",false,fmt.Errorf("Tailscale Serve command completed but port %d was not registered: %s",port,msg)
        }
        return "",false,fmt.Errorf("Tailscale Serve command completed but port %d was not registered",port)
    }

    owned:=tailscaleServeOwnership{DNSName:dns,Port:port,Target:target}
    if err:=writeServeOwnership(owned); err!=nil {
        return "",false,fmt.Errorf("Tailscale Serve configured but ownership state could not be saved: %w",err)
    }
    return fmt.Sprintf("https://%s:%d/",dns,port),true,nil
}

func disableOwnedTailscaleServe(port int) error {
    owned,ok:=readServeOwnership()
    if !ok || owned.Port!=port { return nil }

    status,err:=readTailscaleServeStatus()
    if err!=nil { return err }
    matching,_:=serveConfigState(status,owned.DNSName,owned.Port)
    if matching {
        cmd,err:=tailscaleCommand("serve","--yes","--https="+strconv.Itoa(port),"off")
        if err!=nil { return err }
        output,cmdErr:=cmd.CombinedOutput()
        if cmdErr!=nil {
            msg:=strings.TrimSpace(string(output))
            if msg!="" { return fmt.Errorf("unable to disable Tailscale Serve: %s",msg) }
            return fmt.Errorf("unable to disable Tailscale Serve: %w",cmdErr)
        }
    }
    return os.Remove(tailscaleServeOwnershipPath())
}
