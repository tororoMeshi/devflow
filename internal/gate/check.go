package gate

import (
	"errors"

	"github.com/tororoMeshi/devflow/internal/artifact"
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
)

func InspectEntryGate(projectRoot string, step flow.Step, set *InspectionSet) EntryGateResult {
	blockers := make([]EntryBlocker, 0)
	for _, input := range step.Inputs {
		if !input.Required {
			continue
		}
		inspection := set.inspect(projectRoot, input.Path)
		if inspection.Err == nil {
			continue
		}
		kind := EntryBlockerInputUnavailable
		if errors.Is(inspection.Err, artifact.ErrMissing) {
			kind = EntryBlockerMissingInput
		}
		blockers = append(blockers, EntryBlocker{Kind: kind, Path: input.Path})
	}
	return EntryGateResult{Ready: len(blockers) == 0, Blockers: blockers}
}

// InspectCompletionGate requires a validated state and its current active
// attempt. It performs no state transition; callers must only invoke ApplyDone
// after this result and any destination Entry Gate are ready.
func InspectCompletionGate(projectRoot string, st state.State, step flow.Step, attempt state.StepAttempt, set *InspectionSet) CompletionGateResult {
	blockers := make([]CompletionBlocker, 0)
	if current, _, ok := st.CurrentAttempt(); ok {
		attempt = current
	}

	for _, input := range step.Inputs {
		if !input.Required {
			continue
		}
		inspection := set.inspect(projectRoot, input.Path)
		if inspection.Err == nil {
			continue
		}
		kind := CompletionBlockerInputUnavailable
		if errors.Is(inspection.Err, artifact.ErrMissing) {
			kind = CompletionBlockerMissingInput
		}
		blockers = append(blockers, CompletionBlocker{Kind: kind, Path: input.Path})
	}

	for _, requirement := range step.Artifacts {
		if !requirement.Required {
			continue
		}
		recorded, ok := attempt.ArtifactEvidence[requirement.Path]
		if !ok {
			blockers = append(blockers, CompletionBlocker{Kind: CompletionBlockerMissingArtifactEvidence, Path: requirement.Path})
			continue
		}
		inspection := set.inspect(projectRoot, requirement.Path)
		if inspection.Err != nil {
			kind := CompletionBlockerArtifactUnavailable
			if errors.Is(inspection.Err, artifact.ErrMissing) {
				kind = CompletionBlockerMissingArtifact
			}
			blockers = append(blockers, CompletionBlocker{Kind: kind, Path: requirement.Path})
			continue
		}
		if inspection.Evidence.Digest != recorded.Digest || inspection.Evidence.Size != recorded.Size {
			blockers = append(blockers, CompletionBlocker{Kind: CompletionBlockerArtifactEvidenceMismatch, Path: requirement.Path})
		}
	}

	for _, checkID := range step.RequiredChecks {
		result, ok := attempt.CheckResults[checkID]
		if !ok {
			blockers = append(blockers, CompletionBlocker{Kind: CompletionBlockerMissingCheck, CheckID: checkID})
			continue
		}
		if result.ExitCode != 0 {
			blockers = append(blockers, CompletionBlocker{Kind: CompletionBlockerFailedCheck, CheckID: checkID})
		}
	}

	if step.Approval != nil && step.Approval.Required && attempt.Approval == nil {
		blockers = append(blockers, CompletionBlocker{Kind: CompletionBlockerMissingApproval})
	}
	return CompletionGateResult{Ready: len(blockers) == 0, Blockers: blockers}
}

func InspectArtifacts(projectRoot string, step flow.Step, attempt state.StepAttempt, set *InspectionSet) []ArtifactInspection {
	result := make([]ArtifactInspection, 0, len(step.Artifacts))
	for _, requirement := range step.Artifacts {
		inspection := ArtifactInspection{Path: requirement.Path, Required: requirement.Required}
		actual := set.inspect(projectRoot, requirement.Path)
		inspection.Exists = actual.Err == nil
		recorded, ok := attempt.ArtifactEvidence[requirement.Path]
		if !ok {
			if requirement.Required {
				inspection.Problem = CompletionBlockerMissingArtifactEvidence
			}
			result = append(result, inspection)
			continue
		}
		inspection.Evidence = &state.ArtifactEvidence{Digest: recorded.Digest, Size: recorded.Size}
		if actual.Err != nil {
			matches := false
			inspection.MatchesEvidence = &matches
			inspection.Problem = CompletionBlockerArtifactUnavailable
			if errors.Is(actual.Err, artifact.ErrMissing) {
				inspection.Problem = CompletionBlockerMissingArtifact
			}
			result = append(result, inspection)
			continue
		}
		matches := actual.Evidence.Digest == recorded.Digest && actual.Evidence.Size == recorded.Size
		inspection.MatchesEvidence = &matches
		if !matches {
			inspection.Problem = CompletionBlockerArtifactEvidenceMismatch
		}
		result = append(result, inspection)
	}
	return result
}

func FileAvailable(projectRoot, path string, set *InspectionSet) bool {
	return set.inspect(projectRoot, path).Err == nil
}
