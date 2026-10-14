package preview

import (
    "archive/zip"
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

const maxArtifactArchiveBytes int64 = 150 << 20
const maxBinaryBytes int64 = 110 << 20

type manifest struct {
    Repository string `json:"repository"`
    PRNumber int `json:"pr_number"`
    HeadSHA string `json:"head_sha"`
    RunID int64 `json:"run_id"`
    Workflow string `json:"workflow"`
    SchemaVersion int `json:"schema_version"`
    ExecutionSwitchProtocol int `json:"execution_switch_protocol"`
}

func assetForPlatform(goos,arch string)(string,error){
    switch {
    case goos=="darwin"&&arch=="arm64":return "task-mecca-macos-arm64",nil
    case goos=="darwin"&&arch=="amd64":return "task-mecca-macos-amd64",nil
    case goos=="windows"&&arch=="amd64":return "task-mecca-windows-amd64.exe",nil
    case goos=="linux"&&arch=="amd64":return "task-mecca-linux-amd64",nil
    default:return "",fmt.Errorf("Preview artifacts do not support %s/%s",goos,arch)
    }
}

func readZipEntry(file *zip.File,max int64)([]byte,error){
    if file.UncompressedSize64>uint64(max){return nil,errors.New("Preview artifact entry exceeds size limit")}
    reader,err:=file.Open()
    if err!=nil{return nil,err}
    defer reader.Close()
    data,err:=io.ReadAll(io.LimitReader(reader,max+1))
    if err!=nil{return nil,err}
    if int64(len(data))>max {return nil,errors.New("Preview artifact entry exceeds size limit")}
    return data,nil
}

func checksumEntry(manifest []byte,name string)(string,error) {
    // Legacy builds generated SHA256SUMS from the repository root, leaving
    // "dist/task-mecca-*" paths in the text manifest. GitHub's artifact ZIP
    // contains the same binaries at its root. Only this exact legacy prefix
    // is accepted, not arbitrary paths or basename matching.
    expectedLegacy:="dist/"+name
    digest:=""
    for _,line:=range strings.Split(string(manifest),"\n"){
        fields:=strings.Fields(line)
        if len(fields)!=2 {continue}
        actual:=strings.TrimPrefix(fields[1],"*")
        if actual!=name&&actual!=expectedLegacy {continue}
        if digest!="" {
            return "",errors.New("duplicate Preview platform checksum entry")
        }
        decoded,err:=hex.DecodeString(fields[0])
        if err!=nil||len(decoded)!=sha256.Size {
            return "",errors.New("invalid Preview checksum")
        }
        digest=strings.ToLower(fields[0])
    }
    if digest=="" {
        return "",errors.New("platform binary missing from Preview checksum manifest")
    }
    return digest,nil
}

// DownloadVerified prepares an artifact as non-executable cached bytes.
// The caller must obtain Build from ListBuilds, not untrusted browser input.
// No call to this function executes a binary or touches an operating project.
func (c *Client) DownloadVerified(ctx context.Context, build Build,cacheRoot,goos,arch string,requireSwitchProtocol ...bool)(string,error){
    if build.BuildStatus!="success" || build.ArtifactID<=0 || build.RunID<=0 ||
        build.Number<=0 || !validSHA(build.HeadSHA) ||
        build.ArtifactName!="task-mecca-preview-pr-"+strconv.Itoa(build.Number)+"-"+build.HeadSHA {
        return "",errors.New("Preview artifact identity is not verified")
    }
    name,err:=assetForPlatform(goos,arch)
    if err!=nil{return "",err}

    archiveURL:=strings.TrimRight(c.APIURL,"/")+fmt.Sprintf("/repos/%s/actions/artifacts/%d/zip",Repository,build.ArtifactID)
    // ZIP downloads require an Actions:read credential even when the public
    // PR metadata and artifact listing were accessible anonymously.
    client:=c.HTTP
    if client==nil{
        client=&http.Client{Timeout:65*time.Second}
    }else if client.Timeout>0&&client.Timeout<65*time.Second{
        // Metadata calls should stay quick; a 15MB+ binary ZIP needs longer.
        copyClient:=*client
        copyClient.Timeout=65*time.Second
        client=&copyClient
    }
    fetch:=func(token string)(*http.Response,error){
        req,err:=http.NewRequestWithContext(ctx,http.MethodGet,archiveURL,nil)
        if err!=nil{return nil,err}
        req.Header.Set("Accept","application/vnd.github+json")
        req.Header.Set("X-GitHub-Api-Version","2022-11-28")
        if token!=""{req.Header.Set("Authorization","Bearer "+token)}
        return client.Do(req)
    }
    resp,err:=fetch(c.Token)
    if err!=nil{return "",fmt.Errorf("Preview artifact download failed: %w",err)}
    if resp.StatusCode==http.StatusUnauthorized&&c.Token==""&&c.TokenFallback!=nil {
        _=resp.Body.Close()
        // Existing gh CLI sign-in can supply credentials without making gh
        // a mandatory runtime dependency or forwarding secrets to PR code.
        fallback,tokenErr:=c.TokenFallback(ctx)
        if tokenErr==nil&&fallback!="" {
            resp,err=fetch(fallback)
            if err!=nil{return "",fmt.Errorf("Preview authenticated download failed: %w",err)}
        }else{
            return "",errors.New("GitHub Actions artifact download requires Actions:read authentication. Sign in using 'gh auth login -h github.com' (optional GitHub CLI), or set TASK_MECCA_GITHUB_TOKEN for the Dev Web service")
        }
    }
    defer resp.Body.Close()
    switch resp.StatusCode {
    case http.StatusOK:
    case http.StatusUnauthorized:
        return "",errors.New("GitHub rejected Preview artifact credentials (HTTP 401). Provide a valid TASK_MECCA_GITHUB_TOKEN with Actions:read, or sign in using 'gh auth login -h github.com'")
    case http.StatusForbidden:
        return "",errors.New("GitHub denied Preview artifact access (HTTP 403). Confirm Actions:read permission and repository access for the Dev Web credentials")
    case http.StatusGone:
        return "",errors.New("GitHub Preview artifact has expired (HTTP 410); regenerate the PR build")
    default:
        return "",fmt.Errorf("Preview artifact download failed (HTTP %d)",resp.StatusCode)
    }
    if resp.ContentLength>maxArtifactArchiveBytes{return "",errors.New("Preview artifact archive too large")}
    archive,err:=io.ReadAll(io.LimitReader(resp.Body,maxArtifactArchiveBytes+1))
    if err!=nil{return "",err}
    if int64(len(archive))>maxArtifactArchiveBytes{return "",errors.New("Preview artifact archive too large")}

    reader,err:=zip.NewReader(bytes.NewReader(archive),int64(len(archive)))
    if err!=nil{return "",fmt.Errorf("Preview artifact is not a ZIP: %w",err)}
    if len(reader.File)>20 {return "",errors.New("Preview archive contains too many entries")}
    fileMap:=map[string]*zip.File{}
    for _,f:=range reader.File {
        if f.FileInfo().IsDir()||f.Mode()&os.ModeSymlink!=0 ||
            filepath.Base(f.Name)!=f.Name || strings.Contains(f.Name,"..") ||
            strings.Contains(f.Name,"\\") || f.Name=="" {
            return "",errors.New("Preview ZIP contains an unsafe path or link")
        }
        if _,exists:=fileMap[f.Name];exists{return "",errors.New("Preview ZIP contains duplicate filenames")}
        fileMap[f.Name]=f
    }
    get:=func(key string, max int64)([]byte,error){
        entry:=fileMap[key]
        if entry==nil{return nil,fmt.Errorf("Preview archive missing %s",key)}
        return readZipEntry(entry,max)
    }
    info,err:=get("preview-build.json",16<<10)
    if err!=nil{return "",err}
    var id manifest
    if err=json.Unmarshal(info,&id);err!=nil{return "",fmt.Errorf("invalid Preview provenance: %w",err)}
    if id.Repository!=Repository||id.PRNumber!=build.Number||id.HeadSHA!=build.HeadSHA||
        id.RunID!=build.RunID||id.Workflow!=workflow||id.SchemaVersion!=1 {
        return "",errors.New("Preview provenance does not match PR head, repository or CI run")
    }
    if len(requireSwitchProtocol)>0 && requireSwitchProtocol[0] && id.ExecutionSwitchProtocol!=1 {
        return "",errors.New("this PR build predates the Dev/PR return protocol; rebuild it from current Dev before environment switching")
    }
    checks,err:=get("SHA256SUMS.txt",32<<10)
    if err!=nil{return "",err}
    expected,err:=checksumEntry(checks,name)
    if err!=nil{return "",err}
    binary,err:=get(name,maxBinaryBytes)
    if err!=nil{return "",err}
    sum:=sha256.Sum256(binary)
    if hex.EncodeToString(sum[:])!=expected{return "",errors.New("Preview binary checksum mismatch")}

    destination:=filepath.Join(cacheRoot,strconv.Itoa(build.Number),build.HeadSHA)
    if err=os.MkdirAll(destination,0700);err!=nil{return "",err}
    target:=filepath.Join(destination,name)
    if existing,readErr:=os.ReadFile(target);readErr==nil{
        digest:=sha256.Sum256(existing)
        if hex.EncodeToString(digest[:])==expected {return target,nil}
    }
    tmp,err:=os.CreateTemp(destination,".preview-verified-")
    if err!=nil{return "",err}
    tempName:=tmp.Name()
    defer os.Remove(tempName)
    if err=tmp.Chmod(0600);err==nil {
        _,err=tmp.Write(binary)
    }
    if syncErr:=tmp.Sync();err==nil{err=syncErr}
    if closeErr:=tmp.Close();err==nil{err=closeErr}
    if err!=nil{return "",err}
    if err=os.Rename(tempName,target);err!=nil{return "",err}
    return target,nil
}
