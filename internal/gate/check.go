package gate

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/8noki8/devflow/internal/artifact"
	"github.com/8noki8/devflow/internal/flow"
	statepkg "github.com/8noki8/devflow/internal/state"
)

func CheckDoneGate(step flow.Step, currentState statepkg.State, projectRoot string) Result {
	result := Result{
		MissingInputs:    []string{},
		MissingArtifacts: []string{},
		MissingApprovals: []string{},
		CheckProblems:    []CheckProblem{},
	}

	for _, input := range step.Inputs {
		if !input.Required {
			continue
		}
		if !FileExists(projectRoot, input.Path) {
			result.MissingInputs = append(result.MissingInputs, input.Path)
		}
	}

	attempt, _, hasCurrentAttempt := currentState.CurrentAttempt()
	for _, requirement := range step.Artifacts {
		if !requirement.Required {
			continue
		}
		recorded, ok := attempt.ArtifactEvidence[requirement.Path]
		if !hasCurrentAttempt || attempt.StepID != step.ID || !ok {
			result.MissingEvidence = append(result.MissingEvidence, requirement.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: requirement.Path, Kind: ArtifactEvidenceMissing})
			continue
		}
		actual, err := artifact.ReadFile(projectRoot, requirement.Path)
		if err != nil {
			if errors.Is(err, artifact.ErrMissing) {
				result.MissingArtifacts = append(result.MissingArtifacts, requirement.Path)
				result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: requirement.Path, Kind: ArtifactFileMissing})
			} else {
				result.UnsafeArtifacts = append(result.UnsafeArtifacts, requirement.Path)
				result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: requirement.Path, Kind: ArtifactUnsafe})
			}
			continue
		}
		if actual.Digest != recorded.Digest || actual.Size != recorded.Size {
			result.MismatchedArtifacts = append(result.MismatchedArtifacts, requirement.Path)
			result.ArtifactProblems = append(result.ArtifactProblems, ArtifactProblem{Path: requirement.Path, Kind: ArtifactMismatch})
		}
	}

	if step.Approval != nil && step.Approval.Required {
		attempt, _, ok := currentState.CurrentAttempt()
		if !ok || attempt.Status != statepkg.StepAttemptActive || attempt.StepID != step.ID || attempt.Approval == nil {
			result.MissingApprovals = append(result.MissingApprovals, step.ID)
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

	result.OK = len(result.MissingInputs) == 0 && len(result.MissingEvidence) == 0 && len(result.MissingArtifacts) == 0 && len(result.UnsafeArtifacts) == 0 && len(result.MismatchedArtifacts) == 0 && len(result.MissingApprovals) == 0 && len(result.CheckProblems) == 0
	return result
}

func FileExists(projectRoot string, artifactPath string) bool {
	info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(artifactPath)))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
