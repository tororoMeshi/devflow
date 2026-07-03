package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
)

func ApplyDone(flow flow.Flow, st state.State, gateResult gate.Result) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}

	currentStep, currentIndex, ok := findStep(flow, st.CurrentStepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}

	if !gateResult.OK {
		diagnostics := []Diagnostic{}
		for _, artifact := range gateResult.MissingArtifacts {
			diagnostics = append(diagnostics, Diagnostic{
				Level:     LevelError,
				Code:      CodeMissingRequiredArtifact,
				StepID:    currentStep.ID,
				Artifacts: []string{artifact},
			})
		}
		for _, stepID := range gateResult.MissingApprovals {
			diagnostics = append(diagnostics, errorDiagnostic(CodeMissingRequiredApproval, stepID))
		}
		if len(diagnostics) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic(CodeInvalidGateResult, currentStep.ID))
		}
		return failure(diagnostics...)
	}

	next := st.Clone()
	next.CompletedSteps = append(next.CompletedSteps, currentStep.ID)
	delete(next.SkippedSteps, currentStep.ID)

	if currentIndex+1 < len(flow.Steps) {
		next.Status = state.StatusRunning
		next.CurrentStepID = flow.Steps[currentIndex+1].ID
	} else {
		next.Status = state.StatusCompleted
		next.CurrentStepID = currentStep.ID
	}

	return success(next)
}
