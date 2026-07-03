package command

type CommandResult struct {
	ExitCode int
	Actions  []CommandAction
	Flows    []FlowListItem
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
