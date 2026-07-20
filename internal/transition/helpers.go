package transition

import (
	"math"
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

func currentAttempt(st state.State) (state.StepAttempt, int, bool) {
	attempt, index, ok := st.CurrentAttempt()
	return attempt, index, ok && attempt.Status == state.StepAttemptActive && attempt.StepID == st.CurrentStepID
}

func closeCurrentAttempt(st *state.State, exitReason state.StepAttemptExitReason, reason string) bool {
	attempt, index, ok := currentAttempt(*st)
	if !ok {
		return false
	}
	closed, err := state.CloseStepAttempt(attempt, exitReason, reason)
	if err != nil {
		return false
	}
	st.Attempts[index] = closed
	return true
}

func enterStep(st *state.State, stepID string) bool {
	last, ok := st.LastAttempt()
	if !ok || last.EntrySequence == math.MaxUint64 {
		return false
	}
	attempt, err := state.NewStepAttempt(stepID, last.EntrySequence+1)
	if err != nil {
		return false
	}
	st.Attempts = append(st.Attempts, attempt)
	st.CurrentAttemptID = attempt.ID
	st.CurrentStepID = stepID
	return true
}
