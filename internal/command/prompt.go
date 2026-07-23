package command

import (
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
)

func Prompt(ctx Context) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	requiredArtifacts, optionalArtifacts := promptArtifacts(active)
	requiredApproval := promptRequiredApproval(active)
	gateResult, inspections := gate.InspectDoneGate(active.CurrentStep, active.State, ctx.ProjectRoot)

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
			AfterCompleting:        promptAfterCompleting(active, inspections, requiredApproval, gateResult),
			ArtifactBlockers:       promptArtifactBlockers(inspections),
			CheckBlockers:          promptCheckBlockers(gateResult),
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

func promptAfterCompleting(active ActiveFlow, inspections []gate.ArtifactInspection, approval *RequiredApprovalResult, gateResult gate.Result) AfterCompletingResult {
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
		if inspection.Required && inspection.Problem == gate.ArtifactEvidenceMissing {
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
	if gateResult.OK {
		commands = append(commands, "devflow done")
	}
	return AfterCompletingResult{Commands: commands}
}

func promptCheckBlockers(result gate.Result) []string {
	blockers := []string{}
	for _, problem := range result.CheckProblems {
		if problem.Kind == gate.CheckFailed {
			blockers = append(blockers, problem.CheckID+": recorded check failed; continue in a new attempt to record a different result")
		}
	}
	return blockers
}

func promptArtifactBlockers(inspections []gate.ArtifactInspection) []string {
	blockers := []string{}
	for _, inspection := range inspections {
		if !inspection.Required || inspection.Problem == "" || inspection.Problem == gate.ArtifactEvidenceMissing {
			continue
		}
		blockers = append(blockers, inspection.Path+": "+artifactStatusState(inspection.Problem)+"; recorded evidence is no longer current; continue in a new attempt")
	}
	return blockers
}
