package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestCurrentContextBuildsDeterministicBlockersWithoutChangingState(t *testing.T) {
	root := t.TempDir()
	writeExecutionFlow(t, root, "context-flow", `flow: {
		id: "context-flow"
		title: "Context Flow"
		steps: [{
			id: "design"
			title: "Design"
			instruction: "Create the design."
			inputs: [
				{path: "docs/request.md"},
				{path: "docs/optional-input.md", required: false},
			]
			artifacts: [
				{path: "docs/design.md"},
				{path: "docs/optional-output.md", required: false},
			]
			required_checks: ["validate", "review"]
			approval: {required: true}
		}]
	}`)
	currentState := executionTestState(state.StatusRunning)
	currentState.Attempts[0].CheckResults["review"] = state.CheckResult{ExitCode: 1}
	saveExecutionState(t, root, currentState)
	statePath := currentStatePath(t, root)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	got := CurrentContext(Context{ProjectRoot: root})
	if got.ExitCode != 0 || got.ExecutionContext == nil {
		t.Fatalf("CurrentContext() = %#v", got)
	}
	context := got.ExecutionContext
	if context.SchemaVersion != executionContextSchemaVersion || context.FlowRunID == "" {
		t.Fatalf("Context header = %#v", context)
	}
	if context.Attempt == nil || context.Attempt.ID != currentState.CurrentAttemptID || context.Attempt.EntrySequence != currentState.Attempts[0].EntrySequence {
		t.Fatalf("Attempt = %#v, state = %#v", context.Attempt, currentState)
	}
	if !reflect.DeepEqual(context.TaskSnapshot, executionTestState(state.StatusRunning).TaskSnapshot) {
		t.Fatalf("TaskSnapshot = %#v", context.TaskSnapshot)
	}
	if context.Step == nil || context.Completion == nil || context.Completion.Ready {
		t.Fatalf("Step/Completion = %#v / %#v", context.Step, context.Completion)
	}
	if context.Step.Inputs[0].Exists || context.Step.Artifacts[0].Exists {
		t.Fatalf("exists = inputs %#v artifacts %#v, want false", context.Step.Inputs, context.Step.Artifacts)
	}
	wantBlockers := []ExecutionContextBlocker{
		{Type: CompletionBlockerMissingInput, Path: "docs/request.md"},
		{Type: CompletionBlockerMissingArtifact, Path: "docs/design.md"},
		{Type: CompletionBlockerMissingCheck, CheckID: "validate"},
		{Type: CompletionBlockerFailedCheck, CheckID: "review"},
		{Type: CompletionBlockerMissingApproval, StepID: "design"},
	}
	if !reflect.DeepEqual(context.Completion.Blockers, wantBlockers) {
		t.Fatalf("Blockers = %#v, want %#v", context.Completion.Blockers, wantBlockers)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("context changed state.json")
	}
}

func TestCurrentContextReturnsTerminalState(t *testing.T) {
	for _, status := range []state.Status{state.StatusCompleted, state.StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			root := t.TempDir()
			writeExecutionFlow(t, root, "context-flow", executionTestFlow())
			saveExecutionState(t, root, executionTestState(status))

			got := CurrentContext(Context{ProjectRoot: root})
			if got.ExitCode != 0 || got.ExecutionContext == nil {
				t.Fatalf("CurrentContext() = %#v", got)
			}
			if got.ExecutionContext.State.Status != status || got.ExecutionContext.Attempt != nil || got.ExecutionContext.Step != nil || got.ExecutionContext.Completion != nil {
				t.Fatalf("Context = %#v", got.ExecutionContext)
			}
			if !reflect.DeepEqual(got.ExecutionContext.TaskSnapshot, executionTestState(status).TaskSnapshot) {
				t.Fatalf("terminal TaskSnapshot = %#v", got.ExecutionContext.TaskSnapshot)
			}
		})
	}
}

func TestCurrentContextUsesCurrentAttemptForReenteredStep(t *testing.T) {
	root := t.TempDir()
	writeExecutionFlow(t, root, "context-flow", `flow: {
		id: "context-flow"
		title: "Context Flow"
		steps: [
			{id: "design", title: "Design", instruction: "Create the design.", required_checks: ["validate", "review"], approval: {required: true}},
			{id: "review", title: "Review", instruction: "Review the design."},
		]
	}`)
	currentState := executionTestState(state.StatusRunning)
	firstApproved, err := state.ApproveStepAttempt(currentState.Attempts[0], "old approval")
	if err != nil {
		t.Fatal(err)
	}
	first, err := state.CloseStepAttempt(firstApproved, state.StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := state.NewStepAttempt("review", 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err = state.CloseStepAttempt(second, state.StepAttemptExitBack, "retry design")
	if err != nil {
		t.Fatal(err)
	}
	third, err := state.NewStepAttempt("design", 3)
	if err != nil {
		t.Fatal(err)
	}
	currentState.Attempts = []state.StepAttempt{first, second, third}
	currentState.CurrentAttemptID = third.ID
	saveExecutionState(t, root, currentState)

	got := CurrentContext(Context{ProjectRoot: root})
	if got.ExitCode != 0 || got.ExecutionContext == nil {
		t.Fatalf("CurrentContext() = %#v", got)
	}
	if got.ExecutionContext.Attempt == nil || got.ExecutionContext.Attempt.ID != "attempt_00000000000000000003" || got.ExecutionContext.Attempt.EntrySequence != 3 {
		t.Fatalf("Attempt = %#v, want third Attempt", got.ExecutionContext.Attempt)
	}
	if got.ExecutionContext.Step == nil || got.ExecutionContext.Step.ID != "design" {
		t.Fatalf("Step = %#v, want design", got.ExecutionContext.Step)
	}
	if got.ExecutionContext.Step.Approval.Approved {
		t.Fatal("historical approval leaked into current context")
	}
	if got.ExecutionContext.Completion.Ready {
		t.Fatal("missing current approval did not block completion")
	}
	currentState.Attempts[2], err = state.ApproveStepAttempt(currentState.Attempts[2], "current approval")
	if err != nil {
		t.Fatal(err)
	}
	saveExecutionState(t, root, currentState)
	got = CurrentContext(Context{ProjectRoot: root})
	if got.ExecutionContext == nil || !got.ExecutionContext.Step.Approval.Approved {
		t.Fatal("current approval not exposed")
	}
}

func TestCurrentContextJSONAttemptContract(t *testing.T) {
	for _, status := range []state.Status{state.StatusRunning, state.StatusCompleted, state.StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			root := t.TempDir()
			writeExecutionFlow(t, root, "context-flow", executionTestFlow())
			currentState := executionTestState(status)
			saveExecutionState(t, root, currentState)

			got := CurrentContext(Context{ProjectRoot: root})
			if got.ExitCode != 0 || got.ExecutionContext == nil {
				t.Fatalf("CurrentContext() = %#v", got)
			}
			data, err := json.Marshal(got.ExecutionContext)
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]any
			if err := json.Unmarshal(data, &value); err != nil {
				t.Fatal(err)
			}
			if value["schema_version"] != float64(3) {
				t.Fatalf("schema_version = %#v", value["schema_version"])
			}
			attempt, exists := value["attempt"]
			if !exists {
				t.Fatal("attempt is absent")
			}
			if _, exists := value["attempts"]; exists {
				t.Fatal("attempts is present")
			}
			if _, exists := value["attempt_history"]; exists {
				t.Fatal("attempt_history is present")
			}
			stateValue := value["state"].(map[string]any)
			if _, exists := stateValue["entry_sequence"]; exists {
				t.Fatal("state.entry_sequence is present")
			}
			if _, exists := value["entry_sequence"]; exists {
				t.Fatal("top-level entry_sequence is present")
			}
			if status != state.StatusRunning {
				if attempt != nil {
					t.Fatalf("attempt = %#v, want null", attempt)
				}
				return
			}
			attemptValue, ok := attempt.(map[string]any)
			if !ok || len(attemptValue) != 2 || attemptValue["id"] != currentState.CurrentAttemptID || attemptValue["entry_sequence"] != float64(currentState.Attempts[0].EntrySequence) {
				t.Fatalf("attempt = %#v", attempt)
			}
			if _, exists := attemptValue["step_id"]; exists {
				t.Fatal("attempt.step_id is present")
			}
			if _, exists := attemptValue["status"]; exists {
				t.Fatal("attempt.status is present")
			}
			stepValue := value["step"].(map[string]any)
			if _, exists := stepValue["entry_sequence"]; exists {
				t.Fatal("step.entry_sequence is present")
			}
		})
	}
}

func TestCurrentContextUsesEmptyJSONArrays(t *testing.T) {
	root := t.TempDir()
	writeExecutionFlow(t, root, "context-flow", executionTestFlow())
	saveExecutionState(t, root, executionTestState(state.StatusRunning))

	got := CurrentContext(Context{ProjectRoot: root})
	if got.ExitCode != 0 || got.ExecutionContext == nil {
		t.Fatalf("CurrentContext() = %#v", got)
	}
	data, err := json.Marshal(got.ExecutionContext)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	step := value["step"].(map[string]any)
	for _, field := range []string{"inputs", "artifacts", "checks"} {
		if _, ok := step[field].([]any); !ok {
			t.Fatalf("step.%s = %#v, want JSON array", field, step[field])
		}
	}
	completion := value["completion"].(map[string]any)
	if _, ok := completion["blockers"].([]any); !ok {
		t.Fatalf("completion.blockers = %#v, want JSON array", completion["blockers"])
	}
}

func TestDoneRejectsMissingRequiredInputWithoutChangingState(t *testing.T) {
	root := t.TempDir()
	writeExecutionFlow(t, root, "context-flow", `flow: {
		id: "context-flow"
		title: "Context Flow"
		steps: [{
			id: "design"
			title: "Design"
			instruction: "Create the design."
			inputs: [{path: "docs/request.md"}]
		}, {
			id: "review"
			title: "Review"
			instruction: "Review the design."
		}]
	}`)
	saveExecutionState(t, root, executionTestState(state.StatusRunning))
	before, err := os.ReadFile(currentStatePath(t, root))
	if err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})
	if got.ExitCode == 0 || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != transition.CodeMissingRequiredInput {
		t.Fatalf("Done() = %#v", got)
	}
	after, err := os.ReadFile(currentStatePath(t, root))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("Done changed state.json when required input was missing")
	}

	inputPath := filepath.Join(root, "docs", "request.md")
	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, []byte("request"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = Done(Context{ProjectRoot: root})
	if got.ExitCode != 0 {
		t.Fatalf("Done() after input creation = %#v", got)
	}
	updated := loadCommandState(t, root)
	if updated.Status != state.StatusRunning || updated.CurrentStepID != "review" || updated.EntrySequence() != 2 {
		t.Fatalf("updated state = %#v", updated)
	}
}

func TestExecutionChecksTreatsStaleResultAsPendingAndBlocksCompletion(t *testing.T) {
	attempt, err := state.NewStepAttempt("old-design", 2)
	if err != nil {
		t.Fatal(err)
	}
	attempt.CheckResults["validate"] = state.CheckResult{ExitCode: 0}
	current := state.State{
		CurrentAttemptID: "attempt_00000000000000000003",
		Attempts:         []state.StepAttempt{attempt},
	}
	checks := executionChecks([]string{"validate"}, current)
	if !reflect.DeepEqual(checks, []ExecutionCheckResult{{ID: "validate", Status: CheckStatusPending}}) {
		t.Fatalf("executionChecks() = %#v", checks)
	}
	step := flow.Step{ID: "design", RequiredChecks: []string{"validate"}}
	completion := executionCompletion(gate.CheckDoneGate(step, current, t.TempDir()), step.ID)
	wantBlockers := []ExecutionContextBlocker{{Type: CompletionBlockerMissingCheck, CheckID: "validate"}}
	if completion.Ready || !reflect.DeepEqual(completion.Blockers, wantBlockers) {
		t.Fatalf("completion = %#v, want blockers %#v", completion, wantBlockers)
	}
}

func executionTestFlow() string {
	return `flow: {
		id: "context-flow"
		title: "Context Flow"
		steps: [{id: "design", title: "Design", instruction: "Create the design."}]
	}`
}

func executionTestState(status state.Status) state.State {
	snapshot, err := flow.BuildSnapshot(flow.Flow{ID: "context-flow", Title: "Context Flow", Steps: []flow.Step{{ID: "design", Title: "Design", Instruction: "Create the design.", RequiredChecks: []string{"validate", "review"}}, {ID: "review", Title: "Review", Instruction: "Review the design."}}}, flow.FlowSource{})
	if err != nil {
		panic(err)
	}
	return commandStateWithAttempt(snapshot, testTaskSnapshot(), status, "design", "run_0123456789abcdef0123456789abcdef")
}

func writeExecutionFlow(t *testing.T, root string, id string, content string) {
	t.Helper()
	path := filepath.Join(FlowDir(root), id+".cue")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func saveExecutionState(t *testing.T, root string, value state.State) {
	t.Helper()
	if err := saveCommandState(t, root, value); err != nil {
		t.Fatal(err)
	}
}
