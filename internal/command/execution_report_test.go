package command

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/8noki8/devflow/internal/executionreport"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
	"github.com/8noki8/devflow/internal/workpackage"
)

func TestExecutionReportRecordBindsAndDoesNotChangeCoreState(t *testing.T) {
	root, st, pkg := setupExecutionReportFlow(t)
	store := NewStore(Context{ProjectRoot: root})
	statePath, _ := store.RunStatePath(st.FlowRunID)
	beforeState, _ := os.ReadFile(statePath)
	beforePointer, _ := os.ReadFile(CurrentPath(root))
	beforeObject := st.Clone()
	path := writeExecutionReportFile(t, root, reportForPackage(pkg))

	first := ExecutionReportRecord(Context{ProjectRoot: root}, path)
	if first.ExitCode != 0 || first.ExecutionReport == nil || first.ExecutionReport.Idempotent {
		t.Fatalf("first=%#v", first)
	}
	second := ExecutionReportRecord(Context{ProjectRoot: root}, path)
	if second.ExitCode != 0 || second.ExecutionReport == nil || !second.ExecutionReport.Idempotent {
		t.Fatalf("second=%#v", second)
	}
	after := store.LoadCurrent()
	afterState, _ := os.ReadFile(statePath)
	afterPointer, _ := os.ReadFile(CurrentPath(root))
	if after.Status != state.LoadOK || !reflect.DeepEqual(beforeObject, *after.State) ||
		!bytes.Equal(beforeState, afterState) || !bytes.Equal(beforePointer, afterPointer) {
		t.Fatal("record changed State or pointer")
	}
}

func TestExecutionReportRecordBindingAndConflictDiagnostics(t *testing.T) {
	root, st, pkg := setupExecutionReportFlow(t)
	report := reportForPackage(pkg)
	path := writeExecutionReportFile(t, root, report)
	if got := ExecutionReportRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatal(got)
	}
	report.Summary = "Different."
	path = writeExecutionReportFile(t, root, report)
	got := ExecutionReportRecord(Context{ProjectRoot: root}, path)
	if got.ExitCode != 1 || !hasDiagnostic(got.Diagnostics, CodeConflictingExecutionReport) {
		t.Fatalf("conflict=%#v", got)
	}

	root2, _, pkg2 := setupExecutionReportFlow(t)
	mismatch := reportForPackage(pkg2)
	mismatch.WorkPackageDigest = "sha256:" + string(bytes.Repeat([]byte("0"), 64))
	got = ExecutionReportRecord(Context{ProjectRoot: root2}, writeExecutionReportFile(t, root2, mismatch))
	if got.ExitCode != 1 || !hasDiagnostic(got.Diagnostics, CodeWorkPackageBindingMismatch) {
		t.Fatalf("digest mismatch=%#v", got)
	}

	stale := reportForPackage(pkg2)
	stale.AttemptID = "attempt_00000000000000000099"
	got = ExecutionReportRecord(Context{ProjectRoot: root2}, writeExecutionReportFile(t, root2, stale))
	if got.ExitCode != 1 || !hasDiagnostic(got.Diagnostics, "error_invalid_attempt_id") {
		t.Fatalf("stale=%#v", got)
	}

	runMismatch := reportForPackage(pkg2)
	runMismatch.FlowRunID = "run_11111111111111111111111111111111"
	got = ExecutionReportRecord(Context{ProjectRoot: root2}, writeExecutionReportFile(t, root2, runMismatch))
	if got.ExitCode != 1 || !hasDiagnostic(got.Diagnostics, CodeExecutionReportRunMismatch) {
		t.Fatalf("run mismatch=%#v st=%s", got, st.FlowRunID)
	}
}

func TestExecutionReportRecordProtocolFailsBeforeStateAccess(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "report.json")
	if err := os.WriteFile(path, []byte(`{"summary":"x","summary":"y"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got := ExecutionReportRecord(Context{ProjectRoot: root}, path)
	if got.ExitCode != 1 || !hasDiagnostic(got.Diagnostics, CodeDuplicateJSONKey) {
		t.Fatalf("got=%#v", got)
	}
}

func TestExecutionReportLateRetryRejectedAfterSkipAndFinish(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		root, _, pkg := setupExecutionReportFlow(t)
		path := writeExecutionReportFile(t, root, reportForPackage(pkg))
		if got := ExecutionReportRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
			t.Fatal(got)
		}
		if got := Skip(Context{ProjectRoot: root}, "worker stopped"); got.ExitCode != 0 {
			t.Fatal(got)
		}
		if got := ExecutionReportRecord(Context{ProjectRoot: root}, path); got.ExitCode == 0 {
			t.Fatalf("late retry accepted: %#v", got)
		}
	})
	t.Run("finish", func(t *testing.T) {
		root, _, pkg := setupExecutionReportFlow(t)
		path := writeExecutionReportFile(t, root, reportForPackage(pkg))
		if got := Finish(Context{ProjectRoot: root}, "stop automation"); got.ExitCode != 0 {
			t.Fatal(got)
		}
		if got := ExecutionReportRecord(Context{ProjectRoot: root}, path); got.ExitCode == 0 {
			t.Fatalf("terminal report accepted: %#v", got)
		}
	})
}

func TestExecutionReportABAUsesOnlyThirdAttempt(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	firstPackage, err := workpackage.Generate(st, st.CurrentStepID, st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := state.CloseStepAttempt(st.Attempts[0], state.StepAttemptExitDone, "")
	second, _ := state.NewStepAttempt("review", 2)
	second, _ = state.CloseStepAttempt(second, state.StepAttemptExitBack, "return")
	third, _ := state.NewStepAttempt("quality", 3)
	st.Attempts = []state.StepAttempt{first, second, third}
	st.CurrentStepID = "quality"
	st.CurrentAttemptID = third.ID
	if err := NewStore(ctx).SaveCurrent(st); err != nil {
		t.Fatal(err)
	}
	thirdPackage, err := workpackage.Generate(st, "quality", third.ID)
	if err != nil {
		t.Fatal(err)
	}
	if firstPackage.WorkPackageDigest == thirdPackage.WorkPackageDigest {
		t.Fatal("A→B→A attempts have the same WorkPackage digest")
	}
	oldPath := writeExecutionReportFile(t, root, reportForPackage(firstPackage))
	if got := ExecutionReportRecord(ctx, oldPath); got.ExitCode == 0 ||
		!hasDiagnostic(got.Diagnostics, transition.CodeStaleAttempt) {
		t.Fatalf("Attempt 1 accepted: %#v", got)
	}
	newPath := writeExecutionReportFile(t, root, reportForPackage(thirdPackage))
	if got := ExecutionReportRecord(ctx, newPath); got.ExitCode != 0 {
		t.Fatalf("Attempt 3 rejected: %#v", got)
	}
	reportPath, _ := executionreport.ReportPath(root, st.FlowRunID, third.ID)
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionReportOutcomeDoesNotChangeGateOrState(t *testing.T) {
	for _, outcome := range []executionreport.Outcome{executionreport.OutcomeBlocked, executionreport.OutcomeFailed} {
		t.Run(string(outcome), func(t *testing.T) {
			root, st, pkg := setupExecutionReportFlow(t)
			store := NewStore(Context{ProjectRoot: root})
			statePath, _ := store.RunStatePath(st.FlowRunID)
			beforeState, _ := os.ReadFile(statePath)
			beforePointer, _ := os.ReadFile(CurrentPath(root))
			report := reportForPackage(pkg)
			report.Outcome = outcome
			if got := ExecutionReportRecord(Context{ProjectRoot: root}, writeExecutionReportFile(t, root, report)); got.ExitCode != 0 {
				t.Fatal(got)
			}
			afterState, _ := os.ReadFile(statePath)
			afterPointer, _ := os.ReadFile(CurrentPath(root))
			if !bytes.Equal(beforeState, afterState) || !bytes.Equal(beforePointer, afterPointer) {
				t.Fatal("outcome changed State or pointer")
			}
			loaded := store.LoadCurrent()
			attempt, _, ok := loaded.State.CurrentAttempt()
			if !ok || loaded.State.Status != state.StatusRunning || attempt.Status != state.StepAttemptActive {
				t.Fatalf("outcome changed lifecycle: %#v", loaded.State)
			}
			if got := Done(Context{ProjectRoot: root}); got.ExitCode != 0 {
				t.Fatalf("existing Completion Gate changed: %#v", got)
			}
		})
	}
}

func TestDoneDoesNotRequireExecutionReport(t *testing.T) {
	root, _, _ := setupExecutionReportFlow(t)
	if got := Done(Context{ProjectRoot: root}); got.ExitCode != 0 {
		t.Fatalf("done without Report failed: %#v", got)
	}
}

func setupExecutionReportFlow(t *testing.T) (string, state.State, workpackage.WorkPackage) {
	t.Helper()
	root := t.TempDir()
	if result := Init(Context{ProjectRoot: root}); result.ExitCode != 0 {
		t.Fatal(result)
	}
	if result := startWithTestTask(t, root, "post-task-review"); result.ExitCode != 0 {
		t.Fatal(result)
	}
	loaded := NewStore(Context{ProjectRoot: root}).LoadCurrent()
	if loaded.Status != state.LoadOK || loaded.State == nil {
		t.Fatal(loaded.Err)
	}
	pkg, err := workpackage.Generate(*loaded.State, loaded.State.CurrentStepID, loaded.State.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	return root, loaded.State.Clone(), pkg
}

func reportForPackage(pkg workpackage.WorkPackage) executionreport.Report {
	return executionreport.Report{
		SchemaVersion: executionreport.SchemaVersion, FlowRunID: pkg.FlowRunID,
		StepID: pkg.StepID, AttemptID: pkg.AttemptID, WorkPackageDigest: pkg.WorkPackageDigest,
		Outcome: executionreport.OutcomeCompleted, Summary: "Completed requested work.",
		Decisions: []executionreport.DecisionRecord{}, ArtifactRefs: []string{},
		UnresolvedIssues: []string{}, NextAction: "",
	}
}

func writeExecutionReportFile(t *testing.T, root string, report executionreport.Report) string {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "report.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
