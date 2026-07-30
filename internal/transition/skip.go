package transition

import (
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func ApplySkip(flow flow.Flow, st state.State, reason string) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}
	if blank(reason) {
		return failure(errorDiagnostic(CodeEmptyReason, st.CurrentStepID))
	}

	currentStep, currentIndex, ok := findStep(flow, st.CurrentStepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}

	diagnostics := skipWarnings(currentStep, currentIndex == len(flow.Steps)-1)

	next := st.Clone()
	if !closeCurrentAttempt(&next, state.StepAttemptExitSkip, reason) {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, st.CurrentStepID))
	}
	next.SkippedSteps[currentStep.ID] = state.SkippedStep{Reason: reason}
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

	return success(next, diagnostics...)
}

func skipWarnings(step flow.Step, finalStep bool) []Diagnostic {
	diagnostics := []Diagnostic{}

	if artifacts := requiredArtifactPaths(step); len(artifacts) > 0 {
		diagnostics = append(diagnostics, artifactWarning(step.ID, artifacts))
	}
	if hasRequiredApproval(step) {
		diagnostics = append(diagnostics, warningDiagnostic(CodeSkippedRequiredApproval, step.ID))
	}
	if len(step.RequiredChecks) > 0 {
		diagnostics = append(diagnostics, warningDiagnostic(CodeSkippedRequiredCheck, step.ID))
	}
	if finalStep {
		diagnostics = append(diagnostics, warningDiagnostic(CodeSkippedFinalStep, step.ID))
		if hasRequiredApproval(step) {
			diagnostics = append(diagnostics, warningDiagnostic(CodeSkippedFinalApprovalStep, step.ID))
		}
	}

	return diagnostics
}
