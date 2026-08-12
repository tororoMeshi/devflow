package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tororoMeshi/devflow/internal/state"
)

func TestStartedRunIgnoresObjectiveAndGateChanges(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "fixed-flow.cue")
	writeCommandFlow(t, root, "fixed-flow", fixedFlow(`objective: "Original objective."`))
	assertCommandSuccess(t, startWithTestTask(t, root, "fixed-flow"))

	writeCommandTestFile(t, flowPath, fixedFlow(`
		objective: "Changed objective."
		artifacts: [{path: "missing.txt", required: true}]
		required_checks: ["changed-check"]`))

	prompt := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, prompt)
	if prompt.Prompt.CurrentStepObjective != "Original objective." {
		t.Fatalf("objective = %q", prompt.Prompt.CurrentStepObjective)
	}
	if len(prompt.Prompt.RequiredArtifacts) != 0 || len(prompt.Prompt.RequiredChecks) != 0 {
		t.Fatalf("prompt uses changed gate: %#v", prompt.Prompt)
	}
	assertCommandSuccess(t, Done(Context{ProjectRoot: root}))
	if got := loadCommandState(t, root).CurrentStepID; got != "second" {
		t.Fatalf("CurrentStepID = %q, want second", got)
	}
}

func TestStartedRunIgnoresStepOrderChanges(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "fixed-flow.cue")
	writeCommandFlow(t, root, "fixed-flow", fixedFlow(`objective: "Original objective."`))
	assertCommandSuccess(t, startWithTestTask(t, root, "fixed-flow"))

	writeCommandTestFile(t, flowPath, `flow: {
		id: "fixed-flow"
		title: "Changed title"
		steps: [
			{id: "second", title: "Second", objective: "Changed second."},
			{id: "first", title: "First", objective: "Changed first."}
		]
	}`)

	assertCommandSuccess(t, Done(Context{ProjectRoot: root}))
	if got := loadCommandState(t, root).CurrentStepID; got != "second" {
		t.Fatalf("CurrentStepID = %q, want snapshot successor second", got)
	}
}

func TestCommandsUseSnapshotAfterFlowDeletion(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "checked-flow.cue")
	writeCommandFlow(t, root, "checked-flow", `flow: {
		id: "checked-flow"
		title: "Checked Flow"
		steps: [{
			id: "first"
			title: "First"
			objective: "Original objective."
			required_checks: ["verify"]
		}]
	}`)
	assertCommandSuccess(t, startWithTestTask(t, root, "checked-flow"))
	if err := os.Remove(flowPath); err != nil {
		t.Fatal(err)
	}

	assertCommandSuccess(t, Status(Context{ProjectRoot: root}))
	assertCommandSuccess(t, Prompt(Context{ProjectRoot: root}))
	assertCommandSuccess(t, CurrentContext(Context{ProjectRoot: root}))
	st := loadCommandState(t, root)
	request := CheckRequest(Context{ProjectRoot: root}, "first", st.CurrentAttemptID, "verify")
	assertCommandSuccess(t, request)
	if request.CheckRequest.FlowRunID != st.FlowRunID || request.CheckRequest.StepID != "first" || request.CheckRequest.AttemptID != st.CurrentAttemptID {
		t.Fatalf("CheckRequest = %#v", request.CheckRequest)
	}
}

func TestStateTransitionUsesSnapshotAfterFlowDeletion(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "fixed-flow.cue")
	writeCommandFlow(t, root, "fixed-flow", fixedFlow(`objective: "Original objective."`))
	assertCommandSuccess(t, startWithTestTask(t, root, "fixed-flow"))
	if err := os.Remove(flowPath); err != nil {
		t.Fatal(err)
	}
	assertCommandSuccess(t, Done(Context{ProjectRoot: root}))
	if got := loadCommandState(t, root).CurrentStepID; got != "second" {
		t.Fatalf("CurrentStepID = %q", got)
	}
}

func TestCompletedContextUsesSnapshotAfterFlowDeletion(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "terminal-flow.cue")
	writeCommandFlow(t, root, "terminal-flow", `flow: {
		id: "terminal-flow"
		title: "Terminal Flow"
		steps: [{id: "only", title: "Only", objective: "Finish."}]
	}`)
	assertCommandSuccess(t, startWithTestTask(t, root, "terminal-flow"))
	runID := loadCommandState(t, root).FlowRunID
	assertCommandSuccess(t, Done(Context{ProjectRoot: root}))
	if err := os.Remove(flowPath); err != nil {
		t.Fatal(err)
	}

	got := CurrentContext(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	if got.ExecutionContext.Flow.ID != "terminal-flow" || got.ExecutionContext.Flow.Title != "Terminal Flow" || got.ExecutionContext.FlowRunID != runID || got.ExecutionContext.State.Status != state.StatusCompleted {
		t.Fatalf("ExecutionContext = %#v", got.ExecutionContext)
	}
	if got.ExecutionContext.Step != nil || got.ExecutionContext.Completion != nil {
		t.Fatalf("terminal context exposes current step: %#v", got.ExecutionContext)
	}
}

func TestFinishedContextUsesSnapshotAfterFlowDeletion(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(FlowDir(root), "terminal-flow.cue")
	writeCommandFlow(t, root, "terminal-flow", `flow: {
		id: "terminal-flow"
		title: "Terminal Flow"
		steps: [{id: "only", title: "Only", objective: "Finish."}]
	}`)
	assertCommandSuccess(t, startWithTestTask(t, root, "terminal-flow"))
	runID := loadCommandState(t, root).FlowRunID
	assertCommandSuccess(t, Finish(Context{ProjectRoot: root}, "stopped"))
	if err := os.Remove(flowPath); err != nil {
		t.Fatal(err)
	}

	got := CurrentContext(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	if got.ExecutionContext.Flow.ID != "terminal-flow" || got.ExecutionContext.Flow.Title != "Terminal Flow" || got.ExecutionContext.FlowRunID != runID || got.ExecutionContext.State.Status != state.StatusFinished {
		t.Fatalf("ExecutionContext = %#v", got.ExecutionContext)
	}
}

func fixedFlow(firstBody string) string {
	return `flow: {
		id: "fixed-flow"
		title: "Fixed Flow"
		steps: [{
			id: "first"
			title: "First"
			` + firstBody + `
		}, {
			id: "second"
			title: "Second"
			objective: "Original second."
		}]
	}`
}
