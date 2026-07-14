package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
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
	next.SkippedSteps[currentStep.ID] = state.SkippedStep{Reason: reason}
	if currentIndex+1 < len(flow.Steps) {
		next.Status = state.StatusRunning
		enterStep(&next, flow.Steps[currentIndex+1].ID)
	} else {
		next.Status = state.StatusCompleted
		next.CurrentStepID = currentStep.ID
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
