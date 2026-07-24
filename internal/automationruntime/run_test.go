package automationruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("DEVFLOW_RUNTIME_HELPER") == "1" {
		os.Exit(runHelper())
	}
	os.Exit(m.Run())
}

func runHelper() int {
	args := os.Args[1:]
	if want := os.Getenv("HELPER_CWD"); want != "" {
		got, _ := os.Getwd()
		if got != want {
			return 90
		}
	}
	if len(args) > 0 && args[0] == "work-package" {
		if len(args) != 5 || args[1] != "--step" || args[3] != "--attempt" {
			return 91
		}
		switch os.Getenv("HELPER_WP") {
		case "nonzero":
			fmt.Fprintln(os.Stderr, "error: error_stale_attempt")
			return 1
		case "empty":
			return 0
		case "malformed":
			fmt.Print("{")
			return 0
		case "oversized":
			_, _ = os.Stdout.Write(make([]byte, MaxWorkPackageBytes+1))
			return 0
		case "max":
			prefix := fmt.Sprintf(`{"schema_version":1,"work_package_digest":"%s","flow_run_id":"run_1","step_id":%q,"attempt_id":%q,"padding":"`, testDigest, args[2], args[4])
			suffix := `"}`
			_, _ = os.Stdout.Write([]byte(prefix + strings.Repeat("x", MaxWorkPackageBytes-len(prefix)-len(suffix)) + suffix))
			return 0
		}
		step, attempt := args[2], args[4]
		if os.Getenv("HELPER_WP") == "step-mismatch" {
			step = "other"
		}
		if artifacts := os.Getenv("HELPER_ARTIFACTS"); artifacts != "" {
			fmt.Printf(`{"schema_version":1,"work_package_digest":"%s","flow_run_id":"run_1","step_id":%q,"attempt_id":%q,"step":{"artifacts":%s}}`+"\n", testDigest, step, attempt, artifacts)
		} else {
			fmt.Printf(`{"schema_version":1,"work_package_digest":"%s","flow_run_id":"run_1","step_id":%q,"attempt_id":%q}`+"\n", testDigest, step, attempt)
		}
		return 0
	}
	if len(args) > 2 && args[0] == "execution-report" {
		if len(args) != 4 || args[1] != "record" || args[2] != "--file" {
			return 92
		}
		if os.Getenv("HELPER_RECORD_MARK") != "" {
			_ = os.WriteFile(os.Getenv("HELPER_RECORD_MARK"), []byte("called"), 0o600)
		}
		data, err := os.ReadFile(args[3])
		if err != nil {
			return 2
		}
		info, _ := os.Stat(args[3])
		if info == nil || info.Mode().Perm() != 0o600 || len(data) == 0 {
			return 3
		}
		if os.Getenv("HELPER_RECORD") == "nonzero" {
			fmt.Fprintln(os.Stderr, "error: error_conflicting_execution_report")
			return 1
		}
		if os.Getenv("HELPER_RECORD") == "oversized" {
			_, _ = os.Stdout.Write(make([]byte, MaxExecutorStdoutBytes+1))
			time.Sleep(time.Second)
			return 0
		}
		var report reportHeader
		if json.Unmarshal(data, &report) != nil {
			fmt.Fprintln(os.Stderr, "error: error_invalid_execution_report")
			return 1
		}
		if os.Getenv("HELPER_RECORD") == "malformed" {
			fmt.Println("bad")
			return 0
		}
		attempt := report.AttemptID
		if os.Getenv("HELPER_EXEC") == "mismatch" {
			attempt = "attempt_1"
		}
		fmt.Printf("Recorded execution report\nRun: %s\nStep: %s\nAttempt: %s\nWork package: %s\nExecution report: %s\nOutcome: %s\nIdempotent: %s\n",
			report.FlowRunID, report.StepID, attempt, report.WorkPackageDigest, reportDigest, report.Outcome, envDefault("HELPER_IDEMPOTENT", "false"))
		return 0
	}
	if len(args) > 1 && args[0] == "artifact" && args[1] == "record" {
		if len(args) != 8 || args[2] != "--step" || args[4] != "--attempt" || args[6] != "--path" {
			return 95
		}
		if mark := os.Getenv("HELPER_ARTIFACT_MARK"); mark != "" {
			f, _ := os.OpenFile(mark, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if f != nil {
				_, _ = fmt.Fprintln(f, args[7])
				_ = f.Close()
			}
		}
		if failPath := os.Getenv("HELPER_ARTIFACT_FAIL"); failPath == args[7] {
			fmt.Fprintln(os.Stderr, "error: error_artifact_missing")
			return 1
		}
		if unknownPath := os.Getenv("HELPER_ARTIFACT_MALFORMED"); unknownPath == args[7] {
			fmt.Println("bad")
			return 0
		}
		fmt.Printf("Recorded artifact: %s\nAttempt: %s\nDigest: %s\nSize: 12\n", args[7], args[5], reportDigest)
		return 0
	}
	if os.Getenv("HELPER_EXEC") == "early-close" {
		return 0
	}
	input, _ := io.ReadAll(os.Stdin)
	if encoded := os.Getenv("HELPER_FILES"); encoded != "" {
		var files map[string]string
		if json.Unmarshal([]byte(encoded), &files) != nil {
			return 96
		}
		for path, content := range files {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return 97
			}
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				return 98
			}
		}
	}
	if encoded := os.Getenv("HELPER_EXEC_ARGS"); encoded != "" {
		var want []string
		if json.Unmarshal([]byte(encoded), &want) != nil || !reflect.DeepEqual(args, want) {
			return 93
		}
	}
	if os.Getenv("HELPER_EXEC_MARK") != "" {
		_ = os.WriteFile(os.Getenv("HELPER_EXEC_MARK"), input, 0o600)
	}
	switch os.Getenv("HELPER_EXEC") {
	case "nonzero":
		return 7
	case "empty":
		return 0
	case "malformed":
		fmt.Print("{")
		return 0
	case "two-json":
		fmt.Print(`{} {}`)
		return 0
	case "oversized":
		_, _ = os.Stdout.Write(make([]byte, MaxExecutorStdoutBytes+1))
		time.Sleep(time.Second)
		return 0
	case "sleep":
		time.Sleep(5 * time.Second)
		return 0
	case "signal":
		process, _ := os.FindProcess(os.Getpid())
		_ = process.Signal(os.Interrupt)
		time.Sleep(time.Second)
		return 0
	case "tree":
		child := exec.Command("sleep", "5")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if child.Start() != nil {
			return 94
		}
		time.Sleep(5 * time.Second)
		return 0
	}
	if n, _ := strconv.Atoi(os.Getenv("HELPER_STDERR")); n > 0 {
		_, _ = os.Stderr.Write([]byte(strings.Repeat("z", n)))
	}
	var pkg workPackageHeader
	if json.Unmarshal(input, &pkg) != nil {
		return 8
	}
	outcome := envDefault("HELPER_OUTCOME", "completed")
	if os.Getenv("HELPER_EXEC") == "mismatch" {
		pkg.AttemptID = "other"
	}
	if os.Getenv("HELPER_EXEC") == "max" {
		reportPrefix := fmt.Sprintf(`{"schema_version":1,"flow_run_id":%q,"step_id":%q,"attempt_id":%q,"work_package_digest":%q,"outcome":%q,"summary":"ok","decisions":[],"artifact_refs":[],"unresolved_issues":[],"next_action":"","padding":"`,
			pkg.FlowRunID, pkg.StepID, pkg.AttemptID, pkg.WorkPackageDigest, outcome)
		reportSuffix := `"}`
		fmt.Print(reportPrefix + strings.Repeat("x", MaxExecutorStdoutBytes-len(reportPrefix)-len(reportSuffix)) + reportSuffix)
	} else {
		fmt.Printf(`{"schema_version":1,"flow_run_id":%q,"step_id":%q,"attempt_id":%q,"work_package_digest":%q,"outcome":%q,"summary":"ok","decisions":[],"artifact_refs":%s,"unresolved_issues":[],"next_action":""}`,
			pkg.FlowRunID, pkg.StepID, pkg.AttemptID, pkg.WorkPackageDigest, outcome, envDefault("HELPER_REFS", "[]"))
	}
	return 0
}

func TestRunArtifactModeOrderAndPartialFailure(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	t.Setenv("HELPER_ARTIFACTS", `[{"path":"required.txt","required":true},{"path":"optional.txt","required":false},{"path":"last.txt","required":true}]`)
	t.Setenv("HELPER_REFS", `["optional.txt"]`)
	mark := filepath.Join(cfg.ProjectRoot, "artifact-called")
	t.Setenv("HELPER_ARTIFACT_MARK", mark)
	t.Setenv("HELPER_ARTIFACT_FAIL", "optional.txt")
	got := Run(context.Background(), cfg)
	if got.ExitCode != 3 || got.ResultV2 == nil || got.ResultV2.Status != "failed" ||
		len(got.ResultV2.Artifacts) != 2 || got.ResultV2.Artifacts[0].Status != "recorded" ||
		got.ResultV2.Artifacts[1].Status != "failed" {
		t.Fatalf("Run = %#v", got)
	}
	data, _ := os.ReadFile(mark)
	if string(data) != "required.txt\noptional.txt\n" {
		t.Fatalf("artifact calls = %q", data)
	}
}

func TestRunArtifactModeStoppedEmptyForFailedOutcome(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_OUTCOME", "failed")
	got := Run(context.Background(), cfg)
	if got.ExitCode != 0 || got.ResultV2 == nil || got.ResultV2.Status != "stopped" ||
		got.ResultV2.Artifacts == nil || len(got.ResultV2.Artifacts) != 0 {
		t.Fatalf("Run = %#v", got)
	}
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func helperConfig(t *testing.T) (Config, string) {
	t.Helper()
	t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
	root := t.TempDir()
	mark := filepath.Join(root, "record-called")
	t.Setenv("HELPER_RECORD_MARK", mark)
	t.Setenv("HELPER_CWD", root)
	return Config{
		ProjectRoot: root, Devflow: os.Args[0], StepID: "build", AttemptID: "attempt_1",
		Executor: os.Args[0], TerminateGrace: 20 * time.Millisecond,
	}, mark
}

func TestRunRecordedOutcomesAndIdempotent(t *testing.T) {
	for _, outcome := range []string{"completed", "blocked", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			cfg, _ := helperConfig(t)
			t.Setenv("HELPER_OUTCOME", outcome)
			t.Setenv("HELPER_IDEMPOTENT", "true")
			got := Run(context.Background(), cfg)
			if got.ExitCode != 0 || got.Result.Status != "recorded" || got.Result.ReportOutcome != outcome ||
				!got.Result.ReportIdempotent || got.Result.Error != nil {
				t.Fatalf("Run = %#v", got)
			}
		})
	}
}

func TestRunFailures(t *testing.T) {
	tests := []struct {
		name, env, value, category, code string
		exit                             int
		recorded                         bool
	}{
		{"work package nonzero", "HELPER_WP", "nonzero", "devflow_contract", "error_stale_attempt", 3, false},
		{"work package empty", "HELPER_WP", "empty", "devflow_process", "invalid_output", 3, false},
		{"work package malformed", "HELPER_WP", "malformed", "devflow_process", "invalid_output", 3, false},
		{"work package mismatch", "HELPER_WP", "step-mismatch", "devflow_process", "invalid_output", 3, false},
		{"work package oversized", "HELPER_WP", "oversized", "devflow_process", "oversized_output", 3, false},
		{"executor nonzero", "HELPER_EXEC", "nonzero", "executor_process", "nonzero_exit", 4, false},
		{"executor empty", "HELPER_EXEC", "empty", "executor_protocol", "empty_stdout", 5, false},
		{"executor malformed", "HELPER_EXEC", "malformed", "executor_protocol", "invalid_report_output", 5, false},
		{"executor two JSON", "HELPER_EXEC", "two-json", "executor_protocol", "invalid_report_output", 5, false},
		{"executor mismatch", "HELPER_EXEC", "mismatch", "executor_protocol", "report_identity_mismatch", 5, false},
		{"executor oversized", "HELPER_EXEC", "oversized", "executor_protocol", "oversized_stdout", 5, false},
		{"record nonzero", "HELPER_RECORD", "nonzero", "devflow_contract", "error_conflicting_execution_report", 3, true},
		{"record oversized", "HELPER_RECORD", "oversized", "devflow_process", "oversized_output", 3, true},
		{"record malformed", "HELPER_RECORD", "malformed", "executor_protocol", "invalid_record_output", 5, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, mark := helperConfig(t)
			t.Setenv(tt.env, tt.value)
			got := Run(context.Background(), cfg)
			if got.ExitCode != tt.exit || got.Result.Error == nil ||
				got.Result.Error.Category != tt.category || got.Result.Error.Code != tt.code {
				t.Fatalf("Run = %#v", got)
			}
			if tt.name == "executor nonzero" && (got.Result.ExecutorExitCode == nil || *got.Result.ExecutorExitCode != 7) {
				t.Fatalf("executor exit code = %v", got.Result.ExecutorExitCode)
			}
			if tt.name == "executor oversized" && got.Result.ExecutorExitCode != nil {
				t.Fatalf("overflow executor exit code = %v, want nil", *got.Result.ExecutorExitCode)
			}
			_, err := os.Stat(mark)
			if (err == nil) != tt.recorded {
				t.Fatalf("record called=%t want %t", err == nil, tt.recorded)
			}
		})
	}
}

func TestRunTimeoutCancellationAndStderr(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		cfg, mark := helperConfig(t)
		cfg.Timeout = 30 * time.Millisecond
		t.Setenv("HELPER_EXEC", "sleep")
		got := Run(context.Background(), cfg)
		if got.ExitCode != 124 || got.Result.Status != "timed_out" {
			t.Fatalf("Run = %#v v2=%+v error=%+v artifacts=%+v", got, got.ResultV2, got.ResultV2.Error, got.ResultV2.Artifacts)
		}
		if got.Result.ExecutorExitCode != nil {
			t.Fatalf("executor exit code = %v, want nil", *got.Result.ExecutorExitCode)
		}
		if _, err := os.Stat(mark); err == nil {
			t.Fatal("record called")
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		cfg, mark := helperConfig(t)
		t.Setenv("HELPER_EXEC", "sleep")
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(30*time.Millisecond, cancel)
		got := Run(ctx, cfg)
		if got.ExitCode != 130 || got.Result.Status != "cancelled" {
			t.Fatalf("Run = %#v", got)
		}
		if got.Result.ExecutorExitCode != nil {
			t.Fatalf("executor exit code = %v, want nil", *got.Result.ExecutorExitCode)
		}
		if _, err := os.Stat(mark); err == nil {
			t.Fatal("record called")
		}
	})
	t.Run("stderr tail", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		t.Setenv("HELPER_STDERR", strconv.Itoa(MaxExecutorStderrTailBytes+1))
		got := Run(context.Background(), cfg)
		if got.ExitCode != 0 || !got.Result.StderrTruncated {
			t.Fatalf("Run = %#v", got)
		}
	})
	for _, tt := range []struct {
		name      string
		size      int
		truncated bool
	}{
		{"stderr below limit", MaxExecutorStderrTailBytes - 1, false},
		{"stderr exact limit", MaxExecutorStderrTailBytes, false},
		{"stderr very large", MaxExecutorStderrTailBytes * 16, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := helperConfig(t)
			t.Setenv("HELPER_STDERR", strconv.Itoa(tt.size))
			got := Run(context.Background(), cfg)
			if got.ExitCode != 0 || got.Result.StderrTruncated != tt.truncated {
				t.Fatalf("Run = %#v", got)
			}
		})
	}
}

func TestRunExecutorSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt delivery is not supported on Windows")
	}
	cfg, mark := helperConfig(t)
	t.Setenv("HELPER_EXEC", "signal")
	got := Run(context.Background(), cfg)
	if got.ExitCode != 4 || got.Result.Error == nil || got.Result.Error.Code != "signaled" ||
		got.Result.ExecutorExitCode != nil {
		t.Fatalf("Run = %#v error=%+v", got, got.Result.Error)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("record called")
	}
}

func TestRunTimeoutTerminatesProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("complete process-tree termination is not supported on Windows")
	}
	cfg, mark := helperConfig(t)
	cfg.Timeout = 50 * time.Millisecond
	cfg.TerminateGrace = 20 * time.Millisecond
	t.Setenv("HELPER_EXEC", "tree")
	started := time.Now()
	got := Run(context.Background(), cfg)
	if got.ExitCode != 124 || got.Result.Status != "timed_out" || time.Since(started) > 3*time.Second {
		t.Fatalf("Run = %#v elapsed=%s", got, time.Since(started))
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("record called")
	}
}

func TestRunExecutorStartFailureAndExactStdin(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.Executor = filepath.Join(cfg.ProjectRoot, "missing")
	got := Run(context.Background(), cfg)
	if got.ExitCode != 4 || got.Result.ExecutorExitCode != nil || got.Result.Error.Code != "start_failed" {
		t.Fatalf("Run = %#v", got)
	}

	cfg, _ = helperConfig(t)
	mark := filepath.Join(cfg.ProjectRoot, "stdin")
	t.Setenv("HELPER_EXEC_MARK", mark)
	got = Run(context.Background(), cfg)
	if got.ExitCode != 0 {
		t.Fatalf("Run = %#v", got)
	}
	data, err := os.ReadFile(mark)
	if err != nil || data[len(data)-1] != '\n' {
		t.Fatalf("stdin preserved: %q, %v", data, err)
	}

	cfg, _ = helperConfig(t)
	cfg.ExecutorArgs = []string{"", " x ", "--flag=value"}
	t.Setenv("HELPER_EXEC_ARGS", `[""," x ","--flag=value"]`)
	got = Run(context.Background(), cfg)
	if got.ExitCode != 0 {
		t.Fatalf("args Run = %#v", got)
	}
}

func TestRunRecordedCleanupFailureIsWarning(t *testing.T) {
	cfg, _ := helperConfig(t)
	got := runWithFileOps(context.Background(), cfg, os.CreateTemp, func(path string) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		return fmt.Errorf("injected cleanup failure")
	})
	if got.ExitCode != 0 || got.Result.Status != "recorded" || got.Result.Error != nil ||
		got.CleanupCode != "temporary_report_cleanup_failed" {
		t.Fatalf("Run = %#v", got)
	}
}

func TestRunConfiguredOutputLimitsAndTempCleanup(t *testing.T) {
	t.Run("WorkPackage exact limit", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		t.Setenv("HELPER_WP", "max")
		got := Run(context.Background(), cfg)
		if got.ExitCode != 0 || got.Result.Status != "recorded" {
			t.Fatalf("Run = %#v", got)
		}
	})
	t.Run("Executor stdout exact limit", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		t.Setenv("HELPER_EXEC", "max")
		got := Run(context.Background(), cfg)
		if got.ExitCode != 0 || got.Result.Status != "recorded" {
			t.Fatalf("Run = %#v", got)
		}
	})
	t.Run("temporary report removed", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		tempRoot := t.TempDir()
		t.Setenv("TMPDIR", tempRoot)
		got := Run(context.Background(), cfg)
		if got.ExitCode != 0 {
			t.Fatalf("Run = %#v", got)
		}
		entries, err := os.ReadDir(tempRoot)
		if err != nil || len(entries) != 0 {
			t.Fatalf("temp entries=%v err=%v", entries, err)
		}
	})
}

func TestRunExecutorEarlyStdinClose(t *testing.T) {
	cfg, mark := helperConfig(t)
	t.Setenv("HELPER_WP", "max")
	t.Setenv("HELPER_EXEC", "early-close")
	got := Run(context.Background(), cfg)
	if got.ExitCode != 6 || got.Result.Error == nil || got.Result.Error.Code != "stdin_write_failed" {
		t.Fatalf("Run = %#v error=%+v", got, got.Result.Error)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("record called")
	}
}

func TestIntegrationRealDevflow(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "devflow")
	build := exec.Command("go", "build", "-o", bin, "./cmd/devflow")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build devflow: %v: %s", err, output)
	}
	root := t.TempDir()
	runCLI := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("devflow %v: %v: %s", args, err, output)
		}
		return output
	}
	runCLI("init")
	if err := os.WriteFile(filepath.Join(root, "task.md"), []byte("Do the task.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runCLI("start", "post-task-review", "--task-file", "task.md")
	currentPath := filepath.Join(root, ".devflow", "current.json")
	currentBefore, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	var pointer struct {
		FlowRunID string `json:"flow_run_id"`
	}
	if err := json.Unmarshal(currentBefore, &pointer); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, ".devflow", "runs", pointer.FlowRunID, "state.json")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		CurrentStepID    string `json:"current_step_id"`
		CurrentAttemptID string `json:"current_attempt_id"`
	}
	if err := json.Unmarshal(stateBefore, &state); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
	t.Setenv("HELPER_OUTCOME", "blocked")
	cfg := Config{
		ProjectRoot: root, Devflow: bin, StepID: state.CurrentStepID, AttemptID: state.CurrentAttemptID,
		Executor: os.Args[0], TerminateGrace: 20 * time.Millisecond,
	}
	first := Run(context.Background(), cfg)
	if first.ExitCode != 0 || first.Result.Status != "recorded" || first.Result.ReportIdempotent {
		t.Fatalf("first Run = %#v", first)
	}
	reportPath := filepath.Join(root, ".devflow", "runs", pointer.FlowRunID, "execution-reports", state.CurrentAttemptID+".json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatal(err)
	}
	stateAfter, _ := os.ReadFile(statePath)
	currentAfter, _ := os.ReadFile(currentPath)
	if string(stateAfter) != string(stateBefore) || string(currentAfter) != string(currentBefore) {
		t.Fatal("State or current pointer changed")
	}
	second := Run(context.Background(), cfg)
	if second.ExitCode != 0 || !second.Result.ReportIdempotent {
		t.Fatalf("second Run = %#v", second)
	}
	if runtime.GOOS != "windows" {
		wrapper := filepath.Join(t.TempDir(), "devflow-wrapper")
		script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"execution-report\" ]; then\n  %q \"$@\" >/dev/null || exit $?\n  printf 'bad\\n'\n  exit 0\nfi\nexec %q \"$@\"\n", bin, bin)
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		cfg.Devflow = wrapper
		uncertain := Run(context.Background(), cfg)
		if uncertain.ExitCode != 5 || uncertain.Result.Error == nil ||
			uncertain.Result.Error.Code != "invalid_record_output" {
			t.Fatalf("uncertain Run = %#v error=%+v", uncertain, uncertain.Result.Error)
		}
		if _, err := os.Stat(reportPath); err != nil {
			t.Fatalf("record disappeared after invalid success output: %v", err)
		}
	}
}

const integrationArtifactFlow = `flow: {
	id: "artifact-runtime-review"
	title: "Artifact runtime review"
	description: "Artifact runtime integration fixture."
	steps: [{
		id: "build"
		title: "Build"
		instruction: "Build artifacts."
		artifacts: [
			{path: "out/a.txt", required: true},
			{path: "out/b.txt", required: false},
			{path: "out/c.txt", required: false},
		]
	}]
}
`

type artifactIntegration struct {
	root, bin, runID, stepID, attemptID, statePath, currentPath, reportPath string
	cfg                                                                     Config
}

func setupArtifactIntegration(t *testing.T, bin string) artifactIntegration {
	t.Helper()
	root := t.TempDir()
	runCLI := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("devflow %v: %v: %s", args, err, output)
		}
		return output
	}
	runCLI("init")
	flowPath := filepath.Join(root, ".devflow", "flows", "artifact-runtime-review.cue")
	if err := os.WriteFile(flowPath, []byte(integrationArtifactFlow), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "task.md"), []byte("Build artifacts.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runCLI("start", "artifact-runtime-review", "--task-file", "task.md")
	currentPath := filepath.Join(root, ".devflow", "current.json")
	current := readJSONMap(t, currentPath)
	runID := current["flow_run_id"].(string)
	statePath := filepath.Join(root, ".devflow", "runs", runID, "state.json")
	state := readJSONMap(t, statePath)
	stepID := state["current_step_id"].(string)
	attemptID := state["current_attempt_id"].(string)
	return artifactIntegration{
		root: root, bin: bin, runID: runID, stepID: stepID, attemptID: attemptID,
		statePath: statePath, currentPath: currentPath,
		reportPath: filepath.Join(root, ".devflow", "runs", runID, "execution-reports", attemptID+".json"),
		cfg: Config{
			ProjectRoot: root, Devflow: bin, StepID: stepID, AttemptID: attemptID,
			Executor: os.Args[0], TerminateGrace: 20 * time.Millisecond, RecordArtifacts: true,
		},
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func artifactEvidence(t *testing.T, statePath, attemptID string) map[string]any {
	t.Helper()
	state := readJSONMap(t, statePath)
	for _, raw := range state["attempts"].([]any) {
		attempt := raw.(map[string]any)
		if attempt["id"] == attemptID {
			return attempt["artifact_evidence"].(map[string]any)
		}
	}
	t.Fatalf("Attempt %s absent", attemptID)
	return nil
}

func stateWithoutArtifactEvidence(t *testing.T, path string) []byte {
	t.Helper()
	state := readJSONMap(t, path)
	for _, raw := range state["attempts"].([]any) {
		raw.(map[string]any)["artifact_evidence"] = map[string]any{}
	}
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestIntegrationRealDevflowArtifactMode(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "devflow")
	build := exec.Command("go", "build", "-o", bin, "./cmd/devflow")
	build.Dir = repositoryRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build devflow: %v: %s", err, output)
	}

	t.Run("completed records required and referenced optional in Flow order", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "completed")
		t.Setenv("HELPER_REFS", `["out/b.txt"]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A",%q:"B",%q:"C"}`,
			filepath.Join(fixture.root, "out", "a.txt"), filepath.Join(fixture.root, "out", "b.txt"), filepath.Join(fixture.root, "out", "c.txt")))
		currentBefore, _ := os.ReadFile(fixture.currentPath)
		stateBefore := stateWithoutArtifactEvidence(t, fixture.statePath)
		got := Run(context.Background(), fixture.cfg)
		if got.ExitCode != 0 || got.ResultV2 == nil || got.ResultV2.Status != "stopped" ||
			len(got.ResultV2.Artifacts) != 2 || got.ResultV2.Artifacts[0].Path != "out/a.txt" ||
			got.ResultV2.Artifacts[1].Path != "out/b.txt" {
			t.Fatalf("Run = %#v", got)
		}
		evidence := artifactEvidence(t, fixture.statePath, fixture.attemptID)
		if evidence["out/a.txt"] == nil || evidence["out/b.txt"] == nil || evidence["out/c.txt"] != nil {
			t.Fatalf("evidence = %#v", evidence)
		}
		reportBefore, err := os.ReadFile(fixture.reportPath)
		if err != nil {
			t.Fatal(err)
		}
		currentAfter, _ := os.ReadFile(fixture.currentPath)
		reportAfter, _ := os.ReadFile(fixture.reportPath)
		if !bytes.Equal(currentBefore, currentAfter) || !bytes.Equal(reportBefore, reportAfter) ||
			!bytes.Equal(stateBefore, stateWithoutArtifactEvidence(t, fixture.statePath)) {
			t.Fatal("pointer, Report, or non-Artifact State changed")
		}
	})

	t.Run("blocked records referenced optional only", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "blocked")
		t.Setenv("HELPER_REFS", `["out/b.txt"]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A",%q:"B"}`,
			filepath.Join(fixture.root, "out", "a.txt"), filepath.Join(fixture.root, "out", "b.txt")))
		got := Run(context.Background(), fixture.cfg)
		evidence := artifactEvidence(t, fixture.statePath, fixture.attemptID)
		if got.ExitCode != 0 || got.ResultV2.Status != "stopped" || len(got.ResultV2.Artifacts) != 1 ||
			got.ResultV2.Artifacts[0].Path != "out/b.txt" || evidence["out/a.txt"] != nil || evidence["out/b.txt"] == nil {
			t.Fatalf("Run=%#v v2=%+v error=%+v artifacts=%+v evidence=%#v", got, got.ResultV2, got.ResultV2.Error, got.ResultV2.Artifacts, evidence)
		}
	})

	t.Run("failed records Report only", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "failed")
		t.Setenv("HELPER_REFS", `[]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A"}`, filepath.Join(fixture.root, "out", "a.txt")))
		got := Run(context.Background(), fixture.cfg)
		if got.ExitCode != 0 || got.ResultV2.Status != "stopped" || len(got.ResultV2.Artifacts) != 0 ||
			len(artifactEvidence(t, fixture.statePath, fixture.attemptID)) != 0 {
			t.Fatalf("Run = %#v", got)
		}
		if _, err := os.Stat(fixture.reportPath); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("same Report and files are idempotent", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "completed")
		t.Setenv("HELPER_REFS", `[]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A"}`, filepath.Join(fixture.root, "out", "a.txt")))
		first := Run(context.Background(), fixture.cfg)
		stateAfterFirst, _ := os.ReadFile(fixture.statePath)
		second := Run(context.Background(), fixture.cfg)
		stateAfterSecond, _ := os.ReadFile(fixture.statePath)
		if first.ExitCode != 0 || second.ExitCode != 0 || !second.ResultV2.ReportIdempotent ||
			len(second.ResultV2.Artifacts) != 1 || second.ResultV2.Artifacts[0].Status != "recorded" ||
			!bytes.Equal(stateAfterFirst, stateAfterSecond) {
			t.Fatalf("first=%#v second=%#v", first, second)
		}
	})

	t.Run("changed file conflicts without rollback", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "completed")
		t.Setenv("HELPER_REFS", `["out/b.txt"]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A1",%q:"B1"}`,
			filepath.Join(fixture.root, "out", "a.txt"), filepath.Join(fixture.root, "out", "b.txt")))
		if first := Run(context.Background(), fixture.cfg); first.ExitCode != 0 {
			t.Fatalf("first=%#v v2=%+v error=%+v artifacts=%+v", first, first.ResultV2, first.ResultV2.Error, first.ResultV2.Artifacts)
		}
		evidenceBefore := artifactEvidence(t, fixture.statePath, fixture.attemptID)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A2",%q:"B2"}`,
			filepath.Join(fixture.root, "out", "a.txt"), filepath.Join(fixture.root, "out", "b.txt")))
		second := Run(context.Background(), fixture.cfg)
		evidenceAfter := artifactEvidence(t, fixture.statePath, fixture.attemptID)
		if second.ExitCode != 3 || second.ResultV2.Status != "failed" || !second.ResultV2.ReportIdempotent ||
			len(second.ResultV2.Artifacts) != 1 || second.ResultV2.Artifacts[0].Path != "out/a.txt" ||
			second.ResultV2.Artifacts[0].Status != "failed" || !reflect.DeepEqual(evidenceBefore, evidenceAfter) {
			t.Fatalf("second=%#v before=%#v after=%#v", second, evidenceBefore, evidenceAfter)
		}
	})

	t.Run("missing required leaves Report and skips following target", func(t *testing.T) {
		fixture := setupArtifactIntegration(t, bin)
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "completed")
		t.Setenv("HELPER_REFS", `["out/b.txt"]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"B"}`, filepath.Join(fixture.root, "out", "b.txt")))
		currentBefore, _ := os.ReadFile(fixture.currentPath)
		got := Run(context.Background(), fixture.cfg)
		currentAfter, _ := os.ReadFile(fixture.currentPath)
		if got.ExitCode != 3 || got.ResultV2.Status != "failed" || len(got.ResultV2.Artifacts) != 1 ||
			got.ResultV2.Artifacts[0].Path != "out/a.txt" ||
			len(artifactEvidence(t, fixture.statePath, fixture.attemptID)) != 0 ||
			!bytes.Equal(currentBefore, currentAfter) {
			t.Fatalf("Run = %#v", got)
		}
		if _, err := os.Stat(fixture.reportPath); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("stale Attempt is rejected after Report record", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("shell wrapper fixture is Unix-specific")
		}
		fixture := setupArtifactIntegration(t, bin)
		wrapper := filepath.Join(t.TempDir(), "devflow-stale-wrapper")
		script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = artifact ]; then\n  %q finish --reason stale >/dev/null || exit $?\nfi\nexec %q \"$@\"\n", bin, bin)
		if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		fixture.cfg.Devflow = wrapper
		t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
		t.Setenv("HELPER_OUTCOME", "completed")
		t.Setenv("HELPER_REFS", `[]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A"}`, filepath.Join(fixture.root, "out", "a.txt")))
		got := Run(context.Background(), fixture.cfg)
		if got.ExitCode != 3 || got.ResultV2.Status != "failed" || len(got.ResultV2.Artifacts) != 1 ||
			got.ResultV2.Artifacts[0].Status != "failed" ||
			len(artifactEvidence(t, fixture.statePath, fixture.attemptID)) != 0 {
			t.Fatalf("Run = %#v", got)
		}
		if _, err := os.Stat(fixture.reportPath); err != nil {
			t.Fatal(err)
		}
	})
}
