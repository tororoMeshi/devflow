package command

import (
	"strings"

	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func Back(ctx Context, toStepID string, reason string) CommandResult {
	if strings.TrimSpace(reason) == "" {
		return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{{Level: transition.LevelError, Code: transition.CodeEmptyReason}}}
	}
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	destination, ok := transition.ResolveBackStep(active.Flow, active.CurrentStep.ID, toStepID)
	if !ok {
		return transitionCommandResult(ctx, transition.ApplyBack(active.Flow, active.State, toStepID, reason), nil)
	}
	entry := gate.InspectEntryGate(ctx.ProjectRoot, destination, gate.NewInspectionSet())
	if !entry.Ready {
		return CommandResult{ExitCode: 1, Diagnostics: entryGateDiagnostics(destination.ID, entry)}
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
