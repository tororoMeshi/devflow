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
			CurrentStepID:          active.CurrentStep.ID,
			CurrentStepTitle:       active.CurrentStep.Title,
			CurrentStepInstruction: active.CurrentStep.Instruction,
			RequiredArtifacts:      requiredArtifacts,
			OptionalArtifacts:      optionalArtifacts,
			RequiredApproval:       requiredApproval,
			RequiredChecks:         append([]string(nil), active.CurrentStep.RequiredChecks...),
			AfterCompleting:        promptAfterCompleting(requiredApproval != nil),
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
	return &RequiredApprovalResult{StepID: active.CurrentStep.ID}
}

func promptAfterCompleting(requiresApproval bool) AfterCompletingResult {
	if requiresApproval {
		return AfterCompletingResult{Commands: []string{
			`devflow approve --note "<note>"`,
			"devflow done",
		}}
	}
	return AfterCompletingResult{Commands: []string{"devflow done"}}
}
