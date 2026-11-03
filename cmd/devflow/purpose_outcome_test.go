package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tororoMeshi/devflow/internal/command"
)

func TestPurposeOutcomeHumanAndAIOutput(t *testing.T) {
	var listed bytes.Buffer
	writeFlows(&listed, []command.FlowListItem{{ID: "flow", Title: "Flow", Purpose: "Flow reason.", Status: command.FlowStatusValid, StepCount: 1}})
	assertContains(t, listed.String(), "purpose: Flow reason.\n")
	if strings.Contains(strings.ToLower(listed.String()), "description") {
		t.Fatalf("old list label: %s", listed.String())
	}
	for _, purpose := range []string{"Step reason.", ""} {
		t.Run(purpose, func(t *testing.T) {
			var status, prompt bytes.Buffer
			writeStatus(&status, command.StatusResult{
				FlowTitle: "Flow", FlowPurpose: "Flow reason.", CurrentStepTitle: "Step",
				CurrentStepPurpose: purpose, CurrentStepOutcome: "Required state.",
			})
			writePrompt(&prompt, command.PromptResult{
				FlowPurpose: "Flow reason.", TaskContent: "Task content.", CurrentStepTitle: "Step",
				CurrentStepPurpose: purpose, CurrentStepOutcome: "Required state.",
			})
			for _, output := range []string{status.String(), prompt.String()} {
				assertContains(t, output, "Flow Purpose:")
				assertContains(t, output, "Flow reason.")
				assertContains(t, output, "Step Outcome:")
				assertContains(t, output, "Required state.")
				if purpose != "" {
					assertContains(t, output, "Step Purpose:")
					assertContains(t, output, purpose)
				} else if strings.Contains(output, "Step Purpose:") || strings.Contains(output, "\n\n\n") {
					t.Fatalf("empty purpose section: %q", output)
				}
				if strings.Contains(strings.ToLower(output), "objective") {
					t.Fatalf("old label: %s", output)
				}
			}
		})
	}
}
