package command

import (
	"sort"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/workpackage"
)

const completionContextSchemaVersion = 1

type CompletionContextResult struct {
	SchemaVersion    int                       `json:"schema_version"`
	FlowRunID        string                    `json:"flow_run_id"`
	StepID           string                    `json:"step_id"`
	AttemptID        string                    `json:"attempt_id"`
	AttemptStatus    state.StepAttemptStatus   `json:"attempt_status"`
	IsCurrentAttempt bool                      `json:"is_current_attempt"`
	Artifacts        []CompletionArtifact      `json:"artifacts"`
	Checks           []CompletionCheck         `json:"checks"`
	Approval         CompletionApproval        `json:"approval"`
	Completion       CompletionContextDecision `json:"completion"`
}

type CompletionArtifact struct {
	Path     string  `json:"path"`
	Required bool    `json:"required"`
	Status   string  `json:"status"`
	Digest   *string `json:"digest"`
	Size     *int64  `json:"size"`
}

type CompletionCheck struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

type CompletionApproval struct {
	Required                  bool    `json:"required"`
	Status                    string  `json:"status"`
	EvidenceSetDigest         *string `json:"evidence_set_digest"`
	ApprovedEvidenceSetDigest *string `json:"approved_evidence_set_digest"`
}

type CompletionContextDecision struct {
	Status  string                    `json:"status"`
	Blocker *CompletionContextBlocker `json:"blocker"`
}

type CompletionContextBlocker struct {
	Code      string  `json:"code"`
	SubjectID *string `json:"subject_id"`
}

// CompletionContext returns the immutable completion projection for one StepAttempt.
func CompletionContext(ctx Context, stepID, attemptID string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	if !state.IsValidStepAttemptID(attemptID) {
		return workPackageFailure(workpackage.ErrInvalidAttemptID)
	}

	attempt, ok := completionAttempt(active.State, attemptID)
	if !ok {
		return workPackageFailure(workpackage.ErrAttemptNotFound)
	}
	if attempt.StepID != stepID {
		return workPackageFailure(workpackage.ErrStepAttemptMismatch)
	}
	step, ok := findStep(active.Flow, stepID)
	if !ok {
		return workPackageFailure(workpackage.ErrStepAttemptMismatch)
	}

	current, _, hasCurrent := active.State.CurrentAttempt()
	isCurrent := hasCurrent && current.ID == attempt.ID
	result := CompletionContextResult{
		SchemaVersion:    completionContextSchemaVersion,
		FlowRunID:        active.State.FlowRunID,
		StepID:           stepID,
		AttemptID:        attemptID,
		AttemptStatus:    attempt.Status,
		IsCurrentAttempt: isCurrent,
		Artifacts:        make([]CompletionArtifact, 0),
		Checks:           completionChecks(step, attempt),
		Approval:         completionApproval(step, attempt),
	}

	switch {
	case attempt.Status == state.StepAttemptClosed:
		result.Artifacts = completionArtifacts(ctx.ProjectRoot, step, attempt)
		result.Completion = notApplicableCompletion("attempt_closed")
	case attempt.Status == state.StepAttemptActive && !isCurrent:
		result.Artifacts = completionArtifacts(ctx.ProjectRoot, step, attempt)
		result.Completion = notApplicableCompletion("attempt_not_current")
	case attempt.Status == state.StepAttemptActive:
		set := gate.NewInspectionSet()
		result.Artifacts = completionArtifactsFromInspections(gate.InspectArtifacts(ctx.ProjectRoot, step, attempt, set))
		result.Completion = completionGateDecision(gate.InspectCompletionGate(ctx.ProjectRoot, active.State, step, attempt, set))
	default:
		return commandFailure(CodeInvalidState)
	}
	return CommandResult{ExitCode: 0, CompletionContext: &result}
}

func completionAttempt(st state.State, attemptID string) (state.StepAttempt, bool) {
	for _, attempt := range st.Attempts {
		if attempt.ID == attemptID {
			return attempt, true
		}
	}
	return state.StepAttempt{}, false
}

func completionArtifacts(projectRoot string, step flow.Step, attempt state.StepAttempt) []CompletionArtifact {
	return completionArtifactsFromInspections(gate.InspectArtifacts(projectRoot, step, attempt, gate.NewInspectionSet()))
}

func completionArtifactsFromInspections(inspections []gate.ArtifactInspection) []CompletionArtifact {
	result := make([]CompletionArtifact, 0, len(inspections))
	for _, inspection := range inspections {
		artifact := CompletionArtifact{Path: inspection.Path, Required: inspection.Required, Status: completionArtifactStatus(inspection)}
		if inspection.Evidence != nil {
			digest, size := inspection.Evidence.Digest, inspection.Evidence.Size
			artifact.Digest, artifact.Size = &digest, &size
		}
		result = append(result, artifact)
	}
	return result
}

func completionArtifactStatus(inspection gate.ArtifactInspection) string {
	switch inspection.Problem {
	case gate.CompletionBlockerMissingArtifact, gate.CompletionBlockerMissingArtifactEvidence:
		return "missing"
	case gate.CompletionBlockerArtifactUnavailable:
		return "unavailable"
	case gate.CompletionBlockerArtifactEvidenceMismatch:
		return "mismatch"
	default:
		if inspection.Evidence == nil {
			return "missing"
		}
		return "recorded"
	}
}

func completionChecks(step flow.Step, attempt state.StepAttempt) []CompletionCheck {
	result := make([]CompletionCheck, 0, len(step.RequiredChecks))
	for _, checkID := range step.RequiredChecks {
		stored, ok := attempt.CheckResults[checkID]
		if !ok {
			result = append(result, CompletionCheck{ID: checkID, Status: string(CheckStatusPending)})
			continue
		}
		exitCode := stored.ExitCode
		status := string(CheckStatusPassed)
		if exitCode != 0 {
			status = string(CheckStatusFailed)
		}
		result = append(result, CompletionCheck{ID: checkID, Status: status, ExitCode: &exitCode})
	}
	return result
}

func completionApproval(step flow.Step, attempt state.StepAttempt) CompletionApproval {
	required := step.Approval != nil && step.Approval.Required
	result := CompletionApproval{Required: required}
	if !required {
		result.Status = "not_required"
		return result
	}
	canComputeEvidenceSetDigest := true
	for _, artifact := range step.Artifacts {
		if artifact.Required {
			if _, ok := attempt.ArtifactEvidence[artifact.Path]; !ok {
				canComputeEvidenceSetDigest = false
				break
			}
		}
	}
	if canComputeEvidenceSetDigest {
		evidencePaths := make([]string, 0, len(attempt.ArtifactEvidence))
		for path := range attempt.ArtifactEvidence {
			evidencePaths = append(evidencePaths, path)
		}
		sort.Strings(evidencePaths)
		if digest, err := state.ArtifactEvidenceSetDigest(evidencePaths, attempt.ArtifactEvidence); err == nil {
			result.EvidenceSetDigest = &digest
		}
	}
	if attempt.Approval == nil {
		result.Status = "pending"
		return result
	}
	approved := attempt.Approval.EvidenceSetDigest
	result.Status, result.ApprovedEvidenceSetDigest = "approved", &approved
	return result
}

func completionGateDecision(gateResult gate.CompletionGateResult) CompletionContextDecision {
	if gateResult.Ready {
		return CompletionContextDecision{Status: "ready"}
	}
	blocker := gateResult.Blockers[0]
	return CompletionContextDecision{Status: "blocked", Blocker: completionBlocker(blocker)}
}

func completionBlocker(blocker gate.CompletionBlocker) *CompletionContextBlocker {
	result := &CompletionContextBlocker{Code: string(blocker.Kind)}
	if blocker.Path != "" {
		result.SubjectID = &blocker.Path
	} else if blocker.CheckID != "" {
		result.SubjectID = &blocker.CheckID
	}
	return result
}

func notApplicableCompletion(code string) CompletionContextDecision {
	return CompletionContextDecision{Status: "not_applicable", Blocker: &CompletionContextBlocker{Code: code}}
}
