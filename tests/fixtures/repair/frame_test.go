package fixture

import "testing"

func TestClientFramesMustBeMasked(t *testing.T) {
	if err := Accept(true, true); err != nil {
		t.Fatalf("a masked client frame was refused: %v", err)
	}
	if err := Accept(true, false); err == nil {
		t.Fatal("an unmasked client frame was accepted")
	}
	if err := Accept(false, false); err != nil {
		t.Fatalf("an unmasked server frame was refused: %v", err)
	}
}
