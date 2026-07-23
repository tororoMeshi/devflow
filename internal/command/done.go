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

	attempt, _, ok := active.State.CurrentAttempt()
	if !ok {
		return commandFailure(CodeInvalidState)
	}
	inspections := gate.NewInspectionSet()
	completion := gate.InspectCompletionGate(ctx.ProjectRoot, active.State, active.CurrentStep, attempt, inspections)
	if !completion.Ready {
		return CommandResult{ExitCode: 1, Diagnostics: completionGateDiagnostics(active.CurrentStep.ID, completion)}
	}
	nextStep, hasNext, valid := transition.ResolveNextStep(active.Flow, active.CurrentStep.ID)
	if !valid {
		return commandFailure(CodeStateStepNotInFlow)
	}
	if hasNext {
		entry := gate.InspectEntryGate(ctx.ProjectRoot, nextStep, inspections)
		if !entry.Ready {
			return CommandResult{ExitCode: 1, Diagnostics: entryGateDiagnostics(nextStep.ID, entry)}
		}
	}
	result := transition.ApplyDone(active.Flow, active.State)
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
