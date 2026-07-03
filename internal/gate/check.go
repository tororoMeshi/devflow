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

	result.OK = len(result.MissingArtifacts) == 0 && len(result.MissingApprovals) == 0
	return result
}

func artifactExists(projectRoot string, artifactPath string) bool {
	info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(artifactPath)))
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
