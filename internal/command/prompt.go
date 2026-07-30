package command

import (
	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func Prompt(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	requiredArtifacts, optionalArtifacts := promptArtifacts(active)
	requiredApproval := promptRequiredApproval(active)
	attempt, _, ok := active.State.CurrentAttempt()
	if !ok {
		return commandFailure(CodeInvalidState)
	}
	inspectionSet := gate.NewInspectionSet()
	completion := gate.InspectCompletionGate(ctx.ProjectRoot, active.State, active.CurrentStep, attempt, inspectionSet)
	inspections := gate.InspectArtifacts(ctx.ProjectRoot, active.CurrentStep, attempt, inspectionSet)
	nextEntry := gate.EntryGateResult{Ready: true, Blockers: []gate.EntryBlocker{}}
	var nextStepID string
	if completion.Ready {
		if nextStep, hasNext, valid := transition.ResolveNextStep(active.Flow, active.CurrentStep.ID); valid && hasNext {
			nextStepID = nextStep.ID
			nextEntry = gate.InspectEntryGate(ctx.ProjectRoot, nextStep, inspectionSet)
		} else if !valid {
			return commandFailure(CodeStateStepNotInFlow)
		}
	}

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
			AfterCompleting:        promptAfterCompleting(active, inspections, requiredApproval, completion, nextEntry),
			ArtifactBlockers:       promptArtifactBlockers(inspections),
			CheckBlockers:          promptCheckBlockers(completion),
			CompletionBlockers:     promptCompletionBlockers(completion),
			NextEntryBlockers:      promptNextEntryBlockers(nextStepID, completion, nextEntry),
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

func promptAfterCompleting(active ActiveFlow, inspections []gate.ArtifactInspection, approval *RequiredApprovalResult, completion gate.CompletionGateResult, nextEntry gate.EntryGateResult) AfterCompletingResult {
	attempt, _, ok := active.State.CurrentAttempt()
	commands := []string{}
	if active.State.Status != state.StatusRunning || !ok {
		return AfterCompletingResult{Commands: commands}
	}
	for _, checkID := range active.CurrentStep.RequiredChecks {
		if _, recorded := attempt.CheckResults[checkID]; !recorded {
			commands = append(commands, `devflow check request --step "`+active.CurrentStep.ID+`" --attempt "`+attempt.ID+`" --check "`+checkID+`"`)
		}
	}
	for _, inspection := range inspections {
		if inspection.Required && inspection.Problem == gate.CompletionBlockerMissingArtifactEvidence {
			commands = append(commands, `devflow artifact record --step "`+active.CurrentStep.ID+`" --attempt "`+attempt.ID+`" --path "`+inspection.Path+`"`)
		}
	}
	hasArtifactBlocker := false
	for _, inspection := range inspections {
		if inspection.Required && inspection.Problem != "" {
			hasArtifactBlocker = true
		}
	}
	if !hasArtifactBlocker && approval != nil && ok && attempt.Approval == nil {
		commands = append(commands,
			`devflow approve --step "`+approval.StepID+`" --attempt "`+approval.AttemptID+`" --note "<note>"`,
		)
	}
	if completion.Ready && nextEntry.Ready {
		commands = append(commands, "devflow done")
	}
	return AfterCompletingResult{Commands: commands}
}

func promptCheckBlockers(result gate.CompletionGateResult) []string {
	blockers := []string{}
	for _, blocker := range result.Blockers {
		if blocker.Kind == gate.CompletionBlockerFailedCheck {
			blockers = append(blockers, blocker.CheckID+": recorded check failed; continue in a new attempt to record a different result")
		}
	}
	return blockers
}

func promptArtifactBlockers(inspections []gate.ArtifactInspection) []string {
	blockers := []string{}
	for _, inspection := range inspections {
		if !inspection.Required || inspection.Problem == "" || inspection.Problem == gate.CompletionBlockerMissingArtifactEvidence {
			continue
		}
		blockers = append(blockers, inspection.Path+": "+artifactStatusState(inspection.Problem)+"; recorded evidence is no longer current; continue in a new attempt")
	}
	return blockers
}

func promptCompletionBlockers(result gate.CompletionGateResult) []string {
	blockers := []string{}
	for _, blocker := range result.Blockers {
		if blocker.Kind == gate.CompletionBlockerMissingInput || blocker.Kind == gate.CompletionBlockerInputUnavailable {
			blockers = append(blockers, blocker.Path+": "+string(blocker.Kind))
		}
	}
	return blockers
}

func promptNextEntryBlockers(stepID string, completion gate.CompletionGateResult, entry gate.EntryGateResult) []string {
	if !completion.Ready || entry.Ready {
		return []string{}
	}
	blockers := make([]string, 0, len(entry.Blockers))
	for _, blocker := range entry.Blockers {
		blockers = append(blockers, stepID+": "+blocker.Path+": "+string(blocker.Kind))
	}
	return blockers
}
