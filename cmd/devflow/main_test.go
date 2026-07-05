package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/8noki8/devflow/internal/command"
	"github.com/8noki8/devflow/internal/state"
	"github.com/8noki8/devflow/internal/transition"
)

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
	if st.FlowID != "post-task-review" {
		t.Fatalf("FlowID = %q, want post-task-review", st.FlowID)
	}
}

func TestRunApproveParsesStepAndNote(t *testing.T) {
	root := t.TempDir()
	runSuccess(t, root, []string{"init"})
	runSuccess(t, root, []string{"start", "post-task-review"})

	runSuccess(t, root, []string{"approve", "--step", "human_approval", "--note", "ok"})

	st := loadCLIState(t, root)
	approval := st.Approvals["human_approval"]
	if !approval.Approved || approval.Note != "ok" {
		t.Fatalf("approval = %#v", approval)
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
}

func runSuccess(t *testing.T, root string, args []string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(args, root, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("run(%v) exitCode = %d, want 0; stdout = %q stderr = %q", args, exitCode, stdout.String(), stderr.String())
	}
}

func loadCLIState(t *testing.T, root string) state.State {
	t.Helper()

	loaded := command.NewStore(command.Context{ProjectRoot: root}).Load()
	if loaded.Status != state.LoadOK {
		t.Fatalf("Load status = %q, err = %v", loaded.Status, loaded.Err)
	}
	return *loaded.State
}
