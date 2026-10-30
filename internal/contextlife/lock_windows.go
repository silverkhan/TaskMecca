//go:build windows

package contextlife

import (
 "os"
 "path/filepath"

 "golang.org/x/sys/windows"
 "github.com/silverkhan/TaskMecca/internal/projectguard"
)
func acquire(project string)(func(),error){
 guard,err:=projectguard.AcquireWrite(project);if err!=nil{return nil,err}
 path:=filepath.Join(project,"_task_mecca",".runtime","context_lifecycle","owner.lock")
 if err=os.MkdirAll(filepath.Dir(path),0700);err!=nil{guard();return nil,err}
 file,err:=os.OpenFile(path,os.O_CREATE|os.O_RDWR,0600);if err!=nil{guard();return nil,err}
 o:=new(windows.Overlapped);handle:=windows.Handle(file.Fd())
 if err=windows.LockFileEx(handle,windows.LOCKFILE_EXCLUSIVE_LOCK,0,1,0,o);err!=nil{file.Close();guard();return nil,err}
 return func(){_ = windows.UnlockFileEx(handle,0,1,0,o);_ = file.Close();guard()},nil
}
func replaceFile(temp,dest string)error {
 old,e:=windows.UTF16PtrFromString(temp);if e!=nil{return e}
 target,e:=windows.UTF16PtrFromString(dest);if e!=nil{return e}
 return windows.MoveFileEx(old,target,windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
