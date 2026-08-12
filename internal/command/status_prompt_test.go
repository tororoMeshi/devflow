package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/gate"
	"github.com/tororoMeshi/devflow/internal/state"
)

func TestStatusArtifactsJSONContractAndAllStates(t *testing.T) {
	inspections := []gate.ArtifactInspection{
		{Path: "current", Required: true},
		{Path: "missing-evidence", Required: true, Problem: gate.CompletionBlockerMissingArtifactEvidence},
		{Path: "missing-file", Required: true, Problem: gate.CompletionBlockerMissingArtifact},
		{Path: "changed", Required: true, Problem: gate.CompletionBlockerArtifactEvidenceMismatch},
		{Path: "unavailable", Required: true, Problem: gate.CompletionBlockerArtifactUnavailable},
		{Path: "optional", Required: false},
	}
	want := []ArtifactStatusResult{
		{Path: "current", State: "current"},
		{Path: "missing-evidence", State: "missing_evidence"},
		{Path: "missing-file", State: "missing_file"},
		{Path: "changed", State: "changed"},
		{Path: "unavailable", State: "unavailable"},
	}
	if got := artifactStatusResults(inspections); !reflect.DeepEqual(got, want) {
		t.Fatalf("artifactStatusResults() = %#v, want %#v", got, want)
	}
	if got := artifactStatusState(gate.CompletionBlockerKind("unknown")); got != "unavailable" {
		t.Fatalf("unknown artifact state = %q", got)
	}
	data, err := json.Marshal(StatusResult{Artifacts: []ArtifactStatusResult{}})
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	artifacts, exists := value["artifacts"]
	if !exists || artifacts == nil || len(artifacts.([]any)) != 0 {
		t.Fatalf("status JSON = %s", data)
	}
	if _, exists := value["Artifacts"]; exists {
		t.Fatalf("legacy field name present: %s", data)
	}
}

func TestPromptCompoundArtifactStateOnlySuggestsRecordableEvidence(t *testing.T) {
	inspections := []gate.ArtifactInspection{
		{Path: "missing.md", Required: true, Problem: gate.CompletionBlockerMissingArtifactEvidence},
		{Path: "current.md", Required: true},
		{Path: "changed.md", Required: true, Problem: gate.CompletionBlockerArtifactEvidenceMismatch},
		{Path: "gone.md", Required: true, Problem: gate.CompletionBlockerMissingArtifact},
		{Path: "unsafe.md", Required: true, Problem: gate.CompletionBlockerArtifactUnavailable},
		{Path: "optional.md", Required: false},
	}
	wantBlockers := []string{
		"changed.md: changed; recorded evidence is no longer current",
		"gone.md: missing_file; recorded evidence is no longer current",
		"unsafe.md: unavailable; recorded evidence is no longer current",
	}
	if got := promptArtifactBlockers(inspections); !reflect.DeepEqual(got, wantBlockers) {
		t.Fatalf("blockers = %#v, want %#v", got, wantBlockers)
	}
}

func TestStatusReturnsActiveFlowState(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	st.CompletedSteps = []string{"first"}
	st.SkippedSteps["skipped"] = state.SkippedStep{Reason: "not needed"}
	st.Attempts[0].ArtifactEvidence["docs/required.md"] = state.ArtifactEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	digest, err := state.ArtifactEvidenceSetDigest([]string{"docs/required.md"}, st.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts[0].Approval = &state.ApprovalRecord{Note: "ok", EvidenceSetDigest: digest}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Status(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if got.Status == nil {
		t.Fatalf("Status = nil")
	}
	if got.Status.FlowID != "status-flow" {
		t.Fatalf("FlowID = %q", got.Status.FlowID)
	}
	if got.Status.FlowTitle != "Status Prompt Flow" {
		t.Fatalf("FlowTitle = %q", got.Status.FlowTitle)
	}
	if got.Status.CurrentStepID != "current" {
		t.Fatalf("CurrentStepID = %q", got.Status.CurrentStepID)
	}
	if got.Status.CurrentStepTitle != "Current" {
		t.Fatalf("CurrentStepTitle = %q", got.Status.CurrentStepTitle)
	}
	if len(got.Status.CompletedSteps) != 1 || got.Status.CompletedSteps[0] != "first" {
		t.Fatalf("CompletedSteps = %#v", got.Status.CompletedSteps)
	}
	if got.Status.SkippedSteps["skipped"].Reason != "not needed" {
		t.Fatalf("SkippedSteps = %#v", got.Status.SkippedSteps)
	}
	if got.Status.Approval == nil || !got.Status.Approval.Approved || got.Status.Approval.Note != "ok" {
		t.Fatalf("Approval = %#v", got.Status.Approval)
	}
}

func TestStatusDoesNotExposePastAttemptApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	st.Attempts[0].ArtifactEvidence["docs/required.md"] = state.ArtifactEvidence{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   1,
	}
	oldDigest, err := state.ArtifactEvidenceSetDigest([]string{"docs/required.md"}, st.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := state.ApproveStepAttempt(st.Attempts[0], "old", oldDigest)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := state.CloseStepAttempt(approved, state.StepAttemptExitBack, "retry")
	if err != nil {
		t.Fatal(err)
	}
	current, err := state.NewStepAttempt("current", 2)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts = []state.StepAttempt{closed, current}
	st.CurrentAttemptID = current.ID
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	got := Status(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	if got.Status == nil || got.Status.Approval != nil {
		t.Fatalf("Status = %#v", got.Status)
	}
	if !reflect.DeepEqual(got.Status.Artifacts, []ArtifactStatusResult{{Path: "docs/required.md", State: "missing_evidence"}}) {
		t.Fatalf("historical evidence leaked into status: %#v", got.Status.Artifacts)
	}
	prompt := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, prompt)
	if prompt.Prompt == nil {
		t.Fatalf("Prompt = nil")
	}
}

func TestStatusReportsCurrentArtifactState(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	if got := Status(Context{ProjectRoot: root}).Status.Artifacts; !reflect.DeepEqual(got, []ArtifactStatusResult{{Path: "docs/required.md", State: "missing_evidence"}}) {
		t.Fatalf("missing evidence artifacts = %#v", got)
	}
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "x")
	st.Attempts[0].ArtifactEvidence["docs/required.md"] = state.ArtifactEvidence{Digest: "sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881", Size: 1}
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	if got := Status(Context{ProjectRoot: root}).Status.Artifacts; !reflect.DeepEqual(got, []ArtifactStatusResult{{Path: "docs/required.md", State: "current"}}) {
		t.Fatalf("current artifacts = %#v", got)
	}
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "changed")
	if got := Status(Context{ProjectRoot: root}).Status.Artifacts; !reflect.DeepEqual(got, []ArtifactStatusResult{{Path: "docs/required.md", State: "changed"}}) {
		t.Fatalf("changed artifacts = %#v", got)
	}
	blockedPrompt := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, blockedPrompt)
	if len(blockedPrompt.Prompt.ArtifactBlockers) != 1 {
		t.Fatalf("changed prompt = %#v", blockedPrompt.Prompt)
	}
	if err := os.Remove(filepath.Join(root, "docs", "required.md")); err != nil {
		t.Fatal(err)
	}
	if got := Status(Context{ProjectRoot: root}).Status.Artifacts; !reflect.DeepEqual(got, []ArtifactStatusResult{{Path: "docs/required.md", State: "missing_file"}}) {
		t.Fatalf("missing file artifacts = %#v", got)
	}
	if err := os.Symlink("../target", filepath.Join(root, "docs", "required.md")); err != nil {
		t.Fatal(err)
	}
	if got := Status(Context{ProjectRoot: root}).Status.Artifacts; !reflect.DeepEqual(got, []ArtifactStatusResult{{Path: "docs/required.md", State: "unavailable"}}) {
		t.Fatalf("unavailable artifacts = %#v", got)
	}
}

func TestPromptReturnsCurrentStepDetails(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}

	got := Prompt(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if got.Prompt == nil {
		t.Fatalf("Prompt = nil")
	}
	if got.Prompt.FlowID != "status-flow" {
		t.Fatalf("FlowID = %q", got.Prompt.FlowID)
	}
	if got.Prompt.TaskContent != st.TaskSnapshot.Content {
		t.Fatalf("TaskContent = %q", got.Prompt.TaskContent)
	}
	if got.Prompt.CurrentStepID != "current" {
		t.Fatalf("CurrentStepID = %q", got.Prompt.CurrentStepID)
	}
	if got.Prompt.CurrentStepTitle != "Current" {
		t.Fatalf("CurrentStepTitle = %q", got.Prompt.CurrentStepTitle)
	}
	if got.Prompt.CurrentStepObjective != "Do current work." {
		t.Fatalf("CurrentStepObjective = %q", got.Prompt.CurrentStepObjective)
	}
	assertArtifactPaths(t, got.Prompt.RequiredArtifacts, []string{"docs/required.md"})
	assertArtifactPaths(t, got.Prompt.OptionalArtifacts, []string{"docs/optional.md"})
	if got.Prompt.RequiredApproval == nil {
		t.Fatalf("RequiredApproval = nil")
	}
	if got.Prompt.RequiredApproval.StepID != "current" {
		t.Fatalf("RequiredApproval.StepID = %q", got.Prompt.RequiredApproval.StepID)
	}
	if got.Prompt.RequiredApproval.AttemptID != st.CurrentAttemptID {
		t.Fatalf("AttemptID = %q", got.Prompt.RequiredApproval.AttemptID)
	}
}

func TestPromptArtifactCommandsRemainExplicitUntilApproval(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	st = loadCommandState(t, root)
	writeCommandTestFile(t, filepath.Join(root, "docs", "required.md"), "x")
	st.Attempts[0].ArtifactEvidence["docs/required.md"] = state.ArtifactEvidence{
		Digest: "sha256:2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881",
		Size:   1,
	}
	if err := NewStore(Context{ProjectRoot: root}).SaveCurrent(st); err != nil {
		t.Fatal(err)
	}
	got := Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
	approvalDigest, err := state.ArtifactEvidenceSetDigest([]string{"docs/required.md"}, st.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := state.ApproveStepAttempt(st.Attempts[0], "ok", approvalDigest)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts[0] = approved
	if err := NewStore(Context{ProjectRoot: root}).SaveCurrent(st); err != nil {
		t.Fatal(err)
	}
	got = Prompt(Context{ProjectRoot: root})
	assertCommandSuccess(t, got)
}

func TestPromptTreatsNoArtifactsAndNoApprovalAsEmpty(t *testing.T) {
	for _, currentStepID := range []string{"first", "no_approval"} {
		t.Run(currentStepID, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
			st := statusPromptState("status-flow", state.StatusRunning, currentStepID)
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}

			got := Prompt(Context{ProjectRoot: root})

			assertCommandSuccess(t, got)
			if got.Prompt == nil {
				t.Fatalf("Prompt = nil")
			}
			if got.Prompt.RequiredArtifacts == nil {
				t.Fatalf("RequiredArtifacts = nil, want empty slice")
			}
			if len(got.Prompt.RequiredArtifacts) != 0 {
				t.Fatalf("RequiredArtifacts = %#v, want empty", got.Prompt.RequiredArtifacts)
			}
			if got.Prompt.OptionalArtifacts != nil {
				t.Fatalf("OptionalArtifacts = %#v, want nil when none", got.Prompt.OptionalArtifacts)
			}
			if got.Prompt.RequiredApproval != nil {
				t.Fatalf("RequiredApproval = %#v, want nil", got.Prompt.RequiredApproval)
			}
		})
	}
}

func TestStatusAndPromptRequireActiveFlow(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantStatus string
	}{
		{
			name:       "no state",
			setup:      func(t *testing.T, root string) {},
			wantStatus: CodeNoActiveFlow,
		},
		{
			name: "invalid state",
			setup: func(t *testing.T, root string) {
				writeCommandTestFile(t, LegacyStatePath(root), `{"not":"valid state"}`)
			},
			wantStatus: CodeUnsupportedStateVersion,
		},
		{
			name: "current step missing from snapshot",
			setup: func(t *testing.T, root string) {
				st := statusPromptState("status-flow", state.StatusRunning, "missing")
				st.FlowSnapshot = statusPromptState("status-flow", state.StatusRunning, "current").FlowSnapshot
				writeCommandStateUnchecked(t, root, st)
			},
			wantStatus: CodeInvalidState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			statusResult := Status(Context{ProjectRoot: root})
			promptResult := Prompt(Context{ProjectRoot: root})

			assertCommandFailure(t, statusResult, tt.wantStatus)
			assertCommandFailure(t, promptResult, tt.wantStatus)
		})
	}
}

func TestStatusTerminalDoesNotExposeHistoricalApproval(t *testing.T) {
	for _, status := range []state.Status{state.StatusCompleted, state.StatusFinished} {
		t.Run(string(status), func(t *testing.T) {
			root := t.TempDir()
			st := statusPromptState("status-flow", status, "current")
			fl := st.FlowSnapshot.Flow
			fl.Steps[0].Approval = &flow.Approval{Required: true}
			var err error
			st.FlowSnapshot, err = flow.BuildSnapshot(fl, st.FlowSnapshot.Source)
			if err != nil {
				t.Fatal(err)
			}
			st.Attempts[0].Approval = &state.ApprovalRecord{Note: "historical", EvidenceSetDigest: "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"}
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			got := Status(Context{ProjectRoot: root})
			assertCommandSuccess(t, got)
			if got.Status == nil || got.Status.Approval != nil {
				t.Fatalf("Status = %#v", got.Status)
			}
			if got.Status.Artifacts == nil || len(got.Status.Artifacts) != 0 {
				t.Fatalf("terminal artifacts = %#v, want nonnil empty", got.Status.Artifacts)
			}
			assertCommandFailure(t, Prompt(Context{ProjectRoot: root}), CodeNoActiveFlow)
		})
	}
}

func TestStatusAndPromptDoNotUpdateState(t *testing.T) {
	for _, command := range []struct {
		name string
		run  func(Context) CommandResult
	}{
		{name: "status", run: Status},
		{name: "prompt", run: Prompt},
	} {
		t.Run(command.name, func(t *testing.T) {
			root := t.TempDir()
			writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
			st := statusPromptState("status-flow", state.StatusRunning, "current")
			if err := saveCommandState(t, root, st); err != nil {
				t.Fatal(err)
			}
			before := readCommandFile(t, currentStatePath(t, root))

			got := command.run(Context{ProjectRoot: root})

			assertCommandSuccess(t, got)
			after := readCommandFile(t, currentStatePath(t, root))
			if string(after) != string(before) {
				t.Fatalf("state.json was modified")
			}
		})
	}
}

func TestPromptDoesNotCheckArtifactExistence(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "status-flow", statusPromptTestFlow())
	st := statusPromptState("status-flow", state.StatusRunning, "current")
	if err := saveCommandState(t, root, st); err != nil {
		t.Fatal(err)
	}
	assertNoFile(t, filepath.Join(root, "docs", "required.md"))

	got := Prompt(Context{ProjectRoot: root})

	assertCommandSuccess(t, got)
	if _, err := os.Stat(filepath.Join(root, "docs", "required.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact unexpectedly exists or stat failed: %v", err)
	}
}

func statusPromptTestFlow() string {
	return `flow: {
		id: "status-flow"
		title: "Status Prompt Flow"
		steps: [{
			id: "first"
			title: "First"
			objective: "Do first."
		}, {
			id: "current"
			title: "Current"
			objective: "Do current work."
			artifacts: [{
				path: "docs/required.md"
				required: true
			}, {
				path: "docs/optional.md"
				required: false
			}]
			approval: {
				required: true
			}
		}, {
			id: "no_approval"
			title: "No Approval"
			objective: "Do work without approval."
			approval: {
				required: false
			}
		}, {
			id: "skipped"
			title: "Skipped"
			objective: "Skip me."
		}]
	}`
}

func statusPromptState(flowID string, status state.Status, currentStepID string) state.State {
	return commandStateWithAttempt(testSnapshotForStep(flowID, currentStepID), testTaskSnapshot(), status, currentStepID, "run_00000000000000000000000000000000")
}

func assertArtifactPaths(t *testing.T, got []ArtifactResult, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("len(artifacts) = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Path != want[i] {
			t.Fatalf("artifact[%d].Path = %q, want %q", i, got[i].Path, want[i])
		}
	}
}
