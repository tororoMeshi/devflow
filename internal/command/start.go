package command

import (
	"path/filepath"
	"strings"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func Start(ctx Context, flowID string) CommandResult {
	if diagnostics := validateStartFlowID(flowID); len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	fl, err := flow.LoadFile(filepath.Join(FlowDir(ctx.ProjectRoot), flowID+".cue"))
	if err != nil {
		return commandFailure(CodeStateFlowMismatch)
	}
	if fl.ID != flowID {
		return commandFailure(CodeStateFlowMismatch)
	}

	store := NewStore(ctx)
	loaded := store.Load()
	current, diagnostics := startCurrentState(loaded)
	if len(diagnostics) > 0 {
		return CommandResult{ExitCode: 1, Diagnostics: diagnostics}
	}

	result := transition.ApplyStart(fl, current)
	if saveDiagnostics := SaveTransitionState(ctx, result); len(saveDiagnostics) > 0 {
		result.Diagnostics = append(result.Diagnostics, saveDiagnostics...)
		return CommandResult{ExitCode: 1, Diagnostics: result.Diagnostics}
	}

	return CommandResult{
		ExitCode:    result.ExitCode,
		Diagnostics: result.Diagnostics,
	}
}

func validateStartFlowID(flowID string) []transition.Diagnostic {
	if flowID == "" || !flow.IsValidID(flowID) {
		code := string(flow.ErrorInvalidFlowID)
		if strings.TrimSpace(flowID) == "" {
			code = string(flow.ErrorMissingFlowID)
		}
		return []transition.Diagnostic{commandErrorDiagnostic(code)}
	}
	return nil
}

func startCurrentState(loaded state.LoadResult) (*state.State, []transition.Diagnostic) {
	switch loaded.Status {
	case state.LoadNoState:
		return nil, nil
	case state.LoadInvalid:
		return nil, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	case state.LoadOK:
		return loaded.State, nil
	default:
		return nil, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	}
}

func commandFailure(code string) CommandResult {
	return CommandResult{
		ExitCode:    1,
		Diagnostics: []transition.Diagnostic{commandErrorDiagnostic(code)},
	}
}
