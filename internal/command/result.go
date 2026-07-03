package command

import "github.com/8noki8/devflow/internal/transition"

type CommandResult struct {
	ExitCode    int
	Actions     []CommandAction
	Flows       []FlowListItem
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
