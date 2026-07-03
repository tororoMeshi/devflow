package command

import (
	"github.com/8noki8/devflow/internal/transition"
)

func SaveTransitionState(ctx Context, result transition.TransitionResult) []transition.Diagnostic {
	if result.State == nil {
		return nil
	}
	if err := NewStore(ctx).Save(*result.State); err != nil {
		return []transition.Diagnostic{commandErrorDiagnostic(CodeStateSaveFailed)}
	}
	return nil
}
