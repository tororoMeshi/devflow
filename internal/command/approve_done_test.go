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
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}

	got := Approve(Context{ProjectRoot: root}, "", "approved")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	approval := loaded.Approvals["approval"]
	if !approval.Approved || approval.Note != "approved" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestApproveRecordsSpecifiedStepApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}

	got := Approve(Context{ProjectRoot: root}, "approval", "")

	assertCommandSuccess(t, got)
	loaded := loadCommandState(t, root)
	approval := loaded.Approvals["approval"]
	if !approval.Approved || approval.Note != "" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestApproveRejectsStepWithoutRequiredApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Approve(Context{ProjectRoot: root}, "first", "not needed")

	assertCommandFailure(t, got, transition.CodeApprovalNotRequired)
	assertCommandFileUnchanged(t, StatePath(root), before)
}

func TestApproveRejectsMissingStep(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Approve(Context{ProjectRoot: root}, "missing", "")

	assertCommandFailure(t, got, transition.CodeInvalidCurrentStep)
	assertCommandFileUnchanged(t, StatePath(root), before)
}

func TestApproveRequiresActiveFlow(t *testing.T) {
	assertActiveFlowRequiredByCommand(t, func(ctx Context) CommandResult {
		return Approve(ctx, "", "")
	})
}

func TestDoneMovesToNextStepWhenGateOK(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("first")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
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
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
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
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingRequiredArtifact)
	assertCommandFileUnchanged(t, StatePath(root), before)
}

func TestDoneRejectsMissingRequiredApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("approval")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingRequiredApproval)
	assertCommandFileUnchanged(t, StatePath(root), before)
}

func TestDoneUsesGateArtifactCheck(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "artifact")

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
	st.Approvals["approval"] = state.ApprovalRecord{Approved: true, Note: "ok"}
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
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
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
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
	st := state.State{
		SchemaVersion:        state.CurrentSchemaVersion,
		FlowID:               "approve-done-flow",
		Status:               state.StatusRunning,
		CurrentStepID:        currentStepID,
		FlowRunID:            "run_00000000000000000000000000000000",
		CurrentEntrySequence: 1,
	}
	st.Normalize()
	return st
}

func loadCommandState(t *testing.T, root string) state.State {
	t.Helper()

	loaded := NewStore(Context{ProjectRoot: root}).Load()
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
				st := approveDoneState("first")
				st.Status = state.StatusCompleted
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "finished state",
			setup: func(t *testing.T, root string) {
				st := approveDoneState("first")
				st.Status = state.StatusFinished
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "invalid state",
			setup: func(t *testing.T, root string) {
				writeCommandTestFile(t, StatePath(root), `{"not":"valid state"}`)
			},
			wantStatus: CodeUnsupportedStateVersion,
		},
		{
			name: "flow file missing",
			setup: func(t *testing.T, root string) {
				st := approveDoneState("first")
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeStateFlowMismatch,
		},
		{
			name: "current step missing from flow",
			setup: func(t *testing.T, root string) {
				writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
				st := approveDoneState("missing")
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeStateStepNotInFlow,
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
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(root, "docs", "required.md")
	assertNoFile(t, artifactPath)

	got := Done(Context{ProjectRoot: root})

	assertCommandFailure(t, got, transition.CodeMissingRequiredArtifact)
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}
