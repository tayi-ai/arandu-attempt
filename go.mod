module github.com/tayi-ai/arandu-attempt

go 1.26.0

// v0.1.1 does not compile. It was tagged on a commit that carried half of one
// change: sandbox.go gained the output normaliser whilst runner.go kept the one
// it replaced, so the package has `elapsed` declared twice and the build stops
// at the redeclaration.
//
// A published version cannot be corrected -- the proxy had already resolved and
// cached it, and re-tagging would hand every consumer a checksum mismatch, which
// reads as an attack rather than as a mistake. So it is retracted and v0.1.2
// carries the whole change.
retract v0.1.1

require (
	github.com/arandu-io/framework v0.46.4
	github.com/arandu-io/hesape v0.37.0
)

require (
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)
