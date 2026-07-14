package transition

import "github.com/8noki8/devflow/internal/state"

const (
	LevelError   = "error"
	LevelWarning = "warning"

	CodeNoActiveFlow             = "error_no_active_flow"
	CodeInvalidCurrentStep       = "error_invalid_current_step"
	CodeMissingRequiredArtifact  = "error_missing_required_artifact"
	CodeMissingRequiredApproval  = "error_missing_required_approval"
	CodeEmptyReason              = "error_empty_reason"
	CodeNoPreviousStep           = "error_no_previous_step"
	CodeInvalidBackTarget        = "error_invalid_back_target"
	CodeFlowAlreadyRunning       = "error_flow_already_running"
	CodeFlowHasNoSteps           = "error_flow_has_no_steps"
	CodeApprovalNotRequired      = "error_approval_not_required"
	CodeInvalidGateResult        = "error_invalid_gate_result"
	CodeSkippedRequiredApproval  = "warning_skipped_required_approval"
	CodeSkippedRequiredArtifact  = "warning_skipped_required_artifact"
	CodeSkippedFinalStep         = "warning_skipped_final_step"
	CodeSkippedFinalApprovalStep = "warning_skipped_final_approval_step"
)

type Diagnostic struct {
	Level     string
	Code      string
	StepID    string
	Message   string
	Artifacts []string
}

type TransitionResult struct {
	State       *state.State
	ExitCode    int
	Diagnostics []Diagnostic
}
