package transition

import (
	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func ApplyApprove(flow flow.Flow, st state.State, targetStepID string, note string) TransitionResult {
	if result, ok := requireRunning(st); !ok {
		return result
	}

	if targetStepID == "" {
		targetStepID = st.CurrentStepID
	}

	targetStep, _, ok := findStep(flow, targetStepID)
	if !ok {
		return failure(errorDiagnostic(CodeInvalidCurrentStep, targetStepID))
	}
	if !hasRequiredApproval(targetStep) {
		return failure(errorDiagnostic(CodeApprovalNotRequired, targetStepID))
	}

	next := st.Clone()
	next.Approvals[targetStepID] = state.ApprovalRecord{
		Approved: true,
		Note:     note,
	}

	return success(next)
}
