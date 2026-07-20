package transition

import (
	"math"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
)

func TestAttemptLifecycleAcrossTransitions(t *testing.T) {
	t.Run("done closes current and appends next", func(t *testing.T) {
		input := runningState()
		input.Attempts[0].CheckResults["historic"] = state.CheckResult{ExitCode: 0}
		before := input.Clone()
		got := ApplyDone(testFlow(), input, gate.Result{OK: true})
		assertSuccess(t, got)
		if !reflect.DeepEqual(input, before) {
			t.Fatal("input mutated")
		}
		if len(got.State.Attempts) != 2 || got.State.Attempts[0].Status != state.StepAttemptClosed || got.State.Attempts[0].ExitReason != state.StepAttemptExitDone || got.State.Attempts[1].Status != state.StepAttemptActive || got.State.Attempts[1].EntrySequence != 2 || got.State.CurrentAttemptID != got.State.Attempts[1].ID {
			t.Fatalf("done Attempts = %#v", got.State.Attempts)
		}
		if len(got.State.Attempts[0].CheckResults) != 1 || len(got.State.Attempts[1].CheckResults) != 0 {
			t.Fatalf("done check history = %#v", got.State.Attempts)
		}
	})

	t.Run("final done closes without active attempt", func(t *testing.T) {
		input := runningState()
		setRunningStep(&input, "approval")
		got := ApplyDone(testFlow(), input, gate.Result{OK: true})
		assertSuccess(t, got)
		assertTerminalAttempt(t, *got.State, state.StatusCompleted, state.StepAttemptExitDone, "")
	})

	t.Run("skip records reason and final skip is terminal", func(t *testing.T) {
		input := runningState()
		got := ApplySkip(testFlow(), input, "not needed")
		assertSuccess(t, got)
		if got.State.Attempts[0].ExitReason != state.StepAttemptExitSkip || got.State.Attempts[0].Reason != "not needed" || len(got.State.Attempts) != 2 {
			t.Fatalf("skip Attempts = %#v", got.State.Attempts)
		}
		setRunningStep(&input, "approval")
		got = ApplySkip(testFlow(), input, "skip final")
		assertSuccess(t, got, CodeSkippedRequiredApproval, CodeSkippedFinalStep, CodeSkippedFinalApprovalStep)
		assertTerminalAttempt(t, *got.State, state.StatusCompleted, state.StepAttemptExitSkip, "skip final")
	})

	t.Run("back closes source and appends a fresh target", func(t *testing.T) {
		input := runningState()
		setRunningStep(&input, "second")
		input.Attempts[0].CheckResults["historic"] = state.CheckResult{ExitCode: 1}
		got := ApplyBack(testFlow(), input, "first", "revise")
		assertSuccess(t, got)
		if len(got.State.Attempts) != 2 || got.State.Attempts[0].ExitReason != state.StepAttemptExitBack || got.State.Attempts[0].Reason != "revise" || got.State.Attempts[1].StepID != "first" || got.State.Attempts[1].EntrySequence != 2 || len(got.State.Attempts[0].CheckResults) != 1 || len(got.State.Attempts[1].CheckResults) != 0 {
			t.Fatalf("back Attempts = %#v", got.State.Attempts)
		}
	})

	t.Run("finish closes current without append", func(t *testing.T) {
		input := runningState()
		got := ApplyFinish(input, "out of scope")
		assertSuccess(t, got)
		assertTerminalAttempt(t, *got.State, state.StatusFinished, state.StepAttemptExitFinish, "out of scope")
		if len(got.State.Attempts) != 1 || got.State.Finish == nil || got.State.Finish.Reason != "out of scope" {
			t.Fatalf("finished State = %#v", got.State)
		}
	})
}

func TestEnterStepRejectsSequenceOverflowWithoutMutation(t *testing.T) {
	attempt, err := state.NewStepAttempt("first", math.MaxUint64)
	if err != nil {
		t.Fatal(err)
	}
	input := state.State{Attempts: []state.StepAttempt{attempt}, CurrentAttemptID: attempt.ID, CurrentStepID: attempt.StepID}
	input.Normalize()
	before := input.Clone()
	if enterStep(&input, "second") {
		t.Fatal("enterStep accepted sequence overflow")
	}
	if !reflect.DeepEqual(input, before) {
		t.Fatalf("overflow mutated State: %#v", input)
	}
}

func assertTerminalAttempt(t *testing.T, got state.State, status state.Status, exit state.StepAttemptExitReason, reason string) {
	t.Helper()
	last, ok := got.LastAttempt()
	if !ok || got.Status != status || got.CurrentAttemptID != "" || last.Status != state.StepAttemptClosed || last.ExitReason != exit || last.Reason != reason {
		t.Fatalf("terminal Attempt invariant failed: %#v", got)
	}
	for _, attempt := range got.Attempts {
		if attempt.Status == state.StepAttemptActive {
			t.Fatalf("terminal State has active Attempt: %#v", got.Attempts)
		}
	}
}
