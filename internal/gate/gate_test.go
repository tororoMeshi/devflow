package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/artifact"
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func TestInspectEntryGate(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "available.txt", "ok")
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	step := flow.Step{ID: "build", Inputs: []flow.Artifact{
		{Path: "missing.txt", Required: true},
		{Path: "available.txt", Required: true},
		{Path: "directory", Required: true},
		{Path: "optional.txt", Required: false},
	}}
	before := snapshotTree(t, root)
	got := InspectEntryGate(root, step, NewInspectionSet())
	want := []EntryBlocker{
		{Kind: EntryBlockerMissingInput, Path: "missing.txt"},
		{Kind: EntryBlockerInputUnavailable, Path: "directory"},
	}
	if got.Ready || got.Blockers == nil || !reflect.DeepEqual(got.Blockers, want) {
		t.Fatalf("InspectEntryGate() = %#v, want blockers %#v", got, want)
	}
	if after := snapshotTree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("filesystem changed: before=%#v after=%#v", before, after)
	}
	if again := InspectEntryGate(root, step, NewInspectionSet()); !reflect.DeepEqual(again, got) {
		t.Fatalf("repeated result = %#v, want %#v", again, got)
	}
}

func TestInspectEntryGateUnavailablePolicies(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "target.txt", "ok")
	if err := os.Symlink("target.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}
	tests := []string{"link.txt", ".devflow/state.json", "../outside", "/absolute"}
	for _, path := range tests {
		got := InspectEntryGate(root, flow.Step{Inputs: []flow.Artifact{{Path: path, Required: true}}}, NewInspectionSet())
		if got.Ready || len(got.Blockers) != 1 || got.Blockers[0].Kind != EntryBlockerInputUnavailable {
			t.Fatalf("%q result = %#v", path, got)
		}
	}
}

func TestInspectEntryGateUnicodeAndReady(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "入力/資料.txt", "ok")
	got := InspectEntryGate(root, flow.Step{Inputs: []flow.Artifact{{Path: "入力/資料.txt", Required: true}}}, NewInspectionSet())
	if !got.Ready || got.Blockers == nil || len(got.Blockers) != 0 {
		t.Fatalf("result = %#v", got)
	}
}

func TestInspectCompletionGateOrderingAndCurrentAttempt(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "artifact.txt", "current")
	actual, err := artifact.ReadFile(root, "artifact.txt")
	if err != nil {
		t.Fatal(err)
	}
	attempt, _ := state.NewStepAttempt("build", 2)
	attempt.ArtifactEvidence["artifact.txt"] = state.ArtifactEvidence{Digest: actual.Digest, Size: actual.Size + 1}
	attempt.CheckResults["failed"] = state.CheckResult{ExitCode: 1}
	step := flow.Step{
		ID: "build",
		Inputs: []flow.Artifact{
			{Path: "missing-input", Required: true},
			{Path: ".devflow/input", Required: true},
			{Path: "optional-input"},
		},
		Artifacts: []flow.Artifact{
			{Path: "missing-evidence", Required: true},
			{Path: "artifact.txt", Required: true},
			{Path: "optional-artifact"},
		},
		RequiredChecks: []string{"missing-check", "failed"},
		Approval:       &flow.Approval{Required: true},
	}
	got := InspectCompletionGate(root, state.State{}, step, attempt, NewInspectionSet())
	want := []CompletionBlocker{
		{Kind: CompletionBlockerMissingInput, Path: "missing-input"},
		{Kind: CompletionBlockerInputUnavailable, Path: ".devflow/input"},
		{Kind: CompletionBlockerMissingArtifactEvidence, Path: "missing-evidence"},
		{Kind: CompletionBlockerArtifactEvidenceMismatch, Path: "artifact.txt"},
		{Kind: CompletionBlockerMissingCheck, CheckID: "missing-check"},
		{Kind: CompletionBlockerFailedCheck, CheckID: "failed"},
		{Kind: CompletionBlockerMissingApproval},
	}
	if got.Ready || !reflect.DeepEqual(got.Blockers, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestInspectCompletionGateReadyAndSharedInspection(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "shared.txt", "ok")
	actual, err := artifact.ReadFile(root, "shared.txt")
	if err != nil {
		t.Fatal(err)
	}
	attempt, _ := state.NewStepAttempt("build", 1)
	attempt.ArtifactEvidence["shared.txt"] = state.ArtifactEvidence{Digest: actual.Digest, Size: actual.Size}
	attempt.CheckResults["test"] = state.CheckResult{ExitCode: 0}
	attempt.Approval = &state.ApprovalRecord{}
	step := flow.Step{
		ID:             "build",
		Inputs:         []flow.Artifact{{Path: "shared.txt", Required: true}},
		Artifacts:      []flow.Artifact{{Path: "shared.txt", Required: true}},
		RequiredChecks: []string{"test"},
		Approval:       &flow.Approval{Required: true},
	}
	set := NewInspectionSet()
	got := InspectCompletionGate(root, state.State{}, step, attempt, set)
	if !got.Ready || got.Blockers == nil || len(got.Blockers) != 0 || len(set.files) != 1 {
		t.Fatalf("result=%#v inspections=%d", got, len(set.files))
	}
}

func TestInspectCompletionGateOptionalEvidenceDoesNotChangeRequiredPolicy(t *testing.T) {
	root := t.TempDir()
	writeGateFile(t, root, "optional.txt", "optional")
	actual, err := artifact.ReadFile(root, "optional.txt")
	if err != nil {
		t.Fatal(err)
	}
	attempt, _ := state.NewStepAttempt("build", 1)
	step := flow.Step{ID: "build", Artifacts: []flow.Artifact{
		{Path: "required.txt", Required: true},
		{Path: "optional.txt", Required: false},
	}}
	missingRequired := []CompletionBlocker{{Kind: CompletionBlockerMissingArtifactEvidence, Path: "required.txt"}}
	before := InspectCompletionGate(root, state.State{}, step, attempt, NewInspectionSet())
	if before.Ready || !reflect.DeepEqual(before.Blockers, missingRequired) {
		t.Fatalf("without optional Evidence = %#v", before)
	}
	attempt.ArtifactEvidence["optional.txt"] = state.ArtifactEvidence{Digest: actual.Digest, Size: actual.Size}
	after := InspectCompletionGate(root, state.State{}, step, attempt, NewInspectionSet())
	if after.Ready || !reflect.DeepEqual(after.Blockers, missingRequired) {
		t.Fatalf("with optional Evidence = %#v", after)
	}
	optionalOnly := flow.Step{ID: "build", Artifacts: []flow.Artifact{{Path: "optional.txt", Required: false}}}
	if got := InspectCompletionGate(root, state.State{}, optionalOnly, attempt, NewInspectionSet()); !got.Ready || len(got.Blockers) != 0 {
		t.Fatalf("optional-only Gate = %#v", got)
	}
}

func TestInspectCompletionGateUsesOnlyCurrentAttempt(t *testing.T) {
	root := t.TempDir()
	current, _ := state.NewStepAttempt("build", 2)
	past, _ := state.NewStepAttempt("build", 1)
	past.CheckResults["test"] = state.CheckResult{ExitCode: 0}
	st := state.State{
		CurrentAttemptID: current.ID,
		Attempts:         []state.StepAttempt{past, current},
	}
	got := InspectCompletionGate(root, st, flow.Step{ID: "build", RequiredChecks: []string{"test"}}, past, NewInspectionSet())
	want := []CompletionBlocker{{Kind: CompletionBlockerMissingCheck, CheckID: "test"}}
	if got.Ready || !reflect.DeepEqual(got.Blockers, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestInspectCompletionGateArtifactFileProblems(t *testing.T) {
	root := t.TempDir()
	attempt, _ := state.NewStepAttempt("build", 1)
	recorded := state.ArtifactEvidence{Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1}
	attempt.ArtifactEvidence["missing.txt"] = recorded
	attempt.ArtifactEvidence["directory"] = recorded
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	step := flow.Step{ID: "build", Artifacts: []flow.Artifact{
		{Path: "missing.txt", Required: true},
		{Path: "directory", Required: true},
	}}
	got := InspectCompletionGate(root, state.State{}, step, attempt, NewInspectionSet())
	want := []CompletionBlocker{
		{Kind: CompletionBlockerMissingArtifact, Path: "missing.txt"},
		{Kind: CompletionBlockerArtifactUnavailable, Path: "directory"},
	}
	if got.Ready || !reflect.DeepEqual(got.Blockers, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestInspectionSetCachesSuccessAndErrorPerRootAndPath(t *testing.T) {
	calls := map[string]int{}
	evidence := artifact.FileEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	set := &InspectionSet{
		files: make(map[string]fileInspection),
		readFile: func(root, path string) (artifact.FileEvidence, error) {
			key := root + "\x00" + path
			calls[key]++
			if path == "missing.txt" {
				return artifact.FileEvidence{}, artifact.ErrMissing
			}
			return evidence, nil
		},
	}
	required := flow.Step{Inputs: []flow.Artifact{
		{Path: "shared.txt", Required: true},
		{Path: "missing.txt", Required: true},
	}}
	for range 2 {
		InspectEntryGate("root-a", required, set)
	}
	InspectEntryGate("root-b", required, set)

	for _, key := range []string{
		"root-a\x00shared.txt",
		"root-a\x00missing.txt",
		"root-b\x00shared.txt",
		"root-b\x00missing.txt",
	} {
		if calls[key] != 1 {
			t.Fatalf("read count for %q = %d, want 1", key, calls[key])
		}
	}
}

func TestInspectionSetReusesCurrentInputArtifactAndNextInput(t *testing.T) {
	const path = "shared.txt"
	evidence := artifact.FileEvidence{
		Digest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Size:   7,
	}
	calls := 0
	set := &InspectionSet{
		files: make(map[string]fileInspection),
		readFile: func(_, gotPath string) (artifact.FileEvidence, error) {
			calls++
			if gotPath != path {
				t.Fatalf("path = %q, want %q", gotPath, path)
			}
			return evidence, nil
		},
	}
	attempt, _ := state.NewStepAttempt("current", 1)
	attempt.ArtifactEvidence[path] = state.ArtifactEvidence{Digest: evidence.Digest, Size: evidence.Size}
	current := flow.Step{
		ID:        "current",
		Inputs:    []flow.Artifact{{Path: path, Required: true}},
		Artifacts: []flow.Artifact{{Path: path, Required: true}},
	}
	next := flow.Step{ID: "next", Inputs: []flow.Artifact{{Path: path, Required: true}}}

	completion := InspectCompletionGate("root", state.State{}, current, attempt, set)
	entry := InspectEntryGate("root", next, set)
	if !completion.Ready || !entry.Ready || calls != 1 {
		t.Fatalf("completion=%#v entry=%#v calls=%d", completion, entry, calls)
	}
}

func writeGateFile(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var result []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result = append(result, relative+":"+info.Mode().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
