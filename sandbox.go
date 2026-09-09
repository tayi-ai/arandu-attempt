package attempt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Sandbox is the working copy one attempt is confined to.
//
// It is a directory the runner owns for the life of the attempt, and every
// path a tool receives is resolved against it and checked before anything is
// read or written. The checks are the boundary the trained policy cannot cross:
// a path that climbs out, a symlink that points out, or a reserved prefix is
// refused here, once, for every tool -- so no tool has its own idea of what
// "inside" means.
type Sandbox struct {
	root string
	// allowedWrite lists the relative paths the attempt may change. Empty means
	// nothing may be written, which is the safe default for a read-only probe.
	allowedWrite map[string]bool
	// reserved lists relative prefixes that must never be read or written:
	// the evaluation itself, held-out material, repositories kept out of training.
	reserved []string
}

// ErrOutside is returned for a path that resolves outside the sandbox.
var ErrOutside = errors.New("attempt: path resolves outside the sandbox")

// ErrReserved is returned for a path under a reserved prefix.
var ErrReserved = errors.New("attempt: path is reserved and may not be touched")

// ErrNotWritable is returned for a write to a path the task did not allow.
var ErrNotWritable = errors.New("attempt: path is not in the task's allowed files")

// NewSandbox confines root, which must already exist and be a directory.
func NewSandbox(root string, allowedWrite []string, reserved []string) (*Sandbox, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("attempt: sandbox root %s is not a directory", root)
	}
	s := &Sandbox{root: resolved, allowedWrite: map[string]bool{}}
	for _, p := range allowedWrite {
		s.allowedWrite[path.Clean(filepath.ToSlash(p))] = true
	}
	for _, p := range reserved {
		s.reserved = append(s.reserved, strings.TrimSuffix(path.Clean(filepath.ToSlash(p)), "/"))
	}
	return s, nil
}

// Root is the confined directory.
func (s *Sandbox) Root() string { return s.root }

// Confine turns a tool-supplied relative path into an absolute one inside the
// sandbox, or refuses.
//
// It refuses absolute paths, any component that climbs, any reserved prefix,
// and any existing symlink chain that leaves the root. The last existing
// ancestor is what gets resolved, so a write to a new file under an escaping
// symlinked directory is refused too.
func (s *Sandbox) Confine(rel string) (string, error) {
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) {
		return "", ErrOutside
	}
	clean := path.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrOutside
	}
	for _, r := range s.reserved {
		if clean == r || strings.HasPrefix(clean, r+"/") {
			return "", ErrReserved
		}
	}
	abs := filepath.Join(s.root, filepath.FromSlash(clean))

	// Resolve the deepest ancestor that exists, so a symlink anywhere on the
	// way is followed and compared against the root.
	probe := abs
	for {
		if _, err := os.Lstat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	resolved, err := filepath.EvalSymlinks(probe)
	if err != nil {
		return "", err
	}
	if resolved != s.root && !strings.HasPrefix(resolved, s.root+string(filepath.Separator)) {
		return "", ErrOutside
	}
	return abs, nil
}

// Read returns the content of one file.
func (s *Sandbox) Read(rel string) ([]byte, error) {
	abs, err := s.Confine(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

// List returns the entries under one directory, sorted, directories with a
// trailing slash, without descending.
func (s *Sandbox) List(rel string) ([]string, error) {
	abs, err := s.Confine(rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// Write replaces the content of one file the task allowed.
//
// The allowance is by exact relative path: an attempt is a repair of named
// files, and a policy free to create files anywhere is a policy that writes a
// second test that passes.
func (s *Sandbox) Write(rel string, content []byte) error {
	clean := path.Clean(filepath.ToSlash(rel))
	if !s.allowedWrite[clean] {
		return ErrNotWritable
	}
	abs, err := s.Confine(clean)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, content, 0o644)
}

// TestResult is what one test run reported.
type TestResult struct {
	Package  string
	Passed   bool
	ExitCode int
	Output   string
	Elapsed  time.Duration
}

// RunTests runs `go test` on one package of the sandbox, offline, with the
// workspace off, so the result depends on the sandbox and nothing else.
//
// The package is a relative directory; "." is the module root. A package that
// climbs or is reserved is refused by Confine like any other path.
func (s *Sandbox) RunTests(ctx context.Context, pkg string, timeout time.Duration) (TestResult, error) {
	if pkg == "" {
		pkg = "."
	}
	if _, err := s.Confine(pkg); err != nil {
		return TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./"+filepath.ToSlash(path.Clean(pkg)))
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	started := time.Now()
	err := cmd.Run()
	result := TestResult{Package: pkg, Output: out.String(), Elapsed: time.Since(started)}
	var exit *exec.ExitError
	switch {
	case err == nil:
		result.Passed = true
	case errors.As(err, &exit):
		result.ExitCode = exit.ExitCode()
	default:
		return result, err
	}
	return result, nil
}

// CopyTree copies src into a fresh directory under parent and returns it.
//
// It is how a snapshot becomes a sandbox: the original is never written, and a
// replay gets a copy of the same bytes the first run got. Symlinks are not
// followed and not copied, because a link into the original is a hole in the
// copy.
func CopyTree(src, parent string) (string, error) {
	dst, err := os.MkdirTemp(parent, "attempt-")
	if err != nil {
		return "", err
	}
	src, err = filepath.Abs(src)
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		default:
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
	})
	if err != nil {
		return "", err
	}
	return dst, nil
}
