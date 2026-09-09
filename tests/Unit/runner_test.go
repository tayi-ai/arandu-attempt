package unit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	attempt "github.com/tayi-ai/arandu-attempt"
)

// The fixture is a miniature of a training repair: a Go package whose test
// fails until one condition is inverted back. Every test below copies it into a
// fresh sandbox through the runner, so the fixture itself is never written.
func fixtureTask(t *testing.T) attempt.Task {
	t.Helper()
	snapshot, err := filepath.Abs(filepath.Join("..", "fixtures", "repair"))
	if err != nil {
		t.Fatal(err)
	}
	return attempt.Task{
		ID:           "repair-fixture-mask",
		Repository:   "fixture",
		Revision:     "none",
		Prompt:       "Client frames without a mask are being accepted and masked ones refused. Fix Accept.",
		AllowedFiles: []string{"frame.go"},
		TestPackage:  ".",
		Snapshot:     snapshot,
	}
}

func runner(t *testing.T) attempt.Runner {
	t.Helper()
	return attempt.Runner{Parent: t.TempDir(), Reserved: []string{"evaluation", "heldout"}, TestTimeout: 90 * time.Second}
}

func repairedSource(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "fixtures", "repair", "frame.go"))
	if err != nil {
		t.Fatal(err)
	}
	fixed := strings.Replace(string(src), "if fromClient && masked {", "if fromClient && !masked {", 1)
	if fixed == string(src) {
		t.Fatal("the fixture no longer carries the defect this test repairs")
	}
	return fixed
}

var budget = attempt.Budget{MaxToolCalls: 8, MaxTestRuns: 3}

func TestDefectiveSnapshotEarnsNothingWithoutAWrite(t *testing.T) {
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), budget, "p0",
		attempt.Scripted(nil, "nothing to do", nil))
	if err != nil {
		t.Fatal(err)
	}
	if traj.Reward.Score != 0 || traj.Reward.TestsPassed {
		t.Fatalf("an untouched defective snapshot was rewarded: %+v", traj.Reward)
	}
	if !strings.Contains(traj.Reward.Reason, "final tests failed") {
		t.Fatalf("reason does not name the failing tests: %q", traj.Reward.Reason)
	}
}

func TestRepairWithBackedClaimsIsRewarded(t *testing.T) {
	calls := []attempt.ToolCall{
		{Tool: attempt.ToolReadFile, Args: map[string]any{"path": "frame.go"}},
		{Tool: attempt.ToolWriteFile, Args: map[string]any{"path": "frame.go", "content": repairedSource(t)}},
		{Tool: attempt.ToolRunTests, Args: map[string]any{"package": "."}},
	}
	claims := []attempt.Claim{
		{Kind: attempt.ClaimFileExists, Target: "frame.go", EvidenceRef: 0},
		{Kind: attempt.ClaimTestsPass, EvidenceRef: 2},
	}
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), budget, "p1", attempt.Scripted(calls, "inverted the mask check", claims))
	if err != nil {
		t.Fatal(err)
	}
	if traj.Reward.Score != 1 || !traj.Reward.TestsPassed || traj.Reward.FalseClaims != 0 {
		t.Fatalf("a correct, honest repair was not rewarded: %+v", traj.Reward)
	}
	if len(traj.Steps) != 3 || traj.Steps[2].Observation.IsError {
		t.Fatalf("unexpected record: %d steps, last observation %+v", len(traj.Steps), traj.Steps[len(traj.Steps)-1].Observation)
	}
}

func TestFalseClaimZeroesRewardEvenWhenTestsPass(t *testing.T) {
	calls := []attempt.ToolCall{
		{Tool: attempt.ToolWriteFile, Args: map[string]any{"path": "frame.go", "content": repairedSource(t)}},
	}
	// The claim cites the write as evidence that the tests pass. Nothing ran.
	claims := []attempt.Claim{{Kind: attempt.ClaimTestsPass, EvidenceRef: 0}}
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), budget, "p2", attempt.Scripted(calls, "tests pass now", claims))
	if err != nil {
		t.Fatal(err)
	}
	if !traj.Reward.TestsPassed {
		t.Fatalf("the repair should pass the final tests: %+v", traj.Reward)
	}
	if traj.Reward.Score != 0 || traj.Reward.FalseClaims != 1 {
		t.Fatalf("a claim without evidence kept its reward: %+v", traj.Reward)
	}
}

func TestReservedPathIsRefusedAndEndsTheAttempt(t *testing.T) {
	calls := []attempt.ToolCall{
		{Tool: attempt.ToolReadFile, Args: map[string]any{"path": "evaluation/answer.txt"}},
		{Tool: attempt.ToolWriteFile, Args: map[string]any{"path": "frame.go", "content": repairedSource(t)}},
	}
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), budget, "p3", attempt.Scripted(calls, "", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(traj.Steps) != 1 || !traj.Steps[0].Observation.Refused {
		t.Fatalf("the reserved read was not refused first: %+v", traj.Steps)
	}
	if !traj.Reward.Refused || traj.Reward.Score != 0 {
		t.Fatalf("a refused attempt kept a reward: %+v", traj.Reward)
	}
}

func TestSandboxRefusesEscapesAndUnlistedWrites(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link.txt")); err != nil {
		t.Skip("symlinks unavailable")
	}
	box, err := attempt.NewSandbox(root, []string{"a.txt"}, []string{"heldout"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../x", "/etc/passwd", "a/../../x", "link.txt", "heldout/h.txt"} {
		if _, err := box.Read(p); err == nil {
			t.Errorf("read of %q was allowed", p)
		}
	}
	if err := box.Write("b.txt", []byte("x")); err == nil {
		t.Error("write outside allowed files was allowed")
	}
	if err := box.Write("a.txt", []byte("changed")); err != nil {
		t.Errorf("write to an allowed file was refused: %v", err)
	}
}

func TestUnknownToolIsRefused(t *testing.T) {
	calls := []attempt.ToolCall{{Tool: "shell", Args: map[string]any{"cmd": "rm -rf /"}}}
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), budget, "p4", attempt.Scripted(calls, "", nil))
	if err != nil {
		t.Fatal(err)
	}
	if !traj.Reward.Refused || len(traj.Steps) != 1 || !traj.Steps[0].Observation.Refused {
		t.Fatalf("an unknown tool was not refused: %+v", traj)
	}
}

func TestBudgetEndsTheAttemptWithoutReward(t *testing.T) {
	calls := []attempt.ToolCall{
		{Tool: attempt.ToolListTree, Args: map[string]any{"path": "."}},
		{Tool: attempt.ToolListTree, Args: map[string]any{"path": "."}},
	}
	traj, err := runner(t).Run(context.Background(), fixtureTask(t), attempt.Budget{MaxToolCalls: 1}, "p5", attempt.Scripted(calls, "", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(traj.Steps) != 1 || traj.Reward.Score != 0 || !strings.HasPrefix(traj.Reward.Reason, "budget") {
		t.Fatalf("budget was not enforced: %+v", traj)
	}
}

func TestReplayVerifiesTheRecordAndCatchesTampering(t *testing.T) {
	calls := []attempt.ToolCall{
		{Tool: attempt.ToolWriteFile, Args: map[string]any{"path": "frame.go", "content": repairedSource(t)}},
		{Tool: attempt.ToolRunTests, Args: map[string]any{"package": "."}},
	}
	claims := []attempt.Claim{{Kind: attempt.ClaimTestsPass, EvidenceRef: 1}}
	r := runner(t)
	recorded, err := r.Run(context.Background(), fixtureTask(t), budget, "p6", attempt.Scripted(calls, "fixed", claims))
	if err != nil {
		t.Fatal(err)
	}
	if recorded.Reward.Score != 1 {
		t.Fatalf("setup: repair not rewarded: %+v", recorded.Reward)
	}
	if err := r.Verify(context.Background(), fixtureTask(t), budget, recorded); err != nil {
		t.Fatalf("a faithful record failed replay: %v", err)
	}
	tampered := recorded
	tampered.Reward.FalseClaims = 0
	tampered.Claims = append([]attempt.Claim{}, recorded.Claims...)
	tampered.Claims = append(tampered.Claims, attempt.Claim{Kind: attempt.ClaimFileExists, Target: "ghost.go", EvidenceRef: 0})
	if err := r.Verify(context.Background(), fixtureTask(t), budget, tampered); err == nil {
		t.Fatal("a tampered record passed replay")
	}
}

func TestDigestIsStableAcrossArgumentOrder(t *testing.T) {
	a := attempt.Trajectory{TaskID: "t", Steps: []attempt.Step{{Call: attempt.ToolCall{Tool: attempt.ToolWriteFile, Args: map[string]any{"path": "x", "content": "y"}}}}}
	b := attempt.Trajectory{TaskID: "t", Steps: []attempt.Step{{Call: attempt.ToolCall{Tool: attempt.ToolWriteFile, Args: map[string]any{"content": "y", "path": "x"}}}}}
	da, err := a.Digest()
	if err != nil {
		t.Fatal(err)
	}
	db, err := b.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if da != db {
		t.Fatal("digest depends on map iteration order")
	}
}
