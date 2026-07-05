package command

import "github.com/8noki8/devflow/internal/transition"

type CommandResult struct {
	ExitCode    int
	Actions     []CommandAction
	Flows       []FlowListItem
	Status      *StatusResult
	Prompt      *PromptResult
	Diagnostics []transition.Diagnostic
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
	FlowID           string
	FlowTitle        string
	CurrentStepID    string
	CurrentStepTitle string
	CompletedSteps   []string
	SkippedSteps     map[string]SkippedStepResult
	Approvals        map[string]ApprovalResult
}

type SkippedStepResult struct {
	Reason string
}

type ApprovalResult struct {
	Approved bool
	Note     string
}

type PromptResult struct {
	FlowID                 string
	CurrentStepID          string
	CurrentStepTitle       string
	CurrentStepInstruction string
	RequiredArtifacts      []ArtifactResult
	OptionalArtifacts      []ArtifactResult
	RequiredApproval       *RequiredApprovalResult
	AfterCompleting        AfterCompletingResult
}

type ArtifactResult struct {
	Path string
}

type RequiredApprovalResult struct {
	StepID string
}

type AfterCompletingResult struct {
	Commands []string
}
