//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func lockBatchRegistration(project string)(func(),error){
	path:=filepath.Join(project,"_task_mecca","data","cli_batches",".registration.lock")
	if err:=os.MkdirAll(filepath.Dir(path),0700);err!=nil{return nil,err}
	file,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600)
	if err!=nil{return nil,err}
	o:=&windows.Overlapped{}
	if err:=windows.LockFileEx(windows.Handle(file.Fd()),windows.LOCKFILE_EXCLUSIVE_LOCK,0,1,0,o);err!=nil{file.Close();return nil,err}
	return func(){_ = windows.UnlockFileEx(windows.Handle(file.Fd()),0,1,0,o);_ = file.Close()},nil
}
