package transition

import (
	"reflect"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
)

func TestApplyRecordArtifactEvidence(t *testing.T) {
	st := runningState()
	fl := st.FlowSnapshot.Flow
	fl.Steps[0].Artifacts = []flow.Artifact{{Path: "out/report.md", Required: true}, {Path: "out/optional.md"}}
	st.FlowSnapshot = testSnapshot(fl)
	evidence := state.ArtifactEvidence{Digest: "sha256:" + strings.Repeat("a", 64), Size: 12}
	before := st.Clone()

	got := ApplyRecordArtifactEvidence(st, "first", st.CurrentAttemptID, "out/report.md", evidence)
	assertSuccess(t, got)
	assertStateNotMutated(t, before, st)
	if !reflect.DeepEqual(got.State.Attempts[0].ArtifactEvidence["out/report.md"], evidence) {
		t.Fatalf("evidence = %#v", got.State.Attempts[0].ArtifactEvidence)
	}
	idempotent := ApplyRecordArtifactEvidence(*got.State, "first", st.CurrentAttemptID, "out/report.md", evidence)
	assertSuccess(t, idempotent)
	idempotent.State.Attempts[0].ArtifactEvidence["out/report.md"] = state.ArtifactEvidence{}
	if got.State.Attempts[0].ArtifactEvidence["out/report.md"] != evidence {
		t.Fatal("idempotent result shares Evidence map")
	}
	different := evidence
	different.Size++
	assertFailure(t, ApplyRecordArtifactEvidence(*got.State, "first", st.CurrentAttemptID, "out/report.md", different), CodeArtifactEvidenceAlreadyRecorded)
	optional := ApplyRecordArtifactEvidence(st, "first", st.CurrentAttemptID, "out/optional.md", evidence)
	assertSuccess(t, optional)
	if optional.State.Attempts[0].ArtifactEvidence["out/optional.md"] != evidence {
		t.Fatalf("optional evidence = %#v", optional.State.Attempts[0].ArtifactEvidence)
	}
	optionalAgain := ApplyRecordArtifactEvidence(*optional.State, "first", st.CurrentAttemptID, "out/optional.md", evidence)
	assertSuccess(t, optionalAgain)
	assertFailure(t, ApplyRecordArtifactEvidence(*optional.State, "first", st.CurrentAttemptID, "out/optional.md", different), CodeArtifactEvidenceAlreadyRecorded)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "first", st.CurrentAttemptID, "out/unknown.md", evidence), CodeArtifactNotDeclared)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "first", "bad", "out/report.md", evidence), CodeInvalidAttemptID)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "first", "attempt_00000000000000000099", "out/report.md", evidence), CodeInvalidAttemptID)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "second", st.CurrentAttemptID, "out/report.md", evidence), CodeStepAttemptMismatch)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "first", st.CurrentAttemptID, "../report.md", evidence), CodeInvalidArtifactPath)
	assertFailure(t, ApplyRecordArtifactEvidence(st, "first", st.CurrentAttemptID, "out/report.md", state.ArtifactEvidence{}), CodeInvalidArtifactDigest)

	approved := st.Clone()
	approved.Attempts[0].ArtifactEvidence["out/report.md"] = evidence
	approved.Attempts[0].ArtifactEvidence["out/optional.md"] = evidence
	approvedDigest, err := state.ArtifactEvidenceSetDigest(
		[]string{"out/optional.md", "out/report.md"},
		approved.Attempts[0].ArtifactEvidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	approved.Attempts[0].Approval = &state.ApprovalRecord{Note: "ok", EvidenceSetDigest: approvedDigest}
	assertFailure(t, ApplyRecordArtifactEvidence(approved, "first", approved.CurrentAttemptID, "out/optional.md", evidence), CodeArtifactRecordAfterApproval)

	stale := st.Clone()
	first, err := state.CloseStepAttempt(stale.Attempts[0], state.StepAttemptExitBack, "retry")
	if err != nil {
		t.Fatal(err)
	}
	second, err := state.NewStepAttempt("first", 2)
	if err != nil {
		t.Fatal(err)
	}
	stale.Attempts = []state.StepAttempt{first, second}
	stale.CurrentAttemptID = second.ID
	assertFailure(t, ApplyRecordArtifactEvidence(stale, "first", first.ID, "out/optional.md", evidence), CodeStaleAttempt)

	middleActive, err := state.NewStepAttempt("second", 2)
	if err != nil {
		t.Fatal(err)
	}
	middle, err := state.CloseStepAttempt(middleActive, state.StepAttemptExitDone, "")
	if err != nil {
		t.Fatal(err)
	}
	third, err := state.NewStepAttempt("first", 3)
	if err != nil {
		t.Fatal(err)
	}
	stale.Attempts = []state.StepAttempt{first, middle, third}
	stale.CurrentAttemptID = third.ID
	stale.CurrentStepID = "first"
	assertFailure(t, ApplyRecordArtifactEvidence(stale, "first", first.ID, "out/optional.md", evidence), CodeStaleAttempt)

	terminal := st.Clone()
	terminal.Status = state.StatusCompleted
	terminal.CurrentAttemptID = ""
	assertFailure(t, ApplyRecordArtifactEvidence(terminal, "first", st.CurrentAttemptID, "out/report.md", evidence), CodeNoActiveFlow)
}
