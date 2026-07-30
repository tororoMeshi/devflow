package command

import (
	"github.com/tororoMeshi/devflow/internal/transition"
)

func SaveTransitionState(ctx Context, result transition.TransitionResult) []transition.Diagnostic {
	if result.State == nil {
		return nil
	}
	if err := NewStore(ctx).SaveCurrent(*result.State); err != nil {
		return []transition.Diagnostic{commandErrorDiagnostic(CodeStateSaveFailed)}
	}
	return nil
}
