package rolepolicy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type snapshotLimits struct {
	entries, files, depth int
	fileBytes, totalBytes int64
}

var defaultSnapshotLimits = snapshotLimits{50000, 20000, 64, 32 << 20, 256 << 20}

func validateSnapshotRelative(path string) error {
	local := filepath.FromSlash(path)
	if !filepath.IsLocal(local) || filepath.ToSlash(filepath.Clean(local)) != path {
		return errors.New("source_path_invalid")
	}
	return nil
}

// Snapshot includes every regular file under the literal approved scope,
// including ignored generated/embedded build inputs, plus indexed deletions.
// It never walks the workspace root or unrelated artifacts to discover scope.
// Unsafe or oversized scopes fail closed instead of omitting material files.
func Snapshot(workspace string, scope []string) (Source, error) {
	return snapshotWithLimits(workspace, scope, defaultSnapshotLimits)
}

func snapshotWithLimits(workspace string, scope []string, limits snapshotLimits) (Source, error) {
	s := Source{Files: map[string]string{}}
	if !filepath.IsAbs(workspace) || !validScope(scope) || len(scope) > limits.entries {
		return s, errors.New("exact_workspace_and_bounded_scope_required")
	}
	root, e := filepath.EvalSymlinks(workspace)
	if e != nil {
		return s, e
	}
	// Git is used only for indexed paths/deletions; ignored/untracked files are
	// discovered by bounded, no-follow traversal of the declared scope itself.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	args := append([]string{"--literal-pathspecs", "-C", root, "ls-files", "-z", "--cached", "--"}, scope...)
	cmd := exec.CommandContext(ctx, "git", args...)
	output := &boundedSnapshotOutput{limit: 8 << 20}
	cmd.Stdout = output
	if e = cmd.Run(); e != nil {
		return s, e
	}
	paths := map[string]bool{}
	for _, p := range strings.Split(output.String(), "\x00") {
		if p == "" {
			continue
		}
		if e := validateSnapshotRelative(p); e != nil {
			return s, e
		}
		if !Covers(scope, p) {
			return s, errors.New("source_outside_scope")
		}
		paths[p] = true
		if len(paths) > limits.files {
			return s, errors.New("source_scope_file_limit")
		}
	}
	entries, files := 0, 0
	var total int64
	unsafeBoundary := func(path string) bool {
		for _, part := range strings.Split(path, "/") {
			if part == ".git" || part == ".artifacts" {
				return true
			}
		}
		return false
	}
	add := func(path string) error {
		if _, ok := s.Files[path]; ok {
			return nil
		}
		if unsafeBoundary(path) {
			return errors.New("source_scope_artifact_boundary: use narrower explicit source paths")
		}
		f, e := openSnapshotPath(root, path)
		if e != nil {
			if os.IsNotExist(e) && paths[path] {
				s.Files[path] = "absent"
				return nil
			}
			return e
		}
		defer f.Close()
		info, e := f.Stat()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("source_symlink_or_nonregular")
		}
		files++
		if files > limits.files || info.Size() > limits.fileBytes || total+info.Size() > limits.totalBytes {
			return errors.New("source_scope_size_limit")
		}
		h := sha256.New()
		n, e := io.Copy(h, io.LimitReader(f, limits.fileBytes+1))
		if e != nil {
			return e
		}
		total += n
		if n > limits.fileBytes || total > limits.totalBytes {
			return errors.New("source_scope_size_limit")
		}
		after, e := f.Stat()
		if e != nil {
			return e
		}
		if n != info.Size() || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return errors.New("source_changed_during_hash")
		}
		s.Files[path] = hex.EncodeToString(h.Sum(nil))
		return nil
	}
	var walk func(string, int) error
	walk = func(path string, depth int) error {
		entries++
		if entries > limits.entries || depth > limits.depth {
			return errors.New("source_scope_traversal_limit")
		}
		if unsafeBoundary(path) {
			return errors.New("source_scope_artifact_boundary: use narrower explicit source paths")
		}
		f, e := openSnapshotPath(root, path)
		if e != nil {
			if os.IsNotExist(e) && paths[path] {
				return add(path)
			}
			return e
		}
		info, e := f.Stat()
		if e != nil {
			f.Close()
			return e
		}
		if !info.IsDir() {
			f.Close()
			return add(path)
		}
		defer f.Close()
		for {
			batch, e := f.ReadDir(128)
			if e != nil && e != io.EOF {
				return e
			}
			for _, entry := range batch {
				if e := walk(path+"/"+entry.Name(), depth+1); e != nil {
					return e
				}
			}
			if e == io.EOF {
				return nil
			}
		}
	}
	roots := append([]string(nil), scope...)
	sort.Strings(roots)
	var covered []string
	for _, path := range roots {
		if Covers(covered, path) {
			continue
		}
		covered = append(covered, path)
		if e = walk(path, 0); e != nil {
			// Entire indexed directories may be deleted. Preserve each deleted file.
			if !os.IsNotExist(e) {
				return s, e
			}
			found := false
			for p := range paths {
				if Covers([]string{path}, p) {
					found = true
				}
			}
			if !found {
				return s, e
			}
		}
	}
	for path := range paths {
		if _, ok := s.Files[path]; !ok {
			if e = add(path); e != nil {
				return s, e
			}
		}
	}
	if len(s.Files) == 0 {
		return s, errors.New("source_scope_empty")
	}
	return s, nil
}

type boundedSnapshotOutput struct {
	bytes.Buffer
	limit int
}

func (b *boundedSnapshotOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("source_scope_index_limit")
	}
	return b.Buffer.Write(p)
}
