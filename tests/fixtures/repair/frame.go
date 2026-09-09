// Package fixture is a miniature of the joaju frame reader, with the same
// kind of defect the training repairs fix: a masking check inverted.
package fixture

import "errors"

// ErrUnmaskedClient is returned when a client frame arrives without a mask.
var ErrUnmaskedClient = errors.New("frame: client frames must be masked")

// Accept reports whether a frame from the given side is acceptable.
func Accept(fromClient, masked bool) error {
	if fromClient && masked {
		return ErrUnmaskedClient
	}
	return nil
}
