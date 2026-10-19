package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type nativeSessionEvidence struct {
	Status string `json:"status"`
	Provider string `json:"provider"`
	SessionID string `json:"session_id"`
	SourceToken string `json:"source_token,omitempty"`
	SessionMeta int `json:"session_meta"`
	TaskStarted int `json:"task_started"`
	TurnContext int `json:"turn_context"`
	LinesRead int `json:"lines_read"`
	MetadataMatched bool `json:"metadata_matched"`
	Reason string `json:"reason,omitempty"`
}

var nativeSessionID=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{7,127}$`)

func codexSessionsRoot()string{
	if value:=strings.TrimSpace(os.Getenv("CODEX_HOME"));value!=""{return filepath.Join(value,"sessions")}
	home,err:=os.UserHomeDir()
	if err!=nil{return ""}
	return filepath.Join(home,".codex","sessions")
}

// Local native JSONL inspection is opt-in by an exact session ID. It reads only
// that session's matching file and only emits event counts, never transcript text.
func inspectCodexSessionEvidence(sessionID string)nativeSessionEvidence{
	result:=nativeSessionEvidence{Provider:"codex",SessionID:sessionID,Status:"not_found"}
	if !nativeSessionID.MatchString(sessionID) {
		result.Status="invalid";result.Reason="session ID must be a validated opaque identifier"
		return result
	}
	root:=codexSessionsRoot()
	if root==""{result.Status="unknown";result.Reason="Codex home directory unavailable";return result}
	if info,err:=os.Lstat(root);os.IsNotExist(err){result.Reason="Codex native session directory not found";return result
	}else if err!=nil||!info.IsDir()||info.Mode()&os.ModeSymlink!=0{
		result.Status="unknown";result.Reason="Codex session directory inaccessible or symbolic link";return result
	}
	matches:=[]string{}
	visited:=0
	err:=filepath.WalkDir(root,func(path string,entry fs.DirEntry,err error)error{
		if err!=nil{return err}
		if entry.Type()&os.ModeSymlink!=0{return nil}
		if entry.IsDir(){return nil}
		visited++
		if visited>25000{return errors.New("native session filename scan limit exceeded")}
		if strings.HasSuffix(strings.ToLower(entry.Name()),strings.ToLower(sessionID)+".jsonl"){
			matches=append(matches,path)
		}
		return nil
	})
	if err!=nil{result.Status="unknown";result.Reason="Native filename enumeration incomplete: "+err.Error();return result}
	if len(matches)==0{return result}
	if len(matches)>1{result.Status="ambiguous";result.Reason=fmt.Sprintf("exact session ID matched %d files; no file was read",len(matches));return result}
	path:=matches[0]
	info,err:=os.Lstat(path)
	if err!=nil||!info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{
		result.Status="unknown";result.Reason="native JSONL path not a regular file";return result
	}
	file,err:=os.Open(path)
	if err!=nil{result.Status="unknown";result.Reason="native session file inaccessible";return result}
	defer file.Close()
	result.SourceToken=hashBatchBytes([]byte(path))[:16]
	scanner:=bufio.NewScanner(file)
	scanner.Buffer(make([]byte,64*1024),4*1024*1024)
	var metadataIDs []string
	for scanner.Scan(){
		result.LinesRead++
		if result.LinesRead>1000000{result.Status="unknown";result.Reason="native log line limit exceeded";return result}
		var row struct{Type string `json:"type"`;Payload json.RawMessage `json:"payload"`}
		if json.Unmarshal(scanner.Bytes(),&row)!=nil{continue}
		var payload struct{Type string `json:"type"`;ID string `json:"id"`}
		_ = json.Unmarshal(row.Payload,&payload)
		switch{
		case row.Type=="session_meta":
			result.SessionMeta++
			if payload.ID!=""{metadataIDs=append(metadataIDs,payload.ID)}
		case row.Type=="turn_context"||payload.Type=="turn_context":
			result.TurnContext++
		case row.Type=="task_started"||payload.Type=="task_started":
			result.TaskStarted++
		}
	}
	if err:=scanner.Err();err!=nil{
		result.Status="unknown";result.Reason="native log could not be read completely";return result
	}
	for _,id:=range metadataIDs{
		if id!=sessionID{result.Status="mismatch";result.Reason="native session metadata ID differs from requested session";return result}
	}
	result.MetadataMatched=len(metadataIDs)>0
	result.Status="observed_native"
	if !result.MetadataMatched{result.Status="present_unclassified";result.Reason="native session file found but no matching session_meta ID"}
	return result
}

