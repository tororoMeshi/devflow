package command

import (
	"errors"
	"fmt"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

const (
	CodeNoActiveFlow               = "error_no_active_flow"
	CodeInvalidState               = "error_invalid_state"
	CodeUnsupportedStateVersion    = "error_unsupported_state_version"
	CodeStateFlowMismatch          = "error_state_flow_mismatch"
	CodeStateStepNotInFlow         = "error_state_step_not_in_flow"
	CodeStateSaveFailed            = "error_state_save_failed"
	CodeFlowRunIDGenerationFailed  = "error_flow_run_id_generation_failed"
	CodeInvalidTaskPath            = "error_invalid_task_path"
	CodeTaskFileNotFound           = "error_task_file_not_found"
	CodeTaskPathIsDirectory        = "error_task_path_is_directory"
	CodeTaskFileReadFailed         = "error_task_file_read_failed"
	CodeTaskEmpty                  = "error_task_empty"
	CodeTaskInvalidUTF8            = "error_task_invalid_utf8"
	CodeTaskSnapshotBuildFailed    = "error_task_snapshot_build_failed"
	CodeCheckNotRequired           = "error_check_not_required"
	CodeInvalidCheckRecord         = "error_invalid_check_record"
	CodeDuplicateJSONKey           = "error_duplicate_json_key"
	CodeUnknownCheckRecordField    = "error_unknown_check_record_field"
	CodeTrailingCheckRecordJSON    = "error_trailing_check_record_json"
	CodeUnsupportedCheckSchema     = "error_unsupported_check_schema"
	CodeCheckContextMismatch       = "error_check_context_mismatch"
	CodeCheckRunMismatch           = "error_check_run_mismatch"
	CodeCheckResultAlreadyRecorded = "error_check_result_already_recorded"
	CodeInvalidArtifactPath        = "error_invalid_artifact_path"
	CodeArtifactFileMissing        = "error_artifact_file_missing"
	CodeArtifactNotRegular         = "error_artifact_not_regular"
	CodeArtifactSymlink            = "error_artifact_symlink"
	CodeArtifactOutsideProject     = "error_artifact_outside_project"
	CodeArtifactInsideDevflow      = "error_artifact_inside_devflow"
	CodeArtifactUnreadable         = "error_artifact_unreadable"
	CodeArtifactChanged            = "error_artifact_changed_while_hashing"
	CodeEntryMissingRequiredInput  = "error_entry_missing_required_input"
	CodeEntryInputUnavailable      = "error_entry_input_unavailable"
	CodeCompletionInputUnavailable = "error_input_unavailable"
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
	var legacy *state.LegacyStateError
	return errors.As(err, &target) || errors.As(err, &legacy)
}

func unsupportedStateVersionDiagnostic() transition.Diagnostic {
	return transition.Diagnostic{
		Level:   transition.LevelError,
		Code:    CodeUnsupportedStateVersion,
		Message: "v0.1.xのStateはv0.2.0へ引き継げません。現在の作業状態を確認し、必要なら.devflow/state.jsonを退避または削除してFlowを再度startしてください。devflowはStateを自動削除しません。",
	}
}
