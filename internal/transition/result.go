package transition

import "github.com/8noki8/devflow/internal/state"

const (
	LevelError   = "error"
	LevelWarning = "warning"

	CodeNoActiveFlow                    = "error_no_active_flow"
	CodeInvalidCurrentStep              = "error_invalid_current_step"
	CodeMissingRequiredInput            = "error_missing_required_input"
	CodeMissingRequiredArtifact         = "error_missing_required_artifact"
	CodeMissingArtifactEvidence         = "error_missing_artifact_evidence"
	CodeMissingRequiredApproval         = "error_missing_required_approval"
	CodeEmptyReason                     = "error_empty_reason"
	CodeNoPreviousStep                  = "error_no_previous_step"
	CodeInvalidBackTarget               = "error_invalid_back_target"
	CodeFlowAlreadyRunning              = "error_flow_already_running"
	CodeFlowHasNoSteps                  = "error_flow_has_no_steps"
	CodeApprovalNotRequired             = "error_approval_not_required"
	CodeInvalidAttemptID                = "error_invalid_attempt_id"
	CodeStaleAttempt                    = "error_stale_attempt"
	CodeStepAttemptMismatch             = "error_step_attempt_mismatch"
	CodeAttemptAlreadyApproved          = "error_attempt_already_approved"
	CodeArtifactNotRequired             = "error_artifact_not_required"
	CodeInvalidArtifactPath             = "error_invalid_artifact_path"
	CodeArtifactEvidenceAlreadyRecorded = "error_artifact_evidence_already_recorded"
	CodeArtifactRecordAfterApproval     = "error_artifact_record_after_approval"
	CodeInvalidArtifactEvidence         = "error_invalid_artifact_evidence"
	CodeInvalidArtifactDigest           = "error_invalid_artifact_digest"
	CodeArtifactEvidenceMismatch        = "error_artifact_evidence_mismatch"
	CodeArtifactUnsafe                  = "error_artifact_unsafe"
	CodeInvalidApprovalNote             = "error_invalid_approval_note"
	CodeInvalidState                    = "error_invalid_state"
	CodeMissingRequiredCheck            = "error_missing_required_check"
	CodeFailedRequiredCheck             = "error_failed_required_check"
	CodeCheckNotRequired                = "error_check_not_required"
	CodeInvalidCheckResult              = "error_invalid_check_result"
	CodeConflictingCheckResult          = "error_conflicting_check_result"
	CodeSkippedRequiredApproval         = "warning_skipped_required_approval"
	CodeSkippedRequiredArtifact         = "warning_skipped_required_artifact"
	CodeSkippedRequiredCheck            = "warning_skipped_required_check"
	CodeSkippedFinalStep                = "warning_skipped_final_step"
	CodeSkippedFinalApprovalStep        = "warning_skipped_final_approval_step"
)

type Diagnostic struct {
	Level     string
	Code      string
	StepID    string
	Message   string
	Artifacts []string
}

type TransitionResult struct {
	State                     *state.State
	ExitCode                  int
	Diagnostics               []Diagnostic
	ApprovedEvidenceSetDigest string
	StateChanged              bool
}
