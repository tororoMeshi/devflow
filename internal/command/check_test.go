package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

func TestCheckRequestRejectsLegacyStateWithoutChangingIt(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	legacy := `{"flow_id":"check-flow","status":"running","current_step_id":"quality"}`
	writeCheckRecord(t, LegacyStatePath(root), legacy)
	before := readCommandFile(t, LegacyStatePath(root))

	got := CheckRequest(Context{ProjectRoot: root}, "go-test")

	if got.ExitCode == 0 || got.CheckRequest != nil || hasDiagnostic(got.Diagnostics, CodeUnsupportedStateVersion) == false {
		t.Fatalf("result=%#v", got)
	}
	if after := readCommandFile(t, LegacyStatePath(root)); string(after) != string(before) {
		t.Fatal("legacy state was modified")
	}
}

func TestStartCreatesDifferentFlowRunIDs(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	first := loadCommandState(t, root).FlowRunID
	if got := Finish(Context{ProjectRoot: root}, "restart"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	if !state.IsValidFlowRunID(first) {
		t.Fatalf("invalid first run ID: %q", first)
	}
	if second := loadCommandState(t, root).FlowRunID; first == second || !state.IsValidFlowRunID(second) {
		t.Fatalf("run IDs: %q %q", first, second)
	}
}

func TestCheckRecordStrictValidationAndGate(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatalf("start=%#v", got)
	}
	taskSnapshot := loadCommandState(t, root).TaskSnapshot
	request := CheckRequest(Context{ProjectRoot: root}, "go-test").CheckRequest
	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"task_snapshot", "task_digest", "task_content"} {
		if strings.Contains(string(requestJSON), forbidden) {
			t.Fatalf("CheckRequest JSON contains %q: %s", forbidden, requestJSON)
		}
	}
	path := filepath.Join(root, "result.json")
	writeCheckRecord(t, path, checkRecordJSON(request, 1, `".devflow/logs/test.log"`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatalf("record=%#v", got)
	}
	recorded := loadCommandState(t, root)
	if got := recorded.TaskSnapshot; !reflect.DeepEqual(got, taskSnapshot) {
		t.Fatalf("CheckRecord changed TaskSnapshot: %#v", got)
	}
	attempt, _, ok := recorded.CurrentAttempt()
	if !ok || attempt.CheckResults["go-test"].ExitCode != 1 || len(recorded.Attempts) != 1 {
		t.Fatalf("check was not recorded on current Attempt: %#v", recorded.Attempts)
	}
	data, err := json.Marshal(recorded)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["check_results"]; exists {
		t.Fatalf("top-level check_results exists: %s", data)
	}
	if got := Done(Context{ProjectRoot: root}); got.ExitCode == 0 {
		t.Fatal("done succeeded with missing and failed checks")
	} else {
		assertDiagnosticCodes(t, got.Diagnostics, []string{transition.CodeFailedRequiredCheck, transition.CodeMissingRequiredCheck})
	}

	writeCheckRecord(t, path, checkRecordJSON(request, 0, `"ok.log"`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatalf("replacement=%#v", got)
	}
	requestVet := CheckRequest(Context{ProjectRoot: root}, "go-vet").CheckRequest
	writeCheckRecord(t, path, checkRecordJSON(requestVet, 0, `null`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatalf("vet=%#v", got)
	}
	if got := Done(Context{ProjectRoot: root}); got.ExitCode != 0 {
		t.Fatalf("done=%#v", got)
	}
}

func TestCheckRecordRejectsUnknownTrailingAndStaleContext(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	request := CheckRequest(Context{ProjectRoot: root}, "go-test").CheckRequest
	path := filepath.Join(root, "result.json")
	for _, data := range []string{
		checkRecordJSON(request, 0, `null`) + ` {}`,
		strings.TrimSuffix(checkRecordJSON(request, 0, `null`), "}") + `,"unknown":true}`,
		strings.Replace(checkRecordJSON(request, 0, `null`), request.FlowRunID, "run_00000000000000000000000000000000", 1),
		strings.Replace(checkRecordJSON(request, 0, `null`), request.FlowRunID, "run_BAD", 1),
		strings.Replace(checkRecordJSON(request, 0, `"bad\npath"`), `"bad\npath"`, `"bad\npath"`, 1),
	} {
		writeCheckRecord(t, path, data)
		if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode == 0 {
			t.Fatalf("accepted %s", data)
		}
	}
}

func TestCheckStatusAndPrompt(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	if got := CheckRequest(Context{ProjectRoot: root}, "go-test"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	status := Status(Context{ProjectRoot: root})
	if status.ExitCode != 0 || status.Status.EntrySequence != 1 || len(status.Status.Checks) != 2 || status.Status.Checks[0].Status != "pending" {
		t.Fatalf("status=%#v", status)
	}
	prompt := Prompt(Context{ProjectRoot: root})
	if prompt.ExitCode != 0 || len(prompt.Prompt.RequiredChecks) != 2 || prompt.Prompt.RequiredChecks[0] != "go-test" {
		t.Fatalf("prompt=%#v", prompt)
	}
}

func TestCheckRecordRejectsRequestFromPreviousEntryAfterBack(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "check-flow", checkTestFlow())
	if got := startWithTestTask(t, root, "check-flow"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	request := CheckRequest(Context{ProjectRoot: root}, "go-test").CheckRequest
	path := filepath.Join(root, "result.json")
	writeCheckRecord(t, path, checkRecordJSON(request, 0, `null`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatal(got)
	}
	vet := CheckRequest(Context{ProjectRoot: root}, "go-vet").CheckRequest
	writeCheckRecord(t, path, checkRecordJSON(vet, 0, `null`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode != 0 {
		t.Fatal(got)
	}
	if got := Done(Context{ProjectRoot: root}); got.ExitCode != 0 {
		t.Fatal(got)
	}
	if got := Back(Context{ProjectRoot: root}, "", "recheck"); got.ExitCode != 0 {
		t.Fatal(got)
	}
	afterBack := loadCommandState(t, root)
	if len(afterBack.Attempts) != 3 || len(afterBack.Attempts[0].CheckResults) != 2 || len(afterBack.Attempts[2].CheckResults) != 0 {
		t.Fatalf("Attempt check history was not preserved: %#v", afterBack.Attempts)
	}
	writeCheckRecord(t, path, checkRecordJSON(request, 0, `null`))
	if got := CheckRecord(Context{ProjectRoot: root}, path); got.ExitCode == 0 || !hasDiagnostic(got.Diagnostics, CodeCheckContextMismatch) {
		t.Fatalf("result=%#v", got)
	}
}

func hasDiagnostic(diagnostics []transition.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func checkTestFlow() string {
	return `flow: { id: "check-flow", title: "Check Flow", steps: [{ id: "quality", title: "Quality", instruction: "Check.", required_checks: ["go-test", "go-vet"] }, { id: "review", title: "Review", instruction: "Review." }] }`
}

func checkRecordJSON(request *CheckRequestResult, exitCode int, logPath string) string {
	return `{"schema_version":1,"flow_run_id":"` + request.FlowRunID + `","flow_id":"` + request.FlowID + `","step_id":"` + request.StepID + `","entry_sequence":` + fmtUint(request.EntrySequence) + `,"check_id":"` + request.CheckID + `","exit_code":` + fmtInt(exitCode) + `,"log_path":` + logPath + `}`
}

func fmtUint(value uint64) string { return strconv.FormatUint(value, 10) }
func fmtInt(value int) string     { return strconv.Itoa(value) }
func writeCheckRecord(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
