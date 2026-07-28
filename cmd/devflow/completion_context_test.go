package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunCompletionContextWritesOnlyJSON(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})
	st := loadCLIState(t, root)
	stdout, stderr, exitCode := runCapture(root, []string{"completion-context", "--attempt", st.CurrentAttemptID, "--step", st.CurrentStepID})
	if exitCode != 0 || stderr != "" || !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(stdout), &value); err != nil {
		t.Fatal(err)
	}
	if value["schema_version"] != float64(1) || value["step_id"] != st.CurrentStepID || value["attempt_id"] != st.CurrentAttemptID {
		t.Fatalf("completion context = %#v", value)
	}
}

func TestRunCompletionContextRequiresExactFlags(t *testing.T) {
	stdout, stderr, exitCode := runCapture(t.TempDir(), []string{"completion-context", "--step", "build"})
	if exitCode == 0 || stdout != "" || !strings.Contains(stderr, "Usage:") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}
