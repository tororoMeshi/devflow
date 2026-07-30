package transition

import (
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func ApplyBack(flow flow.Flow, st state.State, toStepID string, reason string) TransitionResult {
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
	fromStepID := st.CurrentStepID
	destination, destinationOK := ResolveBackStep(flow, st.CurrentStepID, toStepID)
	if !destinationOK {
		code := CodeInvalidBackTarget
		diagnosticStepID := toStepID
		if toStepID == "" {
			code = CodeNoPreviousStep
			diagnosticStepID = st.CurrentStepID
		}
		return failure(errorDiagnostic(code, diagnosticStepID))
	}
	toStepID = destination.ID
	_, toIndex, _ := findStep(flow, toStepID)

	next := st.Clone()
	if !closeCurrentAttempt(&next, state.StepAttemptExitBack, reason) {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	invalidatedStepIDs := invalidatedBackStepIDs(flow, st, toIndex, currentIndex)
	invalidated := make(map[string]struct{}, len(flow.Steps)-toIndex)
	for _, step := range flow.Steps[toIndex:] {
		invalidated[step.ID] = struct{}{}
	}
	next.CompletedSteps = removeInvalidatedSteps(next.CompletedSteps, invalidated)
	for stepID := range next.SkippedSteps {
		if _, ok := invalidated[stepID]; ok {
			delete(next.SkippedSteps, stepID)
		}
	}
	next.BackHistory = append(next.BackHistory, state.BackHistory{
		FromStepID:         fromStepID,
		ToStepID:           toStepID,
		Reason:             reason,
		InvalidatedStepIDs: invalidatedStepIDs,
	})
	if !enterStep(&next, toStepID) {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}

	return success(next)
}

func invalidatedBackStepIDs(flow flow.Flow, st state.State, toIndex int, currentIndex int) []string {
	ids := make([]string, 0, len(flow.Steps)-toIndex)
	for index, step := range flow.Steps[toIndex:] {
		stepIndex := toIndex + index
		if stepIndex <= currentIndex || hasStateForStep(st, step.ID) {
			ids = append(ids, step.ID)
		}
	}
	return ids
}

func hasStateForStep(st state.State, stepID string) bool {
	for _, completedStepID := range st.CompletedSteps {
		if completedStepID == stepID {
			return true
		}
	}
	if _, ok := st.SkippedSteps[stepID]; ok {
		return true
	}
	for _, attempt := range st.Attempts {
		if attempt.StepID == stepID {
			return true
		}
	}
	return false
}

func removeInvalidatedSteps(values []string, invalidated map[string]struct{}) []string {
	next := values[:0]
	for _, value := range values {
		if _, ok := invalidated[value]; !ok {
			next = append(next, value)
		}
	}
	return next
}
