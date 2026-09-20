package command

import (
	"github.com/tororoMeshi/devflow/internal/transition"
	"github.com/tororoMeshi/devflow/internal/workpackage"
)

type CommandResult struct {
	ExitCode          int
	Actions           []CommandAction
	Flows             []FlowListItem
	Status            *StatusResult
	Prompt            *PromptResult
	ExecutionContext  *ExecutionContextResult
	CompletionContext *CompletionContextResult
	Success           *SuccessResult
	CheckRequest      *CheckRequestResult
	WorkPackage       *workpackage.WorkPackage
	ExecutionReport   *ExecutionReportRecordResult
	Diagnostics       []transition.Diagnostic
}

type CommandAction struct {
	Path   string
	Status string
}

const (
	ActionCreated = "created"
	ActionExists  = "exists"
)

const (
	FlowStatusValid   = "valid"
	FlowStatusInvalid = "invalid"
)

type FlowListItem struct {
	ID          string
	Title       string
	Description string
	StepCount   int
	FilePath    string
	Status      string
	Err         error
}

type StatusResult struct {
	FlowID               string
	FlowTitle            string
	FlowStatus           string
	CurrentStepID        string
	CurrentStepTitle     string
	CurrentStepObjective string
	CurrentAttemptID     string
	CompletedSteps       []string
	SkippedSteps         map[string]SkippedStepResult
	Approval             *ApprovalResult
	EntrySequence        uint64
	Checks               []CheckStatusResult
	Artifacts            []ArtifactStatusResult `json:"artifacts"`
	CompletionReady      bool                   `json:"-"`
}

type SkippedStepResult struct {
	Reason string
}

type ApprovalResult struct {
	StepID   string
	Approved bool
	Note     string
}

type CheckStatusResult struct {
	CheckID  string
	Status   string
	ExitCode *int
	LogPath  string
}

type ArtifactStatusResult struct {
	Path   string `json:"path"`
	State  string `json:"state"`
	Exists bool   `json:"-"`
}

const (
	ArtifactStatusCurrent         = "current"
	ArtifactStatusMissingEvidence = "missing_evidence"
	ArtifactStatusMissingFile     = "missing_file"
	ArtifactStatusChanged         = "changed"
	ArtifactStatusUnavailable     = "unavailable"
)

type PromptResult struct {
	FlowID               string
	TaskContent          string
	CurrentStepID        string
	CurrentStepTitle     string
	CurrentAttemptID     string
	CurrentStepObjective string
	RequiredInputs       []ArtifactResult
	OptionalInputs       []ArtifactResult
	RequiredArtifacts    []ArtifactResult
	OptionalArtifacts    []ArtifactResult
	RequiredApproval     *RequiredApprovalResult
	RequiredChecks       []string
	ArtifactBlockers     []string
	CheckBlockers        []string
	CompletionBlockers   []string
}

type ArtifactResult struct {
	Path string
}

type RequiredApprovalResult struct {
	StepID    string
	AttemptID string
}

type SuccessResult struct {
	StartedFlowID             string
	CurrentStepID             string
	CompletedStepID           string
	NextStepID                string
	CompletedFlowID           string
	ApprovedStepID            string
	ApprovedAttemptID         string
	ApprovedEvidenceSetDigest string `json:"approved_evidence_set_digest,omitempty"`
	MovedBackToID             string
	SkippedStepID             string
	FinishedFlowID            string
	RecordedArtifactPath      string
	RecordedAttemptID         string
	RecordedArtifactDigest    string
	RecordedArtifactSize      int64
	RecordedCheckRunID        string `json:"recorded_check_run_id,omitempty"`
	RecordedCheckStepID       string `json:"recorded_check_step_id,omitempty"`
	RecordedCheckAttemptID    string `json:"recorded_check_attempt_id,omitempty"`
	RecordedCheckID           string `json:"recorded_check_id,omitempty"`
	RecordedCheckExitCode     *int   `json:"recorded_check_exit_code,omitempty"`
}
