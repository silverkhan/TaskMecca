//go:build darwin || linux || freebsd || openbsd || netbsd

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

func lockBatchRegistration(project string)(func(),error){
	path:=filepath.Join(project,"_task_mecca","data","cli_batches",".registration.lock")
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
	file,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600)
	if err!=nil{return nil,err}
	if err:=syscall.Flock(int(file.Fd()),syscall.LOCK_EX);err!=nil{file.Close();return nil,err}
	return func(){_ = syscall.Flock(int(file.Fd()),syscall.LOCK_UN);_ = file.Close()},nil
}
