package command

import (
	"errors"

	"github.com/8noki8/devflow/internal/transition"
	"github.com/8noki8/devflow/internal/workpackage"
)

func WorkPackage(ctx Context, stepID, attemptID string) CommandResult {
	active, diagnostics := LoadActiveFlow(ctx)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}
	pkg, err := workpackage.Generate(active.State, stepID, attemptID)
	if err != nil {
		return workPackageFailure(err)
	}
	return CommandResult{ExitCode: 0, WorkPackage: &pkg}
}

func workPackageFailure(err error) CommandResult {
	switch {
	case errors.Is(err, workpackage.ErrNoActiveFlow):
		return commandFailure(CodeNoActiveFlow)
	case errors.Is(err, workpackage.ErrInvalidState), errors.Is(err, workpackage.ErrInactiveAttempt):
		return commandFailure(CodeInvalidState)
	case errors.Is(err, workpackage.ErrInvalidAttemptID), errors.Is(err, workpackage.ErrAttemptNotFound):
		return commandFailure(transition.CodeInvalidAttemptID)
	case errors.Is(err, workpackage.ErrStaleAttempt):
		return commandFailure(transition.CodeStaleAttempt)
	case errors.Is(err, workpackage.ErrStepAttemptMismatch):
		return commandFailure(transition.CodeStepAttemptMismatch)
	case errors.Is(err, workpackage.ErrDigestGeneration):
		return commandFailure(CodeWorkPackageDigestFailed)
	default:
		return commandFailure(CodeInvalidWorkPackage)
	}
}
