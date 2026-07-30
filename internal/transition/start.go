package transition

import (
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
)

func ApplyStart(snapshot flow.FlowSnapshot, taskSnapshot task.TaskSnapshot, current *state.State, flowRunID string) TransitionResult {
	if len(snapshot.Flow.Steps) == 0 {
		return failure(errorDiagnostic(CodeFlowHasNoSteps, ""))
	}
	if current != nil && current.Status == state.StatusRunning {
		return failure(errorDiagnostic(CodeFlowAlreadyRunning, current.CurrentStepID))
	}
	firstAttempt, err := state.NewStepAttempt(snapshot.Flow.Steps[0].ID, 1)
	if err != nil {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, snapshot.Flow.Steps[0].ID))
	}

	next := state.State{
		SchemaVersion:    state.CurrentSchemaVersion,
		FlowSnapshot:     flow.CloneSnapshot(snapshot),
		TaskSnapshot:     taskSnapshot,
		Status:           state.StatusRunning,
		CurrentStepID:    snapshot.Flow.Steps[0].ID,
		FlowRunID:        flowRunID,
		Attempts:         []state.StepAttempt{firstAttempt},
		CurrentAttemptID: firstAttempt.ID,
	}
	next.Normalize()

	return success(next)
}
