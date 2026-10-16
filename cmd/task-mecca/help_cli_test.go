package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpCLIIsSideEffectFreeAndUseful(t *testing.T) {
	cases:=[]struct{topic,expected string}{
		{"","contract"},
		{"contract","source_text"},
		{"runtime","--agent-id"},
		{"upgrade","binary"},
		{"migrate","framework"},
		{"update","task-mecca upgrade"},
		{"version","task-mecca --version"},
	}
	for _,tc:=range cases {
		var out,errs bytes.Buffer
		args:=[]string{}
		if tc.topic!=""{args=[]string{tc.topic}}
		if code:=runHelpCLI(args,&out,&errs);code!=0{t.Errorf("%s code=%d: %s",tc.topic,code,errs.String())}
		if !strings.Contains(out.String(),tc.expected){t.Errorf("%s missing %q",tc.topic,tc.expected)}
	}
}

func TestHelpCLIUnknownTopicIsNotSuccess(t *testing.T){
	var out,errs bytes.Buffer
	if code:=runHelpCLI([]string{"not-a-command"},&out,&errs);code!=2{t.Errorf("code=%d",code)}
	if !strings.Contains(errs.String(),"Unknown help topic"){t.Errorf("stderr=%s",errs.String())}
}
