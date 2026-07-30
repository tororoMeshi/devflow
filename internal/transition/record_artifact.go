package transition

import (
	"strings"

	"github.com/tororoMeshi/devflow/internal/pathcheck"
	"github.com/tororoMeshi/devflow/internal/state"
)

func ApplyRecordArtifactEvidence(st state.State, stepID, attemptID, path string, evidence state.ArtifactEvidence) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}
	current, index, ok := st.CurrentAttempt()
	if !ok || current.Status != state.StepAttemptActive {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
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
	if current.ID != attemptID {
		return failure(errorDiagnostic(CodeStaleAttempt, stepID))
	}
	if current.StepID != stepID {
		return failure(errorDiagnostic(CodeStepAttemptMismatch, stepID))
	}
	if err := pathcheck.ValidateArtifactPath(path); err != nil || strings.Split(path, "/")[0] == ".devflow" {
		return failure(errorDiagnostic(CodeInvalidArtifactPath, stepID))
	}
	step, _, ok := findStep(st.FlowSnapshot.Flow, stepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	declared := false
	for _, artifact := range step.Artifacts {
		if artifact.Path == path {
			declared = true
			break
		}
	}
	if !declared {
		return failure(errorDiagnostic(CodeArtifactNotDeclared, stepID))
	}
	if current.Approval != nil {
		return failure(errorDiagnostic(CodeArtifactRecordAfterApproval, stepID))
	}
	if err := state.ValidateArtifactEvidence(evidence); err != nil {
		code := CodeInvalidArtifactEvidence
		if err == state.ErrInvalidArtifactDigest {
			code = CodeInvalidArtifactDigest
		}
		return failure(errorDiagnostic(code, stepID))
	}
	if existing, ok := current.ArtifactEvidence[path]; ok {
		if existing == evidence {
			next := st.Clone()
			if err := state.Validate(next); err != nil {
				return failure(errorDiagnostic(CodeInvalidState, stepID))
			}
			return success(next)
		}
		return failure(errorDiagnostic(CodeArtifactEvidenceAlreadyRecorded, stepID))
	}
	next := st.Clone()
	next.Attempts[index].ArtifactEvidence[path] = evidence
	if err := state.Validate(next); err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	return success(next)
}
