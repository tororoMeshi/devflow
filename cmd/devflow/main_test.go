package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/command"
	"github.com/8noki8/devflow/internal/executionreport"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
	"github.com/8noki8/devflow/internal/workpackage"
)

func TestParseExecutionReportRecordArgs(t *testing.T) {
	tests := []struct {
		args []string
		ok   bool
	}{
		{[]string{"--file", "report.json"}, true},
		{[]string{"--file", ""}, false},
		{[]string{"--file", "\u3000"}, false},
		{[]string{"--file", " report.json"}, false},
		{[]string{"--file", "report.json "}, false},
		{[]string{"--file", "a", "--file", "b"}, false},
		{[]string{"--run", "x"}, false},
		{[]string{"--file=report.json"}, false},
		{[]string{"report.json"}, false},
	}
	for _, tt := range tests {
		_, ok := parseExecutionReportRecordArgs(tt.args)
		if ok != tt.ok {
			t.Fatalf("args=%q ok=%t", tt.args, ok)
		}
	}
}

func TestRunExecutionReportExactHumanOutput(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	pkg, err := workpackage.Generate(st, st.CurrentStepID, st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	report := executionreport.Report{
		SchemaVersion: executionreport.SchemaVersion, FlowRunID: pkg.FlowRunID,
		StepID: pkg.StepID, AttemptID: pkg.AttemptID, WorkPackageDigest: pkg.WorkPackageDigest,
		Outcome: executionreport.OutcomeBlocked, Summary: "Blocked externally.",
		Decisions: []executionreport.DecisionRecord{}, ArtifactRefs: []string{},
		UnresolvedIssues: []string{"Waiting for access."}, NextAction: "Retry later.",
	}
	data, _ := json.Marshal(report)
	path := filepath.Join(root, "report.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, _ := executionreport.Digest(report)
	want := fmt.Sprintf("Recorded execution report\nRun: %s\nStep: %s\nAttempt: %s\nWork package: %s\nExecution report: %s\nOutcome: blocked\nIdempotent: false\n",
		pkg.FlowRunID, pkg.StepID, pkg.AttemptID, pkg.WorkPackageDigest, digest)
	stdout, stderr, code := runCapture(root, []string{"execution-report", "record", "--file", path})
	if code != 0 || stderr != "" || stdout != want {
		t.Fatalf("code=%d stdout=%q stderr=%q want=%q", code, stdout, stderr, want)
	}
	want = strings.Replace(want, "Idempotent: false", "Idempotent: true", 1)
	stdout, stderr, code = runCapture(root, []string{"execution-report", "record", "--file", path})
	if code != 0 || stderr != "" || stdout != want {
		t.Fatalf("retry code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestParseWorkPackageArgs(t *testing.T) {
	validAttempt := "attempt_00000000000000000001"
	tests := []struct {
		name string
		args []string
		ok   bool
	}{
		{"formal", []string{"--step", "build", "--attempt", validAttempt}, true},
		{"reverse order", []string{"--attempt", validAttempt, "--step", "build"}, true},
		{"duplicate step", []string{"--step", "build", "--step", "other"}, false},
		{"duplicate attempt", []string{"--attempt", validAttempt, "--attempt", validAttempt}, false},
		{"unknown", []string{"--unknown", "x", "--attempt", validAttempt}, false},
		{"missing value", []string{"--step", "build", "--attempt"}, false},
		{"empty", []string{"--step", "", "--attempt", validAttempt}, false},
		{"ASCII whitespace", []string{"--step", " \t", "--attempt", validAttempt}, false},
		{"Unicode whitespace", []string{"--step", "\u3000", "--attempt", validAttempt}, false},
		{"leading whitespace", []string{"--step", " build", "--attempt", validAttempt}, false},
		{"trailing whitespace", []string{"--step", "build ", "--attempt", validAttempt}, false},
		{"positional", []string{"build", "--attempt", validAttempt, "--step"}, false},
		{"equals", []string{"--step=build", "--attempt", validAttempt}, false},
		{"Run option", []string{"--run", "run_x", "--attempt", validAttempt}, false},
		{"output option", []string{"--output", "pkg.json", "--attempt", validAttempt}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, attempt, ok := parseWorkPackageArgs(tt.args)
			if ok != tt.ok {
				t.Fatalf("parseWorkPackageArgs(%q) = %q, %q, %t", tt.args, step, attempt, ok)
			}
			if ok && (step != "build" || attempt != validAttempt) {
				t.Fatalf("values = %q, %q", step, attempt)
			}
		})
	}
}

func TestRunWorkPackageExactJSONAndFilesystemImmutability(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	store := command.NewStore(command.Context{ProjectRoot: root})
	statePath, err := store.RunStatePath(st.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	pointerPath := command.CurrentPath(root)
	beforeState, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	beforePointer, err := os.ReadFile(pointerPath)
	if err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Dir(statePath)
	beforeEntries, err := os.ReadDir(runDir)
	if err != nil {
		t.Fatal(err)
	}

	wantPackage, err := workpackage.Generate(st, st.CurrentStepID, st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(wantPackage)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, exitCode := runCapture(root, []string{"work-package", "--attempt", st.CurrentAttemptID, "--step", st.CurrentStepID})
	if exitCode != 0 || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if stdout != string(wantJSON)+"\n" {
		t.Fatalf("stdout = %q, want %q", stdout, string(wantJSON)+"\n")
	}
	var decoded workpackage.WorkPackage
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil || workpackage.Validate(decoded) != nil {
		t.Fatalf("invalid stdout package: %v, %#v", err, decoded)
	}
	afterState, _ := os.ReadFile(statePath)
	afterPointer, _ := os.ReadFile(pointerPath)
	afterEntries, _ := os.ReadDir(runDir)
	if !bytes.Equal(beforeState, afterState) || !bytes.Equal(beforePointer, afterPointer) ||
		!reflect.DeepEqual(entryNames(beforeEntries), entryNames(afterEntries)) {
		t.Fatal("work-package changed State, pointer, or Run directory")
	}
}

func TestRunWorkPackageSnapshotSourceFilesAreNotRead(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	args := []string{"work-package", "--step", st.CurrentStepID, "--attempt", st.CurrentAttemptID}
	first, stderr, exitCode := runCapture(root, args)
	if exitCode != 0 || stderr != "" {
		t.Fatalf("first run exit=%d stderr=%q", exitCode, stderr)
	}
	flowPath := filepath.Join(root, ".devflow", "flows", "post-task-review.cue")
	taskPath := filepath.Join(root, "tasks", "task.md")
	if err := os.Remove(flowPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(taskPath); err != nil {
		t.Fatal(err)
	}
	second, stderr, exitCode := runCapture(root, args)
	if exitCode != 0 || stderr != "" || second != first {
		t.Fatalf("source changes affected package: exit=%d stderr=%q\nfirst=%q\nsecond=%q", exitCode, stderr, first, second)
	}
}

func TestRunWorkPackageFailureWritesNoJSON(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, exitCode := runCapture(root, []string{"work-package", "--step", "build", "--attempt", "attempt_00000000000000000001"})
	if exitCode != 1 || stdout != "" || stderr != "error: "+command.CodeNoActiveFlow+"\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestRunWorkPackageFailureLeavesCurrentRunUnchanged(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	store := command.NewStore(command.Context{ProjectRoot: root})
	statePath, err := store.RunStatePath(st.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	pointerPath := command.CurrentPath(root)
	runDir := filepath.Dir(statePath)
	beforeState, _ := os.ReadFile(statePath)
	beforePointer, _ := os.ReadFile(pointerPath)
	beforeEntries, _ := os.ReadDir(runDir)

	stdout, stderr, exitCode := runCapture(root, []string{
		"work-package", "--step", st.CurrentStepID, "--attempt", "attempt_00000000000000000002",
	})
	if exitCode != 1 || stdout != "" || stderr != "error: "+transition.CodeInvalidAttemptID+"\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	afterState, _ := os.ReadFile(statePath)
	afterPointer, _ := os.ReadFile(pointerPath)
	afterEntries, _ := os.ReadDir(runDir)
	if !bytes.Equal(beforeState, afterState) || !bytes.Equal(beforePointer, afterPointer) ||
		!reflect.DeepEqual(entryNames(beforeEntries), entryNames(afterEntries)) {
		t.Fatal("failed work-package changed State, pointer, or Run directory")
	}
}

func TestRunWorkPackageOutputFailureIsNotSuccess(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	var stderr bytes.Buffer
	exitCode := run(
		[]string{"work-package", "--step", st.CurrentStepID, "--attempt", st.CurrentAttemptID},
		root,
		failingWriter{},
		&stderr,
	)
	if exitCode == 0 {
		t.Fatal("WorkPackage output failure was reported as success")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}

func recordCLIArtifact(t *testing.T, root, path string) {
	t.Helper()
	st := loadCLIState(t, root)
	runSuccess(t, root, []string{"artifact", "record", "--step", st.CurrentStepID, "--attempt", st.CurrentAttemptID, "--path", path})
}

func TestArtifactStateHumanOutput(t *testing.T) {
	var statusOutput bytes.Buffer
	writeStatus(&statusOutput, command.StatusResult{
		FlowID: "flow", FlowTitle: "Flow", Artifacts: []command.ArtifactStatusResult{
			{Path: "out/report.md", State: "current"},
			{Path: "out/summary.md", State: "missing_evidence"},
		},
	})
	statusText := statusOutput.String()
	for _, want := range []string{"Artifacts:\n", "- out/report.md: current\n", "- out/summary.md: missing_evidence\n"} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status output = %q, missing %q", statusText, want)
		}
	}
	for _, forbidden := range []string{"sha256:", "Digest:", "Size:"} {
		if strings.Contains(statusText, forbidden) {
			t.Fatalf("status output leaks %q: %q", forbidden, statusText)
		}
	}

	var promptOutput bytes.Buffer
	writePrompt(&promptOutput, command.PromptResult{
		FlowID: "flow", TaskContent: "task", CurrentStepID: "step", CurrentStepTitle: "Step",
		ArtifactBlockers: []string{"out/report.md: changed; recorded evidence is no longer current; continue in a new attempt"},
	})
	promptText := promptOutput.String()
	if !strings.Contains(promptText, "Artifact blockers:\n- out/report.md: changed; recorded evidence is no longer current; continue in a new attempt\n") {
		t.Fatalf("prompt output = %q", promptText)
	}
	for _, forbidden := range []string{"devflow artifact record", "devflow approve", "devflow done", "sha256:"} {
		if strings.Contains(promptText, forbidden) {
			t.Fatalf("prompt output contains %q: %q", forbidden, promptText)
		}
	}
}

func TestRunInit(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"init"}, root, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), command.ActionCreated) {
		t.Fatalf("stdout = %q, want created action", stdout.String())
	}
}

func TestRunList(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"list"}, root, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "id: post-task-review") {
		t.Fatalf("stdout = %q, want flow id", stdout.String())
	}
}

func TestRunStartPassesFlowID(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})

	runSuccess(t, root, []string{"start", "post-task-review"})

	st := loadCLIState(t, root)
	if st.FlowSnapshot.Flow.ID != "post-task-review" {
		t.Fatalf("FlowSnapshot.Flow.ID = %q, want post-task-review", st.FlowSnapshot.Flow.ID)
	}
}

func TestRunApproveParsesStepAttemptAndNote(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})

	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	writeCLITestFile(t, root, "docs/code-review.md")
	recordCLIArtifact(t, root, "docs/code-review.md")
	runSuccess(t, root, []string{"done"})
	before := loadCLIState(t, root)
	runSuccess(t, root, []string{"approve", "--step", "human_approval", "--attempt", before.CurrentAttemptID, "--note", "ok"})

	st := loadCLIState(t, root)
	approval := st.Attempts[len(st.Attempts)-1].Approval
	if approval == nil || approval.Note != "ok" {
		t.Fatalf("approval = %#v", approval)
	}
}

func TestParseArtifactRecordArgs(t *testing.T) {
	valid := []string{"--path", "out/report.md", "--attempt", "attempt_00000000000000000001", "--step", "review"}
	step, attempt, path, ok := parseArtifactRecordArgs(valid)
	if !ok || step != "review" || attempt != valid[3] || path != "out/report.md" {
		t.Fatalf("parse = %q %q %q %t", step, attempt, path, ok)
	}
	for _, args := range [][]string{
		nil,
		{"--step", "review"},
		{"--step", "review", "--step", "again", "--path", "out"},
		{"--step=review", "--attempt", valid[3], "--path", "out"},
		{"--step", "review", "--attempt", valid[3], "--unknown", "out"},
		{"--step", "\u3000", "--attempt", valid[3], "--path", "out"},
		{"--step", "review", "--attempt", valid[3], "--path", ""},
	} {
		if _, _, _, ok := parseArtifactRecordArgs(args); ok {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestRunArtifactRecordWritesSuccess(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	writeCLITestFile(t, root, "docs/code-review.md")
	st := loadCLIState(t, root)
	stdout, stderr, exitCode := runCapture(root, []string{"artifact", "record", "--step", st.CurrentStepID, "--attempt", st.CurrentAttemptID, "--path", "docs/code-review.md"})
	assertExitCode(t, exitCode, 0, stderr)
	for _, text := range []string{"Recorded artifact: docs/code-review.md", "Attempt: " + st.CurrentAttemptID, "Digest: sha256:", "Size: 3"} {
		assertContains(t, stdout, text)
	}
}

func TestRunCheckRequestWritesOnlyJSON(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, ".devflow", "flows", "check-flow.cue")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowPath, []byte(`flow: { id: "check-flow", title: "Check", steps: [{ id: "quality", title: "Quality", instruction: "Check.", required_checks: ["go-test"] }] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	runSuccess(t, root, []string{"start", "check-flow"})
	statePath := currentStatePath(t, root)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}

	st := loadCLIState(t, root)
	stdout, stderr, exitCode := runCapture(root, []string{"check", "request", "--step", "quality", "--attempt", st.CurrentAttemptID, "--check", "go-test"})

	assertExitCode(t, exitCode, 0, stderr)
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	wantJSON := `{"schema_version":2,"flow_run_id":"` + st.FlowRunID + `","step_id":"quality","attempt_id":"` + st.CurrentAttemptID + `","check_id":"go-test"}` + "\n"
	if stdout != wantJSON {
		t.Fatalf("stdout=%q want=%q", stdout, wantJSON)
	}
	var request map[string]any
	if err := json.Unmarshal([]byte(stdout), &request); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout, err)
	}
	if request["check_id"] != "go-test" || request["attempt_id"] != st.CurrentAttemptID || request["schema_version"] != float64(2) {
		t.Fatalf("request=%#v", request)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("check request modified state.json")
	}
}

func TestRunCheckRecordWritesExactHumanSuccess(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, ".devflow", "flows", "check-flow.cue")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowPath, []byte(`flow: { id: "check-flow", title: "Check", steps: [{ id: "quality", title: "Quality", instruction: "Check.", required_checks: ["go-test"] }] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	runSuccess(t, root, []string{"start", "check-flow"})
	st := loadCLIState(t, root)
	recordPath := filepath.Join(root, "record.json")
	record := `{"schema_version":2,"flow_run_id":"` + st.FlowRunID + `","step_id":"quality","attempt_id":"` + st.CurrentAttemptID + `","check_id":"go-test","result":{"exit_code":0,"log_path":"check.log"}}`
	if err := os.WriteFile(recordPath, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, exitCode := runCapture(root, []string{"check", "record", "--file", recordPath})
	assertExitCode(t, exitCode, 0, stderr)
	want := "Recorded check: go-test\nRun: " + st.FlowRunID + "\nStep: quality\nAttempt: " + st.CurrentAttemptID + "\nExit code: 0\n"
	if stdout != want || stderr != "" {
		t.Fatalf("stdout=%q want=%q stderr=%q", stdout, want, stderr)
	}
	stdout, stderr, exitCode = runCapture(root, []string{"check", "record", "--file", recordPath})
	assertExitCode(t, exitCode, 0, stderr)
	if stdout != want || stderr != "" {
		t.Fatalf("idempotent stdout=%q want=%q stderr=%q", stdout, want, stderr)
	}
}

func TestRunContextWritesOnlyJSON(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, ".devflow", "flows", "context-flow.cue")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowPath, []byte(`flow: { id: "context-flow", title: "Context", steps: [{ id: "design", title: "Design", instruction: "Design.", inputs: [{path: "docs/request.md"}] }] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "request.md"), []byte("request"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSuccess(t, root, []string{"start", "context-flow"})
	stdout, stderr, exitCode := runCapture(root, []string{"context"})
	if exitCode != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatalf("stdout is not JSON: %q: %v", stdout, err)
	}
	attempt, ok := value["attempt"].(map[string]any)
	stateValue := value["state"].(map[string]any)
	_, hasTopLevelEntrySequence := value["entry_sequence"]
	_, hasStateEntrySequence := stateValue["entry_sequence"]
	if value["schema_version"].(float64) != 4 || !ok || len(attempt) != 2 || attempt["id"] == "" || attempt["entry_sequence"].(float64) != 1 || hasTopLevelEntrySequence || hasStateEntrySequence || value["completion"].(map[string]any)["ready"] != true {
		t.Fatalf("context = %#v", value)
	}
}

func TestRunContextRequiresActiveState(t *testing.T) {
	stdout, stderr, exitCode := runCapture(t.TempDir(), []string{"context"})
	if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "error_no_active_flow") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestRunContextReturnsTerminalStateJSON(t *testing.T) {
	for _, tt := range []struct {
		name    string
		advance []string
		want    string
	}{
		{name: "completed", advance: []string{"done"}, want: "completed"},
		{name: "finished", advance: []string{"finish", "--reason", "stop"}, want: "finished"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			flowPath := filepath.Join(root, ".devflow", "flows", "context-flow.cue")
			if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(flowPath, []byte(`flow: { id: "context-flow", title: "Context", steps: [{ id: "design", title: "Design", instruction: "Design." }] }`), 0o644); err != nil {
				t.Fatal(err)
			}
			runSuccess(t, root, []string{"start", "context-flow"})
			runSuccess(t, root, tt.advance)

			stdout, stderr, exitCode := runCapture(root, []string{"context"})
			if exitCode != 0 || stderr != "" {
				t.Fatalf("exit=%d stderr=%q", exitCode, stderr)
			}
			var value map[string]any
			if err := json.Unmarshal([]byte(stdout), &value); err != nil {
				t.Fatalf("stdout is not JSON: %q: %v", stdout, err)
			}
			state := value["state"].(map[string]any)
			attempt, hasAttempt := value["attempt"]
			_, hasTopLevelEntrySequence := value["entry_sequence"]
			_, hasStateEntrySequence := state["entry_sequence"]
			if value["schema_version"].(float64) != 4 || state["status"] != tt.want || !hasAttempt || attempt != nil || hasTopLevelEntrySequence || hasStateEntrySequence || value["step"] != nil || value["completion"] != nil {
				t.Fatalf("context = %#v", value)
			}
		})
	}
}

func TestRunCheckRequestRejectsLegacyStateWithoutOutput(t *testing.T) {
	root := t.TempDir()
	flowPath := filepath.Join(root, ".devflow", "flows", "check-flow.cue")
	if err := os.MkdirAll(filepath.Dir(flowPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flowPath, []byte(`flow: { id: "check-flow", title: "Check", steps: [{ id: "quality", title: "Quality", instruction: "Check.", required_checks: ["go-test"] }] }`), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, ".devflow", "state.json")
	legacy := []byte(`{"flow_id":"check-flow","status":"running","current_step_id":"quality"}`)
	if err := os.WriteFile(statePath, legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, exitCode := runCapture(root, []string{"check", "request", "--step", "quality", "--attempt", "attempt_00000000000000000001", "--check", "go-test"})
	if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "error_unsupported_state_version") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy, after) {
		t.Fatal("legacy state was modified")
	}
}

func TestRunCheckRequestRejectsInvalidArgumentsWithoutChangingState(t *testing.T) {
	for _, args := range [][]string{
		{"check", "request"},
		{"check", "request", "unit-test", "extra"},
		{"check", "request", "--unknown"},
		{"check", "request", "-x"},
		{"check", "request", "--"},
		{"check", "request", "--", "unit-test", "extra"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			runSuccess(t, root, []string{"init"})
			runSuccess(t, root, []string{"start", "post-task-review"})
			statePath := currentStatePath(t, root)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			stdout, stderr, exitCode := runCapture(root, args)
			if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "Usage:") || strings.Contains(stderr, "error_check_not_required") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
			}
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("invalid parser input modified state.json")
			}
		})
	}
}

func TestParseCheckRequestArgs(t *testing.T) {
	for _, tt := range []struct {
		args []string
		ok   bool
	}{
		{[]string{"--step", "quality", "--attempt", "attempt_00000000000000000001", "--check", "unit-test"}, true},
		{[]string{"--check", "unit-test", "--step", "quality", "--attempt", "attempt_00000000000000000001"}, true},
		{[]string{"unit-test"}, false},
		{[]string{"--step=quality", "--attempt", "a", "--check", "c"}, false},
		{[]string{"--step", "quality", "--step", "other", "--check", "c"}, false},
		{[]string{"--step", "　", "--attempt", "a", "--check", "c"}, false},
		{[]string{"--step", " quality", "--attempt", "a", "--check", "c"}, false},
		{[]string{"--step", "quality ", "--attempt", "a", "--check", "c"}, false},
	} {
		_, _, _, ok := parseCheckRequestArgs(tt.args)
		if ok != tt.ok {
			t.Fatalf("parseCheckRequestArgs(%q) ok=%t want=%t", tt.args, ok, tt.ok)
		}
	}
}

func TestRunCheckRecordRejectsInvalidArgumentsWithoutChangingState(t *testing.T) {
	for _, args := range [][]string{
		{"check", "record"},
		{"check", "record", "--file"},
		{"check", "record", "--file", "a.json", "--file", "b.json"},
		{"check", "record", "--unknown", "value"},
		{"check", "record", "--file=result.json"},
		{"check", "record", "--file", "　"},
		{"check", "record", "--file", " result.json"},
		{"check", "record", "--file", "result.json "},
		{"check", "record", "--step", "quality"},
		{"check", "record", "--file", "result.json", "extra"},
		{"check", "record", "extra", "--file", "result.json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := t.TempDir()
			runSuccess(t, root, []string{"init"})
			runSuccess(t, root, []string{"start", "post-task-review"})
			statePath := currentStatePath(t, root)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			stdout, stderr, exitCode := runCapture(root, args)
			if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "Usage:") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
			}
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("invalid parser input modified state.json")
			}
		})
	}
}

func TestRunBackSkipFinishParseReason(t *testing.T) {
	t.Run("back", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"done"})

		runSuccess(t, root, []string{"back", "--reason", "revise"})

		st := loadCLIState(t, root)
		if st.CurrentStepID != "check_changes" {
			t.Fatalf("CurrentStepID = %q, want check_changes", st.CurrentStepID)
		}
		if len(st.BackHistory) != 1 || st.BackHistory[0].Reason != "revise" {
			t.Fatalf("BackHistory = %#v", st.BackHistory)
		}
	})

	t.Run("back with explicit upstream target", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"done"})

		runSuccess(t, root, []string{"back", "--reason", "revise", "--to", "check_changes"})

		st := loadCLIState(t, root)
		if st.CurrentStepID != "check_changes" {
			t.Fatalf("CurrentStepID = %q, want check_changes", st.CurrentStepID)
		}
		if len(st.BackHistory) != 1 {
			t.Fatalf("BackHistory = %#v", st.BackHistory)
		}
		wantInvalidated := []string{"check_changes", "summarize_changes", "check_quality"}
		if !reflect.DeepEqual(st.BackHistory[0].InvalidatedStepIDs, wantInvalidated) {
			t.Fatalf("InvalidatedStepIDs = %#v, want %#v", st.BackHistory[0].InvalidatedStepIDs, wantInvalidated)
		}
	})

	t.Run("skip", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		runSuccess(t, root, []string{"skip", "--reason", "omit"})

		st := loadCLIState(t, root)
		if st.CurrentStepID != "summarize_changes" {
			t.Fatalf("CurrentStepID = %q, want summarize_changes", st.CurrentStepID)
		}
		if st.SkippedSteps["check_changes"].Reason != "omit" {
			t.Fatalf("SkippedSteps = %#v", st.SkippedSteps)
		}
	})

	t.Run("finish", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		runSuccess(t, root, []string{"finish", "--reason", "stop"})

		st := loadCLIState(t, root)
		if st.Status != state.StatusFinished {
			t.Fatalf("Status = %q, want finished", st.Status)
		}
		if st.Finish == nil || st.Finish.Reason != "stop" {
			t.Fatalf("Finish = %#v", st.Finish)
		}
	})
}

func TestRunBackRejectsInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"back", "--to"},
		{"back", "--to", "check_changes"},
		{"back", "--reason", "revise", "--to", "check_changes", "--to", "summarize_changes"},
		{"back", "--reason", "revise", "--unknown", "value"},
	} {
		root := t.TempDir()
		var stdout bytes.Buffer
		var stderr bytes.Buffer

		exitCode := run(args, root, &stdout, &stderr)

		if exitCode == 0 || !strings.Contains(stderr.String(), "Usage:") {
			t.Fatalf("args=%#v exitCode=%d stderr=%q", args, exitCode, stderr.String())
		}
	}
}

func TestRunRejectsMissingRequiredArgs(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"start"}, root, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunStartArgumentContract(t *testing.T) {
	for _, args := range [][]string{
		{"start", "post-task-review"},
		{"start", "post-task-review", "--task-file"},
		{"start", "post-task-review", "--task-file", "one.md", "--task-file", "two.md"},
		{"start", "post-task-review", "--unknown", "task.md"},
		{"start", "post-task-review", "extra", "--task-file", "task.md"},
		{"start", "post-task-review", "--task-file", ""},
		{"start", "post-task-review", "--task-file", "   "},
		{"start", "--task-file", "task.md", "post-task-review"},
		{"start", "post-task-review", "--task-file=task.md"},
		{"start", "--task-file", "task.md"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, exitCode := runCapture(t.TempDir(), args)
			if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "devflow start <flow-id> --task-file <path>") {
				t.Fatalf("args=%#v exit=%d stdout=%q stderr=%q", args, exitCode, stdout, stderr)
			}
		})
	}
}

func TestWritePromptPreservesTaskContentAndSeparatesCurrentStep(t *testing.T) {
	for _, content := range []string{
		"Task without newline",
		"Task with newline\n",
		"Task with blank lines\n\n",
		"# Markdown\n\n- item\n",
	} {
		t.Run(fmt.Sprintf("%q", content), func(t *testing.T) {
			var stdout bytes.Buffer
			writePrompt(&stdout, command.PromptResult{
				FlowID:                 "flow",
				TaskContent:            content,
				CurrentStepID:          "step",
				CurrentStepTitle:       "Step",
				CurrentStepInstruction: "Instruction",
			})
			wantPrefix := "Flow: flow\nTask:\n" + content
			if !strings.HasPrefix(stdout.String(), wantPrefix) {
				t.Fatalf("output changed Task content: %q", stdout.String())
			}
			if !strings.Contains(stdout.String()[len(wantPrefix):], "\nCurrent step: step - Step\n") {
				t.Fatalf("Current step is not separated: %q", stdout.String())
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"unknown"}, root, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestRunApproveRejectsUnknownOption(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"approve", "--unknown"}, root, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestParseApproveArgsAcceptsOnlyCanonicalSyntax(t *testing.T) {
	valid := []string{"--step", "step", "--attempt", "attempt_00000000000000000001", "--note", " note "}
	stepID, attemptID, note, ok := parseApproveArgs(valid)
	if !ok || stepID != "step" || attemptID != valid[3] || note != " note " {
		t.Fatalf("parse = %q %q %q %t", stepID, attemptID, note, ok)
	}
	invalid := [][]string{
		nil,
		{"--step", "step"},
		append(valid, "extra"),
		{"--attempt", valid[3], "--step", "step", "--note", "note"},
		{"--step", "step", "--step", "step", "--note", "note"},
		{"--step", "step", "--attempt", valid[3], "--unknown", "note"},
		{"--step", "step", "--attempt", valid[3], "--note", "note", "--evidence-set-digest", "sha256:" + strings.Repeat("a", 64)},
		{"--step", "step", "--attempt", valid[3], "--note", "note", "--artifact", "out/report.md"},
		{"--step", "step", "--attempt", valid[3], "--note", "note", "--path", "out/report.md"},
		{"--step", "step", "--attempt", valid[3], "--note", "note", "--evidence", "value"},
		{"--step=step", "--attempt", valid[3], "--note", "note"},
		{"--step", "step", "--attempt=" + valid[3], "--note", "note"},
		{"--step", "step", "--attempt", valid[3], "--note=note"},
		{"--step", "", "--attempt", valid[3], "--note", "note"},
		{"--step", " ", "--attempt", valid[3], "--note", "note"},
		{"--step", "\u3000", "--attempt", valid[3], "--note", "note"},
		{"--step", "step", "--attempt", "", "--note", "note"},
		{"--step", "step", "--attempt", " ", "--note", "note"},
		{"--step", "step", "--attempt", "\u3000", "--note", "note"},
		{"--step", "step", "--attempt", valid[3], "--note", ""},
		{"--step", "step", "--attempt", valid[3], "--note", " \t"},
		{"--step", "step", "--attempt", valid[3], "--note", "\u3000"},
		{}, {"--note", "note"}, {"--step", "step"},
	}
	for _, args := range invalid {
		if _, _, _, ok := parseApproveArgs(args); ok {
			t.Fatalf("accepted %#v", args)
		}
	}
}

func TestRunReasonCommandsRejectMissingReasonValue(t *testing.T) {
	for _, commandName := range []string{"back", "skip", "finish"} {
		t.Run(commandName, func(t *testing.T) {
			root := t.TempDir()
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := run([]string{commandName, "--reason"}, root, &stdout, &stderr)

			if exitCode == 0 {
				t.Fatalf("exitCode = 0, want non-zero")
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestRunWritesDiagnosticsToStderr(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"status"}, root, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("exitCode = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), command.CodeNoActiveFlow) {
		t.Fatalf("stderr = %q, want diagnostic", stderr.String())
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunWritesWarningDiagnosticsToStderr(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	runSuccess(t, root, []string{"done"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"skip", "--reason", "omit artifact step"}, root, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), transition.CodeSkippedRequiredArtifact) {
		t.Fatalf("stderr = %q, want warning diagnostic", stderr.String())
	}
}

func TestRunWritesSuccessMessages(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		writeCLITestFile(t, root, "tasks/task.md")

		stdout, stderr, exitCode := runCapture(root, []string{"start", "post-task-review", "--task-file", "tasks/task.md"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Started flow: post-task-review")
		assertContains(t, stdout, "Current step: check_changes")
	})

	t.Run("done next step", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		stdout, stderr, exitCode := runCapture(root, []string{"done"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Completed step: check_changes")
		assertContains(t, stdout, "Next step: summarize_changes")
	})

	t.Run("done flow completed", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"done"})
		writeCLITestFile(t, root, "docs/code-review.md")
		recordCLIArtifact(t, root, "docs/code-review.md")
		runSuccess(t, root, []string{"done"})
		st := loadCLIState(t, root)
		runSuccess(t, root, []string{"approve", "--step", "human_approval", "--attempt", st.CurrentAttemptID, "--note", "ok"})

		stdout, stderr, exitCode := runCapture(root, []string{"done"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Completed step: human_approval")
		assertContains(t, stdout, "Flow completed: post-task-review")
	})

	t.Run("approve", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"done"})
		writeCLITestFile(t, root, "docs/code-review.md")
		recordCLIArtifact(t, root, "docs/code-review.md")
		runSuccess(t, root, []string{"done"})
		st := loadCLIState(t, root)

		stdout, stderr, exitCode := runCapture(root, []string{"approve", "--step", "human_approval", "--attempt", st.CurrentAttemptID, "--note", "ok"})

		assertExitCode(t, exitCode, 0, stderr)
		want := "Approved step: human_approval\n" +
			"Approved attempt: " + st.CurrentAttemptID + "\n" +
			"Evidence set: sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f\n"
		if stdout != want {
			t.Fatalf("stdout = %q, want %q", stdout, want)
		}
	})

	t.Run("back", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"done"})

		stdout, stderr, exitCode := runCapture(root, []string{"back", "--reason", "revise"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Moved back to: check_changes")
	})

	t.Run("skip next step", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		stdout, stderr, exitCode := runCapture(root, []string{"skip", "--reason", "omit"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Skipped step: check_changes")
		assertContains(t, stdout, "Next step: summarize_changes")
	})

	t.Run("skip flow completed", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})
		runSuccess(t, root, []string{"skip", "--reason", "omit"})
		runSuccess(t, root, []string{"skip", "--reason", "omit"})
		runSuccess(t, root, []string{"skip", "--reason", "omit"})
		runSuccess(t, root, []string{"skip", "--reason", "omit"})

		stdout, _, exitCode := runCapture(root, []string{"skip", "--reason", "omit"})

		assertExitCode(t, exitCode, 0, "")
		assertContains(t, stdout, "Skipped step: human_approval")
		assertContains(t, stdout, "Flow completed: post-task-review")
	})

	t.Run("finish", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		stdout, stderr, exitCode := runCapture(root, []string{"finish", "--reason", "stop"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Finished flow: post-task-review")
	})
}

func TestRunWritesNormalResultToStdout(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"prompt"}, root, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Instruction:") {
		t.Fatalf("stdout = %q, want prompt output", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Task:\nok\n\nCurrent step:") {
		t.Fatalf("stdout = %q, want separated Task and Current step sections", stdout.String())
	}
}

func runCapture(root string, args []string) (string, string, int) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(args, root, &stdout, &stderr)
	return stdout.String(), stderr.String(), exitCode
}

func currentStatePath(t testing.TB, root string) string {
	t.Helper()
	store := command.NewStore(command.Context{ProjectRoot: root})
	loaded := store.LoadCurrent()
	if loaded.Status != state.LoadOK || loaded.State == nil {
		t.Fatalf("LoadCurrent() = %#v", loaded)
	}
	path, err := store.RunStatePath(loaded.State.FlowRunID)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func assertExitCode(t *testing.T, got int, want int, stderr string) {
	t.Helper()

	if got != want {
		t.Fatalf("exitCode = %d, want %d; stderr = %q", got, want, stderr)
	}
}

func assertContains(t *testing.T, got string, want string) {
	t.Helper()

	if !strings.Contains(got, want) {
		t.Fatalf("got = %q, want to contain %q", got, want)
	}
}

func runSuccess(t *testing.T, root string, args []string) {
	t.Helper()
	if len(args) == 2 && args[0] == "start" {
		writeCLITestFile(t, root, "tasks/task.md")
		args = append(args, "--task-file", "tasks/task.md")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(args, root, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(%v) exitCode = %d, want 0; stdout = %q stderr = %q", args, exitCode, stdout.String(), stderr.String())
	}
}

func loadCLIState(t *testing.T, root string) state.State {
	t.Helper()

	loaded := command.NewStore(command.Context{ProjectRoot: root}).LoadCurrent()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	return *loaded.State
}

func writeCLITestFile(t *testing.T, root string, path string) {
	t.Helper()

	fullPath := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
