package attempt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// The tools an attempt may call. They are constants so a trajectory can be
// replayed by name and a policy can be listed by reading the source.
const (
	ToolReadFile  = "read_file"
	ToolWriteFile = "write_file"
	ToolListTree  = "list_tree"
	ToolRunTests  = "run_tests"
)

// AllTools is every tool the runner knows, sorted.
func AllTools() []string {
	return []string{ToolListTree, ToolReadFile, ToolRunTests, ToolWriteFile}
}

// Task is what an attempt is asked to do.
//
// It mirrors the repair task of the training release: a repository at a pinned
// revision, a prompt, and the files the fix may touch. The snapshot is the
// directory the task's revision was materialized into; the runner copies it and
// never writes the original.
type Task struct {
	ID           string   `json:"id"`
	Repository   string   `json:"repository"`
	Revision     string   `json:"revision"`
	Prompt       string   `json:"prompt"`
	AllowedFiles []string `json:"allowed_files"`
	// TestPackage is the package whose tests decide the outcome, relative to
	// the snapshot root. "." is the module root.
	TestPackage string `json:"test_package"`
	Snapshot    string `json:"snapshot"`
}

// Budget bounds one attempt.
type Budget struct {
	MaxToolCalls int `json:"max_tool_calls"`
	MaxTestRuns  int `json:"max_test_runs"`
}

// ToolCall is one call the policy asked for.
type ToolCall struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// Observation is what the runner answered.
//
// Refused is set when the deterministic policy stopped the call before it ran:
// a reserved path, a path outside the sandbox, a tool not on the list, a write
// outside the allowed files. A refusal ends the attempt with zero reward.
type Observation struct {
	Index   int    `json:"index"`
	Tool    string `json:"tool"`
	Output  string `json:"output"`
	IsError bool   `json:"is_error"`
	Refused bool   `json:"refused"`
}

// Claim is an assertion the policy made about the state of the sandbox.
//
// Every claim names the observation that supports it. A claim whose evidence
// does not say what the claim says is a false claim, and a false claim costs
// more than saying nothing: the reward for a trajectory with one is below the
// reward for abstaining, regardless of the tests.
type Claim struct {
	// Kind is one of ClaimFileExists, ClaimTestsPass, ClaimTestsFail.
	Kind string `json:"kind"`
	// Target is the path for a file claim; empty for a test claim.
	Target string `json:"target,omitempty"`
	// EvidenceRef is the index of the observation that supports the claim.
	EvidenceRef int `json:"evidence_ref"`
}

const (
	ClaimFileExists = "file_exists"
	ClaimTestsPass  = "tests_pass"
	ClaimTestsFail  = "tests_fail"
)

// Step is one call and its answer.
type Step struct {
	Call        ToolCall    `json:"call"`
	Observation Observation `json:"observation"`
}

// Reward is what the attempt earned and why.
//
// Score is in {0, 1} for now: the tests decide, security is a gate, and a
// false claim zeroes the score even when the tests pass. Cost is recorded so
// that, among rewarded attempts, the cheaper one can be preferred later; it is
// never a reason to reward an attempt that did not deliver.
type Reward struct {
	Score       float64 `json:"score"`
	TestsPassed bool    `json:"tests_passed"`
	Refused     bool    `json:"refused"`
	FalseClaims int     `json:"false_claims"`
	ToolCalls   int     `json:"tool_calls"`
	Reason      string  `json:"reason"`
}

// Trajectory is the full record of one attempt.
type Trajectory struct {
	TaskID        string  `json:"task_id"`
	PolicyVersion string  `json:"policy_version"`
	Steps         []Step  `json:"steps"`
	Claims        []Claim `json:"claims"`
	Final         string  `json:"final"`
	Reward        Reward  `json:"reward"`
}

// Digest is the hash of the trajectory's replayable content: the calls, the
// observations, the claims and the reward. Two runs that agree here agree on
// everything the reward depends on.
func (t Trajectory) Digest() (string, error) {
	// Args are maps; encode with sorted keys so the digest is stable.
	type stableStep struct {
		Tool string
		Args []string
		Obs  Observation
	}
	steps := make([]stableStep, 0, len(t.Steps))
	for _, s := range t.Steps {
		keys := make([]string, 0, len(s.Call.Args))
		for k := range s.Call.Args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		args := make([]string, 0, len(keys))
		for _, k := range keys {
			args = append(args, fmt.Sprintf("%s=%v", k, s.Call.Args[k]))
		}
		steps = append(steps, stableStep{Tool: s.Call.Tool, Args: args, Obs: s.Observation})
	}
	body, err := json.Marshal(struct {
		Task   string
		Steps  []stableStep
		Claims []Claim
		Final  string
		Reward Reward
	}{t.TaskID, steps, t.Claims, t.Final, t.Reward})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
