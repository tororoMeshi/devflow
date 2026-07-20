package command

func Prompt(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	requiredArtifacts, optionalArtifacts := promptArtifacts(active)
	requiredApproval := promptRequiredApproval(active)

	return CommandResult{
		ExitCode: 0,
		Prompt: &PromptResult{
			FlowID:                 active.Flow.ID,
			TaskContent:            active.State.TaskSnapshot.Content,
			CurrentStepID:          active.CurrentStep.ID,
			CurrentStepTitle:       active.CurrentStep.Title,
			CurrentStepInstruction: active.CurrentStep.Instruction,
			RequiredArtifacts:      requiredArtifacts,
			OptionalArtifacts:      optionalArtifacts,
			RequiredApproval:       requiredApproval,
			RequiredChecks:         append([]string(nil), active.CurrentStep.RequiredChecks...),
			AfterCompleting:        promptAfterCompleting(requiredApproval),
		},
	}
}

func promptArtifacts(active ActiveFlow) ([]ArtifactResult, []ArtifactResult) {
	required := []ArtifactResult{}
	var optional []ArtifactResult

	for _, artifact := range active.CurrentStep.Artifacts {
		result := ArtifactResult{Path: artifact.Path}
		if artifact.Required {
			required = append(required, result)
			continue
		}
		optional = append(optional, result)
	}

	return required, optional
}

func promptRequiredApproval(active ActiveFlow) *RequiredApprovalResult {
	if active.CurrentStep.Approval == nil || !active.CurrentStep.Approval.Required {
		return nil
	}
	attempt, _, ok := active.State.CurrentAttempt()
	if !ok {
		return nil
	}
	return &RequiredApprovalResult{StepID: active.CurrentStep.ID, AttemptID: attempt.ID}
}

func promptAfterCompleting(approval *RequiredApprovalResult) AfterCompletingResult {
	if approval != nil {
		return AfterCompletingResult{Commands: []string{
			`devflow approve --step "` + approval.StepID + `" --attempt "` + approval.AttemptID + `" --note "<note>"`,
			"devflow done",
		}}
	}
	return AfterCompletingResult{Commands: []string{"devflow done"}}
}
