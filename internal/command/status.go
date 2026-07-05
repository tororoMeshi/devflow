package command

func Status(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
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
			Approvals:        approvalResults(active),
		},
	}
}

func skippedStepResults(active ActiveFlow) map[string]SkippedStepResult {
	results := make(map[string]SkippedStepResult, len(active.State.SkippedSteps))
	for stepID, skipped := range active.State.SkippedSteps {
		results[stepID] = SkippedStepResult{Reason: skipped.Reason}
	}
	return results
}

func approvalResults(active ActiveFlow) map[string]ApprovalResult {
	results := make(map[string]ApprovalResult, len(active.State.Approvals))
	for stepID, approval := range active.State.Approvals {
		results[stepID] = ApprovalResult{
			Approved: approval.Approved,
			Note:     approval.Note,
		}
	}
	return results
}
