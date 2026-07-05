package command

import "github.com/8noki8/devflow/internal/transition"

func transitionCommandResult(ctx Context, result transition.TransitionResult, success *SuccessResult) CommandResult {
	if saveDiagnostics := SaveTransitionState(ctx, result); len(saveDiagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, saveDiagnostics...)
		return CommandResult{ExitCode: 1, Diagnostics: result.Diagnostics}
	}

	if result.ExitCode != 0 {
		success = nil
	}
	return CommandResult{
		ExitCode:    result.ExitCode,
		Success:     success,
		Diagnostics: result.Diagnostics,
	}
}
