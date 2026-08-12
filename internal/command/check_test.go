package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/transition"
)

func TestCheckRequestV2ExactAndStateUnchanged(t *testing.T) {
	root, st := startCheckFlow(t)
	path, _ := NewStore(Context{ProjectRoot: root}).RunStatePath(st.FlowRunID)
	before := readCommandFile(t, path)
	got := CheckRequest(Context{ProjectRoot: root}, "quality", st.CurrentAttemptID, "go-test")
	if got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	data, err := json.Marshal(got.CheckRequest)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":2,"flow_run_id":"` + st.FlowRunID + `","step_id":"quality","attempt_id":"` + st.CurrentAttemptID + `","check_id":"go-test"}`
	if string(data) != want || strings.Contains(string(data), "flow_id") || strings.Contains(string(data), "entry_sequence") {
		t.Fatalf("request=%s want=%s", data, want)
	}
	if after := readCommandFile(t, path); string(after) != string(before) {
		t.Fatal("request changed State")
	}
}

func TestCheckRequestAttemptClassificationAndRecorded(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	cases := []struct {
		step, attempt, check, code string
	}{
		{"quality", "bad", "go-test", transition.CodeInvalidAttemptID},
		{"quality", "attempt_00000000000000000099", "go-test", transition.CodeInvalidAttemptID},
		{"review", st.CurrentAttemptID, "go-test", transition.CodeStepAttemptMismatch},
		{"quality", st.CurrentAttemptID, "unknown", CodeCheckNotRequired},
	}
	for _, tt := range cases {
		got := CheckRequest(ctx, tt.step, tt.attempt, tt.check)
		if got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, tt.code) {
			t.Fatalf("%+v => %#v", tt, got)
		}
	}
	record := validRecord(st, "go-test", state.CheckResult{ExitCode: 0})
	path := filepath.Join(root, "record.json")
	writeCheckRecord(t, path, record)
	if got := CheckRecord(ctx, path); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	if got := CheckRequest(ctx, "quality", st.CurrentAttemptID, "go-test"); got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, CodeCheckResultAlreadyRecorded) {
		t.Fatalf("recorded request=%#v", got)
	}
}

func TestCheckRecordV2StrictDecode(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	valid := validRecord(st, "go-test", state.CheckResult{ExitCode: 0, LogPath: "ok.log"})
	cases := []string{
		"",
		`null`,
		`[]`,
		`true`,
		valid + `{}`,
		strings.Replace(valid, `"schema_version":2,`, "", 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":null`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":"2"`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":true`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":{}`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":[]`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":2.0`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":-1`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":0`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":1`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":3`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":999999999999999999999999999999999999`, 1),
		strings.Replace(valid, `"schema_version":2`, `"schema_version":2,"schema_version":2`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"a","\u0063heck_id":"b"`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":0,"exit_code":1`, 1),
		strings.Replace(valid, `"result":`, `"unknown":true,"result":`, 1),
		strings.Replace(valid, `"log_path":"ok.log"`, `"log_path":"ok.log","unknown":true`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":{}`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":null`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":"0"`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":0.5`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":999999999999999999999999999999999999`, 1),
		strings.Replace(valid, `"log_path":"ok.log"`, `"log_path":null`, 1),
		strings.Replace(valid, `"log_path":"ok.log"`, `"log_path":7`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":null`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":[]`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":"bad"`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":7`, 1),
		strings.Replace(valid, `"result":{"exit_code":0,"log_path":"ok.log"}`, `"result":true`, 1),
		strings.Replace(valid, `"attempt_id":"`+st.CurrentAttemptID+`"`, `"attempt_id":" bad "`, 1),
		strings.Replace(valid, `"exit_code":0`, `"exit_code":-1`, 1),
	}
	path := filepath.Join(root, "record.json")
	for i, data := range cases {
		writeCheckRecord(t, path, data)
		if got := CheckRecord(ctx, path); got.ExitCode == 0 {
			t.Fatalf("case %d accepted: %s", i, data)
		}
	}
}

func TestCheckRecordRejectsInvalidIdentifierRepresentations(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	valid := validRecord(st, "go-test", state.CheckResult{})
	fields := []struct {
		name, value string
	}{
		{"flow_run_id", st.FlowRunID},
		{"step_id", st.CurrentStepID},
		{"attempt_id", st.CurrentAttemptID},
		{"check_id", "go-test"},
	}
	path := filepath.Join(root, "record.json")
	for _, field := range fields {
		needle := `"` + field.name + `":"` + field.value + `"`
		for _, replacement := range []string{
			`"` + field.name + `":null`,
			`"` + field.name + `":7`,
			`"` + field.name + `":true`,
			`"` + field.name + `:{}`,
			`"` + field.name + `":[]`,
			`"` + field.name + `:""`,
			`"` + field.name + `:" "`,
			`"` + field.name + `:"　"`,
			`"` + field.name + `:" bad"`,
			`"` + field.name + `:"bad "`,
			`"` + field.name + `:"bad/value"`,
		} {
			data := strings.Replace(valid, needle, replacement, 1)
			writeCheckRecord(t, path, data)
			if got := CheckRecord(ctx, path); got.ExitCode == 0 {
				t.Fatalf("%s accepted replacement %s", field.name, replacement)
			}
		}
		data := strings.Replace(valid, needle+",", "", 1)
		writeCheckRecord(t, path, data)
		if got := CheckRecord(ctx, path); got.ExitCode == 0 {
			t.Fatalf("%s accepted missing field", field.name)
		}
	}
}

func TestDuplicateJSONScannerKeepsObjectScopesSeparate(t *testing.T) {
	for _, data := range []string{
		`{"check_id":"a","result":{"check_id":"b"}}`,
		`[{"check_id":"a"},{"check_id":"b"}]`,
	} {
		if err := rejectDuplicateJSONKeys([]byte(data)); err != nil {
			t.Fatalf("scanner rejected separate scopes %s: %v", data, err)
		}
	}
	if err := rejectDuplicateJSONKeys([]byte(`{"check_id":"a","\u0063heck_id":"b"}`)); err != errDuplicateJSONKey {
		t.Fatalf("escaped duplicate error=%v", err)
	}
}

func TestCheckRecordIdempotentConflictAndStale(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	path := filepath.Join(root, "record.json")
	record := validRecord(st, "go-test", state.CheckResult{ExitCode: 1, LogPath: "failed.log"})
	writeCheckRecord(t, path, record)
	first := CheckRecord(ctx, path)
	if first.ExitCode != 0 || first.Success == nil || first.Success.RecordedCheckExitCode == nil || *first.Success.RecordedCheckExitCode != 1 {
		t.Fatalf("first=%#v", first)
	}
	statePath, _ := NewStore(ctx).RunStatePath(st.FlowRunID)
	before := readCommandFile(t, statePath)
	second := CheckRecord(ctx, path)
	if second.ExitCode != 0 || second.Success == nil {
		t.Fatalf("idempotent=%#v", second)
	}
	if after := readCommandFile(t, statePath); string(after) != string(before) {
		t.Fatal("idempotent record saved State")
	}
	writeCheckRecord(t, path, validRecord(st, "go-test", state.CheckResult{ExitCode: 0}))
	if conflict := CheckRecord(ctx, path); conflict.ExitCode == 0 || !hasDiagnostic(conflict.Diagnostics, transition.CodeConflictingCheckResult) {
		t.Fatalf("conflict=%#v", conflict)
	}
	if after := readCommandFile(t, statePath); string(after) != string(before) {
		t.Fatal("conflicting record changed State")
	}

	current := loadCommandState(t, root)
	current.Attempts[0], _ = state.CloseStepAttempt(current.Attempts[0], state.StepAttemptExitBack, "retry")
	next, _ := state.NewStepAttempt("quality", 2)
	current.Attempts = append(current.Attempts, next)
	current.CurrentAttemptID = next.ID
	if err := NewStore(ctx).SaveCurrent(current); err != nil {
		t.Fatal(err)
	}
	writeCheckRecord(t, path, record)
	if stale := CheckRecord(ctx, path); stale.ExitCode == 0 || !hasDiagnostic(stale.Diagnostics, transition.CodeStaleAttempt) {
		t.Fatalf("stale=%#v", stale)
	}
}

func TestCheckRecordRejectsRunMismatchWithoutChangingState(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	statePath, _ := NewStore(ctx).RunStatePath(st.FlowRunID)
	before := readCommandFile(t, statePath)
	record := validRecord(st, "go-test", state.CheckResult{})
	record = strings.Replace(record, st.FlowRunID, "run_00000000000000000000000000000000", 1)
	path := filepath.Join(root, "record.json")
	writeCheckRecord(t, path, record)
	got := CheckRecord(ctx, path)
	if got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, CodeCheckRunMismatch) {
		t.Fatalf("run mismatch=%#v", got)
	}
	if after := readCommandFile(t, statePath); string(after) != string(before) {
		t.Fatal("Run mismatch changed State")
	}
}

func TestCheckRequestAndRecordRejectFirstAttemptAfterABAReentry(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	firstID := st.CurrentAttemptID
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
	statePath, _ := NewStore(ctx).RunStatePath(st.FlowRunID)
	before := readCommandFile(t, statePath)
	if got := CheckRequest(ctx, "quality", firstID, "go-test"); got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, transition.CodeStaleAttempt) {
		t.Fatalf("stale request=%#v", got)
	}
	if after := readCommandFile(t, statePath); string(after) != string(before) {
		t.Fatal("stale request changed State")
	}
	recordState := st
	recordState.CurrentAttemptID = firstID
	path := filepath.Join(root, "record.json")
	writeCheckRecord(t, path, validRecord(recordState, "go-test", state.CheckResult{}))
	if got := CheckRecord(ctx, path); got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, transition.CodeStaleAttempt) {
		t.Fatalf("stale record=%#v", got)
	}
	after := loadCommandState(t, root)
	if len(after.Attempts[2].CheckResults) != 0 {
		t.Fatalf("stale result reached Attempt 3: %#v", after.Attempts[2].CheckResults)
	}
}

func TestCheckRecordSaveFailureHasNoSuccess(t *testing.T) {
	root, st := startCheckFlow(t)
	ctx := Context{ProjectRoot: root}
	statePath, _ := NewStore(ctx).RunStatePath(st.FlowRunID)
	runDir := filepath.Dir(statePath)
	if err := os.Chmod(runDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(runDir, 0o700) })
	path := filepath.Join(root, "record.json")
	writeCheckRecord(t, path, validRecord(st, "go-test", state.CheckResult{}))
	got := CheckRecord(ctx, path)
	if got.ExitCode == 0 || got.Success != nil || !hasDiagnostic(got.Diagnostics, CodeStateSaveFailed) {
		t.Fatalf("save failure=%#v", got)
	}
}

func TestCheckRecordAfterApprovalPreservesApprovalAndEvidence(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "approved-check-flow", `flow: {
		id: "approved-check-flow"
		title: "Approved Check Flow"
		steps: [{
			id: "quality"
			title: "Quality"
			objective: "Check."
			approval: {required: true}
			required_checks: ["go-test"]
		}]
	}`)
	if got := startWithTestTask(t, root, "approved-check-flow"); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	ctx := Context{ProjectRoot: root}
	st := loadCommandState(t, root)
	digest, err := state.ArtifactEvidenceSetDigest(nil, st.Attempts[0].ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	approved, err := state.ApproveStepAttempt(st.Attempts[0], "approved", digest)
	if err != nil {
		t.Fatal(err)
	}
	st.Attempts[0] = approved
	if err := NewStore(ctx).SaveCurrent(st); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "record.json")
	writeCheckRecord(t, path, validRecord(st, "go-test", state.CheckResult{ExitCode: 0}))
	if got := CheckRecord(ctx, path); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	after := loadCommandState(t, root)
	if !reflect.DeepEqual(after.Attempts[0].Approval, approved.Approval) ||
		!reflect.DeepEqual(after.Attempts[0].ArtifactEvidence, approved.ArtifactEvidence) {
		t.Fatalf("record changed approval/evidence: %#v", after.Attempts[0])
	}
}

func TestPromptCompoundCheckArtifactApprovalState(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "compound-check-flow", `flow: {
		id: "compound-check-flow"
		title: "Compound Check Flow"
		steps: [{
			id: "quality"
			title: "Quality"
			objective: "Check."
			artifacts: [{path: "out/report.txt", required: true}]
			approval: {required: true}
			required_checks: ["check-a", "check-b", "check-c"]
		}]
	}`)
	if got := startWithTestTask(t, root, "compound-check-flow"); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	if err := os.MkdirAll(filepath.Join(root, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "out", "report.txt"), []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := Context{ProjectRoot: root}
	st := loadCommandState(t, root)
	if got := RecordArtifact(ctx, "quality", st.CurrentAttemptID, "out/report.txt"); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	st = loadCommandState(t, root)
	st.Attempts[0].CheckResults["check-b"] = state.CheckResult{ExitCode: 0}
	st.Attempts[0].CheckResults["check-c"] = state.CheckResult{ExitCode: 2}
	if err := NewStore(ctx).SaveCurrent(st); err != nil {
		t.Fatal(err)
	}

	got := Prompt(ctx)
	if got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	if len(got.Prompt.CheckBlockers) != 1 || !strings.HasPrefix(got.Prompt.CheckBlockers[0], "check-c:") {
		t.Fatalf("compound blockers=%#v", got.Prompt.CheckBlockers)
	}
}

func startCheckFlow(t *testing.T) (string, state.State) {
	t.Helper()
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got.Diagnostics)
	}
	return root, loadCommandState(t, root)
}

func checkTestFlow() string {
	return `flow: { id: "check-flow", title: "Check Flow", steps: [{ id: "quality", title: "Quality", objective: "Check.", required_checks: ["go-test", "go-vet"] }, { id: "review", title: "Review", objective: "Review." }] }`
}

func validRecord(st state.State, checkID string, result state.CheckResult) string {
	data, _ := json.Marshal(struct {
		SchemaVersion int               `json:"schema_version"`
		FlowRunID     string            `json:"flow_run_id"`
		StepID        string            `json:"step_id"`
		AttemptID     string            `json:"attempt_id"`
		CheckID       string            `json:"check_id"`
		Result        state.CheckResult `json:"result"`
	}{2, st.FlowRunID, st.CurrentStepID, st.CurrentAttemptID, checkID, result})
	return string(data)
}

func hasDiagnostic(diagnostics []transition.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func writeCheckRecord(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
