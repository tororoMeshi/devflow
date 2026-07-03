package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestStartCreatesStateWhenNoStateExists(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))

	got := Start(Context{ProjectRoot: root}, "test-flow")

	assertCommandSuccess(t, got)
	loaded := NewStore(Context{ProjectRoot: root}).Load()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	assertStartedState(t, *loaded.State, "test-flow", "first")
}

func TestStartAllowsCompletedAndFinishedState(t *testing.T) {
	for _, status := range []state.Status{state.StatusCompleted, state.StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
			current := commandStartState("previous-flow", status, "done")
			if err := NewStore(Context{ProjectRoot: root}).Save(current); err != nil {
				t.Fatal(err)
			}

			got := Start(Context{ProjectRoot: root}, "test-flow")

			assertCommandSuccess(t, got)
			loaded := NewStore(Context{ProjectRoot: root}).Load()
			if loaded.Status != state.LoadOK {
				t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
			}
			assertStartedState(t, *loaded.State, "test-flow", "first")
		})
	}
}

func TestStartRejectsRunningStateWithoutSaving(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	current := commandStartState("running-flow", state.StatusRunning, "active")
	if err := NewStore(Context{ProjectRoot: root}).Save(current); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Start(Context{ProjectRoot: root}, "test-flow")

	assertCommandFailure(t, got, transition.CodeFlowAlreadyRunning)
	after := readCommandFile(t, StatePath(root))
	if string(after) != string(before) {
		t.Fatalf("state.json was modified")
	}
}

func TestStartRejectsInvalidStateWithoutSaving(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	writeCommandTestFile(t, StatePath(root), `{"not":"valid state"}`)
	before := readCommandFile(t, StatePath(root))

	got := Start(Context{ProjectRoot: root}, "test-flow")

	assertCommandFailure(t, got, CodeInvalidState)
	after := readCommandFile(t, StatePath(root))
	if string(after) != string(before) {
		t.Fatalf("state.json was modified")
	}
}

func TestStartRejectsMissingOrInvalidFlow(t *testing.T) {
	t.Run("missing flow file", func(t *testing.T) {
		root := t.TempDir()

		got := Start(Context{ProjectRoot: root}, "missing-flow")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, StatePath(root))
	})

	t.Run("invalid flow definition", func(t *testing.T) {
		root := t.TempDir()
		writeCommandFlow(t, root, "broken-flow", `flow: {
			id: "broken-flow"
			title: "Broken Flow"
			steps: []
		}`)

		got := Start(Context{ProjectRoot: root}, "broken-flow")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, StatePath(root))
	})

	t.Run("flow id does not match requested id", func(t *testing.T) {
		root := t.TempDir()
		writeCommandTestFile(t, filepath.Join(FlowDir(root), "requested-flow.cue"), startTestFlow("different-flow"))

		got := Start(Context{ProjectRoot: root}, "requested-flow")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, StatePath(root))
	})
}

func TestStartDoesNotCheckArtifactsOrGate(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "artifact-flow", `flow: {
		id: "artifact-flow"
		title: "Artifact Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
			artifacts: [{
				path: "docs/missing.md"
				required: true
			}]
			approval: {
				required: true
			}
		}]
	}`)

	got := Start(Context{ProjectRoot: root}, "artifact-flow")

	assertCommandSuccess(t, got)
	loaded := NewStore(Context{ProjectRoot: root}).Load()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	assertStartedState(t, *loaded.State, "artifact-flow", "first")
	if _, err := os.Stat(filepath.Join(root, "docs", "missing.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}

func TestStartRejectsInvalidFlowIDBeforeReadingFlow(t *testing.T) {
	tests := []struct {
		name   string
		flowID string
		code   string
	}{
		{name: "parent directory", flowID: "../outside", code: string(flow.ErrorInvalidFlowID)},
		{name: "slash", flowID: "foo/bar", code: string(flow.ErrorInvalidFlowID)},
		{name: "backslash", flowID: `foo\bar`, code: string(flow.ErrorInvalidFlowID)},
		{name: "empty", flowID: "", code: string(flow.ErrorMissingFlowID)},
		{name: "spaces", flowID: "   ", code: string(flow.ErrorMissingFlowID)},
		{name: "japanese", flowID: "レビュー工程", code: string(flow.ErrorInvalidFlowID)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()

			got := Start(Context{ProjectRoot: root}, tt.flowID)

			assertCommandFailure(t, got, tt.code)
			assertNoFile(t, StatePath(root))
		})
	}
}

func TestStartRejectsInvalidFlowIDWithoutOverwritingState(t *testing.T) {
	root := t.TempDir()
	current := commandStartState("existing-flow", state.StatusCompleted, "done")
	if err := NewStore(Context{ProjectRoot: root}).Save(current); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, StatePath(root))

	got := Start(Context{ProjectRoot: root}, "../outside")

	assertCommandFailure(t, got, string(flow.ErrorInvalidFlowID))
	after := readCommandFile(t, StatePath(root))
	if string(after) != string(before) {
		t.Fatalf("state.json was modified")
	}
}

func startTestFlow(id string) string {
	return `flow: {
		id: "` + id + `"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}, {
			id: "second"
			title: "Second"
			instruction: "Do second."
		}]
	}`
}

func commandStartState(flowID string, status state.Status, currentStepID string) state.State {
	st := state.State{
		FlowID:        flowID,
		Status:        status,
		CurrentStepID: currentStepID,
	}
	st.Normalize()
	return st
}

func assertStartedState(t *testing.T, got state.State, flowID string, currentStepID string) {
	t.Helper()

	if got.FlowID != flowID {
		t.Fatalf("FlowID = %q, want %q", got.FlowID, flowID)
	}
	if got.Status != state.StatusRunning {
		t.Fatalf("Status = %q, want running", got.Status)
	}
	if got.CurrentStepID != currentStepID {
		t.Fatalf("CurrentStepID = %q, want %q", got.CurrentStepID, currentStepID)
	}
	if len(got.CompletedSteps) != 0 {
		t.Fatalf("CompletedSteps = %#v, want empty", got.CompletedSteps)
	}
	if len(got.SkippedSteps) != 0 {
		t.Fatalf("SkippedSteps = %#v, want empty", got.SkippedSteps)
	}
	if len(got.Approvals) != 0 {
		t.Fatalf("Approvals = %#v, want empty", got.Approvals)
	}
	if len(got.BackHistory) != 0 {
		t.Fatalf("BackHistory = %#v, want empty", got.BackHistory)
	}
	if got.Finish != nil {
		t.Fatalf("Finish = %#v, want nil", got.Finish)
	}
}

func assertCommandSuccess(t *testing.T, got CommandResult) {
	t.Helper()

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0; diagnostics = %#v", got.ExitCode, got.Diagnostics)
	}
	if len(got.Diagnostics) != 0 {
		t.Fatalf("Diagnostics = %#v, want none", got.Diagnostics)
	}
}

func assertCommandFailure(t *testing.T, got CommandResult, code string) {
	t.Helper()

	if got.ExitCode == 0 {
		t.Fatalf("ExitCode = 0, want non-zero")
	}
	assertDiagnosticCodes(t, got.Diagnostics, []string{code})
}

func readCommandFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
