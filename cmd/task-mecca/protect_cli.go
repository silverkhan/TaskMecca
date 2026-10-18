package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/silverkhan/TaskMecca/internal/maintenance"
	"github.com/silverkhan/TaskMecca/internal/webui"
)

type protectedItem struct {
	Path string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes int64 `json:"bytes"`
	Category string `json:"category"`
}
type protectedSnapshot struct {
	SchemaVersion int `json:"schema_version"`
	Project string `json:"project"`
	CreatedAt string `json:"created_at"`
	Scope []string `json:"scope"`
	Items []protectedItem `json:"items"`
	Digest string `json:"digest"`
}
type protectedReport struct {
	Status string `json:"status"`
	Checked bool `json:"checked"`
	Scope []string `json:"scope"`
	BeforeFiles int `json:"before_files"`
	AfterFiles int `json:"after_files"`
	Missing []string `json:"missing"`
	Changed []string `json:"changed"`
	Added []string `json:"added"`
	Unverified []string `json:"unverified"`
	ManifestDigest string `json:"manifest_digest"`
	BackupPath string `json:"backup_path,omitempty"`
}
var protectedScope=[]string{"_task_mecca/data/**","_task_mecca/backlog*/**","_task_mecca/config.toml"}
func protectHash(raw []byte)string{h:=sha256.Sum256(raw);return hex.EncodeToString(h[:])}

func captureProjectProtection(project string)(protectedSnapshot,error){
	project,err:=filepath.Abs(project)
	if err!=nil{return protectedSnapshot{},err}
	snapshot:=protectedSnapshot{SchemaVersion:1,Project:project,CreatedAt:time.Now().UTC().Format(time.RFC3339Nano),Scope:append([]string{},protectedScope...),Items:[]protectedItem{}}
	root:=filepath.Join(project,"_task_mecca")
	paths:=[]string{filepath.Join(root,"data"),filepath.Join(root,"config.toml")}
	if entries,e:=os.ReadDir(root);e==nil{
		for _,entry:=range entries{
			if strings.HasPrefix(entry.Name(),"backlog"){paths=append(paths,filepath.Join(root,entry.Name()))}
		}
	}else if !os.IsNotExist(e){return snapshot,e}
	seen:=map[string]bool{}
	for _,path:=range paths{
		info,e:=os.Lstat(path)
		if os.IsNotExist(e){continue}
		if e!=nil{return snapshot,e}
		if info.Mode()&os.ModeSymlink!=0{return snapshot,fmt.Errorf("protected scope is a symlink: %s",path)}
		if !info.IsDir(){
			item,e:=hashProtectedFile(project,path)
			if e!=nil{return snapshot,e}
			if !seen[item.Path]{snapshot.Items=append(snapshot.Items,item);seen[item.Path]=true}
			continue
		}
		e=filepath.WalkDir(path,func(filename string,entry fs.DirEntry,walkErr error)error{
			if walkErr!=nil{return walkErr}
			if entry.Type()&os.ModeSymlink!=0{return fmt.Errorf("protected symlink requires explicit review: %s",filename)}
			if entry.IsDir(){return nil}
			item,e:=hashProtectedFile(project,filename)
			if e!=nil{return e}
			if !seen[item.Path]{snapshot.Items=append(snapshot.Items,item);seen[item.Path]=true}
			return nil
		})
		if e!=nil{return snapshot,e}
	}
	sort.Slice(snapshot.Items,func(i,j int)bool{return snapshot.Items[i].Path<snapshot.Items[j].Path})
	raw,_:=json.Marshal(struct{Project string;Scope []string;Items []protectedItem}{project,snapshot.Scope,snapshot.Items})
	snapshot.Digest=protectHash(raw)
	return snapshot,nil
}

func hashProtectedFile(project,path string)(protectedItem,error){
	result:=protectedItem{}
	info,err:=os.Lstat(path)
	if err!=nil{return result,err}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink!=0{return result,fmt.Errorf("unsupported protected file type: %s",path)}
	data,err:=os.ReadFile(path)
	if err!=nil{return result,err}
	rel,err:=filepath.Rel(project,path)
	if err!=nil||rel==".."||strings.HasPrefix(rel,".."+string(filepath.Separator)){return result,fmt.Errorf("file outside project: %s",path)}
	category:="config"
	if strings.Contains(filepath.ToSlash(rel),"/data/"){category="project_data"}
	if strings.Contains(filepath.ToSlash(rel),"/backlog"){category="backlog"}
	return protectedItem{Path:filepath.ToSlash(rel),Bytes:int64(len(data)),SHA256:protectHash(data),Category:category},nil
}

func verifyProtection(before,after protectedSnapshot)protectedReport{
	report:=protectedReport{Status:"unknown",Checked:false,Scope:append([]string{},protectedScope...),BeforeFiles:len(before.Items),AfterFiles:len(after.Items),
		Missing:[]string{},Changed:[]string{},Added:[]string{},Unverified:[]string{},ManifestDigest:before.Digest}
	if before.SchemaVersion!=1||after.SchemaVersion!=1||before.Project!=after.Project||before.Digest==""||after.Digest==""{
		report.Unverified=append(report.Unverified,"snapshot_schema_or_project_mismatch")
		return report
	}
	a:=map[string]protectedItem{}
	b:=map[string]protectedItem{}
	for _,item:=range before.Items{a[item.Path]=item}
	for _,item:=range after.Items{b[item.Path]=item}
	for path,old:=range a{
		current,ok:=b[path]
		if !ok{report.Missing=append(report.Missing,path)
		}else if current.SHA256!=old.SHA256||current.Bytes!=old.Bytes{report.Changed=append(report.Changed,path)}
	}
	for path:=range b{if _,exists:=a[path];!exists{report.Added=append(report.Added,path)}}
	sort.Strings(report.Missing);sort.Strings(report.Changed);sort.Strings(report.Added)
	report.Checked=true;report.Status="verified"
	if len(report.Missing)>0||len(report.Changed)>0||len(report.Added)>0{report.Status="changed"}
	return report
}

func runProtectCLI(args []string,out,stderr io.Writer)int{
	if len(args)==0||args[0]=="--help"||args[0]=="help"{
		fmt.Fprintln(out,"task-mecca protect snapshot --project DIR --output FILE --json")
		fmt.Fprintln(out,"task-mecca protect verify --project DIR --snapshot FILE --json")
		fmt.Fprintln(out,"task-mecca protect versions --project DIR --json")
		fmt.Fprintln(out,"Scope: _task_mecca/data/**, _task_mecca/backlog*/**, _task_mecca/config.toml. Unknown/unreadable entries are never called preserved.")
		return 0
	}
	action:=args[0]
	if action!="snapshot"&&action!="verify"&&action!="versions"{fmt.Fprintln(stderr,"unknown protect action");return 2}
	flags:=flag.NewFlagSet("protect "+action,flag.ContinueOnError)
	flags.SetOutput(stderr)
	project:=flags.String("project",".","project path")
	output:=flags.String("output","","required external manifest path for snapshot")
	beforeFile:=flags.String("snapshot","","existing manifest for verify")
	asJSON:=flags.Bool("json",false,"machine-readable result")
	if err:=flags.Parse(args[1:]);err!=nil{return 2}
	if flags.NArg()!=0{return 2}
	abs,err:=filepath.Abs(*project)
	if err!=nil{fmt.Fprintln(stderr,err);return 2}
	if action=="versions"{
		v:=map[string]any{"cli_version":version,"channel":maintenance.CurrentChannel(),"framework_version":"unknown","web_running":false,"web_version":"unknown","web_applied":"unknown"}
		data,e:=os.ReadFile(filepath.Join(abs,"_task_mecca","VERSION"))
		if e==nil{v["framework_version"]=strings.TrimSpace(string(data))}
		state:=webui.ServiceStatus()
		if state.Running{v["web_running"]=true;v["web_version"]=state.Version;v["web_applied"]=state.Project==abs}
		if *asJSON{_ = json.NewEncoder(out).Encode(v)}else{fmt.Fprintf(out,"%+v\n",v)}
		return 0
	}
	if action=="snapshot"{
		if *output==""{fmt.Fprintln(stderr,"snapshot requires --output outside protected project-owned data");return 2}
		folder,err:=filepath.Abs(*output)
		if err!=nil{fmt.Fprintln(stderr,err);return 2}
		protectedRoot:=filepath.Join(abs,"_task_mecca")
		if relative,e:=filepath.Rel(protectedRoot,folder);e==nil&&(relative=="."||relative!=""&&!strings.HasPrefix(relative,"..")){fmt.Fprintln(stderr,"manifest path must not be under protected _task_mecca");return 2}
		snapshot,err:=captureProjectProtection(abs)
		if err!=nil{fmt.Fprintln(stderr,err);return 3}
		if err:=writeProtectionSnapshot(folder,snapshot);err!=nil{fmt.Fprintln(stderr,err);return 3}
		if *asJSON{_ = json.NewEncoder(out).Encode(map[string]any{"status":"captured","manifest":folder,"count":len(snapshot.Items),"digest":snapshot.Digest,"scope":snapshot.Scope})}else{fmt.Fprintf(out,"Protected snapshot captured: %d files; %s\n",len(snapshot.Items),folder)}
		return 0
	}
	if *beforeFile==""{fmt.Fprintln(stderr,"verify requires --snapshot FILE");return 2}
	original,err:=loadProtectionSnapshot(*beforeFile)
	if err!=nil{fmt.Fprintln(stderr,err);return 3}
	current,err:=captureProjectProtection(abs)
	if err!=nil{
		report:=protectedReport{Status:"unknown",Checked:false,Scope:protectedScope,ManifestDigest:original.Digest,Unverified:[]string{err.Error()}}
		if *asJSON{_ = json.NewEncoder(out).Encode(report)}else{fmt.Fprintln(stderr,err)}
		return 3
	}
	report:=verifyProtection(original,current)
	if *asJSON{_ = json.NewEncoder(out).Encode(report)}else{fmt.Fprintf(out,"%+v\n",report)}
	if report.Status!="verified"{return 3}
	return 0
}

func writeProtectionSnapshot(path string,s protectedSnapshot)error{
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return err}
	body,err:=json.MarshalIndent(s,"","  ");if err!=nil{return err}
	temp,err:=os.CreateTemp(filepath.Dir(path),".protect-*.tmp")
	if err!=nil{return err}
	defer os.Remove(temp.Name())
	if _,err=temp.Write(append(body,'\n'));err==nil{err=temp.Sync()}
	if closeErr:=temp.Close();err==nil{err=closeErr}
	if err!=nil{return err}
	if err=os.Link(temp.Name(),path);err!=nil{return fmt.Errorf("manifest already exists or cannot be safely published: %w",err)}
	return nil
}
func loadProtectionSnapshot(path string)(protectedSnapshot,error){
	var before protectedSnapshot
	body,err:=os.ReadFile(path)
	if err!=nil{return before,err}
	dec:=json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err:=dec.Decode(&before);err!=nil{return before,err}
	var trailing any
	if err:=dec.Decode(&trailing);err!=io.EOF{return before,errors.New("invalid trailing snapshot content")}
	if before.SchemaVersion!=1||before.Digest==""{return before,errors.New("invalid protected snapshot schema")}
	raw,_:=json.Marshal(struct{Project string;Scope []string;Items []protectedItem}{before.Project,before.Scope,before.Items})
	if protectHash(raw)!=before.Digest{return before,errors.New("protected manifest digest mismatch")}
	if !equalStrings(before.Scope,protectedScope){return before,errors.New("unsupported protected snapshot scope")}
	for _,item:=range before.Items {
		p:=filepath.FromSlash(item.Path)
		if filepath.IsAbs(p)||p==".."||strings.HasPrefix(p,".."+string(filepath.Separator))||item.SHA256==""{
			return before,errors.New("invalid protected manifest item path or SHA")
		}
	}
	return before,nil
}
func equalStrings(a,b []string)bool{if len(a)!=len(b){return false};for i:=range a{if a[i]!=b[i]{return false}};return true}
