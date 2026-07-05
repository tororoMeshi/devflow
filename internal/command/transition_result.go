package command

import "github.com/8noki8/devflow/internal/transition"

func transitionCommandResult(ctx Context, result transition.TransitionResult) CommandResult {
	if saveDiagnostics := SaveTransitionState(ctx, result); len(saveDiagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, saveDiagnostics...)
		return CommandResult{ExitCode: 1, Diagnostics: result.Diagnostics}
	}

	return CommandResult{
		ExitCode:    result.ExitCode,
		Diagnostics: result.Diagnostics,
	}
}
