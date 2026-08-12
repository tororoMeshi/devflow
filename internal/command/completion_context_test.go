package command

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func TestCompletionContextCurrentAttemptGateProjectionAndImmutability(t *testing.T) {
	root := t.TempDir()
	st := completionContextTestState(t)
	saveExecutionState(t, root, st)
	statePath := currentStatePath(t, root)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	got := CompletionContext(Context{ProjectRoot: root}, "design", st.CurrentAttemptID)
	if got.ExitCode != 0 || got.CompletionContext == nil {
		t.Fatalf("CompletionContext() = %#v", got)
	}
	context := got.CompletionContext
	if context.Completion.Status != "blocked" || context.Completion.Blocker == nil || context.Completion.Blocker.Code != "missing_artifact_evidence" || context.Completion.Blocker.SubjectID == nil || *context.Completion.Blocker.SubjectID != "docs/design.md" {
		t.Fatalf("Completion = %#v, blocker = %#v", context.Completion, context.Completion.Blocker)
	}
	if got, want := context.Artifacts[0].Status, "missing"; got != want {
		t.Fatalf("artifact status = %q, want %q", got, want)
	}
	if context.Artifacts[0].Digest != nil || context.Artifacts[0].Size != nil || context.Checks[0].ExitCode != nil {
		t.Fatalf("null fields = %#v %#v", context.Artifacts[0], context.Checks[0])
	}
	if !reflect.DeepEqual(context.Checks, []CompletionCheck{{ID: "validate", Status: "pending"}, {ID: "review", Status: "pending"}}) {
		t.Fatalf("Checks = %#v", context.Checks)
	}
	if context.Approval.Status != "pending" || context.Approval.EvidenceSetDigest != nil || context.Approval.ApprovedEvidenceSetDigest != nil {
		t.Fatalf("Approval = %#v", context.Approval)
	}
	first, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(context)
	if err != nil || string(first) != string(second) {
		t.Fatalf("JSON is not deterministic: %q / %q", first, second)
	}
	if string(first[:len(`{"schema_version":1,"flow_run_id":`)]) != `{"schema_version":1,"flow_run_id":` {
		t.Fatalf("field order = %s", first)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("CompletionContext changed state")
	}
}

func TestCompletionContextApprovalAndAttemptLifecycle(t *testing.T) {
	root := t.TempDir()
	st := completionContextTestState(t)
	writeCommandTestFile(t, root+"/docs/design.md", "x")
	writeCommandTestFile(t, root+"/docs/optional.md", "y")
	st.Attempts[0].ArtifactEvidence["docs/design.md"] = state.ArtifactEvidence{Digest: "sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881", Size: 1}
	st.Attempts[0].ArtifactEvidence["docs/optional.md"] = state.ArtifactEvidence{Digest: "sha256:a1fce4363854ff888cff4b8e7f3620e956c1d6a266702ed8e29a8e13c8c2b7c0", Size: 1}
	st.Attempts[0].CheckResults["validate"] = state.CheckResult{ExitCode: 0}
	st.Attempts[0].CheckResults["review"] = state.CheckResult{ExitCode: 0}
	digest, err := state.ArtifactEvidenceSetDigest([]string{"docs/design.md", "docs/optional.md"}, st.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts[0], err = state.ApproveStepAttempt(st.Attempts[0], "approved", digest)
	if err != nil {
		t.Fatal(err)
	}
	saveExecutionState(t, root, st)
	ready := CompletionContext(Context{ProjectRoot: root}, "design", st.CurrentAttemptID).CompletionContext
	if ready.Completion.Status != "ready" || ready.Completion.Blocker != nil || ready.Approval.Status != "approved" || ready.Approval.EvidenceSetDigest == nil || *ready.Approval.EvidenceSetDigest != digest || ready.Artifacts[0].Status != "recorded" {
		t.Fatalf("ready context = %#v", ready)
	}
	st.Attempts[0].CheckResults["review"] = state.CheckResult{ExitCode: 1}
	saveExecutionState(t, root, st)
	failed := CompletionContext(Context{ProjectRoot: root}, "design", st.CurrentAttemptID).CompletionContext
	if failed.Completion.Status != "blocked" || failed.Completion.Blocker == nil || failed.Completion.Blocker.Code != "failed_check" || *failed.Completion.Blocker.SubjectID != "review" {
		t.Fatalf("failed check context = %#v", failed)
	}
	st.Attempts[0].CheckResults["review"] = state.CheckResult{ExitCode: 0}

	closed, err := state.CloseStepAttempt(st.Attempts[0], state.StepAttemptExitBack, "retry")
	if err != nil {
		t.Fatal(err)
	}
	next, err := state.NewStepAttempt("design", 2)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts = []state.StepAttempt{closed, next}
	st.CurrentAttemptID = next.ID
	saveExecutionState(t, root, st)
	got := CompletionContext(Context{ProjectRoot: root}, "design", closed.ID)
	if got.ExitCode != 0 || got.CompletionContext.AttemptStatus != state.StepAttemptClosed || got.CompletionContext.IsCurrentAttempt || got.CompletionContext.Completion.Status != "not_applicable" || got.CompletionContext.Completion.Blocker.Code != "attempt_closed" {
		t.Fatalf("closed context = %#v", got)
	}
}

func TestCompletionContextApprovalNotRequiredReady(t *testing.T) {
	root := t.TempDir()
	st := completionContextTestState(t)
	st.FlowSnapshot.Flow.Steps[0].Approval = nil
	snapshot, err := flow.BuildSnapshot(st.FlowSnapshot.Flow, flow.FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	st.FlowSnapshot = snapshot
	writeCommandTestFile(t, root+"/docs/design.md", "x")
	st.Attempts[0].ArtifactEvidence["docs/design.md"] = state.ArtifactEvidence{Digest: "sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881", Size: 1}
	st.Attempts[0].CheckResults["validate"] = state.CheckResult{ExitCode: 0}
	st.Attempts[0].CheckResults["review"] = state.CheckResult{ExitCode: 0}
	saveExecutionState(t, root, st)
	got := CompletionContext(Context{ProjectRoot: root}, "design", st.CurrentAttemptID).CompletionContext
	if got.Completion.Status != "ready" || got.Approval.Status != "not_required" || got.Approval.EvidenceSetDigest != nil || got.Approval.ApprovedEvidenceSetDigest != nil {
		t.Fatalf("not-required context = %#v", got)
	}
}

func completionContextTestState(t *testing.T) state.State {
	t.Helper()
	snapshot, err := flow.BuildSnapshot(flow.Flow{
		ID:    "completion-flow",
		Title: "Completion Flow",
		Steps: []flow.Step{{
			ID:             "design",
			Title:          "Design",
			Objective:    "Create design.",
			Artifacts:      []flow.Artifact{{Path: "docs/design.md", Required: true}, {Path: "docs/optional.md", Required: false}},
			RequiredChecks: []string{"validate", "review"},
			Approval:       &flow.Approval{Required: true},
		}},
	}, flow.FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	return commandStateWithAttempt(snapshot, testTaskSnapshot(), state.StatusRunning, "design", "run_0123456789abcdef0123456789abcdef")
}

func TestCompletionContextAttemptDiagnostics(t *testing.T) {
	root := t.TempDir()
	if result := Init(Context{ProjectRoot: root}); result.ExitCode != 0 {
		t.Fatalf("Init() = %#v", result)
	}
	if result := startWithTestTask(t, root, "post-task-review"); result.ExitCode != 0 {
		t.Fatalf("Start() = %#v", result)
	}
	st := NewStore(Context{ProjectRoot: root}).LoadCurrent().State
	for _, tt := range []struct {
		step, attempt, code string
	}{
		{st.CurrentStepID, "bad", transition.CodeInvalidAttemptID},
		{st.CurrentStepID, "attempt_00000000000000000002", transition.CodeInvalidAttemptID},
		{"other", st.CurrentAttemptID, transition.CodeStepAttemptMismatch},
	} {
		got := CompletionContext(Context{ProjectRoot: root}, tt.step, tt.attempt)
		if got.ExitCode != 1 || got.CompletionContext != nil || len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != tt.code {
			t.Fatalf("CompletionContext(%q, %q) = %#v", tt.step, tt.attempt, got)
		}
	}
}
