package transition

import (
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/state"
)

func TestApplyRecordCheckResultContracts(t *testing.T) {
	st := runningState()
	fl := testFlow()
	fl.Steps[0].RequiredChecks = []string{"check"}
	st.FlowSnapshot = testSnapshot(fl)
	before := st.Clone()
	result := state.CheckResult{ExitCode: 1, LogPath: "check.log"}
	got := ApplyRecordCheckResult(st, "first", st.CurrentAttemptID, "check", result)
	assertSuccess(t, got)
	if !got.StateChanged || !reflect.DeepEqual(st, before) || got.State.Attempts[0].CheckResults["check"] != result {
		t.Fatalf("new result=%#v input=%#v", got, st)
	}
	idempotent := ApplyRecordCheckResult(*got.State, "first", st.CurrentAttemptID, "check", result)
	assertSuccess(t, idempotent)
	if idempotent.StateChanged {
		t.Fatal("idempotent result marked changed")
	}
	beforeConflict := got.State.Clone()
	conflict := ApplyRecordCheckResult(*got.State, "first", st.CurrentAttemptID, "check", state.CheckResult{ExitCode: 0})
	assertFailure(t, conflict, CodeConflictingCheckResult)
	if !reflect.DeepEqual(*got.State, beforeConflict) {
		t.Fatal("conflict mutated input State")
	}
	if !reflect.DeepEqual(*got.State, *idempotent.State) {
		t.Fatal("idempotent result changed logical State")
	}
}

func TestApplyRecordCheckResultAttemptClassification(t *testing.T) {
	st := runningState()
	fl := testFlow()
	fl.Steps[0].RequiredChecks = []string{"check"}
	st.FlowSnapshot = testSnapshot(fl)
	assertFailure(t, ApplyRecordCheckResult(st, "first", "bad", "check", state.CheckResult{}), CodeInvalidAttemptID)
	assertFailure(t, ApplyRecordCheckResult(st, "first", "attempt_00000000000000000099", "check", state.CheckResult{}), CodeInvalidAttemptID)
	assertFailure(t, ApplyRecordCheckResult(st, "second", st.CurrentAttemptID, "check", state.CheckResult{}), CodeStepAttemptMismatch)
	assertFailure(t, ApplyRecordCheckResult(st, "first", st.CurrentAttemptID, "unknown", state.CheckResult{}), CodeCheckNotRequired)
	assertFailure(t, ApplyRecordCheckResult(st, "first", st.CurrentAttemptID, "check", state.CheckResult{ExitCode: -1}), CodeInvalidCheckResult)

	first, _ := state.CloseStepAttempt(st.Attempts[0], state.StepAttemptExitBack, "retry")
	second, _ := state.NewStepAttempt("first", 2)
	st.Attempts = []state.StepAttempt{first, second}
	st.CurrentAttemptID = second.ID
	assertFailure(t, ApplyRecordCheckResult(st, "first", first.ID, "check", state.CheckResult{}), CodeStaleAttempt)
}
