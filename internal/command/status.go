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
			EntrySequence:    active.State.CurrentEntrySequence,
			Checks:           checkStatusResults(active),
		},
	}
}

func checkStatusResults(active ActiveFlow) []CheckStatusResult {
	results := make([]CheckStatusResult, 0, len(active.CurrentStep.RequiredChecks))
	for _, checkID := range active.CurrentStep.RequiredChecks {
		stored, ok := active.State.CheckResults[checkID]
		if !ok || stored.EntrySequence != active.State.CurrentEntrySequence {
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
