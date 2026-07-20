package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/task"
)

func ApplyStart(snapshot flow.FlowSnapshot, taskSnapshot task.TaskSnapshot, current *state.State, flowRunID string) TransitionResult {
	if len(snapshot.Flow.Steps) == 0 {
		return failure(errorDiagnostic(CodeFlowHasNoSteps, ""))
	}
	if current != nil && current.Status == state.StatusRunning {
		return failure(errorDiagnostic(CodeFlowAlreadyRunning, current.CurrentStepID))
	}

	next := state.State{
		SchemaVersion:        state.CurrentSchemaVersion,
		FlowSnapshot:         flow.CloneSnapshot(snapshot),
		TaskSnapshot:         taskSnapshot,
		Status:               state.StatusRunning,
		CurrentStepID:        snapshot.Flow.Steps[0].ID,
		FlowRunID:            flowRunID,
		CurrentEntrySequence: 1,
	}
	next.Normalize()

	return success(next)
}
