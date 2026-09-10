//go:build unix

package attempt

import (
	"errors"
	"os/exec"
	"syscall"
)

// detach puts the test process in its own process group.
//
// `go test` is a launcher: it compiles a binary and runs it as a child of its
// own, and a test is free to spawn more. Killing the process the runner started
// leaves every one of those behind, still holding whatever they took -- a port,
// a temporary directory, a card. The group is what makes them reachable.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup signals the whole group the test process leads.
//
// The negative pid is the point, and this fleet has already paid for learning
// it once: a cleanup that signalled a launcher alone left the workers running
// and the cards occupied, and the next run found them taken.
func killGroup(c *exec.Cmd) error {
	if c == nil || c.Process == nil {
		return errors.New("attempt: there is no process to signal")
	}
	return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
}
