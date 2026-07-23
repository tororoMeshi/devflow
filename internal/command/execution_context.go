package command

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/task"
	"github.com/8noki8/devflow/internal/transition"
)

const executionContextSchemaVersion = 4

type CheckStatus string

const (
	CheckStatusPending CheckStatus = "pending"
	CheckStatusPassed  CheckStatus = "passed"
	CheckStatusFailed  CheckStatus = "failed"
)

type CompletionBlockerType string

const (
	CompletionBlockerMissingInput             CompletionBlockerType = "missing_input"
	CompletionBlockerInputUnavailable         CompletionBlockerType = "input_unavailable"
	CompletionBlockerMissingArtifact          CompletionBlockerType = "missing_artifact"
	CompletionBlockerMissingArtifactEvidence  CompletionBlockerType = "missing_artifact_evidence"
	CompletionBlockerArtifactEvidenceMismatch CompletionBlockerType = "artifact_evidence_mismatch"
	CompletionBlockerArtifactUnavailable      CompletionBlockerType = "artifact_unavailable"
	CompletionBlockerMissingCheck             CompletionBlockerType = "missing_check"
	CompletionBlockerFailedCheck              CompletionBlockerType = "failed_check"
	CompletionBlockerMissingApproval          CompletionBlockerType = "missing_approval"
)

type ExecutionContextResult struct {
	SchemaVersion int                        `json:"schema_version"`
	FlowRunID     string                     `json:"flow_run_id"`
	Flow          ExecutionFlowResult        `json:"flow"`
	TaskSnapshot  task.TaskSnapshot          `json:"task_snapshot"`
	State         ExecutionStateResult       `json:"state"`
	Attempt       *ExecutionAttemptResult    `json:"attempt"`
	Step          *ExecutionStepResult       `json:"step"`
	Completion    *ExecutionCompletionResult `json:"completion"`
}

type ExecutionFlowResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ExecutionStateResult struct {
	Status state.Status `json:"status"`
}

type ExecutionAttemptResult struct {
	ID            string `json:"id"`
	EntrySequence uint64 `json:"entry_sequence"`
}

type ExecutionStepResult struct {
	ID          string                    `json:"id"`
	Title       string                    `json:"title"`
	Instruction string                    `json:"instruction"`
	Inputs      []ExecutionArtifactResult `json:"inputs"`
	Artifacts   []ExecutionArtifactResult `json:"artifacts"`
	Checks      []ExecutionCheckResult    `json:"checks"`
	Approval    ExecutionApprovalResult   `json:"approval"`
}

type ExecutionArtifactResult struct {
	Path            string                           `json:"path"`
	Required        bool                             `json:"required"`
	Exists          bool                             `json:"exists"`
	Evidence        *ExecutionArtifactEvidenceResult `json:"evidence"`
	MatchesEvidence *bool                            `json:"matches_evidence"`
}

type ExecutionArtifactEvidenceResult struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

type ExecutionCheckResult struct {
	ID     string      `json:"id"`
	Status CheckStatus `json:"status"`
}

type ExecutionApprovalResult struct {
	Required bool `json:"required"`
	Approved bool `json:"approved"`
}

type ExecutionCompletionResult struct {
	Ready    bool                      `json:"ready"`
	Blockers []ExecutionContextBlocker `json:"blockers"`
}

type ExecutionContextBlocker struct {
	Type    CompletionBlockerType `json:"type"`
	Path    string                `json:"path,omitempty"`
	CheckID string                `json:"check_id,omitempty"`
	StepID  string                `json:"step_id,omitempty"`
}

type LoadedExecutionContext struct {
	Flow  flow.Flow
	State state.State
	Step  *flow.Step
}

func CurrentContext(ctx Context) CommandResult {
	loaded, diagnostics := LoadExecutionContext(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := ExecutionContextResult{
		SchemaVersion: executionContextSchemaVersion,
		FlowRunID:     loaded.State.FlowRunID,
		TaskSnapshot:  loaded.State.TaskSnapshot,
		Flow: ExecutionFlowResult{
			ID:    loaded.Flow.ID,
			Title: loaded.Flow.Title,
		},
		State: ExecutionStateResult{
			Status: loaded.State.Status,
		},
	}
	if loaded.Step != nil {
		attempt, _, ok := loaded.State.CurrentAttempt()
		if !ok || attempt.Status != state.StepAttemptActive || attempt.StepID != loaded.Step.ID {
			return commandFailure(CodeInvalidState)
		}
		result.Attempt = &ExecutionAttemptResult{
			ID:            attempt.ID,
			EntrySequence: attempt.EntrySequence,
		}
		inspectionSet := gate.NewInspectionSet()
		gateResult := gate.InspectCompletionGate(ctx.ProjectRoot, loaded.State, *loaded.Step, attempt, inspectionSet)
		inspections := gate.InspectArtifacts(ctx.ProjectRoot, *loaded.Step, attempt, inspectionSet)
		result.Step = executionStep(*loaded.Step, loaded.State, ctx.ProjectRoot, inspections, inspectionSet)
		result.Completion = executionCompletion(gateResult, loaded.Step.ID)
	}

	return CommandResult{ExitCode: 0, ExecutionContext: &result}
}

func LoadExecutionContext(ctx Context) (LoadedExecutionContext, []transition.Diagnostic) {
	loaded := NewStore(ctx).LoadCurrent()
	switch loaded.Status {
	case state.LoadNoState:
		return LoadedExecutionContext{}, []transition.Diagnostic{commandErrorDiagnostic(CodeNoActiveFlow)}
	case state.LoadInvalid:
		if isUnsupportedStateVersion(loaded.Err) {
			return LoadedExecutionContext{}, []transition.Diagnostic{unsupportedStateVersionDiagnostic()}
		}
		return LoadedExecutionContext{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	case state.LoadOK:
		if loaded.State == nil {
			return LoadedExecutionContext{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
		}
		active, diagnostics := activeFlowFromState(*loaded.State)
		if len(diagnostics) > 0 {
			return LoadedExecutionContext{}, diagnostics
		}
		result := LoadedExecutionContext{Flow: active.Flow, State: active.State}
		if active.State.Status == state.StatusRunning {
			step := active.CurrentStep
			result.Step = &step
		}
		return result, nil
	default:
		return LoadedExecutionContext{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	}
}

func executionStep(step flow.Step, current state.State, projectRoot string, inspections []gate.ArtifactInspection, inspectionSet *gate.InspectionSet) *ExecutionStepResult {
	return &ExecutionStepResult{
		ID:          step.ID,
		Title:       step.Title,
		Instruction: step.Instruction,
		Inputs:      executionInputArtifacts(step.Inputs, projectRoot, inspectionSet),
		Artifacts:   executionArtifacts(inspections),
		Checks:      executionChecks(step.RequiredChecks, current),
		Approval:    executionApproval(step, current),
	}
}

func executionArtifacts(inspections []gate.ArtifactInspection) []ExecutionArtifactResult {
	result := make([]ExecutionArtifactResult, 0, len(inspections))
	for _, inspection := range inspections {
		artifact := ExecutionArtifactResult{Path: inspection.Path, Required: inspection.Required, Exists: inspection.Exists, MatchesEvidence: inspection.MatchesEvidence}
		if inspection.Evidence != nil {
			artifact.Evidence = &ExecutionArtifactEvidenceResult{Digest: inspection.Evidence.Digest, Size: inspection.Evidence.Size}
		}
		result = append(result, artifact)
	}
	return result
}

func executionInputArtifacts(artifacts []flow.Artifact, projectRoot string, inspectionSet *gate.InspectionSet) []ExecutionArtifactResult {
	result := make([]ExecutionArtifactResult, 0, len(artifacts))
	for _, artifact := range artifacts {
		exists := gate.FileAvailable(projectRoot, artifact.Path, inspectionSet)
		result = append(result, ExecutionArtifactResult{Path: artifact.Path, Required: artifact.Required, Exists: exists})
	}
	return result
}

func executionChecks(requiredChecks []string, current state.State) []ExecutionCheckResult {
	result := make([]ExecutionCheckResult, 0, len(requiredChecks))
	attempt, _, hasCurrentAttempt := current.CurrentAttempt()
	for _, checkID := range requiredChecks {
		stored, ok := attempt.CheckResults[checkID]
		if !hasCurrentAttempt || !ok {
			result = append(result, ExecutionCheckResult{ID: checkID, Status: CheckStatusPending})
			continue
		}
		status := CheckStatusFailed
		if stored.ExitCode == 0 {
			status = CheckStatusPassed
		}
		result = append(result, ExecutionCheckResult{ID: checkID, Status: status})
	}
	return result
}

func executionApproval(step flow.Step, current state.State) ExecutionApprovalResult {
	result := ExecutionApprovalResult{}
	if step.Approval == nil || !step.Approval.Required {
		return result
	}
	attempt, _, ok := current.CurrentAttempt()
	return ExecutionApprovalResult{
		Required: true,
		Approved: ok && attempt.Status == state.StepAttemptActive && attempt.StepID == step.ID && attempt.Approval != nil,
	}
}

func executionCompletion(gateResult gate.CompletionGateResult, stepID string) *ExecutionCompletionResult {
	blockers := make([]ExecutionContextBlocker, 0, len(gateResult.Blockers))
	for _, blocker := range gateResult.Blockers {
		projected := ExecutionContextBlocker{Type: CompletionBlockerType(blocker.Kind), Path: blocker.Path, CheckID: blocker.CheckID}
		if blocker.Kind == gate.CompletionBlockerMissingApproval {
			projected.StepID = stepID
		}
		blockers = append(blockers, projected)
	}
	return &ExecutionCompletionResult{Ready: gateResult.Ready, Blockers: blockers}
}
