package command

import "github.com/8noki8/devflow/internal/transition"

func Back(ctx Context, toStepID string, reason string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := transition.ApplyBack(active.Flow, active.State, toStepID, reason)
	return transitionCommandResult(ctx, result, backSuccess(result))
}

func backSuccess(result transition.TransitionResult) *SuccessResult {
	if result.State == nil {
		return nil
	}
	return &SuccessResult{MovedBackToID: result.State.CurrentStepID}
}
