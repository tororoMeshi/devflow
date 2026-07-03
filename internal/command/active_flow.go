package command

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

var (
	ErrNoActiveFlow         = errors.New("no active flow")
	ErrInvalidState         = errors.New("invalid state")
	ErrStateFlowMismatch    = errors.New("state flow mismatch")
	ErrStateStepNotInFlow   = errors.New("state current step is not in flow")
	ErrMissingLoadedState   = errors.New("missing loaded state")
	ErrUnexpectedLoadStatus = errors.New("unexpected state load status")
)

type ActiveFlow struct {
	Flow        flow.Flow
	State       state.State
	CurrentStep flow.Step
}

func LoadActiveFlow(ctx Context) (ActiveFlow, []transition.Diagnostic) {
	store := NewStore(ctx)
	loaded := store.Load()
	return ActiveFlowFromLoadResult(ctx, loaded)
}

func ActiveFlowFromLoadResult(ctx Context, loaded state.LoadResult) (ActiveFlow, []transition.Diagnostic) {
	switch loaded.Status {
	case state.LoadNoState:
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeNoActiveFlow)}
	case state.LoadInvalid:
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	case state.LoadOK:
		if loaded.State == nil {
			return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
		}
		if loaded.State.Status != state.StatusRunning {
			return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeNoActiveFlow)}
		}
		return loadAndValidateActiveFlow(ctx, *loaded.State)
	default:
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeInvalidState)}
	}
}

func loadAndValidateActiveFlow(ctx Context, st state.State) (ActiveFlow, []transition.Diagnostic) {
	flowPath := filepath.Join(FlowDir(ctx.ProjectRoot), st.FlowID+".cue")
	loadedFlow, err := flow.LoadFile(flowPath)
	if err != nil {
		return ActiveFlow{}, []transition.Diagnostic{commandErrorDiagnostic(CodeStateFlowMismatch)}
	}

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

func (a ActiveFlow) Validate() error {
	if a.State.FlowID == "" {
		return fmt.Errorf("%w: empty flow_id", ErrInvalidState)
	}
	if a.Flow.ID != a.State.FlowID {
		return fmt.Errorf("%w: state flow_id %q, flow id %q", ErrStateFlowMismatch, a.State.FlowID, a.Flow.ID)
	}
	if _, ok := findStep(a.Flow, a.State.CurrentStepID); !ok {
		return fmt.Errorf("%w: %s", ErrStateStepNotInFlow, a.State.CurrentStepID)
	}
	return nil
}
