package command

import (
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func Done(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	gateResult := gate.CheckDoneGate(active.CurrentStep, active.State, ctx.ProjectRoot)
	result := transition.ApplyDone(active.Flow, active.State, gateResult)
	return transitionCommandResult(ctx, result, doneSuccess(active, result))
}

func doneSuccess(active ActiveFlow, result transition.TransitionResult) *SuccessResult {
	if result.State == nil {
		return nil
	}
	success := &SuccessResult{CompletedStepID: active.CurrentStep.ID}
	if result.State.Status == state.StatusCompleted {
		success.CompletedFlowID = active.Flow.ID
	} else {
		success.NextStepID = result.State.CurrentStepID
	}
	return success
}
