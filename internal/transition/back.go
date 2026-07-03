package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func ApplyBack(flow flow.Flow, st state.State, reason string) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}
	if blank(reason) {
		return failure(errorDiagnostic(CodeEmptyReason, st.CurrentStepID))
	}

	_, currentIndex, ok := findStep(flow, st.CurrentStepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	if currentIndex == 0 {
		return failure(errorDiagnostic(CodeNoPreviousStep, st.CurrentStepID))
	}

	fromStepID := st.CurrentStepID
	toStepID := flow.Steps[currentIndex-1].ID
	next := st.Clone()
	next.CurrentStepID = toStepID
	next.BackHistory = append(next.BackHistory, state.BackHistory{
		FromStepID: fromStepID,
		ToStepID:   toStepID,
		Reason:     reason,
	})
	next.CompletedSteps = removeString(next.CompletedSteps, toStepID)

	return success(next)
}
