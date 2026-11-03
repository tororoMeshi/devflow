package command

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPurposeOutcomeJSONContractsUseSnapshot(t *testing.T) {
	for _, purpose := range []string{"Explain why this step exists.", ""} {
		t.Run(purpose, func(t *testing.T) {
			root := t.TempDir()
			purposeDefinition := ""
			if purpose != "" {
				purposeDefinition = fmt.Sprintf("purpose: %q", purpose)
			}
			definition := fmt.Sprintf(`flow: {
				id: "contract-flow"
				title: "Contract Flow"
				purpose: "Explain why this flow exists."
				steps: [{
					id: "work"
					title: "Work"
					%s
					outcome: "The required state has been established."
				}]
			}`, purposeDefinition)
			writeCommandFlow(t, root, "contract-flow", definition)
			ctx := Context{ProjectRoot: root}
			listed := List(ctx)
			assertCommandSuccess(t, listed)
			if len(listed.Flows) != 1 {
				t.Fatalf("flows = %#v", listed.Flows)
			}
			assertPurposeOutcomeJSON(t, listed.Flows[0], map[string]string{
				"purpose": "Explain why this flow exists.",
			}, "description", "Description")

			assertCommandSuccess(t, startWithTestTask(t, root, "contract-flow"))
			// Sources may change after start; all run contracts use the fixed snapshot.
			writeCommandFlow(t, root, "contract-flow", `flow: {
				id: "contract-flow", title: "Changed", purpose: "Changed flow purpose."
				steps: [{id: "work", title: "Changed", purpose: "Changed step purpose.", outcome: "Changed outcome."}]
			}`)
			writeCommandTestFile(t, filepath.Join(root, "tasks", "task.md"), "Unrelated task content.")
			want := map[string]string{
				"flow_purpose":         "Explain why this flow exists.",
				"current_step_purpose": purpose,
				"current_step_outcome": "The required state has been established.",
			}
			status := Status(ctx)
			assertCommandSuccess(t, status)
			assertPurposeOutcomeJSON(t, status.Status, want, "current_step_objective", "CurrentStepObjective")
			prompt := Prompt(ctx)
			assertCommandSuccess(t, prompt)
			assertPurposeOutcomeJSON(t, prompt.Prompt, want, "current_step_objective", "CurrentStepObjective")
			if prompt.Prompt.TaskContent != "Test task\n" {
				t.Fatalf("task content = %q, want snapshot content", prompt.Prompt.TaskContent)
			}
			current := CurrentContext(ctx)
			assertCommandSuccess(t, current)
			if current.ExecutionContext.TaskSnapshot.Content != "Test task\n" {
				t.Fatalf("task snapshot = %#v", current.ExecutionContext.TaskSnapshot)
			}
			if current.ExecutionContext.SchemaVersion != 6 {
				t.Fatalf("schema_version = %d", current.ExecutionContext.SchemaVersion)
			}
			assertPurposeOutcomeJSON(t, current.ExecutionContext.Flow, map[string]string{
				"id": "contract-flow", "title": "Contract Flow", "purpose": "Explain why this flow exists.",
			})
			assertPurposeOutcomeJSON(t, current.ExecutionContext.Step, map[string]string{
				"id": "work", "title": "Work", "purpose": purpose, "outcome": "The required state has been established.",
			}, "objective", "Objective")
		})
	}
}

func assertPurposeOutcomeJSON(t *testing.T, value any, want map[string]string, absent ...string) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for key, expected := range want {
		if got, exists := fields[key]; !exists || got != expected {
			t.Errorf("%s = %#v (present %v), want %q", key, got, exists, expected)
		}
	}
	for _, key := range absent {
		if _, exists := fields[key]; exists {
			t.Errorf("old field %s present: %s", key, data)
		}
	}
}
