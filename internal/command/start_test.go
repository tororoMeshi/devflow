package command

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func TestStartCreatesStateWhenNoStateExists(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	writeCommandTask(t, root, "tasks/task.md", "Do the task.\r\n")

	got := Start(Context{ProjectRoot: root}, "test-flow", "./tasks/task.md")

	assertCommandSuccess(t, got)
	loaded := NewStore(Context{ProjectRoot: root}).LoadCurrent()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	assertStartedState(t, *loaded.State, "test-flow", "first")
	flowPath := filepath.Join(FlowDir(root), "test-flow.cue")
	if loaded.State.FlowSnapshot.Source.Path != flowPath {
		t.Fatalf("FlowSnapshot.Source.Path = %q, want %q", loaded.State.FlowSnapshot.Source.Path, flowPath)
	}
	if !state.IsValidFlowRunID(loaded.State.FlowRunID) {
		t.Fatalf("FlowRunID = %q", loaded.State.FlowRunID)
	}
	if currentStatePath(t, root) != filepath.Join(RunsDir(root), loaded.State.FlowRunID, "state.json") {
		t.Fatalf("current state path = %q", currentStatePath(t, root))
	}
	var pointer state.CurrentPointer
	if err := json.Unmarshal(readCommandFile(t, CurrentPath(root)), &pointer); err != nil {
		t.Fatal(err)
	}
	if pointer.SchemaVersion != state.CurrentPointerSchemaVersion || pointer.FlowRunID != loaded.State.FlowRunID {
		t.Fatalf("current pointer = %#v", pointer)
	}
	assertNoFile(t, LegacyStatePath(root))
	expectedFlow, err := flow.LoadFile(flowPath)
	if err != nil {
		t.Fatal(err)
	}
	expectedSnapshot, err := flow.BuildSnapshot(expectedFlow, flow.FlowSource{Path: flowPath})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.State.FlowSnapshot, expectedSnapshot) {
		t.Fatalf("FlowSnapshot = %#v, want %#v", loaded.State.FlowSnapshot, expectedSnapshot)
	}
	expectedTask, err := task.BuildSnapshot("Do the task.\r\n", task.TaskSource{Path: "tasks/task.md"})
	if err != nil || !reflect.DeepEqual(loaded.State.TaskSnapshot, expectedTask) {
		t.Fatalf("TaskSnapshot = %#v, want %#v, err=%v", loaded.State.TaskSnapshot, expectedTask, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(readCommandFile(t, currentStatePath(t, root)), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["flow_snapshot"]; !ok {
		t.Fatal("top-level flow_snapshot is missing")
	}
	if _, ok := raw["task_snapshot"]; !ok {
		t.Fatal("top-level task_snapshot is missing")
	}
	for _, field := range []string{"task_content", "task_digest", "task_source_path"} {
		if _, ok := raw[field]; ok {
			t.Fatalf("duplicate top-level Task field %q is present", field)
		}
	}
	if _, ok := raw["flow_id"]; ok {
		t.Fatal("legacy top-level flow_id is present")
	}
}

func TestStartCurrentStateRecognizesTypedUnsupportedVersion(t *testing.T) {
	_, diagnostics := startCurrentState(state.LoadResult{
		Status: state.LoadInvalid,
		Err:    &state.UnsupportedSchemaVersionError{Actual: 3},
	})
	assertDiagnosticCodes(t, diagnostics, []string{CodeUnsupportedStateVersion})
}

func TestStartAllowsCompletedAndFinishedState(t *testing.T) {
	for _, status := range []state.Status{state.StatusCompleted, state.StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
			writeCommandTask(t, root, "tasks/task.md", "New task\n")
			current := commandStartState("previous-flow", status, "done")
			if err := saveCommandState(t, root, current); err != nil {
				t.Fatal(err)
			}
			previousPath := currentStatePath(t, root)
			previousData := readCommandFile(t, previousPath)
			previousTask := current.TaskSnapshot

			got := Start(Context{ProjectRoot: root}, "test-flow", "tasks/task.md")

			assertCommandSuccess(t, got)
			loaded := NewStore(Context{ProjectRoot: root}).LoadCurrent()
			if loaded.Status != state.LoadOK {
				t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
			}
			assertStartedState(t, *loaded.State, "test-flow", "first")
			if loaded.State.TaskSnapshot.Content != "New task\n" {
				t.Fatalf("new Run TaskSnapshot = %#v", loaded.State.TaskSnapshot)
			}
			if loaded.State.FlowRunID == current.FlowRunID {
				t.Fatal("current pointer did not switch to the new Run")
			}
			if got := readCommandFile(t, previousPath); !reflect.DeepEqual(got, previousData) {
				t.Fatal("previous terminal Run state changed")
			}
			previousLoaded := NewStore(Context{ProjectRoot: root}).LoadRun(current.FlowRunID)
			if previousLoaded.Status != state.LoadOK || !reflect.DeepEqual(previousLoaded.State.TaskSnapshot, previousTask) {
				t.Fatalf("previous TaskSnapshot = %#v", previousLoaded)
			}
			entries, err := os.ReadDir(RunsDir(root))
			if err != nil || len(entries) != 2 {
				t.Fatalf("Run directories = %d, err = %v", len(entries), err)
			}
		})
	}
}

func TestStartRejectsRunningStateWithoutSaving(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	current := commandStartState("running-flow", state.StatusRunning, "active")
	if err := saveCommandState(t, root, current); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))
	entriesBefore, err := os.ReadDir(RunsDir(root))
	if err != nil {
		t.Fatal(err)
	}

	got := Start(Context{ProjectRoot: root}, "test-flow", "tasks/missing.md")

	assertCommandFailure(t, got, transition.CodeFlowAlreadyRunning)
	after := readCommandFile(t, currentStatePath(t, root))
	if string(after) != string(before) {
		t.Fatalf("state.json was modified")
	}
	entriesAfter, err := os.ReadDir(RunsDir(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("Run directories increased: %d -> %d", len(entriesBefore), len(entriesAfter))
	}
}

func TestStartRejectsInvalidStateWithoutSaving(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	writeCommandTestFile(t, LegacyStatePath(root), `{"not":"valid state"}`)
	before := readCommandFile(t, LegacyStatePath(root))

	got := Start(Context{ProjectRoot: root}, "test-flow", "tasks/missing.md")

	assertCommandFailure(t, got, CodeUnsupportedStateVersion)
	after := readCommandFile(t, LegacyStatePath(root))
	if string(after) != string(before) {
		t.Fatalf("state.json was modified")
	}
}

func TestStartRejectsMissingOrInvalidFlow(t *testing.T) {
	t.Run("missing flow file", func(t *testing.T) {
		root := t.TempDir()

		got := Start(Context{ProjectRoot: root}, "missing-flow", "tasks/missing.md")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, LegacyStatePath(root))
	})

	t.Run("invalid flow definition", func(t *testing.T) {
		root := t.TempDir()
		writeCommandFlow(t, root, "broken-flow", `flow: {
			id: "broken-flow"
			title: "Broken Flow"
			steps: []
		}`)

		got := Start(Context{ProjectRoot: root}, "broken-flow", "tasks/missing.md")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, LegacyStatePath(root))
	})

	t.Run("flow id does not match requested id", func(t *testing.T) {
		root := t.TempDir()
		writeCommandTestFile(t, filepath.Join(FlowDir(root), "requested-flow.cue"), startTestFlow("different-flow"))

		got := Start(Context{ProjectRoot: root}, "requested-flow", "tasks/missing.md")

		assertCommandFailure(t, got, CodeStateFlowMismatch)
		assertNoFile(t, LegacyStatePath(root))
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
	writeCommandTask(t, root, "tasks/task.md", "Test task\n")

	got := Start(Context{ProjectRoot: root}, "artifact-flow", "tasks/task.md")

	assertCommandSuccess(t, got)
	loaded := NewStore(Context{ProjectRoot: root}).LoadCurrent()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	assertStartedState(t, *loaded.State, "artifact-flow", "first")
	if _, err := os.Stat(filepath.Join(root, "docs", "missing.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}

func TestStartRejectsInvalidTaskInputsWithoutPersistence(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		setup    func(testing.TB, string)
		wantCode string
	}{
		{"empty path", "", nil, CodeInvalidTaskPath},
		{"whitespace path", "   ", nil, CodeInvalidTaskPath},
		{"absolute path", "/tmp/task.md", nil, CodeInvalidTaskPath},
		{"parent component", "tasks/../task.md", nil, CodeInvalidTaskPath},
		{"missing", "tasks/missing.md", nil, CodeTaskFileNotFound},
		{"directory", "tasks", func(t testing.TB, root string) {
			if err := os.MkdirAll(filepath.Join(root, "tasks"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, CodeTaskPathIsDirectory},
		{"empty task", "tasks/task.md", func(t testing.TB, root string) { writeCommandTask(t, root, "tasks/task.md", " \n") }, CodeTaskEmpty},
		{"invalid UTF-8", "tasks/task.md", func(t testing.TB, root string) { writeCommandTask(t, root, "tasks/task.md", string([]byte{0xff})) }, CodeTaskInvalidUTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
			if tt.setup != nil {
				tt.setup(t, root)
			}
			got := Start(Context{ProjectRoot: root}, "test-flow", tt.path)
			assertCommandFailure(t, got, tt.wantCode)
			if _, err := os.Stat(RunsDir(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("runs directory exists or stat failed: %v", err)
			}
			if _, err := os.Stat(CurrentPath(root)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("pointer exists or stat failed: %v", err)
			}
		})
	}
}

func TestStartedRunUsesTaskSnapshotAfterTaskFileChangesAndDeletion(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "test-flow", startTestFlow("test-flow"))
	writeCommandTask(t, root, "tasks/task.md", "# Original\r\n\r\nKeep **markdown**.\r\n")
	assertCommandSuccess(t, Start(Context{ProjectRoot: root}, "test-flow", "./tasks/task.md"))
	want := loadCommandState(t, root).TaskSnapshot
	writeCommandTask(t, root, "tasks/task.md", "Changed\n")
	if got := loadCommandState(t, root).TaskSnapshot; !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot changed: %#v", got)
	}
	if got := CurrentContext(Context{ProjectRoot: root}); got.ExitCode != 0 || !reflect.DeepEqual(got.ExecutionContext.TaskSnapshot, want) {
		t.Fatalf("context = %#v", got)
	}
	if got := Prompt(Context{ProjectRoot: root}); got.ExitCode != 0 || got.Prompt.TaskContent != want.Content {
		t.Fatalf("prompt = %#v", got)
	}
	if err := os.Remove(filepath.Join(root, "tasks", "task.md")); err != nil {
		t.Fatal(err)
	}
	if got := CurrentContext(Context{ProjectRoot: root}); got.ExitCode != 0 || !reflect.DeepEqual(got.ExecutionContext.TaskSnapshot, want) {
		t.Fatalf("context after deletion = %#v", got)
	}
	if got := Prompt(Context{ProjectRoot: root}); got.ExitCode != 0 || got.Prompt.TaskContent != want.Content {
		t.Fatalf("prompt after deletion = %#v", got)
	}
	if got := Done(Context{ProjectRoot: root}); got.ExitCode != 0 {
		t.Fatalf("done after deletion = %#v", got)
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

			got := Start(Context{ProjectRoot: root}, tt.flowID, "tasks/task.md")

			assertCommandFailure(t, got, tt.code)
			assertNoFile(t, LegacyStatePath(root))
		})
	}
}

func TestStartRejectsInvalidFlowIDWithoutOverwritingState(t *testing.T) {
	root := t.TempDir()
	current := commandStartState("existing-flow", state.StatusCompleted, "done")
	if err := saveCommandState(t, root, current); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))

	got := Start(Context{ProjectRoot: root}, "../outside", "tasks/task.md")

	assertCommandFailure(t, got, string(flow.ErrorInvalidFlowID))
	after := readCommandFile(t, currentStatePath(t, root))
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
	return commandStateWithAttempt(testSnapshotForStep(flowID, currentStepID), testTaskSnapshot(), status, currentStepID, "run_00000000000000000000000000000000")
}

func writeCommandTask(t testing.TB, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertStartedState(t *testing.T, got state.State, flowID string, currentStepID string) {
	t.Helper()

	if got.SchemaVersion != state.CurrentSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", got.SchemaVersion, state.CurrentSchemaVersion)
	}
	if got.FlowSnapshot.Flow.ID != flowID {
		t.Fatalf("FlowSnapshot.Flow.ID = %q, want %q", got.FlowSnapshot.Flow.ID, flowID)
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
