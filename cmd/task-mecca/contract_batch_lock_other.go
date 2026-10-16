//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !windows

package main

import "errors"

func lockBatchRegistration(string)(func(),error){
	return nil,errors.New("batch registration requires a supported OS filesystem lock")
}
