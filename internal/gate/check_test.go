package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

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
			wantMissingArtifacts: []string{"docs/code-review.md"},
		},
		{
			name: "passes when required approval is approved",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state: state.State{
				Approvals: map[string]state.ApprovalRecord{
					"human_approval": {Approved: true},
				},
			},
			wantOK: true,
		},
		{
			name: "fails when required approval is missing",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state:                state.State{Approvals: map[string]state.ApprovalRecord{}},
			wantMissingApprovals: []string{"human_approval"},
		},
		{
			name: "fails when required approval is false",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state: state.State{
				Approvals: map[string]state.ApprovalRecord{
					"human_approval": {Approved: false},
				},
			},
			wantMissingApprovals: []string{"human_approval"},
		},
		{
			name: "does not use approval from another step",
			step: flow.Step{
				ID:       "human_approval",
				Approval: &flow.Approval{Required: true},
			},
			state: state.State{
				Approvals: map[string]state.ApprovalRecord{
					"other_step": {Approved: true},
				},
			},
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
			state:                state.State{Approvals: map[string]state.ApprovalRecord{}},
			wantMissingArtifacts: []string{"docs/code-review.md"},
			wantMissingApprovals: []string{"human_approval"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			createFiles(t, root, tt.files)
			createDirs(t, root, tt.dirs)

			got := CheckDoneGate(tt.step, tt.state, root)

			assertGateResult(t, got, tt.wantOK, tt.wantMissingArtifacts, tt.wantMissingApprovals)
		})
	}
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
		{"all failed", map[string]state.CheckResult{"go-test": {EntrySequence: 1, ExitCode: 1}, "go-vet": {EntrySequence: 1, ExitCode: 1}, "gofmt": {EntrySequence: 1, ExitCode: 1}}, []CheckProblem{{"go-test", CheckFailed}, {"go-vet", CheckFailed}, {"gofmt", CheckFailed}}, false},
		{"missing failed missing", map[string]state.CheckResult{"go-vet": {EntrySequence: 1, ExitCode: 1}}, []CheckProblem{{"go-test", CheckMissing}, {"go-vet", CheckFailed}, {"gofmt", CheckMissing}}, false},
		{"failed missing failed", map[string]state.CheckResult{"go-test": {EntrySequence: 1, ExitCode: 1}, "gofmt": {EntrySequence: 1, ExitCode: 1}}, []CheckProblem{{"go-test", CheckFailed}, {"go-vet", CheckMissing}, {"gofmt", CheckFailed}}, false},
		{"all passed", map[string]state.CheckResult{"go-test": {EntrySequence: 1, ExitCode: 0}, "go-vet": {EntrySequence: 1, ExitCode: 0}, "gofmt": {EntrySequence: 1, ExitCode: 0}}, []CheckProblem{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckDoneGate(step, state.State{CurrentEntrySequence: 1, CheckResults: tt.results}, t.TempDir())
			if got.OK != tt.ok || !reflect.DeepEqual(got.CheckProblems, tt.want) {
				t.Fatalf("result=%#v want=%#v", got, tt.want)
			}
		})
	}

	if got := CheckDoneGate(flow.Step{ID: "quality"}, state.State{}, t.TempDir()); !got.OK || len(got.CheckProblems) != 0 {
		t.Fatalf("required_checksなしの結果=%#v", got)
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
