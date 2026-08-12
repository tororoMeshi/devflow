package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func TestBackMovesToPreviousStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("third")
	st.CompletedSteps = []string{"first", "second", "third"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.CurrentStepID != "second" {
		t.Fatalf("CurrentStepID = %q, want second", loaded.CurrentStepID)
	}
}

func TestBackRemovesOnlyDestinationStepFromCompletedSteps(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("third")
	st.CompletedSteps = []string{"first", "second", "third"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	assertStringSlice(t, loaded.CompletedSteps, []string{"first"})
}

func TestBackKeepsApprovalsAndSkippedSteps(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("third")
	st.CompletedSteps = []string{"first", "second"}
	st.SkippedSteps["approval"] = state.SkippedStep{Reason: "skip approval"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if len(loaded.SkippedSteps) != 0 {
		t.Fatalf("downstream skipped state was not invalidated: %#v", loaded.SkippedSteps)
	}
}

func TestBackRecordsHistory(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("third")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if len(loaded.BackHistory) != 1 {
		t.Fatalf("BackHistory len = %d, want 1", len(loaded.BackHistory))
	}
	history := loaded.BackHistory[0]
	if history.FromStepID != "third" || history.ToStepID != "second" || history.Reason != "revise" {
		t.Fatalf("BackHistory[0] = %#v", history)
	}
	assertStringSlice(t, history.InvalidatedStepIDs, []string{"second", "third"})
}

func TestBackRejectsFirstStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandFailure(t, got, transition.CodeNoPreviousStep)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestBackRejectsEmptyReason(t *testing.T) {
	for _, reason := range []string{"", "   "} {
		t.Run("reason="+reason, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
			st := backSkipFinishState("second")
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, currentStatePath(t, root))

			got := Back(Context{ProjectRoot: root}, "", reason)

			assertCommandFailure(t, got, transition.CodeEmptyReason)
			assertCommandFileUnchanged(t, currentStatePath(t, root), before)
		})
	}
}

func TestBackRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, func(ctx Context) CommandResult {
		return Back(ctx, "", "revise")
	})
}

func TestBackMovesToSpecifiedUpstreamStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("final_approval")
	st.CompletedSteps = []string{"first", "second", "third", "approval", "artifact", "final_approval"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "second", "revise")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.CurrentStepID != "second" {
		t.Fatalf("CurrentStepID = %q, want second", loaded.CurrentStepID)
	}
	if len(loaded.CompletedSteps) != 1 || loaded.CompletedSteps[0] != "first" {
		t.Fatalf("state = %#v", loaded)
	}
}

func TestBackKeepsArtifactFile(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "docs", "required.md")
	content := []byte("artifact must remain\n")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	got := Back(Context{ProjectRoot: root}, "", "revise")

	assertCommandSuccess(t, got)
	after, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("artifact was deleted: %v", err)
	}
	if string(after) != string(content) {
		t.Fatalf("artifact content = %q, want %q", after, content)
	}
}

func TestBackRejectsInvalidTargetWithoutSaving(t *testing.T) {
	for _, target := range []string{"missing", "third", "final_approval"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
			st := backSkipFinishState("third")
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, currentStatePath(t, root))

			got := Back(Context{ProjectRoot: root}, target, "revise")

			assertCommandFailure(t, got, transition.CodeInvalidBackTarget)
			assertCommandFileUnchanged(t, currentStatePath(t, root), before)
		})
	}
}

func TestSkipRecordsSkippedStepAndMovesNext(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("second")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "not needed")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.SkippedSteps["second"].Reason != "not needed" {
		t.Fatalf("SkippedSteps = %#v", loaded.SkippedSteps)
	}
	if len(loaded.CompletedSteps) != 0 {
		t.Fatalf("CompletedSteps = %#v, want empty", loaded.CompletedSteps)
	}
	if loaded.CurrentStepID != "third" {
		t.Fatalf("CurrentStepID = %q, want third", loaded.CurrentStepID)
	}
}

func TestSkipCompletesFinalStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("final_approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "complete without approval")

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0; diagnostics = %#v", got.ExitCode, got.Diagnostics)
	}
	loaded := loadCommandState(t, root)
	if loaded.Status != state.StatusCompleted {
		t.Fatalf("Status = %q, want completed", loaded.Status)
	}
	if loaded.CurrentStepID != "final_approval" {
		t.Fatalf("CurrentStepID = %q, want final_approval", loaded.CurrentStepID)
	}
}

func TestSkipWarnsForRequiredApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "skip approval")

	assertCommandWarningSuccess(t, got, transition.CodeSkippedRequiredApproval)
}

func TestSkipWarnsForRequiredArtifact(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "skip artifact")

	assertCommandWarningSuccess(t, got, transition.CodeSkippedRequiredArtifact)
}

func TestSkipWarnsForFinalStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("final_approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "skip final")

	assertCommandWarningSuccess(t, got, transition.CodeSkippedFinalStep)
}

func TestSkipWarnsForFinalApprovalStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("final_approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Skip(Context{ProjectRoot: root}, "skip final approval")

	assertCommandWarningSuccess(t, got, transition.CodeSkippedFinalApprovalStep)
}

func TestSkipRejectsEmptyReason(t *testing.T) {
	for _, reason := range []string{"", "   "} {
		t.Run("reason="+reason, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
			st := backSkipFinishState("second")
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, currentStatePath(t, root))

			got := Skip(Context{ProjectRoot: root}, reason)

			assertCommandFailure(t, got, transition.CodeEmptyReason)
			assertCommandFileUnchanged(t, currentStatePath(t, root), before)
		})
	}
}

func TestSkipRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, func(ctx Context) CommandResult {
		return Skip(ctx, "skip")
	})
}

func TestFinishMarksFlowFinished(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Finish(Context{ProjectRoot: root}, "stop here")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.Status != state.StatusFinished {
		t.Fatalf("Status = %q, want finished", loaded.Status)
	}
	if loaded.Finish == nil || loaded.Finish.Reason != "stop here" {
		t.Fatalf("Finish = %#v", loaded.Finish)
	}
	if loaded.CurrentStepID != "artifact" {
		t.Fatalf("CurrentStepID = %q, want artifact", loaded.CurrentStepID)
	}
}

func TestFinishKeepsExistingProgress(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
	st := backSkipFinishState("artifact")
	st.CompletedSteps = []string{"first"}
	st.SkippedSteps["second"] = state.SkippedStep{Reason: "not needed"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Finish(Context{ProjectRoot: root}, "stop here")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	assertStringSlice(t, loaded.CompletedSteps, []string{"first"})
	if loaded.SkippedSteps["second"].Reason != "not needed" {
		t.Fatalf("SkippedSteps = %#v", loaded.SkippedSteps)
	}
}

func TestFinishRejectsEmptyReason(t *testing.T) {
	for _, reason := range []string{"", "   "} {
		t.Run("reason="+reason, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "back-skip-finish-flow", backSkipFinishTestFlow())
			st := backSkipFinishState("artifact")
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, currentStatePath(t, root))

			got := Finish(Context{ProjectRoot: root}, reason)

			assertCommandFailure(t, got, transition.CodeEmptyReason)
			assertCommandFileUnchanged(t, currentStatePath(t, root), before)
		})
	}
}

func TestFinishRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, func(ctx Context) CommandResult {
		return Finish(ctx, "finish")
	})
}

func backSkipFinishTestFlow() string {
	return `flow: {
		id: "back-skip-finish-flow"
		title: "Back Skip Finish Flow"
		steps: [{
			id: "first"
			title: "First"
			objective: "Do first."
		}, {
			id: "second"
			title: "Second"
			objective: "Do second."
		}, {
			id: "third"
			title: "Third"
			objective: "Do third."
		}, {
			id: "artifact"
			title: "Artifact"
			objective: "Create artifact."
			artifacts: [{
				path: "docs/required.md"
				required: true
			}]
		}, {
			id: "approval"
			title: "Approval"
			objective: "Get approval."
			approval: {
				required: true
			}
		}, {
			id: "final_approval"
			title: "Final Approval"
			objective: "Get final approval."
			approval: {
				required: true
			}
		}]
	}`
}

func backSkipFinishState(currentStepID string) state.State {
	return commandStateWithAttempt(testSnapshot("back-skip-finish-flow"), testTaskSnapshot(), state.StatusRunning, currentStepID, "run_00000000000000000000000000000000")
}

func assertCommandWarningSuccess(t *testing.T, got CommandResult, code string) {
	t.Helper()

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0; diagnostics = %#v", got.ExitCode, got.Diagnostics)
	}
	for _, diagnostic := range got.Diagnostics {
		if diagnostic.Code == code && diagnostic.Level == transition.LevelWarning {
			return
		}
	}
	t.Fatalf("warning %q not found in diagnostics: %#v", code, got.Diagnostics)
}
