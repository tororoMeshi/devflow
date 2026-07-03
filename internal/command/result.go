package command

type CommandResult struct {
	ExitCode int
	Actions  []CommandAction
}

type CommandAction struct {
	Path   string
	Status string
}

const (
	ActionCreated = "created"
	ActionExists  = "exists"
)
