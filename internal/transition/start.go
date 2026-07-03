package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func ApplyStart(flow flow.Flow, current *state.State) TransitionResult {
	if len(flow.Steps) == 0 {
		return failure(errorDiagnostic(CodeFlowHasNoSteps, ""))
	}
	if current != nil && current.Status == state.StatusRunning {
		return failure(errorDiagnostic(CodeFlowAlreadyRunning, current.CurrentStepID))
	}

	next := state.State{
		FlowID:        flow.ID,
		Status:        state.StatusRunning,
		CurrentStepID: flow.Steps[0].ID,
	}
	next.Normalize()

	return success(next)
}
