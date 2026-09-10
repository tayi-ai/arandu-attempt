package attempt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
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
	// sealed is what the reserved subtrees hashed to before the attempt started.
	//
	// Confine is a boundary for the tools and not for the attempt. RunTests
	// executes `go test`, which compiles and runs code the policy wrote, as this
	// user, with this user's access to the filesystem -- so the reserved list,
	// the allowed-files list and every path check are invisible to it. A policy
	// that wants to rewrite the test that judges it does not need a tool call.
	//
	// This is the part that can be checked rather than prevented: what the
	// reserved subtrees contained before, compared with what they contain after.
	// It does not stop the write. It stops the reward.
	sealed map[string]string
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
	if err := s.seal(); err != nil {
		return nil, err
	}
	return s, nil
}

// ErrReservedChanged is returned when a reserved subtree is not what it was.
//
// It is separate from ErrReserved because they describe different events: one
// is a tool call refused at the boundary, the other is a boundary that was
// bypassed and only shows up afterwards.
var ErrReservedChanged = errors.New("attempt: a reserved path changed during the attempt")

// seal records what each reserved subtree contains, before anything runs.
func (s *Sandbox) seal() error {
	s.sealed = map[string]string{}
	for _, prefix := range s.reserved {
		sum, err := s.digestReserved(prefix)
		if err != nil {
			return err
		}
		s.sealed[prefix] = sum
	}
	return nil
}

// VerifySeal reports whether every reserved subtree is still what it was.
//
// A reserved path that is absent stays absent and a reserved path that existed
// keeps its contents; either changing is the same finding. Deleting the
// evaluation is not a smaller offence than rewriting it.
func (s *Sandbox) VerifySeal() error {
	for _, prefix := range s.reserved {
		sum, err := s.digestReserved(prefix)
		if err != nil {
			return err
		}
		if sum != s.sealed[prefix] {
			return fmt.Errorf("%w: %s", ErrReservedChanged, prefix)
		}
	}
	return nil
}

// digestReserved hashes one reserved subtree: every path under it, in sorted
// order, with its contents.
//
// The path goes into the hash beside the contents, so a file renamed within
// the subtree is a change even when the bytes are the same. Symlinks are
// hashed by their target and never followed -- following one would hash
// whatever it points at, which is the thing a symlink is used to hide.
//
// An absent subtree hashes to the empty string, which is what makes "absent
// before and absent after" agree without a special case.
func (s *Sandbox) digestReserved(prefix string) (string, error) {
	root := filepath.Join(s.root, filepath.FromSlash(prefix))
	if _, err := os.Lstat(root); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	sum := sha256.New()
	walked := []string{}
	contents := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			walked = append(walked, "l "+rel)
			contents[rel] = []byte(target)
		case d.IsDir():
			walked = append(walked, "d "+rel)
		default:
			body, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			walked = append(walked, "f "+rel)
			contents[rel] = body
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(walked)
	for _, entry := range walked {
		sum.Write([]byte(entry))
		sum.Write([]byte{0})
		if body, ok := contents[entry[2:]]; ok {
			sum.Write(body)
		}
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
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

	// `go test` compiles a binary and runs it as a child of its own, and the test
	// may spawn more. CommandContext kills the process it started and nothing
	// below it, so a timeout used to end the attempt whilst the test binary kept
	// running -- recorded as bounded, actually still going, sharing the machine
	// with whatever ran next. The group is what reaches them.
	detach(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }
	// The group is signalled, then the pipes are given a moment to drain before
	// Wait gives up on them. Without a delay, a grandchild holding the write end
	// open makes Wait block past the timeout it was supposed to enforce.
	cmd.WaitDelay = 5 * time.Second

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	started := time.Now()
	err := cmd.Run()
	result := TestResult{Package: pkg, Output: s.stable(out.String()), Elapsed: time.Since(started)}
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

// Everything in a test run's output that differs between two runs of the same
// sandbox, so that a replay can be compared against a record byte for byte.
//
// Each of these was measured on 2026-09-10 by running the same panicking test
// twice and diffing, rather than guessed:
//
//   - durations, which the go command prints for every package ("0.350s"
//     against "0.133s"), and "(cached)" when it skips the run entirely
//   - addresses in a panic's stack trace, which move with ASLR on every
//     execution: `panic({0x102a278d8?, 0x1029fc3c0?})` against
//     `panic({0x100e9b8d8?, 0x100e703c0?})`
//   - goroutine numbers, which depend on how many the runtime started first
//   - the sandbox root, a fresh temporary directory per attempt, which a stack
//     trace carries as the absolute path the binary was compiled from
//
// A compile error needs none of this: the go command prints those relative to
// its working directory, which is the root.
//
// Left unnormalised, a replay's digest differs from the record's for reasons
// that have nothing to do with what the policy did, and Verify reports a
// divergence that is its own -- an accusation against a policy that did exactly
// what it was recorded doing.
var (
	elapsed    = regexp.MustCompile(`\b\d+\.\d+s\b|\(cached\)`)
	address    = regexp.MustCompile(`0x[0-9a-f]{4,}`)
	goroutines = regexp.MustCompile(`\bgoroutine \d+\b`)
)

func (s *Sandbox) stable(output string) string {
	if s.root != "" {
		output = strings.ReplaceAll(output, s.root, "<sandbox>")
	}
	output = elapsed.ReplaceAllString(output, "<elapsed>")
	output = address.ReplaceAllString(output, "0x<addr>")
	return goroutines.ReplaceAllString(output, "goroutine <n>")
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
