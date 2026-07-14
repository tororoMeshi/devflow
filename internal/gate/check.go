package gate

import (
	"os"
	"path/filepath"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func CheckDoneGate(step flow.Step, state state.State, projectRoot string) Result {
	result := Result{
		MissingArtifacts: []string{},
		MissingApprovals: []string{},
		CheckProblems:    []CheckProblem{},
	}

	for _, artifact := range step.Artifacts {
		if !artifact.Required {
			continue
		}
		if !artifactExists(projectRoot, artifact.Path) {
			result.MissingArtifacts = append(result.MissingArtifacts, artifact.Path)
		}
	}

	if step.Approval != nil && step.Approval.Required {
		approval := state.Approvals[step.ID]
		if !approval.Approved {
			result.MissingApprovals = append(result.MissingApprovals, step.ID)
		}
	}

	for _, checkID := range step.RequiredChecks {
		checkResult, ok := state.CheckResults[checkID]
		if !ok || checkResult.EntrySequence != state.CurrentEntrySequence {
			result.CheckProblems = append(result.CheckProblems, CheckProblem{CheckID: checkID, Kind: CheckMissing})
			continue
		}
		if checkResult.ExitCode != 0 {
			result.CheckProblems = append(result.CheckProblems, CheckProblem{CheckID: checkID, Kind: CheckFailed})
		}
	}

	result.OK = len(result.MissingArtifacts) == 0 && len(result.MissingApprovals) == 0 && len(result.CheckProblems) == 0
	return result
}

func artifactExists(projectRoot string, artifactPath string) bool {
	info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(artifactPath)))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
