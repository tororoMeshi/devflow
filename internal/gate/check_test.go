package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	artifactpkg "github.com/8noki8/devflow/internal/artifact"
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func TestInspectArtifactsUsesOneReadPerPathAndKeepsNullableMatchesDistinct(t *testing.T) {
	attempt, err := state.NewStepAttempt("step", 1)
	if err != nil {
		t.Fatal(err)
	}
	attempt.ArtifactEvidence["a"] = state.ArtifactEvidence{Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
	attempt.ArtifactEvidence["b"] = state.ArtifactEvidence{Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 1}
	st := state.State{Attempts: []state.StepAttempt{attempt}, CurrentAttemptID: attempt.ID}
	step := flow.Step{ID: "step", Artifacts: []flow.Artifact{
		{Path: "a", Required: true},
		{Path: "b", Required: true},
		{Path: "optional", Required: false},
	}}
	reads := map[string]int{}
	reader := func(_ string, path string) (artifactpkg.FileEvidence, error) {
		reads[path]++
		switch path {
		case "a":
			return artifactpkg.FileEvidence{Digest: attempt.ArtifactEvidence[path].Digest, Size: 1}, nil
		default:
			return artifactpkg.FileEvidence{}, artifactpkg.ErrMissing
		}
	}

	got := inspectArtifacts(step, st, t.TempDir(), reader)

	if !reflect.DeepEqual(reads, map[string]int{"a": 1, "b": 1, "optional": 1}) {
		t.Fatalf("reads = %#v", reads)
	}
	if got[0].MatchesEvidence == nil || !*got[0].MatchesEvidence || got[1].MatchesEvidence == nil || *got[1].MatchesEvidence {
		t.Fatalf("matches = %#v, %#v", got[0].MatchesEvidence, got[1].MatchesEvidence)
	}
	if got[0].MatchesEvidence == got[1].MatchesEvidence {
		t.Fatal("matches pointers are shared")
	}
	if got[2].Evidence != nil || got[2].MatchesEvidence != nil || got[2].Problem != "" {
		t.Fatalf("optional inspection = %#v", got[2])
	}
}

func TestInspectRequiredArtifactsSkipsOptionalFiles(t *testing.T) {
	step := flow.Step{ID: "step", Artifacts: []flow.Artifact{
		{Path: "required", Required: true},
		{Path: "optional", Required: false},
	}}
	got := InspectRequiredArtifacts(step, state.State{}, t.TempDir())
	if len(got) != 1 || got[0].Path != "required" {
		t.Fatalf("inspections = %#v", got)
	}
}

func TestDoneGateReusesArtifactInspectionForSameInputPath(t *testing.T) {
	attempt, err := state.NewStepAttempt("step", 1)
	if err != nil {
		t.Fatal(err)
	}
	st := state.State{Attempts: []state.StepAttempt{attempt}, CurrentAttemptID: attempt.ID}
	step := flow.Step{
		ID:        "step",
		Inputs:    []flow.Artifact{{Path: "shared.md", Required: true}},
		Artifacts: []flow.Artifact{{Path: "shared.md", Required: true}},
	}
	matches := true
	inspections := []ArtifactInspection{{
		Path: "shared.md", Required: true, Exists: true,
		Evidence:        &state.ArtifactEvidence{Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1},
		MatchesEvidence: &matches,
	}}
	got := checkDoneGateFromInspections(step, st, filepath.Join(t.TempDir(), "missing-root"), inspections)
	if !got.OK || len(got.MissingInputs) != 0 || len(got.ArtifactProblems) != 0 {
		t.Fatalf("gate = %#v", got)
	}
}

func TestCheckDoneGate(t *testing.T) {
	tests := []struct {
		name                 string
		step                 flow.Step
		state                state.State
		files                []string
		dirs                 []string
		wantOK               bool
		wantMissingArtifacts []string
		wantMissingApprovals []string
	}{
		{
			name: "passes when no artifacts and no approval are required",
			step: flow.Step{
				ID:        "check_changes",
				Artifacts: []flow.Artifact{},
			},
			state:  state.State{},
			wantOK: true,
		},
		{
			name: "passes when required artifact exists",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
				},
			},
			files:  []string{"docs/code-review.md"},
			wantOK: true,
		},
		{
			name: "fails when required artifact is missing",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
				},
			},
			wantMissingArtifacts: []string{"docs/code-review.md"},
		},
		{
			name: "reports only missing required artifacts",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
					{Path: "docs/review/result.md", Required: true},
				},
			},
			files:                []string{"docs/code-review.md"},
			wantMissingArtifacts: []string{"docs/review/result.md"},
		},
		{
			name: "reports all missing required artifacts",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
					{Path: "docs/review/result.md", Required: true},
				},
			},
			wantMissingArtifacts: []string{"docs/code-review.md", "docs/review/result.md"},
		},
		{
			name: "ignores missing optional artifact",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/optional.md", Required: false},
				},
			},
			wantOK: true,
		},
		{
			name: "treats artifact directory as missing",
			step: flow.Step{
				ID: "write_review",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
				},
			},
			dirs:                 []string{"docs/code-review.md"},
			wantMissingArtifacts: []string{},
		},
		{
			name: "passes when required approval is approved",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state:  approvalGateState(t, "human_approval", &state.ApprovalRecord{Note: "ok", EvidenceSetDigest: "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}),
			wantOK: true,
		},
		{
			name: "fails when required approval is missing",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state:                approvalGateState(t, "human_approval", nil),
			wantMissingApprovals: []string{"human_approval"},
		},
		{
			name: "does not use approval from a past attempt",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state:                approvalGateReentryState(t),
			wantMissingApprovals: []string{"human_approval"},
		},
		{
			name: "does not use approval from another step",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state:                approvalGateState(t, "other_step", &state.ApprovalRecord{Note: "ok", EvidenceSetDigest: "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}),
			wantMissingApprovals: []string{"human_approval"},
		},
		{
			name: "reports missing artifact and approval together",
			step: flow.Step{
				ID: "human_approval",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
				},
				Approval: &flow.Approval{Required: true},
			},
			state:                approvalGateState(t, "human_approval", nil),
			wantMissingArtifacts: []string{"docs/code-review.md"},
			wantMissingApprovals: []string{"human_approval"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			createFiles(t, root, tt.files)
			createDirs(t, root, tt.dirs)
			if len(tt.files) > 0 && tt.state.CurrentAttemptID == "" {
				tt.state = approvalGateState(t, tt.step.ID, nil)
			}
			if tt.state.CurrentAttemptID == "" {
				tt.state = approvalGateState(t, tt.step.ID, nil)
			}
			attempt, index, ok := tt.state.CurrentAttempt()
			if ok {
				for _, requirement := range tt.step.Artifacts {
					if requirement.Required {
						attempt.ArtifactEvidence[requirement.Path] = state.ArtifactEvidence{
							Digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Size:   0,
						}
					}
				}
				for _, file := range tt.files {
					value, err := artifactpkg.ReadFile(root, file)
					if err != nil {
						t.Fatal(err)
					}
					attempt.ArtifactEvidence[file] = state.ArtifactEvidence{Digest: value.Digest, Size: value.Size}
				}
				tt.state.Attempts[index] = attempt
			}

			got := CheckDoneGate(tt.step, tt.state, root)

			assertGateResult(t, got, tt.wantOK, tt.wantMissingArtifacts, tt.wantMissingApprovals)
		})
	}
}

func approvalGateState(t testing.TB, stepID string, approval *state.ApprovalRecord) state.State {
	t.Helper()
	attempt, err := state.NewStepAttempt(stepID, 1)
	if err != nil {
		t.Fatal(err)
	}
	attempt.Approval = approval
	return state.State{Attempts: []state.StepAttempt{attempt}, CurrentAttemptID: attempt.ID}
}

func approvalGateReentryState(t testing.TB) state.State {
	t.Helper()
	first := approvalGateState(t, "human_approval", &state.ApprovalRecord{Note: "old", EvidenceSetDigest: "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}).Attempts[0]
	first, _ = state.CloseStepAttempt(first, state.StepAttemptExitBack, "retry")
	current, _ := state.NewStepAttempt("human_approval", 2)
	return state.State{Attempts: []state.StepAttempt{first, current}, CurrentAttemptID: current.ID}
}

func TestCheckDoneGatePreservesRequiredCheckOrder(t *testing.T) {
	step := flow.Step{ID: "quality", RequiredChecks: []string{"go-test", "go-vet", "gofmt"}}
	for _, tt := range []struct {
		name    string
		results map[string]state.CheckResult
		want    []CheckProblem
		ok      bool
	}{
		{"all missing", nil, []CheckProblem{{"go-test", CheckMissing}, {"go-vet", CheckMissing}, {"gofmt", CheckMissing}}, false},
		{"all failed", map[string]state.CheckResult{"go-test": {ExitCode: 1}, "go-vet": {ExitCode: 1}, "gofmt": {ExitCode: 1}}, []CheckProblem{{"go-test", CheckFailed}, {"go-vet", CheckFailed}, {"gofmt", CheckFailed}}, false},
		{"missing failed missing", map[string]state.CheckResult{"go-vet": {ExitCode: 1}}, []CheckProblem{{"go-test", CheckMissing}, {"go-vet", CheckFailed}, {"gofmt", CheckMissing}}, false},
		{"failed missing failed", map[string]state.CheckResult{"go-test": {ExitCode: 1}, "gofmt": {ExitCode: 1}}, []CheckProblem{{"go-test", CheckFailed}, {"go-vet", CheckMissing}, {"gofmt", CheckFailed}}, false},
		{"all passed", map[string]state.CheckResult{"go-test": {ExitCode: 0}, "go-vet": {ExitCode: 0}, "gofmt": {ExitCode: 0}}, []CheckProblem{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attempt, err := state.NewStepAttempt("quality", 1)
			if err != nil {
				t.Fatal(err)
			}
			attempt.CheckResults = tt.results
			got := CheckDoneGate(step, state.State{Attempts: []state.StepAttempt{attempt}, CurrentAttemptID: attempt.ID}, t.TempDir())
			if got.OK != tt.ok || !reflect.DeepEqual(got.CheckProblems, tt.want) {
				t.Fatalf("result=%#v want=%#v", got, tt.want)
			}
		})
	}

	if got := CheckDoneGate(flow.Step{ID: "quality"}, state.State{}, t.TempDir()); !got.OK || len(got.CheckProblems) != 0 {
		t.Fatalf("required_checksなしの結果=%#v", got)
	}
}

func TestCheckDoneGateArtifactEvidenceProblemsFollowFlowOrder(t *testing.T) {
	root := t.TempDir()
	createFiles(t, root, []string{"out/mismatch.md", "out/ok.md"})
	step := flow.Step{
		ID: "artifact",
		Artifacts: []flow.Artifact{
			{Path: "out/no-evidence.md", Required: true},
			{Path: "out/missing-file.md", Required: true},
			{Path: "out/mismatch.md", Required: true},
			{Path: "out/ok.md", Required: true},
		},
	}
	st := approvalGateState(t, step.ID, nil)
	attempt := st.Attempts[0]
	attempt.ArtifactEvidence["out/missing-file.md"] = state.ArtifactEvidence{
		Digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Size:   0,
	}
	attempt.ArtifactEvidence["out/mismatch.md"] = state.ArtifactEvidence{
		Digest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Size:   0,
	}
	ok, err := artifactpkg.ReadFile(root, "out/ok.md")
	if err != nil {
		t.Fatal(err)
	}
	attempt.ArtifactEvidence["out/ok.md"] = state.ArtifactEvidence{Digest: ok.Digest, Size: ok.Size}
	st.Attempts[0] = attempt

	got := CheckDoneGate(step, st, root)
	want := []ArtifactProblem{
		{Path: "out/no-evidence.md", Kind: ArtifactEvidenceMissing},
		{Path: "out/missing-file.md", Kind: ArtifactFileMissing},
		{Path: "out/mismatch.md", Kind: ArtifactMismatch},
	}
	if got.OK || !reflect.DeepEqual(got.ArtifactProblems, want) {
		t.Fatalf("ArtifactProblems = %#v, want %#v", got.ArtifactProblems, want)
	}
}

func TestCheckDoneGateUsesOnlyCurrentAttemptEvidence(t *testing.T) {
	root := t.TempDir()
	createFiles(t, root, []string{"out/report.md"})
	value, err := artifactpkg.ReadFile(root, "out/report.md")
	if err != nil {
		t.Fatal(err)
	}
	first, _ := state.NewStepAttempt("artifact", 1)
	first.ArtifactEvidence["out/report.md"] = state.ArtifactEvidence{Digest: value.Digest, Size: value.Size}
	first, _ = state.CloseStepAttempt(first, state.StepAttemptExitBack, "retry")
	current, _ := state.NewStepAttempt("artifact", 2)
	st := state.State{Attempts: []state.StepAttempt{first, current}, CurrentAttemptID: current.ID}
	step := flow.Step{ID: "artifact", Artifacts: []flow.Artifact{{Path: "out/report.md", Required: true}}}

	got := CheckDoneGate(step, st, root)
	if got.OK || !reflect.DeepEqual(got.ArtifactProblems, []ArtifactProblem{{Path: "out/report.md", Kind: ArtifactEvidenceMissing}}) {
		t.Fatalf("result = %#v", got)
	}
}

func createFiles(t *testing.T, root string, files []string) {
	t.Helper()

	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("artifact"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func createDirs(t *testing.T, root string, dirs []string) {
	t.Helper()

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func assertGateResult(t *testing.T, got Result, wantOK bool, wantMissingArtifacts []string, wantMissingApprovals []string) {
	t.Helper()

	if got.OK != wantOK {
		t.Fatalf("OK = %v, want %v", got.OK, wantOK)
	}
	if wantMissingArtifacts == nil {
		wantMissingArtifacts = []string{}
	}
	if wantMissingApprovals == nil {
		wantMissingApprovals = []string{}
	}
	if !reflect.DeepEqual(got.MissingArtifacts, wantMissingArtifacts) {
		t.Fatalf("MissingArtifacts = %#v, want %#v", got.MissingArtifacts, wantMissingArtifacts)
	}
	if !reflect.DeepEqual(got.MissingApprovals, wantMissingApprovals) {
		t.Fatalf("MissingApprovals = %#v, want %#v", got.MissingApprovals, wantMissingApprovals)
	}
}
