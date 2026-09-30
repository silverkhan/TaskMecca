package webui

import (
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "encoding/pem"
    "fmt"
    "net"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

type tailscaleStatus struct {
    Self struct {
        DNSName string `json:"DNSName"`
        TailscaleIPs []string `json:"TailscaleIPs"`
    } `json:"Self"`
}

type TLSInfo struct {
    Enabled bool
    DNSName string
    IP string
    CertFile string
    KeyFile string
    Error string
}

func tailscaleStatusInfo() (string,string,error) {
    cmd:=exec.Command("tailscale","status","--json")
    out,err:=cmd.Output()
    if err!=nil { return "","",fmt.Errorf("tailscale status --json failed: %w",err) }
    var status tailscaleStatus
    if err=json.Unmarshal(out,&status); err!=nil { return "","",err }
    dns:=strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName),".")
    ip:=""
    for _,raw:=range status.Self.TailscaleIPs {
        parsed:=net.ParseIP(strings.TrimSpace(raw))
        if isTailscaleIPv4(parsed) { ip=parsed.String(); break }
    }
    if ip=="" { ip=tailscaleIPv4() }
    if dns=="" || ip=="" { return dns,ip,fmt.Errorf("Tailscale DNS name or IPv4 address is unavailable") }
    return dns,ip,nil
}

func safeTLSName(name string) string {
    var b strings.Builder
    for _,r:=range name {
        if (r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='.'||r=='-' { b.WriteRune(r) } else { b.WriteRune('_') }
    }
    return b.String()
}

func certValidFor(path,dns string,min time.Duration) bool {
    raw,err:=os.ReadFile(path)
    if err!=nil { return false }
    block,_:=pem.Decode(raw)
    if block==nil { return false }
    cert,err:=x509.ParseCertificate(block.Bytes)
    if err!=nil { return false }
    if time.Until(cert.NotAfter)<min { return false }
    return cert.VerifyHostname(dns)==nil
}

func ensureTailscaleTLS() TLSInfo {
    dns,ip,err:=tailscaleStatusInfo()
    if err!=nil { return TLSInfo{DNSName:dns,IP:ip,Error:err.Error()} }
    dir:=filepath.Join(webServiceDir(),"tls")
    if err=os.MkdirAll(dir,0700); err!=nil { return TLSInfo{DNSName:dns,IP:ip,Error:err.Error()} }
    base:=safeTLSName(dns)
    certFile:=filepath.Join(dir,base+".crt")
    keyFile:=filepath.Join(dir,base+".key")
    if !certValidFor(certFile,dns,7*24*time.Hour) {
        cmd:=exec.Command("tailscale","cert",
            "--cert-file="+certFile,
            "--key-file="+keyFile,
            "--min-validity=168h",
            dns,
        )
        output,cmdErr:=cmd.CombinedOutput()
        if cmdErr!=nil {
            _=os.Remove(certFile)
            _=os.Remove(keyFile)
            msg:=strings.TrimSpace(string(output))
            if msg!="" { msg=": "+msg }
            return TLSInfo{DNSName:dns,IP:ip,Error:"unable to provision Tailscale HTTPS certificate"+msg}
        }
        _=os.Chmod(keyFile,0600)
        _=os.Chmod(certFile,0644)
    }
    if !certValidFor(certFile,dns,time.Hour) {
        return TLSInfo{DNSName:dns,IP:ip,Error:"provisioned Tailscale certificate is not valid for "+dns}
    }
    if _,err=tls.LoadX509KeyPair(certFile,keyFile); err!=nil {
        return TLSInfo{DNSName:dns,IP:ip,Error:"unable to load Tailscale TLS keypair: "+err.Error()}
    }
    return TLSInfo{Enabled:true,DNSName:dns,IP:ip,CertFile:certFile,KeyFile:keyFile}
}

func tailscaleTLSConfig(info TLSInfo) (*tls.Config,error) {
    cert,err:=tls.LoadX509KeyPair(info.CertFile,info.KeyFile)
    if err!=nil { return nil,err }
    return &tls.Config{
        Certificates:[]tls.Certificate{cert},
        MinVersion:tls.VersionTLS12,
    },nil
}
