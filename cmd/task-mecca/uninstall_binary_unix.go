//go:build !windows

package main

import "os"

// POSIX permits unlinking the executable inode while this short-lived
// CLI process is still running. The Web process has already been stopped.
func removeInstalledBinary(path string) (string,error) {
    if err:=os.Remove(path);err!=nil{return "",err}
    return "removed",nil
}
