package state

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
)

func TestStateV6JSONContract(t *testing.T) {
	value := testStateWithRequiredCheck(t, "unused")
	value.Attempts[0].CheckResults["unused"] = CheckResult{ExitCode: 0}
	if err := validateState(value); err != nil {
		t.Fatalf("fixture is invalid: %v", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if value.SchemaVersion != 6 || raw["attempts"] == nil || raw["current_attempt_id"] == nil {
		t.Fatalf("v6 fields missing: %s", data)
	}
	if _, ok := raw["approvals"]; ok {
		t.Fatal("top-level approvals must not be present")
	}
	if _, ok := raw["current_entry_sequence"]; ok {
		t.Fatal("current_entry_sequence must not be present")
	}
	if _, ok := raw["check_results"]; ok {
		t.Fatal("top-level check_results must not be present")
	}
	var attempts []map[string]json.RawMessage
	if err := json.Unmarshal(raw["attempts"], &attempts); err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 || attempts[0]["check_results"] == nil {
		t.Fatalf("nested check_results missing: %s", raw["attempts"])
	}
	var roundTrip State
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, value) {
		t.Fatalf("round trip mismatch\ngot=%#v\nwant=%#v", roundTrip, value)
	}
}

func TestValidateStateV6AttemptInvariants(t *testing.T) {
	valid := testState(t, StatusRunning, "first")
	closed, err := CloseStepAttempt(valid.Attempts[0], StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewStepAttempt("second", 2)
	if err != nil {
		t.Fatal(err)
	}
	twoAttempts := valid.Clone()
	twoAttempts.Attempts = []StepAttempt{closed, second}
	twoAttempts.CurrentAttemptID = second.ID
	twoAttempts.CurrentStepID = "second"

	tests := []struct {
		name   string
		base   State
		mutate func(*State)
	}{
		{"schema v4", valid, func(s *State) { s.SchemaVersion = 4 }},
		{"attempts nil", valid, func(s *State) { s.Attempts = nil }},
		{"attempts empty", valid, func(s *State) { s.Attempts = []StepAttempt{} }},
		{"completed steps nil", valid, func(s *State) { s.CompletedSteps = nil }},
		{"skipped steps nil", valid, func(s *State) { s.SkippedSteps = nil }},
		{"back history nil", valid, func(s *State) { s.BackHistory = nil }},
		{"sequence starts zero", valid, func(s *State) { s.Attempts[0].EntrySequence = 0 }},
		{"sequence gap", twoAttempts, func(s *State) {
			s.Attempts[1].EntrySequence = 3
			s.Attempts[1].ID, _ = StepAttemptID(3)
			s.CurrentAttemptID = s.Attempts[1].ID
		}},
		{"sequence reverse", twoAttempts, func(s *State) {
			s.Attempts[0], s.Attempts[1] = s.Attempts[1], s.Attempts[0]
			s.CurrentAttemptID = s.Attempts[1].ID
			s.CurrentStepID = s.Attempts[1].StepID
		}},
		{"duplicate sequence", twoAttempts, func(s *State) {
			s.Attempts[1].EntrySequence = 1
			s.Attempts[1].ID, _ = StepAttemptID(1)
			s.CurrentAttemptID = s.Attempts[1].ID
		}},
		{"duplicate id", twoAttempts, func(s *State) { s.Attempts[1].ID = s.Attempts[0].ID; s.CurrentAttemptID = s.Attempts[1].ID }},
		{"id sequence mismatch", valid, func(s *State) {
			s.Attempts[0].ID = "attempt_00000000000000000002"
			s.CurrentAttemptID = s.Attempts[0].ID
		}},
		{"unknown step", valid, func(s *State) { s.Attempts[0].StepID = "missing"; s.CurrentStepID = "missing" }},
		{"unknown attempt status", valid, func(s *State) { s.Attempts[0].Status = "unknown" }},
		{"nil attempt checks", valid, func(s *State) { s.Attempts[0].CheckResults = nil }},
		{"running missing current id", valid, func(s *State) { s.CurrentAttemptID = "" }},
		{"running no active", valid, func(s *State) { s.Attempts[0] = closed }},
		{"running multiple active", twoAttempts, func(s *State) { s.Attempts[0], _ = NewStepAttempt("first", 1) }},
		{"running current id mismatch", valid, func(s *State) { s.CurrentAttemptID = "attempt_00000000000000000002" }},
		{"active not last", twoAttempts, func(s *State) {
			s.Attempts[0], _ = NewStepAttempt("first", 1)
			s.Attempts[1], _ = CloseStepAttempt(s.Attempts[1], StepAttemptExitDone, "")
			s.CurrentAttemptID = s.Attempts[0].ID
			s.CurrentStepID = "first"
		}},
		{"current step mismatch", valid, func(s *State) { s.CurrentStepID = "second" }},
		{"unknown check", valid, func(s *State) { s.Attempts[0].CheckResults["unknown"] = CheckResult{} }},
		{"negative exit code", testStateWithRequiredCheck(t, "check"), func(s *State) {
			s.Attempts[0].CheckResults["check"] = CheckResult{ExitCode: -1}
		}},
		{"invalid log path", testStateWithRequiredCheck(t, "check"), func(s *State) {
			s.Attempts[0].CheckResults["check"] = CheckResult{LogPath: "bad\npath"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := tt.base.Clone()
			tt.mutate(&candidate)
			if err := validateState(candidate); err == nil {
				t.Fatalf("invalid State accepted: %#v", candidate)
			}
		})
	}
}

func TestValidateStateV6ApprovalBelongsOnlyToRequiredStep(t *testing.T) {
	value := testState(t, StatusRunning, "first")
	value.Attempts[0].Approval = &ApprovalRecord{Note: "ok"}
	if err := validateState(value); err == nil {
		t.Fatal("approval on non-required step accepted")
	}
	fl := value.FlowSnapshot.Flow
	fl.Steps[0].Approval = &flow.Approval{Required: true}
	snapshot, err := flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	value.FlowSnapshot = snapshot
	if err := validateState(value); err != nil {
		t.Fatalf("required approval rejected: %v", err)
	}
	value.FlowSnapshot.Flow.Steps[0].Approval.Required = false
	if err := validateState(value); err == nil {
		t.Fatal("Required:false approval accepted")
	}
}

func TestStateV6ApprovalJSONShapeAndRoundTrip(t *testing.T) {
	value := testState(t, StatusRunning, "first")
	fl := value.FlowSnapshot.Flow
	fl.Steps[0].Approval = &flow.Approval{Required: true}
	var err error
	value.FlowSnapshot, err = flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	value.Attempts[0].Approval = &ApprovalRecord{Note: " keep whitespace "}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["approvals"]; ok {
		t.Fatal("top-level approvals is present")
	}
	attempt := raw["attempts"].([]any)[0].(map[string]any)
	approval := attempt["approval"].(map[string]any)
	if len(approval) != 1 || approval["note"] != " keep whitespace " {
		t.Fatalf("approval JSON = %#v", approval)
	}
	if _, ok := approval["approved"]; ok {
		t.Fatal("approved field is present")
	}
	var roundTrip State
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if err := validateState(roundTrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value, roundTrip) {
		t.Fatalf("round trip mismatch")
	}
}

func TestAttemptLookupDoesNotShareApproval(t *testing.T) {
	value := testState(t, StatusRunning, "first")
	value.Attempts[0].Approval = &ApprovalRecord{Note: "original"}
	current, _, ok := value.CurrentAttempt()
	if !ok {
		t.Fatal("current attempt missing")
	}
	current.Approval.Note = "changed"
	last, ok := value.LastAttempt()
	if !ok {
		t.Fatal("last attempt missing")
	}
	last.Approval.Note = "changed again"
	if value.Attempts[0].Approval.Note != "original" {
		t.Fatal("lookup shares Approval pointer")
	}
}

func testStateWithRequiredCheck(t testing.TB, checkID string) State {
	t.Helper()
	value := testState(t, StatusRunning, "first")
	fl := value.FlowSnapshot.Flow
	fl.Steps[0].RequiredChecks = []string{checkID}
	snapshot, err := flow.BuildSnapshot(fl, value.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	value.FlowSnapshot = snapshot
	return value
}

func TestValidateStateV6TerminalInvariants(t *testing.T) {
	completed := testState(t, StatusCompleted, "first")
	finished := testState(t, StatusFinished, "first")
	tests := []struct {
		name   string
		base   State
		mutate func(*State)
	}{
		{"completed current id", completed, func(s *State) { s.CurrentAttemptID = s.Attempts[0].ID }},
		{"completed active", completed, func(s *State) { s.Attempts[0], _ = NewStepAttempt("first", 1) }},
		{"completed back", completed, func(s *State) {
			a, _ := NewStepAttempt("first", 1)
			s.Attempts[0], _ = CloseStepAttempt(a, StepAttemptExitBack, "back")
		}},
		{"completed finish", completed, func(s *State) {
			a, _ := NewStepAttempt("first", 1)
			s.Attempts[0], _ = CloseStepAttempt(a, StepAttemptExitFinish, "finish")
		}},
		{"finished current id", finished, func(s *State) { s.CurrentAttemptID = s.Attempts[0].ID }},
		{"finished active", finished, func(s *State) { s.Attempts[0], _ = NewStepAttempt("first", 1) }},
		{"finished done", finished, func(s *State) {
			a, _ := NewStepAttempt("first", 1)
			s.Attempts[0], _ = CloseStepAttempt(a, StepAttemptExitDone, "")
		}},
		{"finished skip", finished, func(s *State) {
			a, _ := NewStepAttempt("first", 1)
			s.Attempts[0], _ = CloseStepAttempt(a, StepAttemptExitSkip, "skip")
		}},
		{"finished reason mismatch", finished, func(s *State) { s.Finish.Reason = "different" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := tt.base.Clone()
			tt.mutate(&candidate)
			if err := validateState(candidate); err == nil {
				t.Fatalf("invalid terminal State accepted: %#v", candidate)
			}
		})
	}
}

func TestStateCloneDeepCopiesAttempts(t *testing.T) {
	original := testState(t, StatusRunning, "first")
	original.FlowSnapshot.Flow.Steps[0].RequiredChecks = []string{"check"}
	original.Attempts[0].CheckResults["check"] = CheckResult{ExitCode: 1, LogPath: "old.log"}
	clone := original.Clone()
	clone.Attempts[0].Status = StepAttemptClosed
	clone.Attempts[0].Reason = "changed"
	clone.Attempts[0].CheckResults["check"] = CheckResult{ExitCode: 2}
	clone.Attempts[0].CheckResults["new"] = CheckResult{}
	attempt, _ := NewStepAttempt("second", 2)
	clone.Attempts = append(clone.Attempts, attempt)
	if original.Attempts[0].Status != StepAttemptActive || original.Attempts[0].Reason != "" || original.Attempts[0].CheckResults["check"].ExitCode != 1 || len(original.Attempts[0].CheckResults) != 1 || len(original.Attempts) != 1 {
		t.Fatalf("clone mutated original: %#v", original.Attempts)
	}
}

func TestStateAttemptLookupDoesNotExposeCheckResultsMap(t *testing.T) {
	original := testStateWithRequiredCheck(t, "check")
	original.Attempts[0].CheckResults["check"] = CheckResult{ExitCode: 1}
	current, _, ok := original.CurrentAttempt()
	if !ok {
		t.Fatal("current Attempt not found")
	}
	current.CheckResults["check"] = CheckResult{ExitCode: 2}
	current.CheckResults["new"] = CheckResult{}
	last, ok := original.LastAttempt()
	if !ok {
		t.Fatal("last Attempt not found")
	}
	last.CheckResults["check"] = CheckResult{ExitCode: 3}
	if original.Attempts[0].CheckResults["check"].ExitCode != 1 || len(original.Attempts[0].CheckResults) != 1 {
		t.Fatalf("Attempt lookup exposed State map: %#v", original.Attempts[0].CheckResults)
	}
}
