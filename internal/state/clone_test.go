package state

import "testing"

func TestStateCloneDoesNotShareCollectionsOrPointers(t *testing.T) {
	original := State{
		FlowID:         "post-task-review",
		Status:         StatusRunning,
		CurrentStepID:  "check_changes",
		CompletedSteps: []string{"check_changes"},
		SkippedSteps: map[string]SkippedStep{
			"check_docs": {Reason: "not needed"},
		},
		Approvals: map[string]ApprovalRecord{
			"human_approval": {Approved: true, Note: "ok"},
		},
		BackHistory: []BackHistory{
			{FromStepID: "human_approval", ToStepID: "write_review", Reason: "revise"},
		},
		Finish: &Finish{Reason: "out of scope"},
	}

	cloned := original.Clone()

	cloned.CompletedSteps[0] = "changed"
	cloned.SkippedSteps["check_docs"] = SkippedStep{Reason: "changed"}
	cloned.Approvals["human_approval"] = ApprovalRecord{Approved: false, Note: "changed"}
	cloned.BackHistory[0] = BackHistory{FromStepID: "changed", ToStepID: "changed", Reason: "changed"}
	cloned.Finish.Reason = "changed"

	if original.CompletedSteps[0] != "check_changes" {
		t.Fatalf("CompletedSteps shares backing array")
	}
	if original.SkippedSteps["check_docs"].Reason != "not needed" {
		t.Fatalf("SkippedSteps shares map")
	}
	if !original.Approvals["human_approval"].Approved || original.Approvals["human_approval"].Note != "ok" {
		t.Fatalf("Approvals shares map")
	}
	if original.BackHistory[0].FromStepID != "human_approval" {
		t.Fatalf("BackHistory shares backing array")
	}
	if original.Finish.Reason != "out of scope" {
		t.Fatalf("Finish pointer was shared")
	}
}

func TestStateCloneNormalizesNilCollections(t *testing.T) {
	original := State{
		FlowID:        "post-task-review",
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
	if original.Approvals != nil {
		t.Fatalf("Clone mutated original Approvals")
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
	if state.Approvals == nil {
		t.Fatalf("Approvals is nil")
	}
	if state.BackHistory == nil {
		t.Fatalf("BackHistory is nil")
	}
}
