package command

import (
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func Skip(ctx Context, reason string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := transition.ApplySkip(active.Flow, active.State, reason)
	return transitionCommandResult(ctx, result, skipSuccess(active, result))
}

func skipSuccess(active ActiveFlow, result transition.TransitionResult) *SuccessResult {
	if result.State == nil {
		return nil
	}
	success := &SuccessResult{SkippedStepID: active.CurrentStep.ID}
	if result.State.Status == state.StatusCompleted {
		success.CompletedFlowID = active.Flow.ID
	} else {
		success.NextStepID = result.State.CurrentStepID
	}
	return success
}
