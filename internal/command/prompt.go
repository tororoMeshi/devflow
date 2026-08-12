package command

import (
	"github.com/tororoMeshi/devflow/internal/gate"
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

	return CommandResult{
		ExitCode: 0,
		Prompt: &PromptResult{
			FlowID:               active.Flow.ID,
			TaskContent:          active.State.TaskSnapshot.Content,
			CurrentStepID:        active.CurrentStep.ID,
			CurrentStepTitle:     active.CurrentStep.Title,
			CurrentStepObjective: active.CurrentStep.Objective,
			RequiredArtifacts:    requiredArtifacts,
			OptionalArtifacts:    optionalArtifacts,
			RequiredApproval:     requiredApproval,
			RequiredChecks:       append([]string(nil), active.CurrentStep.RequiredChecks...),
			ArtifactBlockers:     promptArtifactBlockers(inspections),
			CheckBlockers:        promptCheckBlockers(completion),
			CompletionBlockers:   promptCompletionBlockers(completion),
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

func promptCheckBlockers(result gate.CompletionGateResult) []string {
	blockers := []string{}
	for _, blocker := range result.Blockers {
		if blocker.Kind == gate.CompletionBlockerFailedCheck {
			blockers = append(blockers, blocker.CheckID+": recorded check failed")
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
		blockers = append(blockers, inspection.Path+": "+artifactStatusState(inspection.Problem)+"; recorded evidence is no longer current")
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
