package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/state"
)

func TestStatusReturnsActiveFlowState(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	st.CompletedSteps = []string{"first"}
	st.SkippedSteps["skipped"] = state.SkippedStep{Reason: "not needed"}
	st.Approvals["current"] = state.ApprovalRecord{Approved: true, Note: "ok"}
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}

	got := Status(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if got.Status == nil {
		t.Fatalf("Status = nil")
	}
	if got.Status.FlowID != "status-flow" {
		t.Fatalf("FlowID = %q", got.Status.FlowID)
	}
	if got.Status.FlowTitle != "Status Prompt Flow" {
		t.Fatalf("FlowTitle = %q", got.Status.FlowTitle)
	}
	if got.Status.CurrentStepID != "current" {
		t.Fatalf("CurrentStepID = %q", got.Status.CurrentStepID)
	}
	if got.Status.CurrentStepTitle != "Current" {
		t.Fatalf("CurrentStepTitle = %q", got.Status.CurrentStepTitle)
	}
	if len(got.Status.CompletedSteps) != 1 || got.Status.CompletedSteps[0] != "first" {
		t.Fatalf("CompletedSteps = %#v", got.Status.CompletedSteps)
	}
	if got.Status.SkippedSteps["skipped"].Reason != "not needed" {
		t.Fatalf("SkippedSteps = %#v", got.Status.SkippedSteps)
	}
	if !got.Status.Approvals["current"].Approved || got.Status.Approvals["current"].Note != "ok" {
		t.Fatalf("Approvals = %#v", got.Status.Approvals)
	}
}

func TestPromptReturnsCurrentStepDetails(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}

	got := Prompt(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if got.Prompt == nil {
		t.Fatalf("Prompt = nil")
	}
	if got.Prompt.FlowID != "status-flow" {
		t.Fatalf("FlowID = %q", got.Prompt.FlowID)
	}
	if got.Prompt.CurrentStepID != "current" {
		t.Fatalf("CurrentStepID = %q", got.Prompt.CurrentStepID)
	}
	if got.Prompt.CurrentStepTitle != "Current" {
		t.Fatalf("CurrentStepTitle = %q", got.Prompt.CurrentStepTitle)
	}
	if got.Prompt.CurrentStepInstruction != "Do current work." {
		t.Fatalf("CurrentStepInstruction = %q", got.Prompt.CurrentStepInstruction)
	}
	assertArtifactPaths(t, got.Prompt.RequiredArtifacts, []string{"docs/required.md"})
	assertArtifactPaths(t, got.Prompt.OptionalArtifacts, []string{"docs/optional.md"})
	if got.Prompt.RequiredApproval == nil {
		t.Fatalf("RequiredApproval = nil")
	}
	if got.Prompt.RequiredApproval.StepID != "current" {
		t.Fatalf("RequiredApproval.StepID = %q", got.Prompt.RequiredApproval.StepID)
	}
	if len(got.Prompt.AfterCompleting.Commands) != 2 {
		t.Fatalf("AfterCompleting.Commands = %#v", got.Prompt.AfterCompleting.Commands)
	}
}

func TestPromptTreatsNoArtifactsAndNoApprovalAsEmpty(t *testing.T) {
	for _, currentStepID := range []string{"first", "no_approval"} {
		t.Run(currentStepID, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
			st := statusPromptState("status-flow", state.StatusRunning, currentStepID)
			if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
				t.Fatal(err)
			}

			got := Prompt(Context{ProjectRoot: root})

			assertCommandSuccess(t, got)
			if got.Prompt == nil {
				t.Fatalf("Prompt = nil")
			}
			if got.Prompt.RequiredArtifacts == nil {
				t.Fatalf("RequiredArtifacts = nil, want empty slice")
			}
			if len(got.Prompt.RequiredArtifacts) != 0 {
				t.Fatalf("RequiredArtifacts = %#v, want empty", got.Prompt.RequiredArtifacts)
			}
			if got.Prompt.OptionalArtifacts != nil {
				t.Fatalf("OptionalArtifacts = %#v, want nil when none", got.Prompt.OptionalArtifacts)
			}
			if got.Prompt.RequiredApproval != nil {
				t.Fatalf("RequiredApproval = %#v, want nil", got.Prompt.RequiredApproval)
			}
			assertCommands(t, got.Prompt.AfterCompleting.Commands, []string{"devflow done"})
		})
	}
}

func TestStatusAndPromptRequireActiveFlow(t *testing.T) {
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
				st := statusPromptState("status-flow", state.StatusCompleted, "current")
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "finished state",
			setup: func(t *testing.T, root string) {
				st := statusPromptState("status-flow", state.StatusFinished, "current")
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
			wantStatus: CodeInvalidState,
		},
		{
			name: "flow file missing",
			setup: func(t *testing.T, root string) {
				st := statusPromptState("status-flow", state.StatusRunning, "current")
				if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
					t.Fatal(err)
				}
			},
			wantStatus: CodeStateFlowMismatch,
		},
		{
			name: "current step missing from flow",
			setup: func(t *testing.T, root string) {
				writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
				st := statusPromptState("status-flow", state.StatusRunning, "missing")
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

			statusResult := Status(Context{ProjectRoot: root})
			promptResult := Prompt(Context{ProjectRoot: root})

			assertCommandFailure(t, statusResult, tt.wantStatus)
			assertCommandFailure(t, promptResult, tt.wantStatus)
		})
	}
}

func TestStatusAndPromptDoNotUpdateState(t *testing.T) {
	for _, command := range []struct {
		name string
		run  func(Context) CommandResult
	}{
		{name: "status", run: Status},
		{name: "prompt", run: Prompt},
	} {
		t.Run(command.name, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
			st := statusPromptState("status-flow", state.StatusRunning, "current")
			if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, StatePath(root))

			got := command.run(Context{ProjectRoot: root})

			assertCommandSuccess(t, got)
			after := readCommandFile(t, StatePath(root))
			if string(after) != string(before) {
				t.Fatalf("state.json was modified")
			}
		})
	}
}

func TestPromptDoesNotCheckArtifactExistence(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := NewStore(Context{ProjectRoot: root}).Save(st); err != nil {
		t.Fatal(err)
	}
	assertNoFile(t, filepath.Join(root, "docs", "required.md"))

	got := Prompt(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if _, err := os.Stat(filepath.Join(root, "docs", "required.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}

func statusPromptTestFlow() string {
	return `flow: {
		id: "status-flow"
		title: "Status Prompt Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}, {
			id: "current"
			title: "Current"
			instruction: "Do current work."
			artifacts: [{
				path: "docs/required.md"
				required: true
			}, {
				path: "docs/optional.md"
				required: false
			}]
			approval: {
				required: true
			}
		}, {
			id: "no_approval"
			title: "No Approval"
			instruction: "Do work without approval."
			approval: {
				required: false
			}
		}, {
			id: "skipped"
			title: "Skipped"
			instruction: "Skip me."
		}]
	}`
}

func statusPromptState(flowID string, status state.Status, currentStepID string) state.State {
	st := state.State{
		FlowID:        flowID,
		Status:        status,
		CurrentStepID: currentStepID,
	}
	st.Normalize()
	return st
}

func assertArtifactPaths(t *testing.T, got []ArtifactResult, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(artifacts) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Path != want[i] {
			t.Fatalf("artifact[%d].Path = %q, want %q", i, got[i].Path, want[i])
		}
	}
}

func assertCommands(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(commands) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("command[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
