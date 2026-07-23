package gate

import (
	"errors"
	"github.com/8noki8/devflow/internal/artifact"
	"github.com/8noki8/devflow/internal/flow"
	statepkg "github.com/8noki8/devflow/internal/state"
)

func CheckDoneGate(step flow.Step, currentState statepkg.State, projectRoot string) Result {
	return checkDoneGateFromInspections(step, currentState, projectRoot, InspectRequiredArtifacts(step, currentState, projectRoot))
}

// InspectDoneGate returns the done-gate result and the ordered artifact
// inspections used to produce it. Optional artifacts are included for
// execution-context projection but never affect the gate.
func InspectDoneGate(step flow.Step, currentState statepkg.State, projectRoot string) (Result, []ArtifactInspection) {
	inspections := InspectArtifacts(step, currentState, projectRoot)
	return checkDoneGateFromInspections(step, currentState, projectRoot, inspections), inspections
}

func checkDoneGateFromInspections(step flow.Step, currentState statepkg.State, projectRoot string, inspections []ArtifactInspection) Result {
	result := Result{
		MissingInputs:    []string{},
		MissingArtifacts: []string{},
		MissingApprovals: []string{},
		CheckProblems:    []CheckProblem{},
	}

	inspectedExists := make(map[string]bool, len(inspections))
	for _, inspection := range inspections {
		inspectedExists[inspection.Path] = inspection.Exists
	}
	for _, input := range step.Inputs {
		if !input.Required {
			continue
		}
		exists, inspected := inspectedExists[input.Path]
		if !inspected {
			exists = FileExists(projectRoot, input.Path)
		}
		if !exists {
			result.MissingInputs = append(result.MissingInputs, input.Path)
		}
	}

	attempt, _, hasCurrentAttempt := currentState.CurrentAttempt()
	for _, inspection := range inspections {
		if !inspection.Required {
			continue
		}
		switch inspection.Problem {
		case ArtifactEvidenceMissing:
			result.MissingEvidence = append(result.MissingEvidence, inspection.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: inspection.Path, Kind: inspection.Problem})
		case ArtifactFileMissing:
			result.MissingArtifacts = append(result.MissingArtifacts, inspection.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: inspection.Path, Kind: inspection.Problem})
		case ArtifactUnsafe:
			result.UnsafeArtifacts = append(result.UnsafeArtifacts, inspection.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: inspection.Path, Kind: inspection.Problem})
		case ArtifactMismatch:
			result.MismatchedArtifacts = append(result.MismatchedArtifacts, inspection.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: inspection.Path, Kind: inspection.Problem})
		}
	}

	for _, checkID := range step.RequiredChecks {
		checkResult, ok := attempt.CheckResults[checkID]
		if !hasCurrentAttempt || attempt.StepID != step.ID || !ok {
			result.CheckProblems = append(result.CheckProblems, CheckProblem{CheckID: checkID, Kind: CheckMissing})
			continue
		}
		if checkResult.ExitCode != 0 {
			result.CheckProblems = append(result.CheckProblems, CheckProblem{CheckID: checkID, Kind: CheckFailed})
		}
	}

	if step.Approval != nil && step.Approval.Required {
		attempt, _, ok := currentState.CurrentAttempt()
		if !ok || attempt.Status != statepkg.StepAttemptActive || attempt.StepID != step.ID || attempt.Approval == nil {
			result.MissingApprovals = append(result.MissingApprovals, step.ID)
		}
	}

	result.OK = len(result.MissingInputs) == 0 && len(result.MissingEvidence) == 0 && len(result.MissingArtifacts) == 0 && len(result.UnsafeArtifacts) == 0 && len(result.MismatchedArtifacts) == 0 && len(result.MissingApprovals) == 0 && len(result.CheckProblems) == 0
	return result
}

func InspectRequiredArtifacts(step flow.Step, currentState statepkg.State, projectRoot string) []ArtifactInspection {
	required := step
	required.Artifacts = make([]flow.Artifact, 0, len(step.Artifacts))
	for _, artifact := range step.Artifacts {
		if artifact.Required {
			required.Artifacts = append(required.Artifacts, artifact)
		}
	}
	return InspectArtifacts(required, currentState, projectRoot)
}

// InspectArtifacts reads each artifact at most once and applies the same
// filesystem policy used by evidence recording and the done gate.
func InspectArtifacts(step flow.Step, currentState statepkg.State, projectRoot string) []ArtifactInspection {
	return inspectArtifacts(step, currentState, projectRoot, artifact.ReadFile)
}

func inspectArtifacts(step flow.Step, currentState statepkg.State, projectRoot string, readFile func(string, string) (artifact.FileEvidence, error)) []ArtifactInspection {
	attempt, _, hasCurrentAttempt := currentState.CurrentAttempt()
	result := make([]ArtifactInspection, 0, len(step.Artifacts))
	for _, requirement := range step.Artifacts {
		inspection := ArtifactInspection{Path: requirement.Path, Required: requirement.Required}
		actual, err := readFile(projectRoot, requirement.Path)
		if err == nil {
			inspection.Exists = true
		}
		if !requirement.Required {
			result = append(result, inspection)
			continue
		}
		recorded, ok := attempt.ArtifactEvidence[requirement.Path]
		if !hasCurrentAttempt || attempt.Status != statepkg.StepAttemptActive || attempt.StepID != step.ID || !ok {
			inspection.Problem = ArtifactEvidenceMissing
			result = append(result, inspection)
			continue
		}
		inspection.Evidence = &statepkg.ArtifactEvidence{Digest: recorded.Digest, Size: recorded.Size}
		if err != nil {
			matches := false
			inspection.MatchesEvidence = &matches
			if errors.Is(err, artifact.ErrMissing) {
				inspection.Problem = ArtifactFileMissing
			} else {
				inspection.Problem = ArtifactUnsafe
			}
			result = append(result, inspection)
			continue
		}
		matches := actual.Digest == recorded.Digest && actual.Size == recorded.Size
		inspection.MatchesEvidence = &matches
		if !matches {
			inspection.Problem = ArtifactMismatch
		}
		result = append(result, inspection)
	}
	return result
}

func FileExists(projectRoot string, artifactPath string) bool {
	_, err := artifact.ReadFile(projectRoot, artifactPath)
	return err == nil
}
