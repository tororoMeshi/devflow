package command

import (
	"fmt"

	"github.com/8noki8/devflow/internal/transition"
)

const (
	CodeNoActiveFlow       = "error_no_active_flow"
	CodeInvalidState       = "error_invalid_state"
	CodeStateFlowMismatch  = "error_state_flow_mismatch"
	CodeStateStepNotInFlow = "error_state_step_not_in_flow"
	CodeStateSaveFailed    = "error_state_save_failed"
)

func WriteDiagnostics(ctx Context, diagnostics []transition.Diagnostic) {
	for _, diagnostic := range diagnostics {
		writeDiagnostic(ctx, diagnostic)
	}
}

func writeDiagnostic(ctx Context, diagnostic transition.Diagnostic) {
	writer := ctx.Stdout
	if diagnostic.Level == transition.LevelError || diagnostic.Level == transition.LevelWarning {
		writer = ctx.Stderr
	}
	if writer == nil {
		return
	}

	if diagnostic.StepID != "" {
		_, _ = fmt.Fprintf(writer, "%s: %s (%s)\n", diagnostic.Level, diagnostic.Code, diagnostic.StepID)
		return
	}
	_, _ = fmt.Fprintf(writer, "%s: %s\n", diagnostic.Level, diagnostic.Code)
}

func commandErrorDiagnostic(code string) transition.Diagnostic {
	return transition.Diagnostic{
		Level: transition.LevelError,
		Code:  code,
	}
}
