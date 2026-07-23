package transition

import (
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/gate"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/task"
)

func TestApplyStart(t *testing.T) {
	fl := testFlow()

	t.Run("starts when no current state exists", func(t *testing.T) {
		got := ApplyStart(testSnapshot(fl), transitionTaskSnapshot(), nil, "run_test")

		assertSuccess(t, got)
		attempt, _ := state.NewStepAttempt("first", 1)
		assertStateEqual(t, *got.State, state.State{
			SchemaVersion:    state.CurrentSchemaVersion,
			FlowSnapshot:     testSnapshot(fl),
			TaskSnapshot:     transitionTaskSnapshot(),
			Status:           state.StatusRunning,
			CurrentStepID:    "first",
			FlowRunID:        "run_test",
			Attempts:         []state.StepAttempt{attempt},
			CurrentAttemptID: attempt.ID,
			CompletedSteps:   []string{},
			SkippedSteps:     map[string]state.SkippedStep{},
			BackHistory:      []state.BackHistory{},
		})
	})

	t.Run("starts when previous state is completed", func(t *testing.T) {
		current := runningState()
		current.Status = state.StatusCompleted

		got := ApplyStart(testSnapshot(fl), transitionTaskSnapshot(), &current, "run_test")

		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
	})

	t.Run("starts when previous state is finished", func(t *testing.T) {
		current := runningState()
		current.Status = state.StatusFinished

		got := ApplyStart(testSnapshot(fl), transitionTaskSnapshot(), &current, "run_test")

		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
	})

	t.Run("does not share snapshot memory with input", func(t *testing.T) {
		snapshot := testSnapshot(fl)
		taskSnapshot := transitionTaskSnapshot()
		got := ApplyStart(snapshot, taskSnapshot, nil, "run_test")
		assertSuccess(t, got)

		snapshot.Flow.Steps[0].Instruction = "changed input"
		snapshot.Flow.Steps[2].Approval.Required = false
		if got.State.FlowSnapshot.Flow.Steps[0].Instruction != "Do first." || !got.State.FlowSnapshot.Flow.Steps[2].Approval.Required {
			t.Fatal("State snapshot shares memory with input snapshot")
		}
		got.State.FlowSnapshot.Flow.Steps[0].Instruction = "changed state"
		got.State.FlowSnapshot.Flow.Steps[2].Approval.Required = true
		if snapshot.Flow.Steps[0].Instruction != "changed input" || snapshot.Flow.Steps[2].Approval.Required {
			t.Fatal("input snapshot shares memory with State snapshot")
		}
		got.State.TaskSnapshot.Content = "changed state"
		got.State.TaskSnapshot.Source.Path = "changed.md"
		if taskSnapshot.Content != "Test task\n" || taskSnapshot.Source.Path != "tasks/task.md" {
			t.Fatal("input TaskSnapshot changed")
		}
	})

	t.Run("fails when flow is already running", func(t *testing.T) {
		current := runningState()

		got := ApplyStart(testSnapshot(fl), transitionTaskSnapshot(), &current, "run_test")

		assertFailure(t, got, CodeFlowAlreadyRunning)
		assertStateEqual(t, current, runningState())
	})

	t.Run("fails when flow has no steps", func(t *testing.T) {
		got := ApplyStart(flow.FlowSnapshot{Flow: flow.Flow{ID: "empty"}}, transitionTaskSnapshot(), nil, "run_test")

		assertFailure(t, got, CodeFlowHasNoSteps)
	})
}

func transitionTaskSnapshot() task.TaskSnapshot {
	snapshot, err := task.BuildSnapshot("Test task\n", task.TaskSource{Path: "tasks/task.md"})
	if err != nil {
		panic(err)
	}
	return snapshot
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
		setRunningStep(&st, "approval")
		st.SchemaVersion = state.CurrentSchemaVersion
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
		if got.State.EntrySequence() != 1 || got.State.SchemaVersion != state.CurrentSchemaVersion || got.State.CurrentAttemptID != "" {
			t.Fatalf("state=%#v", got.State)
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
			OK: false,
			ArtifactProblems: []gate.ArtifactProblem{
				{Path: "docs/code-review.md", Kind: gate.ArtifactFileMissing},
			},
			MissingApprovals: []string{"first"},
		})

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeMissingRequiredArtifact, CodeMissingRequiredApproval)
	})

	t.Run("preserves Flow artifact problem order across kinds", func(t *testing.T) {
		st := runningState()
		got := ApplyDone(testFlow(), st, gate.Result{
			ArtifactProblems: []gate.ArtifactProblem{
				{Path: "out/evidence.md", Kind: gate.ArtifactEvidenceMissing},
				{Path: "out/file.md", Kind: gate.ArtifactFileMissing},
				{Path: "out/unsafe.md", Kind: gate.ArtifactUnsafe},
				{Path: "out/mismatch.md", Kind: gate.ArtifactMismatch},
			},
		})
		assertFailure(t, got,
			CodeMissingArtifactEvidence,
			CodeMissingRequiredArtifact,
			CodeArtifactUnsafe,
			CodeArtifactEvidenceMismatch,
		)
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
	t.Run("approves current active attempt", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "approval")
		before := st.Clone()

		got := ApplyApprove(testFlow(), st, "approval", st.CurrentAttemptID, " approved ")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		approval := got.State.Attempts[0].Approval
		if approval == nil || approval.Note != " approved " {
			t.Fatalf("approval = %#v", approval)
		}
	})

	t.Run("rejects approval not required", func(t *testing.T) {
		st := runningState()
		before := st.Clone()
		got := ApplyApprove(testFlow(), st, "first", st.CurrentAttemptID, "ok")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeApprovalNotRequired)
	})

	t.Run("rejects invalid stale mismatch blank and duplicate", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "approval")
		before := st.Clone()
		assertFailure(t, ApplyApprove(testFlow(), st, "approval", "bad", "ok"), CodeInvalidAttemptID)
		assertFailure(t, ApplyApprove(testFlow(), st, "approval", "attempt_00000000000000000099", "ok"), CodeInvalidAttemptID)
		closed := st.Attempts[0]
		closed, _ = state.CloseStepAttempt(closed, state.StepAttemptExitBack, "retry")
		current, _ := state.NewStepAttempt("approval", 2)
		st.Attempts = []state.StepAttempt{closed, current}
		st.CurrentAttemptID = current.ID
		assertFailure(t, ApplyApprove(testFlow(), st, "approval", closed.ID, "ok"), CodeStaleAttempt)
		assertFailure(t, ApplyApprove(testFlow(), st, "other", current.ID, "ok"), CodeStepAttemptMismatch)
		assertFailure(t, ApplyApprove(testFlow(), st, "approval", current.ID, " "), CodeInvalidApprovalNote)
		approved := ApplyApprove(testFlow(), st, "approval", current.ID, "first")
		assertSuccess(t, approved)
		assertFailure(t, ApplyApprove(testFlow(), *approved.State, "approval", current.ID, "second"), CodeAttemptAlreadyApproved)
		assertStateNotMutated(t, before, before)
	})
}

func TestApplyApproveRequiresCurrentAttemptArtifactEvidence(t *testing.T) {
	fl := testFlow()
	fl.Steps[0].Approval = &flow.Approval{Required: true}
	fl.Steps[0].Artifacts = []flow.Artifact{{Path: "out/report.md", Required: true}}
	st := runningState()
	st.FlowSnapshot = testSnapshot(fl)
	before := st.Clone()

	missing := ApplyApprove(fl, st, "first", st.CurrentAttemptID, "ok")
	assertFailure(t, missing, CodeMissingArtifactEvidence)
	assertStateNotMutated(t, before, st)

	st.Attempts[0].ArtifactEvidence["out/report.md"] = state.ArtifactEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	got := ApplyApprove(fl, st, "first", st.CurrentAttemptID, "ok")
	assertSuccess(t, got)
	if got.State.Attempts[0].Approval == nil || got.State.Attempts[0].ArtifactEvidence["out/report.md"].Size != 1 {
		t.Fatalf("Attempt = %#v", got.State.Attempts[0])
	}
}

func TestApplyBack(t *testing.T) {
	t.Run("moves to previous step and records history", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "second")
		st.CompletedSteps = []string{"first", "second"}
		st.SkippedSteps["first"] = state.SkippedStep{Reason: "kept"}
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "", "revise")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		if got.State.CurrentStepID != "first" {
			t.Fatalf("CurrentStepID = %q, want first", got.State.CurrentStepID)
		}
		assertStrings(t, got.State.CompletedSteps, []string{})
		if len(got.State.BackHistory) != 1 {
			t.Fatalf("BackHistory len = %d, want 1", len(got.State.BackHistory))
		}
		if len(got.State.SkippedSteps) != 0 {
			t.Fatalf("downstream skipped state was not invalidated: %#v", got.State.SkippedSteps)
		}
		assertStrings(t, got.State.BackHistory[0].InvalidatedStepIDs, []string{"first", "second"})
	})

	t.Run("fails when no previous step exists", func(t *testing.T) {
		st := runningState()
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "", "revise")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeNoPreviousStep)
	})

	t.Run("fails when reason is empty", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "second")
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "", " ")

		assertStateNotMutated(t, before, st)
		assertFailure(t, got, CodeEmptyReason)
	})

	t.Run("moves to specified upstream step", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "approval")
		st.CompletedSteps = []string{"first", "second", "approval"}
		before := st.Clone()

		got := ApplyBack(testFlow(), st, "first", "revise")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got)
		assertStrings(t, got.State.BackHistory[0].InvalidatedStepIDs, []string{"first", "second", "approval"})
	})

	for _, target := range []string{"missing", "second", "approval"} {
		t.Run("rejects invalid target "+target, func(t *testing.T) {
			st := runningState()
			setRunningStep(&st, "second")
			before := st.Clone()

			got := ApplyBack(testFlow(), st, target, "revise")

			assertStateNotMutated(t, before, st)
			assertFailure(t, got, CodeInvalidBackTarget)
		})
	}
}

func TestApplyBackInvalidatesFutureSkippedApprovalAndKeepsFinish(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*state.State)
	}{
		{name: "future skipped step", setup: func(st *state.State) {
			st.SkippedSteps["approval"] = state.SkippedStep{Reason: "not needed"}
		}},
		{name: "future approval", setup: func(st *state.State) {
			// Approval cannot exist before its StepAttempt.
		}},
		{name: "future step has no state", setup: func(st *state.State) {}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := runningState()
			setRunningStep(&st, "second")
			st.CompletedSteps = []string{"first", "second"}
			st.Finish = &state.Finish{Reason: "keep this value"}
			tt.setup(&st)

			got := ApplyBack(testFlow(), st, "first", "revise")

			assertSuccess(t, got)
			want := []string{"first", "second"}
			if tt.name == "future skipped step" {
				want = append(want, "approval")
			}
			assertStrings(t, got.State.BackHistory[0].InvalidatedStepIDs, want)
			if _, ok := got.State.SkippedSteps["approval"]; ok {
				t.Fatalf("future skipped step was not invalidated")
			}
			if got.State.Finish == nil || got.State.Finish.Reason != "keep this value" {
				t.Fatalf("Finish = %#v, want preserved", got.State.Finish)
			}
		})
	}
}

func TestEntrySequenceChangesOnlyOnSuccessfulMoves(t *testing.T) {
	st := runningState()
	st.FlowRunID = "run_test"
	st.Attempts[0].CheckResults["old"] = state.CheckResult{ExitCode: 0}

	failed := ApplyDone(testFlow(), st, gate.Result{OK: false})
	assertFailure(t, failed, CodeInvalidGateResult)
	if st.EntrySequence() != 1 || len(st.Attempts[0].CheckResults) != 1 {
		t.Fatalf("failed transition mutated state: %#v", st)
	}

	done := ApplyDone(testFlow(), st, gate.Result{OK: true})
	assertSuccess(t, done)
	if done.State.EntrySequence() != 2 || len(done.State.Attempts[1].CheckResults) != 0 || len(done.State.Attempts[0].CheckResults) != 1 {
		t.Fatalf("done state=%#v", done.State)
	}

	back := ApplyBack(testFlow(), *done.State, "", "revise")
	assertSuccess(t, back)
	if back.State.EntrySequence() != 3 || len(back.State.Attempts[2].CheckResults) != 0 || len(back.State.Attempts[0].CheckResults) != 1 {
		t.Fatalf("back state=%#v", back.State)
	}
}

func TestApplySkipWarnsForRequiredChecks(t *testing.T) {
	fl := testFlow()
	fl.Steps[0].RequiredChecks = []string{"go-test"}
	st := runningState()
	got := ApplySkip(fl, st, "skip checks")
	assertSuccess(t, got, CodeSkippedRequiredCheck)
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
		setRunningStep(&st, "approval")
		st.SchemaVersion = state.CurrentSchemaVersion
		before := st.Clone()

		got := ApplySkip(testFlow(), st, "skip final")

		assertStateNotMutated(t, before, st)
		assertSuccess(t, got, CodeSkippedRequiredApproval, CodeSkippedFinalStep, CodeSkippedFinalApprovalStep)
		if got.State.Status != state.StatusCompleted {
			t.Fatalf("Status = %q, want completed", got.State.Status)
		}
		if got.State.EntrySequence() != 1 || got.State.SchemaVersion != state.CurrentSchemaVersion || got.State.CurrentAttemptID != "" {
			t.Fatalf("state=%#v", got.State)
		}
	})

	t.Run("warns when required artifact step is skipped", func(t *testing.T) {
		st := runningState()
		setRunningStep(&st, "second")
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
		st.SchemaVersion = state.CurrentSchemaVersion
		st.Attempts[0].CheckResults["go-test"] = state.CheckResult{ExitCode: 1}
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
		if got.State.SchemaVersion != state.CurrentSchemaVersion || got.State.EntrySequence() != 1 || got.State.Attempts[0].CheckResults["go-test"].ExitCode != 1 {
			t.Fatalf("check context was not preserved: %#v", got.State)
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
	attempt, err := state.NewStepAttempt("first", 1)
	if err != nil {
		panic(err)
	}
	st := state.State{
		SchemaVersion:    state.CurrentSchemaVersion,
		FlowSnapshot:     testSnapshot(testFlow()),
		TaskSnapshot:     transitionTaskSnapshot(),
		Status:           state.StatusRunning,
		CurrentStepID:    "first",
		FlowRunID:        "run_00000000000000000000000000000000",
		Attempts:         []state.StepAttempt{attempt},
		CurrentAttemptID: attempt.ID,
	}
	st.Normalize()
	return st
}

func setRunningStep(st *state.State, stepID string) {
	attempt, err := state.NewStepAttempt(stepID, 1)
	if err != nil {
		panic(err)
	}
	st.CurrentStepID = stepID
	st.Attempts = []state.StepAttempt{attempt}
	st.CurrentAttemptID = attempt.ID
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
