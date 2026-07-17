package transition

import (
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
)

func TestApplyDoneRejectsMissingRequiredInput(t *testing.T) {
	fl := flow.Flow{Steps: []flow.Step{{ID: "design"}}}
	st := state.State{Status: state.StatusRunning, CurrentStepID: "design"}
	got := ApplyDone(fl, st, gate.Result{MissingInputs: []string{"docs/request.md"}})
	if got.ExitCode == 0 || got.State != nil {
		t.Fatalf("ApplyDone() = %#v, want failure without state", got)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != CodeMissingRequiredInput || got.Diagnostics[0].Artifacts[0] != "docs/request.md" {
		t.Fatalf("Diagnostics = %#v", got.Diagnostics)
	}
}
