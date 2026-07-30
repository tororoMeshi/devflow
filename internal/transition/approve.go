package transition

import (
	"errors"
	"sort"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func ApplyApprove(_ flow.Flow, st state.State, stepID string, attemptID string, note string) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}
	current, currentIndex, ok := st.CurrentAttempt()
	if !ok {
		return failure(errorDiagnostic(CodeInvalidState, st.CurrentStepID))
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
	if attemptID != current.ID {
		return failure(errorDiagnostic(CodeStaleAttempt, stepID))
	}
	if stepID != current.StepID {
		return failure(errorDiagnostic(CodeStepAttemptMismatch, stepID))
	}
	targetStep, _, ok := findStep(st.FlowSnapshot.Flow, stepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	if !hasRequiredApproval(targetStep) {
		return failure(errorDiagnostic(CodeApprovalNotRequired, stepID))
	}
	requiredPaths := requiredArtifactPaths(targetStep)
	sort.Strings(requiredPaths)
	for _, path := range requiredPaths {
		if _, ok := current.ArtifactEvidence[path]; !ok {
			return failure(Diagnostic{Level: LevelError, Code: CodeMissingArtifactEvidence, StepID: stepID, Artifacts: []string{path}})
		}
	}
	evidencePaths := make([]string, 0, len(current.ArtifactEvidence))
	for path := range current.ArtifactEvidence {
		evidencePaths = append(evidencePaths, path)
	}
	sort.Strings(evidencePaths)
	evidenceSetDigest, err := state.ArtifactEvidenceSetDigest(evidencePaths, current.ArtifactEvidence)
	if err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	approved, err := state.ApproveStepAttempt(current, note, evidenceSetDigest)
	if errors.Is(err, state.ErrStepAttemptAlreadyApproved) {
		return failure(errorDiagnostic(CodeAttemptAlreadyApproved, stepID))
	}
	if errors.Is(err, state.ErrInvalidApprovalNote) {
		return failure(errorDiagnostic(CodeInvalidApprovalNote, stepID))
	}
	if err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	next := st.Clone()
	next.Attempts[currentIndex] = approved
	if err := state.Validate(next); err != nil {
		return failure(errorDiagnostic(CodeInvalidState, stepID))
	}
	result := success(next)
	result.ApprovedEvidenceSetDigest = evidenceSetDigest
	return result
}
