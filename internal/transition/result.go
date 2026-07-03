package transition

import "github.com/8noki8/devflow/internal/state"

type Diagnostic struct {
	Code    string
	Message string
}

type TransitionResult struct {
	State       *state.State
	ExitCode    int
	Diagnostics []Diagnostic
}
