package transition

import (
	"github.com/8noki8/devflow/internal/state"
)

func ApplyFinish(st state.State, reason string) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}
	if blank(reason) {
		return failure(errorDiagnostic(CodeEmptyReason, st.CurrentStepID))
	}

	next := st.Clone()
	if !closeCurrentAttempt(&next, state.StepAttemptExitFinish, reason) {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	next.Status = state.StatusFinished
	next.Finish = &state.Finish{Reason: reason}
	next.CurrentAttemptID = ""

	return success(next)
}
