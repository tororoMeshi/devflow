package gate

import (
	"os"
	"path/filepath"

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

	for _, artifact := range step.Artifacts {
		if !artifact.Required {
			continue
		}
		if !FileExists(projectRoot, artifact.Path) {
			result.MissingArtifacts = append(result.MissingArtifacts, artifact.Path)
		}
	}

	if step.Approval != nil && step.Approval.Required {
		attempt, _, ok := currentState.CurrentAttempt()
		if !ok || attempt.Status != statepkg.StepAttemptActive || attempt.StepID != step.ID || attempt.Approval == nil {
			result.MissingApprovals = append(result.MissingApprovals, step.ID)
		}
	}

	attempt, _, hasCurrentAttempt := currentState.CurrentAttempt()
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

	result.OK = len(result.MissingInputs) == 0 && len(result.MissingArtifacts) == 0 && len(result.MissingApprovals) == 0 && len(result.CheckProblems) == 0
	return result
}

func FileExists(projectRoot string, artifactPath string) bool {
	info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(artifactPath)))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
