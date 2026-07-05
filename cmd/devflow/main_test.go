package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestRunWritesSuccessMessages(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})

		stdout, stderr, exitCode := runCapture(root, []string{"start", "post-task-review"})

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
		runSuccess(t, root, []string{"done"})
		runSuccess(t, root, []string{"approve", "--note", "ok"})

		stdout, stderr, exitCode := runCapture(root, []string{"done"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Completed step: human_approval")
		assertContains(t, stdout, "Flow completed: post-task-review")
	})

	t.Run("approve", func(t *testing.T) {
		root := t.TempDir()
		runSuccess(t, root, []string{"init"})
		runSuccess(t, root, []string{"start", "post-task-review"})

		stdout, stderr, exitCode := runCapture(root, []string{"approve", "--step", "human_approval"})

		assertExitCode(t, exitCode, 0, stderr)
		assertContains(t, stdout, "Approved step: human_approval")
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
}

func runCapture(root string, args []string) (string, string, int) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(args, root, &stdout, &stderr)
	return stdout.String(), stderr.String(), exitCode
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
