package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

// ApplyDone assumes command-layer Completion and destination Entry Gates have
// succeeded. Filesystem preconditions are intentionally kept out of transition.
func ApplyDone(flow flow.Flow, st state.State) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}

	currentStep, _, ok := findStep(flow, st.CurrentStepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}

	next := st.Clone()
	if !closeCurrentAttempt(&next, state.StepAttemptExitDone, "") {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	next.CompletedSteps = append(next.CompletedSteps, currentStep.ID)
	delete(next.SkippedSteps, currentStep.ID)

	nextStep, hasNext, valid := ResolveNextStep(flow, currentStep.ID)
	if !valid {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	if hasNext {
		next.Status = state.StatusRunning
		if !enterStep(&next, nextStep.ID) {
			return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
		}
	} else {
		next.Status = state.StatusCompleted
		next.CurrentStepID = currentStep.ID
		next.CurrentAttemptID = ""
	}

	return success(next)
}
