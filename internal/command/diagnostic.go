package command

import (
	"errors"
	"fmt"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

const (
	CodeNoActiveFlow              = "error_no_active_flow"
	CodeInvalidState              = "error_invalid_state"
	CodeUnsupportedStateVersion   = "error_unsupported_state_version"
	CodeStateFlowMismatch         = "error_state_flow_mismatch"
	CodeStateStepNotInFlow        = "error_state_step_not_in_flow"
	CodeStateSaveFailed           = "error_state_save_failed"
	CodeFlowRunIDGenerationFailed = "error_flow_run_id_generation_failed"
	CodeCheckNotRequired          = "error_check_not_required"
	CodeInvalidCheckRecord        = "error_invalid_check_record"
	CodeUnsupportedCheckSchema    = "error_unsupported_check_schema"
	CodeCheckContextMismatch      = "error_check_context_mismatch"
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
		if diagnostic.Message != "" {
			_, _ = fmt.Fprintln(writer, diagnostic.Message)
		}
		return
	}
	_, _ = fmt.Fprintf(writer, "%s: %s\n", diagnostic.Level, diagnostic.Code)
	if diagnostic.Message != "" {
		_, _ = fmt.Fprintln(writer, diagnostic.Message)
	}
}

func commandErrorDiagnostic(code string) transition.Diagnostic {
	return transition.Diagnostic{
		Level: transition.LevelError,
		Code:  code,
	}
}

func isUnsupportedStateVersion(err error) bool {
	var target *state.UnsupportedSchemaVersionError
	return errors.As(err, &target)
}

func unsupportedStateVersionDiagnostic() transition.Diagnostic {
	return transition.Diagnostic{
		Level:   transition.LevelError,
		Code:    CodeUnsupportedStateVersion,
		Message: "v0.1.xのStateはv0.2.0へ引き継げません。現在の作業状態を確認し、必要なら.devflow/state.jsonを退避または削除してFlowを再度startしてください。devflowはStateを自動削除しません。",
	}
}
