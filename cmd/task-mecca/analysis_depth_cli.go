package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/silverkhan/TaskMecca/internal/analysisdepth"
)

func runAnalysisDepthCLI(path, task string, out, stderr io.Writer) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		fmt.Fprintln(stderr, "analysis input must be a regular JSON file of at most 2 MiB")
		return 2
	}
	d := json.NewDecoder(io.LimitReader(f, 2<<20))
	d.DisallowUnknownFields()
	var in analysisdepth.Input
	if err = d.Decode(&in); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var trailing any
	if err = d.Decode(&trailing); err != io.EOF {
		fmt.Fprintln(stderr, "analysis input must contain exactly one JSON object")
		return 2
	}
	if in.Current.Task != task {
		fmt.Fprintln(stderr, "analysis task must exactly match --task")
		return 2
	}
	if err = json.NewEncoder(out).Encode(analysisdepth.Evaluate(in)); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
