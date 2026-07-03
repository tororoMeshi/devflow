package transition

import (
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
)

func TestApplyStart(t *testing.T) {
	fl := testFlow()

	t.Run("starts when no current state exists", func(t *testing.T) {
		got := ApplyStart(fl, nil)

		assertSuccess(t, got)
		assertStateEqual(t, *got.State, state.State{
			FlowID:         "test-flow",
			Status:         state.StatusRunning,
			CurrentStepID:  "first",
			CompletedSteps: []string{},
			SkippedSteps:   map[string]state.SkippedStep{},
			Approvals:      map[string]state.ApprovalRecord{},
			BackHistory:    []state.BackHistory{},
		})
	})

	t.Run("starts when previous state is completed", func(t *testing.T) {
		current := runningState()
		current.Status = state.StatusCompleted

		got := ApplyStart(fl, &current)

		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
	})

	t.Run("starts when previous state is finished", func(t *testing.T) {
		current := runningState()
		current.Status = state.StatusFinished

		got := ApplyStart(fl, &current)

		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
	})

	t.Run("fails when flow is already running", func(t *testing.T) {
		current := runningState()

		got := ApplyStart(fl, &current)

		assertFailure(t, got, CodeFlowAlreadyRunning)
		assertStateEqual(t, current, runningState())
	})

	t.Run("fails when flow has no steps", func(t *testing.T) {
		got := ApplyStart(flow.Flow{ID: "empty"}, nil)

		assertFailure(t, got, CodeFlowHasNoSteps)
	})
}

func TestApplyDone(t *testing.T) {
	t.Run("moves to next step when gate is ok", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{OK: true})

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.CurrentStepID != "second" {
			t.Fatalf("CurrentStepID = %q, want second", got.State.CurrentStepID)
		}
		assertStrings(t, got.State.CompletedSteps, []string{"first"})
	})

	t.Run("completes flow when current step is final", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "approval"
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{OK: true})

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.Status != state.StatusCompleted {
			t.Fatalf("Status = %q, want completed", got.State.Status)
		}
		if got.State.CurrentStepID != "approval" {
			t.Fatalf("CurrentStepID = %q, want approval", got.State.CurrentStepID)
		}
	})

	t.Run("removes current step from skipped steps", func(t *testing.T) {
		st := runningState()
		st.SkippedSteps["first"] = state.SkippedStep{Reason: "retry as done"}
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{OK: true})

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if _, ok := got.State.SkippedSteps["first"]; ok {
			t.Fatalf("first remained in skipped steps")
		}
	})

	t.Run("returns diagnostics when gate is missing artifact and approval", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{
			OK:               false,
			MissingArtifacts: []string{"docs/code-review.md"},
			MissingApprovals: []string{"first"},
		})

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeMissingRequiredArtifact, CodeMissingRequiredApproval)
	})

	t.Run("returns diagnostic when gate result is inconsistent", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{OK: false})

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeInvalidGateResult)
	})

	t.Run("fails when current step is invalid", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "missing"
		before := st.Clone()

		got := ApplyDone(testFlow(), st, gate.Result{OK: true})

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeInvalidCurrentStep)
	})
}

func TestApplyApprove(t *testing.T) {
	t.Run("approves current step when target is empty", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "approval"
		before := st.Clone()

		got := ApplyApprove(testFlow(), st, "", "approved")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		approval := got.State.Approvals["approval"]
		if !approval.Approved || approval.Note != "approved" {
			t.Fatalf("approval = %#v", approval)
		}
	})

	t.Run("approves specified step", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyApprove(testFlow(), st, "approval", "")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if !got.State.Approvals["approval"].Approved {
			t.Fatalf("approval was not recorded")
		}
	})

	t.Run("fails when approval is not required", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyApprove(testFlow(), st, "first", "")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeApprovalNotRequired)
	})

	t.Run("fails when target step does not exist", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyApprove(testFlow(), st, "missing", "")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeInvalidCurrentStep)
	})
}

func TestApplyBack(t *testing.T) {
	t.Run("moves to previous step and records history", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "second"
		st.CompletedSteps = []string{"first", "second"}
		st.SkippedSteps["first"] = state.SkippedStep{Reason: "kept"}
		st.Approvals["approval"] = state.ApprovalRecord{Approved: true}
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "revise")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
		assertStrings(t, got.State.CompletedSteps, []string{"second"})
		if len(got.State.BackHistory) != 1 {
			t.Fatalf("BackHistory len = %d, want 1", len(got.State.BackHistory))
		}
		if _, ok := got.State.SkippedSteps["first"]; !ok {
			t.Fatalf("skipped_steps was modified")
		}
		if !got.State.Approvals["approval"].Approved {
			t.Fatalf("approvals was modified")
		}
	})

	t.Run("fails when no previous step exists", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "revise")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeNoPreviousStep)
	})

	t.Run("fails when reason is empty", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "second"
		before := st.Clone()

		got := ApplyBack(testFlow(), st, " ")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeEmptyReason)
	})
}

func TestApplySkip(t *testing.T) {
	t.Run("skips current step and moves to next without completing it", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "not needed")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.CurrentStepID != "second" {
			t.Fatalf("CurrentStepID = %q, want second", got.State.CurrentStepID)
		}
		assertStrings(t, got.State.CompletedSteps, []string{})
		if got.State.SkippedSteps["first"].Reason != "not needed" {
			t.Fatalf("skipped step not recorded")
		}
	})

	t.Run("completes flow when final step is skipped", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "approval"
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "skip final")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got, CodeSkippedRequiredApproval, CodeSkippedFinalStep, CodeSkippedFinalApprovalStep)
		if got.State.Status != state.StatusCompleted {
			t.Fatalf("Status = %q, want completed", got.State.Status)
		}
	})

	t.Run("warns when required artifact step is skipped", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "second"
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "skip artifact")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got, CodeSkippedRequiredArtifact)
	})

	t.Run("fails when current step is invalid", func(t *testing.T) {
		st := runningState()
		st.CurrentStepID = "missing"
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "skip")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeInvalidCurrentStep)
	})

	t.Run("fails when reason is empty", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeEmptyReason)
	})

	t.Run("fails when reason is blank", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "   ")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeEmptyReason)
	})
}

func TestApplyFinish(t *testing.T) {
	t.Run("finishes flow and preserves existing state details", func(t *testing.T) {
		st := runningState()
		st.CompletedSteps = []string{"first"}
		st.SkippedSteps["second"] = state.SkippedStep{Reason: "skipped"}
		st.Approvals["approval"] = state.ApprovalRecord{Approved: true}
		before := st.Clone()

		got := ApplyFinish(st, "out of scope")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.Status != state.StatusFinished {
			t.Fatalf("Status = %q, want finished", got.State.Status)
		}
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
		if got.State.Finish == nil || got.State.Finish.Reason != "out of scope" {
			t.Fatalf("Finish = %#v", got.State.Finish)
		}
		assertStrings(t, got.State.CompletedSteps, []string{"first"})
		if got.State.SkippedSteps["second"].Reason != "skipped" {
			t.Fatalf("skipped_steps was not preserved")
		}
		if !got.State.Approvals["approval"].Approved {
			t.Fatalf("approvals was not preserved")
		}
	})

	t.Run("fails when state is not running", func(t *testing.T) {
		st := runningState()
		st.Status = state.StatusCompleted
		before := st.Clone()

		got := ApplyFinish(st, "done")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeNoActiveFlow)
	})
}

func testFlow() flow.Flow {
	return flow.Flow{
		ID:    "test-flow",
		Title: "Test Flow",
		Steps: []flow.Step{
			{
				ID:          "first",
				Title:       "First",
				Instruction: "Do first.",
				Artifacts:   []flow.Artifact{},
			},
			{
				ID:          "second",
				Title:       "Second",
				Instruction: "Do second.",
				Artifacts: []flow.Artifact{
					{Path: "docs/code-review.md", Required: true},
				},
			},
			{
				ID:          "approval",
				Title:       "Approval",
				Instruction: "Approve.",
				Artifacts:   []flow.Artifact{},
				Approval:    &flow.Approval{Required: true},
			},
		},
	}
}

func runningState() state.State {
	st := state.State{
		FlowID:        "test-flow",
		Status:        state.StatusRunning,
		CurrentStepID: "first",
	}
	st.Normalize()
	return st
}

func assertSuccess(t *testing.T, got TransitionResult, wantCodes ...string) {
	t.Helper()

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if got.State == nil {
		t.Fatalf("State is nil")
	}
	assertDiagnosticCodes(t, got.Diagnostics, wantCodes)
}

func assertFailure(t *testing.T, got TransitionResult, wantCodes ...string) {
	t.Helper()

	if got.ExitCode == 0 {
		t.Fatalf("ExitCode = 0, want non-zero")
	}
	if got.State != nil {
		t.Fatalf("State = %#v, want nil", got.State)
	}
	assertDiagnosticCodes(t, got.Diagnostics, wantCodes)
}

func assertDiagnosticCodes(t *testing.T, diagnostics []Diagnostic, wantCodes []string) {
	t.Helper()

	gotCodes := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		gotCodes[i] = diagnostic.Code
	}
	if wantCodes == nil {
		wantCodes = []string{}
	}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic codes = %#v, want %#v", gotCodes, wantCodes)
	}
}

func assertStateNotMutated(t *testing.T, before state.State, after state.State) {
	t.Helper()

	if !reflect.DeepEqual(before, after) {
		t.Fatalf("state mutated\ngot:  %#v\nwant: %#v", after, before)
	}
}

func assertStateEqual(t *testing.T, got state.State, want state.State) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("state = %#v, want %#v", got, want)
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("strings = %#v, want %#v", got, want)
	}
}
