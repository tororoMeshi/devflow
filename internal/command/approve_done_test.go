package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestApproveRecordsCurrentStepApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Approve(Context{ProjectRoot: root}, "approval", st.CurrentAttemptID, "approved")

	assertCommandSuccess(t, got)
	if got.Success == nil || got.Success.ApprovedStepID != "approval" || got.Success.ApprovedAttemptID != st.CurrentAttemptID {
		t.Fatalf("Success = %#v", got.Success)
	}
	loaded := loadCommandState(t, root)
	approval := loaded.Attempts[0].Approval
	if approval == nil || approval.Note != "approved" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestApproveSaveFailureDoesNotPersistApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	statePath, err := NewStore(Context{ProjectRoot: root}).RunStatePath(st.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Dir(statePath)
	if err := os.Chmod(runDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o755) })
	got := Approve(Context{ProjectRoot: root}, "approval", st.CurrentAttemptID, "ok")
	assertCommandFailure(t, got, CodeStateSaveFailed)
	if err := os.Chmod(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	loaded := loadCommandState(t, root)
	if loaded.Attempts[0].Approval != nil {
		t.Fatal("approval persisted after save failure")
	}
}

func TestApproveRejectsFutureStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Approve(Context{ProjectRoot: root}, "approval", st.CurrentAttemptID, "approved")

	assertCommandFailure(t, got, transition.CodeStepAttemptMismatch)
}

func TestApproveRejectsStepWithoutRequiredApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Approve(Context{ProjectRoot: root}, "first", st.CurrentAttemptID, "not needed")

	assertCommandFailure(t, got, transition.CodeApprovalNotRequired)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestApproveRejectsMissingStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Approve(Context{ProjectRoot: root}, "missing", st.CurrentAttemptID, "ok")

	assertCommandFailure(t, got, transition.CodeStepAttemptMismatch)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestApproveRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, func(ctx Context) CommandResult {
		return Approve(ctx, "approval", "attempt_00000000000000000001", "ok")
	})
}

func TestDoneMovesToNextStepWhenGateOK(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.Status != state.StatusRunning {
		t.Fatalf("Status = %q, want running", loaded.Status)
	}
	if loaded.CurrentStepID != "artifact" {
		t.Fatalf("CurrentStepID = %q, want artifact", loaded.CurrentStepID)
	}
	assertStringSlice(t, loaded.CompletedSteps, []string{"first"})
}

func TestDoneCompletesFinalStepWhenGateOK(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("final")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.Status != state.StatusCompleted {
		t.Fatalf("Status = %q, want completed", loaded.Status)
	}
	if loaded.CurrentStepID != "final" {
		t.Fatalf("CurrentStepID = %q, want final", loaded.CurrentStepID)
	}
	assertStringSlice(t, loaded.CompletedSteps, []string{"final"})
}

func TestDoneRejectsMissingRequiredArtifact(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingArtifactEvidence)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestDoneRejectsMissingRequiredApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("approval")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingRequiredApproval)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestDoneUsesGateArtifactCheck(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "artifact")
	recorded := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/required.md")
	assertCommandSuccess(t, recorded)

	got := Done(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.CurrentStepID != "approval" {
		t.Fatalf("CurrentStepID = %q, want approval", loaded.CurrentStepID)
	}
	assertStringSlice(t, loaded.CompletedSteps, []string{"artifact"})
}

func TestDoneUsesGateApprovalCheckBeforeApplyingDone(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("approval")
	st.Attempts[0].Approval = &state.ApprovalRecord{Note: "ok"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	if loaded.CurrentStepID != "final" {
		t.Fatalf("CurrentStepID = %q, want final", loaded.CurrentStepID)
	}
	assertStringSlice(t, loaded.CompletedSteps, []string{"approval"})
}

func TestDoneRemovesCurrentStepFromSkippedSteps(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	st.SkippedSteps["first"] = state.SkippedStep{Reason: "retry"}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	assertStringSlice(t, loaded.CompletedSteps, []string{"first"})
	if _, ok := loaded.SkippedSteps["first"]; ok {
		t.Fatalf("first remained in skipped_steps")
	}
}

func TestDoneRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, Done)
}

func approveDoneTestFlow() string {
	return `flow: {
		id: "approve-done-flow"
		title: "Approve Done Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}, {
			id: "artifact"
			title: "Artifact"
			instruction: "Create artifact."
			artifacts: [{
				path: "docs/required.md"
				required: true
			}]
		}, {
			id: "approval"
			title: "Approval"
			instruction: "Get approval."
			approval: {
				required: true
			}
		}, {
			id: "final"
			title: "Final"
			instruction: "Finish."
		}]
	}`
}

func approveDoneState(currentStepID string) state.State {
	return commandStateWithAttempt(testSnapshot("approve-done-flow"), testTaskSnapshot(), state.StatusRunning, currentStepID, "run_00000000000000000000000000000000")
}

func loadCommandState(t *testing.T, root string) state.State {
	t.Helper()

	loaded := NewStore(Context{ProjectRoot: root}).LoadCurrent()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	return *loaded.State
}

func assertCommandFileUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()

	after := readCommandFile(t, path)
	if string(after) != string(before) {
		t.Fatalf("%s was modified", path)
	}
}

func assertStringSlice(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(slice) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func assertActiveFlowRequiredByCommand(t *testing.T, run func(Context) CommandResult) {
	t.Helper()

	tests := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantStatus string
	}{
		{
			name:       "no state",
			setup:      func(t *testing.T, root string) {},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "completed state",
			setup: func(t *testing.T, root string) {
				st := commandStateWithAttempt(testSnapshot("approve-done-flow"), testTaskSnapshot(), state.StatusCompleted, "first", "run_00000000000000000000000000000000")
				if err := saveCommandState(t, root, st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "finished state",
			setup: func(t *testing.T, root string) {
				st := commandStateWithAttempt(testSnapshot("approve-done-flow"), testTaskSnapshot(), state.StatusFinished, "first", "run_00000000000000000000000000000000")
				if err := saveCommandState(t, root, st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "invalid state",
			setup: func(t *testing.T, root string) {
				writeCommandTestFile(t, LegacyStatePath(root), `{"not":"valid state"}`)
			},
			wantStatus: CodeUnsupportedStateVersion,
		},
		{
			name: "current step missing from snapshot",
			setup: func(t *testing.T, root string) {
				st := approveDoneState("missing")
				st.FlowSnapshot = approveDoneState("first").FlowSnapshot
				writeCommandStateUnchecked(t, root, st)
			},
			wantStatus: CodeInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			got := run(Context{ProjectRoot: root})

			assertCommandFailure(t, got, tt.wantStatus)
		})
	}
}

func TestDoneDoesNotCreateMissingArtifact(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "docs", "required.md")
	assertNoFile(t, artifactPath)

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingArtifactEvidence)
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}
