package command

import (
	"errors"
	"io"
	"os"

	"github.com/tororoMeshi/devflow/internal/executionreport"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
	"github.com/tororoMeshi/devflow/internal/workpackage"
)

type ExecutionReportRecordResult struct {
	FlowRunID             string
	StepID                string
	AttemptID             string
	WorkPackageDigest     string
	ExecutionReportDigest string
	Outcome               executionreport.Outcome
	Idempotent            bool
}

func ExecutionReportRecord(ctx Context, path string) CommandResult {
	report, err := readExecutionReport(path)
	if err != nil {
		return executionReportFailure(err)
	}
	store := NewStore(ctx)
	pointer, err := store.LoadCurrentPointer()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return commandFailure(CodeNoActiveFlow)
		}
		return commandFailure(CodeInvalidState)
	}
	if report.FlowRunID != pointer.FlowRunID {
		return commandFailure(CodeExecutionReportRunMismatch)
	}
	loaded := store.LoadCurrent()
	if loaded.Status == state.LoadNoState {
		return commandFailure(CodeNoActiveFlow)
	}
	if loaded.Status != state.LoadOK || loaded.State == nil {
		return commandFailure(CodeInvalidState)
	}
	if report.FlowRunID != loaded.State.FlowRunID {
		return commandFailure(CodeExecutionReportRunMismatch)
	}
	pkg, err := workpackage.Generate(*loaded.State, report.StepID, report.AttemptID)
	if err != nil {
		return workPackageBindingFailure(err)
	}
	if report.WorkPackageDigest != pkg.WorkPackageDigest {
		return commandFailure(CodeWorkPackageBindingMismatch)
	}
	if err := executionreport.ValidateBinding(report, pkg); err != nil {
		switch {
		case errors.Is(err, executionreport.ErrUnknownEvidence):
			return commandFailure(CodeUnknownEvidenceReference)
		case errors.Is(err, executionreport.ErrUnknownArtifact):
			return commandFailure(CodeUnknownArtifactReference)
		default:
			return commandFailure(CodeWorkPackageBindingMismatch)
		}
	}
	recorded, err := executionreport.Record(ctx.ProjectRoot, report)
	if err != nil {
		switch {
		case errors.Is(err, executionreport.ErrConflict):
			return commandFailure(CodeConflictingExecutionReport)
		case errors.Is(err, executionreport.ErrInvalidExisting):
			return commandFailure(CodeInvalidExistingExecutionReport)
		case errors.Is(err, executionreport.ErrUnsafeStore):
			return commandFailure(CodeUnsafeExecutionReportStore)
		default:
			return commandFailure(CodeExecutionReportSaveFailed)
		}
	}
	return CommandResult{ExitCode: 0, ExecutionReport: &ExecutionReportRecordResult{
		FlowRunID: report.FlowRunID, StepID: report.StepID, AttemptID: report.AttemptID,
		WorkPackageDigest: report.WorkPackageDigest, ExecutionReportDigest: recorded.Digest,
		Outcome: report.Outcome, Idempotent: recorded.Idempotent,
	}}
}

func readExecutionReport(path string) (executionreport.Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return executionreport.Report{}, executionreport.ErrInvalidReport
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, executionreport.MaxDocumentBytes+1))
	if err != nil {
		return executionreport.Report{}, executionreport.ErrInvalidReport
	}
	if len(data) > executionreport.MaxDocumentBytes {
		return executionreport.Report{}, executionreport.ErrTooLarge
	}
	return executionreport.Decode(data)
}

func executionReportFailure(err error) CommandResult {
	switch {
	case errors.Is(err, executionreport.ErrTooLarge):
		return commandFailure(CodeExecutionReportTooLarge)
	case errors.Is(err, executionreport.ErrDuplicateJSONKey):
		return commandFailure(CodeDuplicateJSONKey)
	case errors.Is(err, executionreport.ErrTrailingJSON):
		return commandFailure(CodeTrailingExecutionReportJSON)
	case errors.Is(err, executionreport.ErrUnknownField):
		return commandFailure(CodeUnknownExecutionReportField)
	case errors.Is(err, executionreport.ErrUnsupportedSchema):
		return commandFailure(CodeUnsupportedExecutionReportSchema)
	default:
		return commandFailure(CodeInvalidExecutionReport)
	}
}

func workPackageBindingFailure(err error) CommandResult {
	switch {
	case errors.Is(err, workpackage.ErrInvalidAttemptID),
		errors.Is(err, workpackage.ErrAttemptNotFound):
		return commandFailure(transition.CodeInvalidAttemptID)
	case errors.Is(err, workpackage.ErrStaleAttempt):
		return commandFailure(transition.CodeStaleAttempt)
	case errors.Is(err, workpackage.ErrStepAttemptMismatch):
		return commandFailure(transition.CodeStepAttemptMismatch)
	case errors.Is(err, workpackage.ErrInactiveAttempt),
		errors.Is(err, workpackage.ErrNoActiveFlow):
		return commandFailure(CodeInvalidState)
	default:
		return commandFailure(CodeInvalidState)
	}
}
