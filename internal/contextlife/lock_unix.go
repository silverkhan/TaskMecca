//go:build darwin || linux || freebsd || openbsd || netbsd

package contextlife

import (
 "os"
 "path/filepath"
 "syscall"

 "github.com/silverkhan/TaskMecca/internal/projectguard"
)
func acquire(project string)(func(),error){
 guard,err:=projectguard.AcquireWrite(project);if err!=nil{return nil,err}
 path:=filepath.Join(project,"_task_mecca",".runtime","context_lifecycle","owner.lock")
 if err=os.MkdirAll(filepath.Dir(path),0700);err!=nil{guard();return nil,err}
 file,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600);if err!=nil{guard();return nil,err}
 if err=syscall.Flock(int(file.Fd()),syscall.LOCK_EX);err!=nil{file.Close();guard();return nil,err}
 return func(){_ = syscall.Flock(int(file.Fd()),syscall.LOCK_UN);_ = file.Close();guard()},nil
}
func replaceFile(temp,dest string)error {return os.Rename(temp,dest)}
