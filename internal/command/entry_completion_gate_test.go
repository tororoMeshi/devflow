package command

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func TestStartEntryGateFailureLeavesNoRunArtifactsAndCanRetry(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "entry-flow", `flow: {
		id: "entry-flow"
		title: "Entry"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Work."
			inputs: [
				{path: "inputs/missing.txt"}
				{path: "inputs/unavailable"}
				{path: "inputs/optional.txt", required: false}
			]
		}]
	}`)
	writeCommandTask(t, root, "tasks/task.md", "task")
	if err := os.MkdirAll(filepath.Join(root, "inputs", "unavailable"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := Start(Context{ProjectRoot: root}, "entry-flow", "tasks/task.md")
	if got.ExitCode == 0 || len(got.Diagnostics) != 2 ||
		got.Diagnostics[0].Code != CodeEntryMissingRequiredInput ||
		got.Diagnostics[1].Code != CodeEntryInputUnavailable {
		t.Fatalf("Start() = %#v", got)
	}
	if _, err := os.Stat(RunsDir(root)); !os.IsNotExist(err) {
		t.Fatalf("runs directory exists or stat failed: %v", err)
	}
	if _, err := os.Stat(CurrentPath(root)); !os.IsNotExist(err) {
		t.Fatalf("current pointer exists or stat failed: %v", err)
	}

	writeCommandTestFile(t, filepath.Join(root, "inputs", "missing.txt"), "ready")
	if err := os.Remove(filepath.Join(root, "inputs", "unavailable")); err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, filepath.Join(root, "inputs", "unavailable"), "ready")
	got = Start(Context{ProjectRoot: root}, "entry-flow", "tasks/task.md")
	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	attempt, _, ok := loaded.CurrentAttempt()
	if !ok || attempt.ArtifactEvidence == nil || len(attempt.ArtifactEvidence) != 0 ||
		attempt.CheckResults == nil || len(attempt.CheckResults) != 0 || attempt.Approval != nil {
		t.Fatalf("first attempt = %#v", attempt)
	}
}

func TestDoneNextEntryFailureIsAtomic(t *testing.T) {
	root := t.TempDir()
	writeCommandTestFile(t, filepath.Join(root, "first", "input.txt"), "ready")
	st := gateCommandState(t, "first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	path := currentStatePath(t, root)
	before := readCommandFile(t, path)

	got := Done(Context{ProjectRoot: root})
	if got.ExitCode == 0 || len(got.Diagnostics) != 1 ||
		got.Diagnostics[0].Code != CodeEntryMissingRequiredInput ||
		got.Diagnostics[0].StepID != "second" {
		t.Fatalf("Done() = %#v", got)
	}
	assertCommandFileUnchanged(t, path, before)
	after := loadCommandState(t, root)
	if after.CurrentStepID != "first" || after.CurrentAttemptID != st.CurrentAttemptID ||
		len(after.CompletedSteps) != 0 || len(after.Attempts) != 1 ||
		after.Attempts[0].Status != state.StepAttemptActive || after.Attempts[0].ExitReason != "" {
		t.Fatalf("state changed = %#v", after)
	}
}

func TestSkipAndBackRequireDestinationEntry(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		root := t.TempDir()
		st := gateCommandState(t, "first")
		if err := saveCommandState(t, root, st); err != nil {
			t.Fatal(err)
		}
		path := currentStatePath(t, root)
		before := readCommandFile(t, path)
		got := Skip(Context{ProjectRoot: root}, "skip")
		if got.ExitCode == 0 || got.Diagnostics[0].Code != CodeEntryMissingRequiredInput {
			t.Fatalf("Skip() = %#v", got)
		}
		assertCommandFileUnchanged(t, path, before)
	})

	t.Run("back", func(t *testing.T) {
		root := t.TempDir()
		st := gateCommandState(t, "second")
		if err := saveCommandState(t, root, st); err != nil {
			t.Fatal(err)
		}
		path := currentStatePath(t, root)
		before := readCommandFile(t, path)
		got := Back(Context{ProjectRoot: root}, "", "revise")
		if got.ExitCode == 0 || got.Diagnostics[0].Code != CodeEntryMissingRequiredInput ||
			got.Diagnostics[0].StepID != "first" {
			t.Fatalf("Back() = %#v", got)
		}
		assertCommandFileUnchanged(t, path, before)
	})
}

func TestPromptSeparatesNextEntryBlockersAndSuppressesDone(t *testing.T) {
	root := t.TempDir()
	writeCommandTestFile(t, filepath.Join(root, "first", "input.txt"), "ready")
	st := gateCommandState(t, "first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := st.Clone()
	got := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	if got.Prompt == nil || len(got.Prompt.NextEntryBlockers) != 1 ||
		got.Prompt.NextEntryBlockers[0] != "second: next/input.txt: missing_input" ||
		containsCommand(got.Prompt.AfterCompleting.Commands, "devflow done") {
		t.Fatalf("Prompt() = %#v", got.Prompt)
	}
	after := loadCommandState(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("Prompt changed state: before=%#v after=%#v", before, after)
	}
}

func TestPromptDoneMatchesDoneCommand(t *testing.T) {
	root := t.TempDir()
	writeCommandTestFile(t, filepath.Join(root, "first", "input.txt"), "ready")
	writeCommandTestFile(t, filepath.Join(root, "next", "input.txt"), "ready")
	st := gateCommandState(t, "first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	prompt := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, prompt)
	if prompt.Prompt == nil || !containsCommand(prompt.Prompt.AfterCompleting.Commands, "devflow done") {
		t.Fatalf("Prompt() = %#v, want devflow done", prompt.Prompt)
	}
	done := Done(Context{ProjectRoot: root})
	assertCommandSuccess(t, done)
	after := loadCommandState(t, root)
	if after.CurrentStepID != "second" || after.EntrySequence() != 2 {
		t.Fatalf("Done state = %#v", after)
	}
}

func TestFinalDoneDoesNotRequireEntryGate(t *testing.T) {
	root := t.TempDir()
	writeCommandTestFile(t, filepath.Join(root, "next", "input.txt"), "ready")
	st := gateCommandState(t, "second")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	after := loadCommandState(t, root)
	if after.Status != state.StatusCompleted || after.CurrentAttemptID != "" ||
		after.Attempts[0].ExitReason != state.StepAttemptExitDone {
		t.Fatalf("final Done state = %#v", after)
	}
}

func TestCompletionGateDiagnosticsPreserveBlockerOrderAndInformationLimits(t *testing.T) {
	result := gate.CompletionGateResult{Blockers: []gate.CompletionBlocker{
		{Kind: gate.CompletionBlockerMissingInput, Path: "input.txt"},
		{Kind: gate.CompletionBlockerInputUnavailable, Path: "unsafe-input.txt"},
		{Kind: gate.CompletionBlockerMissingArtifactEvidence, Path: "evidence.txt"},
		{Kind: gate.CompletionBlockerMissingArtifact, Path: "artifact.txt"},
		{Kind: gate.CompletionBlockerArtifactEvidenceMismatch, Path: "changed.txt"},
		{Kind: gate.CompletionBlockerArtifactUnavailable, Path: "unsafe-artifact.txt"},
		{Kind: gate.CompletionBlockerMissingCheck, CheckID: "missing-check"},
		{Kind: gate.CompletionBlockerFailedCheck, CheckID: "failed-check"},
		{Kind: gate.CompletionBlockerMissingApproval},
	}}
	got := completionGateDiagnostics("step", result)
	wantCodes := []string{
		transition.CodeMissingRequiredInput,
		CodeCompletionInputUnavailable,
		transition.CodeMissingArtifactEvidence,
		transition.CodeMissingRequiredArtifact,
		transition.CodeArtifactEvidenceMismatch,
		transition.CodeArtifactUnsafe,
		transition.CodeMissingRequiredCheck,
		transition.CodeFailedRequiredCheck,
		transition.CodeMissingRequiredApproval,
	}
	if len(got) != len(wantCodes) {
		t.Fatalf("diagnostics = %#v", got)
	}
	for index, wantCode := range wantCodes {
		if got[index].Code != wantCode {
			t.Fatalf("diagnostic[%d] = %#v, want code %q", index, got[index], wantCode)
		}
	}
	if got[0].Artifacts[0] != "input.txt" || got[6].StepID != "missing-check" ||
		got[7].StepID != "failed-check" || got[8].StepID != "step" {
		t.Fatalf("diagnostic projection = %#v", got)
	}
}

func gateCommandState(t *testing.T, currentStepID string) state.State {
	t.Helper()
	fl := flow.Flow{ID: "gate-flow", Title: "Gate", Steps: []flow.Step{
		{ID: "first", Title: "First", Instruction: "First.", Inputs: []flow.Artifact{{Path: "first/input.txt", Required: true}}},
		{ID: "second", Title: "Second", Instruction: "Second.", Inputs: []flow.Artifact{{Path: "next/input.txt", Required: true}}},
	}}
	snapshot, err := flow.BuildSnapshot(fl, flow.FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	return commandStateWithAttempt(snapshot, testTaskSnapshot(), state.StatusRunning, currentStepID, "run_11111111111111111111111111111111")
}

func containsCommand(commands []string, target string) bool {
	for _, command := range commands {
		if command == target {
			return true
		}
	}
	return false
}
