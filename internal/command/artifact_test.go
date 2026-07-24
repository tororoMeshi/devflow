package command

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestRecordArtifactSuccessIdempotencyAndImmutability(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "artifact")

	first := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/required.md")
	assertCommandSuccess(t, first)
	if first.Success.RecordedArtifactPath != "docs/required.md" ||
		first.Success.RecordedAttemptID != st.CurrentAttemptID ||
		first.Success.RecordedArtifactDigest == "" ||
		first.Success.RecordedArtifactSize != 8 {
		t.Fatalf("Success = %#v", first.Success)
	}
	recorded := loadCommandState(t, root)
	before := readCommandFile(t, currentStatePath(t, root))
	second := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/required.md")
	assertCommandSuccess(t, second)
	after := readCommandFile(t, currentStatePath(t, root))
	if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(loadCommandState(t, root), recorded) {
		t.Fatal("idempotent record changed State meaning")
	}

	if err := os.WriteFile(filepath.Join(root, "docs", "required.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/required.md")
	assertCommandFailure(t, failed, transition.CodeArtifactEvidenceAlreadyRecorded)
	assertCommandFileUnchanged(t, currentStatePath(t, root), after)
}

func TestRecordOptionalArtifactSuccessIdempotencyConflictAndSafety(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	optionalPath := filepath.Join(root, "docs", "optional.md")
	writeCommandTestFile(t, optionalPath, "optional")

	first := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional.md")
	assertCommandSuccess(t, first)
	if first.Success.RecordedArtifactPath != "docs/optional.md" ||
		first.Success.RecordedArtifactDigest == "" || first.Success.RecordedArtifactSize != 8 {
		t.Fatalf("Success = %#v", first.Success)
	}
	recorded := loadCommandState(t, root)
	evidence := recorded.Attempts[0].ArtifactEvidence["docs/optional.md"]
	before := readCommandFile(t, currentStatePath(t, root))
	second := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional.md")
	assertCommandSuccess(t, second)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
	if loadCommandState(t, root).Attempts[0].ArtifactEvidence["docs/optional.md"] != evidence {
		t.Fatal("idempotent optional Evidence changed")
	}

	if err := os.WriteFile(optionalPath, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflict := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional.md")
	assertCommandFailure(t, conflict, transition.CodeArtifactEvidenceAlreadyRecorded)
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)

	missing := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional-directory")
	assertCommandFailure(t, missing, CodeArtifactFileMissing)
	if err := os.Mkdir(filepath.Join(root, "docs", "optional-directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	directory := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional-directory")
	assertCommandFailure(t, directory, CodeArtifactNotRegular)
	if err := os.Symlink("optional.md", filepath.Join(root, "docs", "optional-link")); err == nil {
		link := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/optional-link")
		assertCommandFailure(t, link, CodeArtifactSymlink)
	}
	assertCommandFileUnchanged(t, currentStatePath(t, root), before)
}

func TestRecordArtifactRejectsContractErrorsBeforeFilesystemAndDoesNotSave(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approve-done-flow", approveDoneTestFlow())
	st := approveDoneState("artifact")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	before := readCommandFile(t, currentStatePath(t, root))
	for _, tt := range []struct {
		name, step, attempt, path, code string
	}{
		{"malformed", "artifact", "bad", "docs/required.md", transition.CodeInvalidAttemptID},
		{"nonexistent", "artifact", "attempt_00000000000000000099", "docs/required.md", transition.CodeInvalidAttemptID},
		{"mismatch", "other", st.CurrentAttemptID, "docs/required.md", transition.CodeStepAttemptMismatch},
		{"unknown", "artifact", st.CurrentAttemptID, "docs/unknown.md", transition.CodeArtifactNotDeclared},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := RecordArtifact(Context{ProjectRoot: root}, tt.step, tt.attempt, tt.path)
			assertCommandFailure(t, got, tt.code)
			assertCommandFileUnchanged(t, currentStatePath(t, root), before)
		})
	}

	approved := loadCommandState(t, root)
	fl := approved.FlowSnapshot.Flow
	for i := range fl.Steps {
		if fl.Steps[i].ID == "artifact" {
			fl.Steps[i].Approval = &flow.Approval{Required: true}
		}
	}
	var err error
	approved.FlowSnapshot, err = flow.BuildSnapshot(fl, approved.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	approved.Attempts[0].ArtifactEvidence["docs/required.md"] = state.ArtifactEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	digest, err := state.ArtifactEvidenceSetDigest([]string{"docs/required.md"}, approved.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	approved.Attempts[0].Approval = &state.ApprovalRecord{Note: "ok", EvidenceSetDigest: digest}
	if err := NewStore(Context{ProjectRoot: root}).SaveCurrent(approved); err != nil {
		t.Fatal(err)
	}
	afterApproval := readCommandFile(t, currentStatePath(t, root))
	got := RecordArtifact(Context{ProjectRoot: root}, "artifact", st.CurrentAttemptID, "docs/required.md")
	assertCommandFailure(t, got, transition.CodeArtifactRecordAfterApproval)
	assertCommandFileUnchanged(t, currentStatePath(t, root), afterApproval)
}
