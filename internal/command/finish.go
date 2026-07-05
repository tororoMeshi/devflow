package command

import "github.com/8noki8/devflow/internal/transition"

func Finish(ctx Context, reason string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := transition.ApplyFinish(active.State, reason)
	return transitionCommandResult(ctx, result)
}
