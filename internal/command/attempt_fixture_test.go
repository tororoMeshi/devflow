package command

import (
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
)

func commandStateWithAttempt(snapshot flow.FlowSnapshot, taskSnapshot task.TaskSnapshot, status state.Status, stepID, runID string) state.State {
	attempt, err := state.NewStepAttempt(stepID, 1)
	if err != nil {
		panic(err)
	}
	result := state.State{
		SchemaVersion:    state.CurrentSchemaVersion,
		FlowSnapshot:     snapshot,
		TaskSnapshot:     taskSnapshot,
		Status:           status,
		CurrentStepID:    stepID,
		FlowRunID:        runID,
		Attempts:         []state.StepAttempt{attempt},
		CurrentAttemptID: attempt.ID,
	}
	if status != state.StatusRunning {
		exit := state.StepAttemptExitDone
		reason := ""
		if status == state.StatusFinished {
			exit = state.StepAttemptExitFinish
			reason = "finished"
			result.Finish = &state.Finish{Reason: reason}
		}
		result.Attempts[0], err = state.CloseStepAttempt(attempt, exit, reason)
		if err != nil {
			panic(err)
		}
		result.CurrentAttemptID = ""
	}
	result.Normalize()
	return result
}
