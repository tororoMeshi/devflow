package command

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

const executionContextSchemaVersion = 1

type CheckStatus string

const (
	CheckStatusPending CheckStatus = "pending"
	CheckStatusPassed  CheckStatus = "passed"
	CheckStatusFailed  CheckStatus = "failed"
)

type CompletionBlockerType string

const (
	CompletionBlockerMissingInput    CompletionBlockerType = "missing_input"
	CompletionBlockerMissingArtifact CompletionBlockerType = "missing_artifact"
	CompletionBlockerMissingCheck    CompletionBlockerType = "missing_check"
	CompletionBlockerFailedCheck     CompletionBlockerType = "failed_check"
	CompletionBlockerMissingApproval CompletionBlockerType = "missing_approval"
)

type ExecutionContextResult struct {
	SchemaVersion int                        `json:"schema_version"`
	FlowRunID     string                     `json:"flow_run_id"`
	Flow          ExecutionFlowResult        `json:"flow"`
	State         ExecutionStateResult       `json:"state"`
	Step          *ExecutionStepResult       `json:"step"`
	Completion    *ExecutionCompletionResult `json:"completion"`
}

type ExecutionFlowResult struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ExecutionStateResult struct {
	Status        state.Status `json:"status"`
	EntrySequence uint64       `json:"entry_sequence"`
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
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
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
		Flow: ExecutionFlowResult{
			ID:    loaded.Flow.ID,
			Title: loaded.Flow.Title,
		},
		State: ExecutionStateResult{
			Status:        loaded.State.Status,
			EntrySequence: loaded.State.CurrentEntrySequence,
		},
	}
	if loaded.Step != nil {
		result.Step = executionStep(*loaded.Step, loaded.State, ctx.ProjectRoot)
		gateResult := gate.CheckDoneGate(*loaded.Step, loaded.State, ctx.ProjectRoot)
		result.Completion = executionCompletion(gateResult, loaded.Step.ID)
	}

	return CommandResult{ExitCode: 0, ExecutionContext: &result}
}

func LoadExecutionContext(ctx Context) (LoadedExecutionContext, []transition.Diagnostic) {
	loaded := NewStore(ctx).Load()
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
		active, diagnostics := loadAndValidateActiveFlow(ctx, *loaded.State)
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

func executionStep(step flow.Step, current state.State, projectRoot string) *ExecutionStepResult {
	return &ExecutionStepResult{
		ID:          step.ID,
		Title:       step.Title,
		Instruction: step.Instruction,
		Inputs:      executionArtifacts(step.Inputs, projectRoot),
		Artifacts:   executionArtifacts(step.Artifacts, projectRoot),
		Checks:      executionChecks(step.RequiredChecks, current),
		Approval:    executionApproval(step, current),
	}
}

func executionArtifacts(artifacts []flow.Artifact, projectRoot string) []ExecutionArtifactResult {
	result := make([]ExecutionArtifactResult, 0, len(artifacts))
	for _, artifact := range artifacts {
		result = append(result, ExecutionArtifactResult{
			Path:     artifact.Path,
			Required: artifact.Required,
			Exists:   gate.FileExists(projectRoot, artifact.Path),
		})
	}
	return result
}

func executionChecks(requiredChecks []string, current state.State) []ExecutionCheckResult {
	result := make([]ExecutionCheckResult, 0, len(requiredChecks))
	for _, checkID := range requiredChecks {
		stored, ok := current.CheckResults[checkID]
		if !ok || stored.EntrySequence != current.CurrentEntrySequence {
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
	return ExecutionApprovalResult{
		Required: true,
		Approved: current.Approvals[step.ID].Approved,
	}
}

func executionCompletion(gateResult gate.Result, stepID string) *ExecutionCompletionResult {
	blockers := make([]ExecutionContextBlocker, 0, len(gateResult.MissingInputs)+len(gateResult.MissingArtifacts)+len(gateResult.CheckProblems)+len(gateResult.MissingApprovals))
	for _, path := range gateResult.MissingInputs {
		blockers = append(blockers, ExecutionContextBlocker{Type: CompletionBlockerMissingInput, Path: path})
	}
	for _, path := range gateResult.MissingArtifacts {
		blockers = append(blockers, ExecutionContextBlocker{Type: CompletionBlockerMissingArtifact, Path: path})
	}
	for _, problem := range gateResult.CheckProblems {
		blockerType := CompletionBlockerMissingCheck
		if problem.Kind == gate.CheckFailed {
			blockerType = CompletionBlockerFailedCheck
		}
		blockers = append(blockers, ExecutionContextBlocker{Type: blockerType, CheckID: problem.CheckID})
	}
	for range gateResult.MissingApprovals {
		blockers = append(blockers, ExecutionContextBlocker{Type: CompletionBlockerMissingApproval, StepID: stepID})
	}
	return &ExecutionCompletionResult{Ready: gateResult.OK, Blockers: blockers}
}
