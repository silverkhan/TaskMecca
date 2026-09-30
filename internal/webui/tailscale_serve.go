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
    HTTPSPort int `json:"https_port"`
    BackendPort int `json:"backend_port"`
    Target string `json:"target"`
}

func tailscaleServeTarget(backendPort int) string {
    return fmt.Sprintf("http://127.0.0.1:%d",backendPort)
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

func serveConfigState(status tailscaleServeStatus,dns string,httpsPort,backendPort int) (matching bool,occupied bool) {
    portKey:=strconv.Itoa(httpsPort)
    target:=tailscaleServeTarget(backendPort)
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

func ownershipMatches(dns string,httpsPort,backendPort int) bool {
    owned,ok:=readServeOwnership()
    if !ok { return false }
    return owned.DNSName==dns &&
        owned.HTTPSPort==httpsPort &&
        owned.BackendPort==backendPort &&
        strings.TrimRight(owned.Target,"/")==tailscaleServeTarget(backendPort)
}

func ensureTailscaleServe(dns string,httpsPort,backendPort int) (url string,managed bool,err error) {
    if dns=="" { return "",false,fmt.Errorf("Tailscale DNS name is unavailable") }
    status,err:=readTailscaleServeStatus()
    if err!=nil { return "",false,err }

    matching,occupied:=serveConfigState(status,dns,httpsPort,backendPort)
    if matching {
        if httpsPort==443 {
        return fmt.Sprintf("https://%s/",dns),ownershipMatches(dns,httpsPort,backendPort),nil
    }
    return fmt.Sprintf("https://%s:%d/",dns,httpsPort),ownershipMatches(dns,httpsPort,backendPort),nil
    }
    if occupied {
        return "",false,fmt.Errorf("Tailscale Serve HTTPS port %d is already configured for another service",httpsPort)
    }

    target:=tailscaleServeTarget(backendPort)
    cmd,err:=tailscaleCommand("serve","--bg","--yes","--https="+strconv.Itoa(httpsPort),target)
    if err!=nil { return "",false,err }
    output,cmdErr:=cmd.CombinedOutput()
    if cmdErr!=nil {
        msg:=strings.TrimSpace(string(output))
        if msg!="" { return "",false,fmt.Errorf("unable to configure Tailscale Serve: %s",msg) }
        return "",false,fmt.Errorf("unable to configure Tailscale Serve: %w",cmdErr)
    }

    status,err=readTailscaleServeStatus()
    if err!=nil { return "",false,err }
    matching,_=serveConfigState(status,dns,httpsPort,backendPort)
    if !matching {
        msg:=strings.TrimSpace(string(output))
        if msg!="" {
            return "",false,fmt.Errorf("Tailscale Serve command completed but HTTPS port %d was not registered: %s",httpsPort,msg)
        }
        return "",false,fmt.Errorf("Tailscale Serve command completed but HTTPS port %d was not registered",httpsPort)
    }

    owned:=tailscaleServeOwnership{DNSName:dns,HTTPSPort:httpsPort,BackendPort:backendPort,Target:target}
    if err:=writeServeOwnership(owned); err!=nil {
        return "",false,fmt.Errorf("Tailscale Serve configured but ownership state could not be saved: %w",err)
    }
    if httpsPort==443 {
        return fmt.Sprintf("https://%s/",dns),true,nil
    }
    return fmt.Sprintf("https://%s:%d/",dns,httpsPort),true,nil
}

func disableOwnedTailscaleServe(backendPort int) error {
    owned,ok:=readServeOwnership()
    if !ok || owned.BackendPort!=backendPort { return nil }

    status,err:=readTailscaleServeStatus()
    if err!=nil { return err }
    matching,_:=serveConfigState(status,owned.DNSName,owned.HTTPSPort,owned.BackendPort)
    if matching {
        cmd,err:=tailscaleCommand("serve","--yes","--https="+strconv.Itoa(owned.HTTPSPort),"off")
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
