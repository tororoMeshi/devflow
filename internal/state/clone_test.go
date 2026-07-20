package state

import "testing"

func TestStateCloneDoesNotShareCollectionsOrPointers(t *testing.T) {
	original := State{
		FlowSnapshot:   testState(t, StatusRunning, "first").FlowSnapshot,
		TaskSnapshot:   testState(t, StatusRunning, "first").TaskSnapshot,
		Status:         StatusRunning,
		CurrentStepID:  "check_changes",
		CompletedSteps: []string{"check_changes"},
		SkippedSteps: map[string]SkippedStep{
			"check_docs": {Reason: "not needed"},
		},
		Attempts:         []StepAttempt{{ID: "attempt_00000000000000000001", StepID: "human_approval", EntrySequence: 1, Status: StepAttemptActive, CheckResults: map[string]CheckResult{}, Approval: &ApprovalRecord{Note: "ok"}}},
		CurrentAttemptID: "attempt_00000000000000000001",
		BackHistory: []BackHistory{
			{FromStepID: "human_approval", ToStepID: "write_review", Reason: "revise", InvalidatedStepIDs: []string{"write_review", "human_approval"}},
		},
		Finish: &Finish{Reason: "out of scope"},
	}

	cloned := original.Clone()

	cloned.CompletedSteps[0] = "changed"
	cloned.SkippedSteps["check_docs"] = SkippedStep{Reason: "changed"}
	cloned.Attempts[0].Approval.Note = "changed"
	cloned.BackHistory[0].FromStepID = "changed"
	cloned.BackHistory[0].InvalidatedStepIDs[0] = "changed"
	cloned.BackHistory[0].InvalidatedStepIDs = append(cloned.BackHistory[0].InvalidatedStepIDs, "added")
	cloned.Finish.Reason = "changed"
	cloned.FlowSnapshot.Flow.Steps[0].Instruction = "changed"
	cloned.TaskSnapshot.Content = "changed"
	cloned.TaskSnapshot.Source.Path = "changed.md"

	if original.CompletedSteps[0] != "check_changes" {
		t.Fatalf("CompletedSteps shares backing array")
	}
	if original.SkippedSteps["check_docs"].Reason != "not needed" {
		t.Fatalf("SkippedSteps shares map")
	}
	if original.Attempts[0].Approval.Note != "ok" {
		t.Fatalf("Approval pointer is shared")
	}
	if original.BackHistory[0].FromStepID != "human_approval" {
		t.Fatalf("BackHistory record was shared")
	}
	if original.BackHistory[0].InvalidatedStepIDs[0] != "write_review" {
		t.Fatalf("BackHistory shares backing array")
	}
	if len(original.BackHistory) != 1 {
		t.Fatalf("BackHistory length changed: %d", len(original.BackHistory))
	}
	if len(original.BackHistory[0].InvalidatedStepIDs) != 2 {
		t.Fatalf("InvalidatedStepIDs length changed: %d", len(original.BackHistory[0].InvalidatedStepIDs))
	}
	if original.Finish.Reason != "out of scope" {
		t.Fatalf("Finish pointer was shared")
	}
	if original.FlowSnapshot.Flow.Steps[0].Instruction != "Do first." {
		t.Fatalf("FlowSnapshot shares Flow memory")
	}
	if original.TaskSnapshot.Content != "Test task\n" || original.TaskSnapshot.Source.Path != "tasks/task.md" {
		t.Fatal("TaskSnapshot changed through clone")
	}
}

func TestStateCloneNormalizesNilCollections(t *testing.T) {
	original := State{
		FlowSnapshot:  testState(t, StatusRunning, "first").FlowSnapshot,
		Status:        StatusRunning,
		CurrentStepID: "check_changes",
	}

	cloned := original.Clone()

	assertNonNilCollections(t, cloned)
	if original.CompletedSteps != nil {
		t.Fatalf("Clone mutated original CompletedSteps")
	}
	if original.SkippedSteps != nil {
		t.Fatalf("Clone mutated original SkippedSteps")
	}
	if original.BackHistory != nil {
		t.Fatalf("Clone mutated original BackHistory")
	}
}

func TestStateNormalizeNormalizesNilCollections(t *testing.T) {
	state := State{}

	state.Normalize()

	assertNonNilCollections(t, state)
}

func TestStateNormalizeAllowsNilReceiver(t *testing.T) {
	var state *State

	state.Normalize()
}

func assertNonNilCollections(t *testing.T, state State) {
	t.Helper()

	if state.CompletedSteps == nil {
		t.Fatalf("CompletedSteps is nil")
	}
	if state.SkippedSteps == nil {
		t.Fatalf("SkippedSteps is nil")
	}
	if state.BackHistory == nil {
		t.Fatalf("BackHistory is nil")
	}
}
