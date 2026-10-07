package main

import (
	"bytes"
	"testing"
)

func TestOperationsCLIRequiresExactRootIncidentAndRejectsUnsupportedActions(t *testing.T) {
	for _, args := range [][]string{nil, {"list"}, {"reconcile"}, {"reconcile", "--project", "relative", "--incident", "ops-1"}, {"reconcile", "--project", t.TempDir(), "--incident", "ops-1"}, {"reconcile", "--project", t.TempDir(), "--incident", "ops-1", "extra"}} {
		var out, errors bytes.Buffer
		if code := runOperationsCLI(args, &out, &errors); code != 2 {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
}
