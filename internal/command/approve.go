package command

import "github.com/8noki8/devflow/internal/transition"

func Approve(ctx Context, stepID string, note string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := transition.ApplyApprove(active.Flow, active.State, stepID, note)
	return transitionCommandResult(ctx, result)
}
