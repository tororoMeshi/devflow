package transition

import (
	"strings"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func success(next state.State, diagnostics ...Diagnostic) TransitionResult {
	return TransitionResult{
		State:       &next,
		ExitCode:    0,
		Diagnostics: diagnostics,
	}
}

func failure(diagnostics ...Diagnostic) TransitionResult {
	return TransitionResult{
		State:       nil,
		ExitCode:    1,
		Diagnostics: diagnostics,
	}
}

func errorDiagnostic(code string, stepID string) Diagnostic {
	return Diagnostic{Level: LevelError, Code: code, StepID: stepID}
}

func warningDiagnostic(code string, stepID string) Diagnostic {
	return Diagnostic{Level: LevelWarning, Code: code, StepID: stepID}
}

func artifactWarning(stepID string, artifacts []string) Diagnostic {
	return Diagnostic{
		Level:     LevelWarning,
		Code:      CodeSkippedRequiredArtifact,
		StepID:    stepID,
		Artifacts: artifacts,
	}
}

func requireRunning(st state.State) (TransitionResult, bool) {
	if st.Status != state.StatusRunning {
		return failure(errorDiagnostic(CodeNoActiveFlow, st.CurrentStepID)), false
	}
	return TransitionResult{}, true
}

func findStep(fl flow.Flow, stepID string) (flow.Step, int, bool) {
	for index, step := range fl.Steps {
		if step.ID == stepID {
			return step, index, true
		}
	}
	return flow.Step{}, -1, false
}

func hasRequiredApproval(step flow.Step) bool {
	return step.Approval != nil && step.Approval.Required
}

func requiredArtifactPaths(step flow.Step) []string {
	paths := []string{}
	for _, artifact := range step.Artifacts {
		if artifact.Required {
			paths = append(paths, artifact.Path)
		}
	}
	return paths
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}

func removeString(values []string, target string) []string {
	next := values[:0]
	for _, value := range values {
		if value != target {
			next = append(next, value)
		}
	}
	return next
}
