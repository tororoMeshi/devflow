package command

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestPaths(t *testing.T) {
	root := t.TempDir()

	if got := StatePath(root); got != filepath.Join(root, ".devflow", "state.json") {
		t.Fatalf("StatePath = %q", got)
	}
	if got := FlowDir(root); got != filepath.Join(root, ".devflow", "flows") {
		t.Fatalf("FlowDir = %q", got)
	}
	if got := NewStore(Context{ProjectRoot: root}).Path; got != StatePath(root) {
		t.Fatalf("NewStore path = %q", got)
	}
}

func TestActiveFlowFromLoadResult(t *testing.T) {
	t.Run("returns no active flow for no state", func(t *testing.T) {
		_, diagnostics := ActiveFlowFromLoadResult(Context{}, state.LoadResult{Status: state.LoadNoState})

		assertDiagnosticCodes(t, diagnostics, []string{CodeNoActiveFlow})
	})

	t.Run("returns invalid state for invalid load result", func(t *testing.T) {
		_, diagnostics := ActiveFlowFromLoadResult(Context{}, state.LoadResult{Status: state.LoadInvalid})

		assertDiagnosticCodes(t, diagnostics, []string{CodeInvalidState})
	})

	t.Run("returns unsupported version for typed schema error", func(t *testing.T) {
		_, diagnostics := ActiveFlowFromLoadResult(Context{}, state.LoadResult{
			Status: state.LoadInvalid,
			Err:    &state.UnsupportedSchemaVersionError{Actual: 1},
		})

		assertDiagnosticCodes(t, diagnostics, []string{CodeUnsupportedStateVersion})
	})

	t.Run("returns no active flow for completed and finished states", func(t *testing.T) {
		for _, status := range []state.Status{state.StatusCompleted, state.StatusFinished} {
			st := validRunningState()
			st.Status = status

			_, diagnostics := ActiveFlowFromLoadResult(Context{}, state.LoadResult{Status: state.LoadOK, State: &st})

			assertDiagnosticCodes(t, diagnostics, []string{CodeNoActiveFlow})
		}
	})

	t.Run("loads running state and validates current step", func(t *testing.T) {
		root := t.TempDir()
		writeFlow(t, root, "test-flow", `flow: {
			id: "test-flow"
			title: "Test Flow"
			steps: [{
				id: "first"
				title: "First"
				instruction: "Do first."
			}]
		}`)
		st := validRunningState()

		active, diagnostics := ActiveFlowFromLoadResult(Context{ProjectRoot: root}, state.LoadResult{Status: state.LoadOK, State: &st})

		assertNoDiagnostics(t, diagnostics)
		if active.Flow.ID != "test-flow" {
			t.Fatalf("Flow.ID = %q", active.Flow.ID)
		}
		if active.CurrentStep.ID != "first" {
			t.Fatalf("CurrentStep.ID = %q", active.CurrentStep.ID)
		}
	})

	t.Run("returns mismatch when flow file is missing", func(t *testing.T) {
		root := t.TempDir()
		st := validRunningState()

		_, diagnostics := ActiveFlowFromLoadResult(Context{ProjectRoot: root}, state.LoadResult{Status: state.LoadOK, State: &st})

		assertDiagnosticCodes(t, diagnostics, []string{CodeStateFlowMismatch})
	})

	t.Run("returns mismatch when current step is not in flow", func(t *testing.T) {
		root := t.TempDir()
		writeFlow(t, root, "test-flow", `flow: {
			id: "test-flow"
			title: "Test Flow"
			steps: [{
				id: "first"
				title: "First"
				instruction: "Do first."
			}]
		}`)
		st := validRunningState()
		st.CurrentStepID = "missing"

		_, diagnostics := ActiveFlowFromLoadResult(Context{ProjectRoot: root}, state.LoadResult{Status: state.LoadOK, State: &st})

		assertDiagnosticCodes(t, diagnostics, []string{CodeStateStepNotInFlow})
	})
}

func TestWriteDiagnostics(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	ctx := Context{Stdout: &stdout, Stderr: &stderr}

	WriteDiagnostics(ctx, []transition.Diagnostic{
		{Level: transition.LevelError, Code: "error_code"},
		{Level: transition.LevelWarning, Code: "warning_code", StepID: "step"},
		{Level: "info", Code: "info_code"},
	})

	if stdout.String() != "info: info_code\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "error: error_code\nwarning: warning_code (step)\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSaveTransitionState(t *testing.T) {
	t.Run("does not save when state is nil", func(t *testing.T) {
		root := t.TempDir()

		diagnostics := SaveTransitionState(Context{ProjectRoot: root}, transition.TransitionResult{})

		assertNoDiagnostics(t, diagnostics)
		if _, err := os.Stat(StatePath(root)); !os.IsNotExist(err) {
			t.Fatalf("state file exists or stat failed unexpectedly: %v", err)
		}
	})

	t.Run("saves when state is present", func(t *testing.T) {
		root := t.TempDir()
		st := validRunningState()

		diagnostics := SaveTransitionState(Context{ProjectRoot: root}, transition.TransitionResult{State: &st})

		assertNoDiagnostics(t, diagnostics)
		loaded := NewStore(Context{ProjectRoot: root}).Load()
		if loaded.Status != state.LoadOK {
			t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
		}
	})
}

func validRunningState() state.State {
	st := state.State{
		SchemaVersion:        state.CurrentSchemaVersion,
		FlowID:               "test-flow",
		Status:               state.StatusRunning,
		CurrentStepID:        "first",
		FlowRunID:            "run_00000000000000000000000000000000",
		CurrentEntrySequence: 1,
	}
	st.Normalize()
	return st
}

func writeFlow(t *testing.T, root string, id string, content string) {
	t.Helper()

	path := filepath.Join(FlowDir(root), id+".cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertDiagnosticCodes(t *testing.T, diagnostics []transition.Diagnostic, want []string) {
	t.Helper()

	if len(diagnostics) != len(want) {
		t.Fatalf("len(diagnostics) = %d, want %d: %#v", len(diagnostics), len(want), diagnostics)
	}
	for i, diagnostic := range diagnostics {
		if diagnostic.Code != want[i] {
			t.Fatalf("diagnostic[%d].Code = %q, want %q", i, diagnostic.Code, want[i])
		}
	}
}

func assertNoDiagnostics(t *testing.T, diagnostics []transition.Diagnostic) {
	t.Helper()

	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diagnostics)
	}
}
