package command

import (
	"strings"

	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func Skip(ctx Context, reason string) CommandResult {
	if strings.TrimSpace(reason) == "" {
		return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{{Level: transition.LevelError, Code: transition.CodeEmptyReason}}}
	}
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	nextStep, hasNext, valid := transition.ResolveNextStep(active.Flow, active.CurrentStep.ID)
	if !valid {
		return commandFailure(CodeStateStepNotInFlow)
	}
	if hasNext {
		entry := gate.InspectEntryGate(ctx.ProjectRoot, nextStep, gate.NewInspectionSet())
		if !entry.Ready {
			return CommandResult{ExitCode: 1, Diagnostics: entryGateDiagnostics(nextStep.ID, entry)}
		}
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
