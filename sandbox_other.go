//go:build !unix

package attempt

import (
	"errors"
	"os/exec"
)

// detach is a no-op where process groups are not what this module relies on.
func detach(c *exec.Cmd) {}

// killGroup refuses rather than killing only the test process. A timeout that
// leaves the compiled test binary running is worse than one that says it did
// nothing: the attempt is recorded as timed out whilst the work continues, and
// the next attempt shares the machine with it.
func killGroup(c *exec.Cmd) error {
	return errors.New("attempt: bounding a test run needs process groups, which this platform does not provide; run attempts on a unix host")
}
