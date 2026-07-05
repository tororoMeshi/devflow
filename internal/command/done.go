package command

import (
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/transition"
)

func Done(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	gateResult := gate.CheckDoneGate(active.CurrentStep, active.State, ctx.ProjectRoot)
	result := transition.ApplyDone(active.Flow, active.State, gateResult)
	return transitionCommandResult(ctx, result)
}
