package webui

import (
    stdcontext "context"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "encoding/pem"
    "fmt"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strconv"
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

func tailscaleCLIPath() (string,error) {
    if override:=strings.TrimSpace(os.Getenv("TASK_MECCA_TAILSCALE_CLI")); override!="" {
        info,err:=os.Stat(override)
        if err!=nil || info.IsDir() {
            return "",fmt.Errorf("TASK_MECCA_TAILSCALE_CLI does not point to a usable file: %s",override)
        }
        return override,nil
    }

    if path,err:=exec.LookPath("tailscale"); err==nil {
        return path,nil
    }

    candidates:=[]string{}
    switch runtime.GOOS {
    case "darwin":
        candidates=[]string{
            "/Applications/Tailscale.app/Contents/MacOS/Tailscale",
            "/usr/local/bin/tailscale",
            "/opt/homebrew/bin/tailscale",
        }
    case "windows":
        if root:=strings.TrimSpace(os.Getenv("ProgramFiles")); root!="" {
            candidates=append(candidates,filepath.Join(root,"Tailscale","tailscale.exe"))
        }
        if root:=strings.TrimSpace(os.Getenv("ProgramW6432")); root!="" {
            candidates=append(candidates,filepath.Join(root,"Tailscale","tailscale.exe"))
        }
        if root:=strings.TrimSpace(os.Getenv("LOCALAPPDATA")); root!="" {
            candidates=append(candidates,filepath.Join(root,"Tailscale","tailscale.exe"))
        }
    default:
        candidates=[]string{
            "/usr/bin/tailscale",
            "/usr/local/bin/tailscale",
            "/opt/homebrew/bin/tailscale",
        }
    }

    for _,candidate:=range candidates {
        info,err:=os.Stat(candidate)
        if err==nil && !info.IsDir() {
            return candidate,nil
        }
    }

    return "",fmt.Errorf("Tailscale CLI not found in PATH or known installation locations")
}

func tailscaleCommand(args ...string) (*exec.Cmd,error) {
    path,err:=tailscaleCLIPath()
    if err!=nil { return nil,err }

    cmd:=exec.Command(path,args...)
    if runtime.GOOS=="darwin" && strings.Contains(path,"/Tailscale.app/") {
        cmd.Env=append(os.Environ(),"TAILSCALE_BE_CLI=1")
    }
    return cmd,nil
}

func tailscaleStatusInfo() (string,string,error) {
    cmd,err:=tailscaleCommand("status","--json")
    if err!=nil { return "","",err }

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
        cmd,commandErr:=tailscaleCommand("cert",
            "--cert-file="+certFile,
            "--key-file="+keyFile,
            "--min-validity=168h",
            dns,
        )
        if commandErr!=nil {
            return TLSInfo{DNSName:dns,IP:ip,Error:commandErr.Error()}
        }
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


func verifyTailscaleHTTPS(info TLSInfo,port int,instanceID string) error {
    if !info.Enabled || info.IP=="" || info.DNSName=="" {
        return fmt.Errorf("Tailscale HTTPS verification requires an enabled TLS endpoint")
    }

    dialer:=&net.Dialer{Timeout:500*time.Millisecond}
    transport:=&http.Transport{
        TLSClientConfig:&tls.Config{
            ServerName:info.DNSName,
            MinVersion:tls.VersionTLS12,
        },
        DialContext:func(ctx stdcontext.Context,network,address string) (net.Conn,error) {
            return dialer.DialContext(ctx,"tcp4",net.JoinHostPort(info.IP,strconv.Itoa(port)))
        },
    }
    defer transport.CloseIdleConnections()

    client:=&http.Client{Transport:transport,Timeout:800*time.Millisecond}
    endpoint:=fmt.Sprintf("https://%s:%d/api/health",info.DNSName,port)
    deadline:=time.Now().Add(2*time.Second)
    var lastErr error

    for {
        resp,err:=client.Get(endpoint)
        if err==nil {
            var payload map[string]any
            decodeErr:=json.NewDecoder(resp.Body).Decode(&payload)
            _=resp.Body.Close()
            if resp.StatusCode==http.StatusOK && decodeErr==nil && fmt.Sprint(payload["instance_id"])==instanceID {
                return nil
            }
            if decodeErr!=nil {
                lastErr=fmt.Errorf("HTTPS health response decode failed: %w",decodeErr)
            } else {
                lastErr=fmt.Errorf("HTTPS health response mismatch: HTTP %d instance=%v",resp.StatusCode,payload["instance_id"])
            }
        } else {
            lastErr=err
        }

        if time.Now().After(deadline) {
            return fmt.Errorf("Tailscale HTTPS self-check failed for %s:%d: %w",info.IP,port,lastErr)
        }
        time.Sleep(100*time.Millisecond)
    }
}
