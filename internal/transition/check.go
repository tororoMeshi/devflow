package transition

import (
	"strings"

	"github.com/8noki8/devflow/internal/state"
)

func ApplyRecordCheckResult(st state.State, stepID, attemptID, checkID string, result state.CheckResult) TransitionResult {
	if err := state.Validate(st); err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	if applied, ok := requireRunning(st); !ok {
		return applied
	}
	if !state.IsValidStepAttemptID(attemptID) {
		return failure(errorDiagnostic(CodeInvalidAttemptID, stepID))
	}
	found := false
	for _, attempt := range st.Attempts {
		if attempt.ID == attemptID {
			found = true
			break
		}
	}
	if !found {
		return failure(errorDiagnostic(CodeInvalidAttemptID, stepID))
	}
	current, index, ok := st.CurrentAttempt()
	if !ok || current.Status != state.StepAttemptActive {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	if current.ID != attemptID {
		return failure(errorDiagnostic(CodeStaleAttempt, stepID))
	}
	if current.StepID != stepID || st.CurrentStepID != stepID {
		return failure(errorDiagnostic(CodeStepAttemptMismatch, stepID))
	}
	step, _, ok := findStep(st.FlowSnapshot.Flow, stepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	required := false
	for _, requiredCheck := range step.RequiredChecks {
		if requiredCheck == checkID {
			required = true
			break
		}
	}
	if !required {
		return failure(errorDiagnostic(CodeCheckNotRequired, stepID))
	}
	if result.ExitCode < 0 || strings.ContainsAny(result.LogPath, "\n\r\x00") {
		return failure(errorDiagnostic(CodeInvalidCheckResult, stepID))
	}
	if existing, exists := current.CheckResults[checkID]; exists {
		if existing == result {
			next := st.Clone()
			return TransitionResult{State: &next, ExitCode: 0, StateChanged: false}
		}
		return failure(errorDiagnostic(CodeConflictingCheckResult, stepID))
	}
	next := st.Clone()
	next.Attempts[index].CheckResults[checkID] = result
	if err := state.Validate(next); err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	return TransitionResult{State: &next, ExitCode: 0, StateChanged: true}
}
