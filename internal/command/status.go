package command

import (
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func Status(ctx Context) CommandResult {
	loaded := NewStore(ctx).LoadCurrent()
	if loaded.Status == state.LoadNoState {
		return commandFailure(CodeNoActiveFlow)
	}
	if loaded.Status == state.LoadInvalid {
		if isUnsupportedStateVersion(loaded.Err) {
			return CommandResult{ExitCode: 1, Diagnostics: []transition.Diagnostic{unsupportedStateVersionDiagnostic()}}
		}
		return commandFailure(CodeInvalidState)
	}
	if loaded.State == nil {
		return commandFailure(CodeInvalidState)
	}
	active, diagnostics := activeFlowFromState(*loaded.State)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	return CommandResult{
		ExitCode: 0,
		Status: &StatusResult{
			FlowID:           active.Flow.ID,
			FlowTitle:        active.Flow.Title,
			CurrentStepID:    active.CurrentStep.ID,
			CurrentStepTitle: active.CurrentStep.Title,
			CompletedSteps:   append([]string(nil), active.State.CompletedSteps...),
			SkippedSteps:     skippedStepResults(active),
			Approval:         approvalResult(active),
			EntrySequence:    active.State.EntrySequence(),
			Checks:           checkStatusResults(active),
		},
	}
}

func checkStatusResults(active ActiveFlow) []CheckStatusResult {
	results := make([]CheckStatusResult, 0, len(active.CurrentStep.RequiredChecks))
	attempt, _, hasCurrentAttempt := active.State.CurrentAttempt()
	for _, checkID := range active.CurrentStep.RequiredChecks {
		stored, ok := attempt.CheckResults[checkID]
		if !hasCurrentAttempt || attempt.StepID != active.CurrentStep.ID || !ok {
			results = append(results, CheckStatusResult{CheckID: checkID, Status: "pending"})
			continue
		}
		exitCode := stored.ExitCode
		status := "failed"
		if exitCode == 0 {
			status = "passed"
		}
		results = append(results, CheckStatusResult{CheckID: checkID, Status: status, ExitCode: &exitCode, LogPath: stored.LogPath})
	}
	return results
}

func skippedStepResults(active ActiveFlow) map[string]SkippedStepResult {
	results := make(map[string]SkippedStepResult, len(active.State.SkippedSteps))
	for stepID, skipped := range active.State.SkippedSteps {
		results[stepID] = SkippedStepResult{Reason: skipped.Reason}
	}
	return results
}

func approvalResult(active ActiveFlow) *ApprovalResult {
	attempt, _, ok := active.State.CurrentAttempt()
	if !ok || attempt.Status != state.StepAttemptActive || attempt.Approval == nil {
		return nil
	}
	return &ApprovalResult{StepID: attempt.StepID, Approved: true, Note: attempt.Approval.Note}
}
