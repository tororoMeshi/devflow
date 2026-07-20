package command

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

type ActiveFlow struct {
	Flow        flow.Flow
	State       state.State
	CurrentStep flow.Step
}

func LoadActiveFlow(ctx Context) (ActiveFlow, []transition.Diagnostic) {
	store := NewStore(ctx)
	loaded := store.LoadCurrent()
	return ActiveFlowFromLoadResult(ctx, loaded)
}

func ActiveFlowFromLoadResult(ctx Context, loaded state.LoadResult) (ActiveFlow, []transition.Diagnostic) {
	switch loaded.Status {
	case state.LoadNoState:
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeNoActiveFlow)}
	case state.LoadInvalid:
		if isUnsupportedStateVersion(loaded.Err) {
			return ActiveFlow{}, []transition.Diagnostic{unsupportedStateVersionDiagnostic()}
		}
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	case state.LoadOK:
		if loaded.State == nil {
			return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
		}
		if loaded.State.Status != state.StatusRunning {
			return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeNoActiveFlow)}
		}
		return activeFlowFromState(*loaded.State)
	default:
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	}
}

func activeFlowFromState(st state.State) (ActiveFlow, []transition.Diagnostic) {
	loadedFlow := st.FlowSnapshot.Flow
	currentStep, ok := findStep(loadedFlow, st.CurrentStepID)
	if !ok {
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeStateStepNotInFlow)}
	}

	return ActiveFlow{
		Flow:        loadedFlow,
		State:       st,
		CurrentStep: currentStep,
	}, nil
}

func findStep(fl flow.Flow, stepID string) (flow.Step, bool) {
	for _, step := range fl.Steps {
		if step.ID == stepID {
			return step, true
		}
	}
	return flow.Step{}, false
}
