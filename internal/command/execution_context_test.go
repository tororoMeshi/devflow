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
	saveExecutionState(t, root, executionTestState(state.StatusRunning))
	statePath := StatePath(root)
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
			if got.ExecutionContext.State.Status != status || got.ExecutionContext.Step != nil || got.ExecutionContext.Completion != nil {
				t.Fatalf("Context = %#v", got.ExecutionContext)
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
	before, err := os.ReadFile(StatePath(root))
	if err != nil {
		t.Fatal(err)
	}

	got := Done(Context{ProjectRoot: root})
	if got.ExitCode == 0 || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != transition.CodeMissingRequiredInput {
		t.Fatalf("Done() = %#v", got)
	}
	after, err := os.ReadFile(StatePath(root))
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
	if updated.Status != state.StatusRunning || updated.CurrentStepID != "review" || updated.CurrentEntrySequence != 4 {
		t.Fatalf("updated state = %#v", updated)
	}
}

func TestExecutionChecksTreatsStaleResultAsPendingAndBlocksCompletion(t *testing.T) {
	current := state.State{
		CurrentEntrySequence: 3,
		CheckResults: map[string]state.CheckResult{
			"validate": {EntrySequence: 2, ExitCode: 0},
		},
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
	return state.State{
		SchemaVersion:        state.CurrentSchemaVersion,
		FlowID:               "context-flow",
		Status:               status,
		CurrentStepID:        "design",
		FlowRunID:            "run_0123456789abcdef0123456789abcdef",
		CurrentEntrySequence: 3,
		CheckResults: map[string]state.CheckResult{
			"review": {EntrySequence: 3, ExitCode: 1},
		},
	}
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
	if err := NewStore(Context{ProjectRoot: root}).Save(value); err != nil {
		t.Fatal(err)
	}
}
