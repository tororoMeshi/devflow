package automationruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	if len(args) == 1 && args[0] == "bounded" {
		if path := os.Getenv("HELPER_BOUNDED_CHILD_PID"); path != "" {
			child := exec.Command("sleep", "5")
			if err := child.Start(); err != nil {
				return 106
			}
			if err := os.WriteFile(path, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
				return 107
			}
		}
		if n, _ := strconv.Atoi(os.Getenv("HELPER_BOUNDED_STDOUT")); n > 0 {
			_, _ = os.Stdout.Write([]byte(strings.Repeat("o", n)))
		}
		if n, _ := strconv.Atoi(os.Getenv("HELPER_BOUNDED_STDERR")); n > 0 {
			_, _ = os.Stderr.Write([]byte(strings.Repeat("e", n)))
		}
		return 0
	}
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
			checks := envDefault("HELPER_CHECKS", "[]")
			fmt.Printf(`{"schema_version":1,"work_package_digest":"%s","flow_run_id":"run_1","step_id":%q,"attempt_id":%q,"step":{"artifacts":%s,"required_checks":%s}}`+"\n", testDigest, step, attempt, artifacts, checks)
		} else {
			fmt.Printf(`{"schema_version":1,"work_package_digest":"%s","flow_run_id":"run_1","step_id":%q,"attempt_id":%q}`+"\n", testDigest, step, attempt)
		}
		return 0
	}
	if len(args) > 0 && args[0] == "completion-context" {
		if len(args) != 5 || args[1] != "--step" || args[3] != "--attempt" {
			return 104
		}
		if mark := os.Getenv("HELPER_COMPLETION_MARK"); mark != "" {
			_ = os.WriteFile(mark, []byte(strings.Join(args, " ")), 0o600)
		}
		switch os.Getenv("HELPER_COMPLETION") {
		case "nonzero":
			fmt.Fprintln(os.Stderr, "sensitive core stderr")
			return 1
		case "malformed":
			fmt.Print("{")
			return 0
		case "mismatch":
			args[2] = "other"
		case "oversized":
			_, _ = os.Stdout.Write(make([]byte, MaxExecutorStdoutBytes+1))
			return 0
		case "sleep":
			time.Sleep(5 * time.Second)
		}
		fmt.Printf(`{"schema_version":1,"flow_run_id":"run_1","step_id":%q,"attempt_id":%q,"attempt_status":"active","is_current_attempt":true,"artifacts":[],"checks":[],"approval":{"required":false,"status":"not_required","evidence_set_digest":null,"approved_evidence_set_digest":null},"completion":{"status":"ready","blocker":null}}`+"\n", args[2], args[4])
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
	if len(args) > 1 && args[0] == "check" && args[1] == "request" {
		if len(args) != 8 || args[2] != "--step" || args[4] != "--attempt" || args[6] != "--check" {
			return 99
		}
		if os.Getenv("HELPER_ALREADY") == args[7] {
			fmt.Fprintln(os.Stderr, "error: error_check_result_already_recorded")
			return 1
		}
		if n, _ := strconv.Atoi(os.Getenv("HELPER_CHECK_REQUEST_STDOUT")); n > 0 {
			_, _ = os.Stdout.Write([]byte(strings.Repeat("r", n)))
			time.Sleep(time.Second)
			return 0
		}
		fmt.Printf(`{"schema_version":2,"flow_run_id":"run_1","step_id":%q,"attempt_id":%q,"check_id":%q}`+"\n",
			args[3], args[5], args[7])
		return 0
	}
	if len(args) > 1 && args[0] == "check" && args[1] == "record" {
		if len(args) != 4 || args[2] != "--file" {
			return 100
		}
		data, err := os.ReadFile(args[3])
		info, statErr := os.Stat(args[3])
		if err != nil || statErr != nil || info.Mode().Perm() != 0o600 {
			return 101
		}
		var record struct {
			FlowRunID string `json:"flow_run_id"`
			StepID    string `json:"step_id"`
			AttemptID string `json:"attempt_id"`
			CheckID   string `json:"check_id"`
			Result    struct {
				ExitCode int `json:"exit_code"`
			} `json:"result"`
		}
		if json.Unmarshal(data, &record) != nil {
			return 102
		}
		if mark := os.Getenv("HELPER_CHECK_RECORD_MARK"); mark != "" {
			f, _ := os.OpenFile(mark, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if f != nil {
				_, _ = fmt.Fprintln(f, record.CheckID)
				_ = f.Close()
			}
		}
		fmt.Printf("Recorded check: %s\nRun: %s\nStep: %s\nAttempt: %s\nExit code: %d\n",
			record.CheckID, record.FlowRunID, record.StepID, record.AttemptID, record.Result.ExitCode)
		return 0
	}
	if len(args) > 0 && args[0] == "adapter" {
		input, _ := io.ReadAll(os.Stdin)
		var request checkIdentity
		if json.Unmarshal(input, &request) != nil {
			return 103
		}
		if mark := os.Getenv("HELPER_ADAPTER_MARK"); mark != "" {
			f, _ := os.OpenFile(mark, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if f != nil {
				_, _ = fmt.Fprintln(f, request.CheckID)
				_ = f.Close()
			}
		}
		if os.Getenv("HELPER_ADAPTER_NONZERO") == request.CheckID {
			return 7
		}
		if os.Getenv("HELPER_ADAPTER_MALFORMED") == request.CheckID {
			fmt.Print("{")
			return 0
		}
		if os.Getenv("HELPER_CLOSE_ATTEMPT") == request.CheckID {
			cmd := exec.Command(os.Getenv("HELPER_REAL_DEVFLOW"), "skip", "--reason", "fixture-stale-attempt")
			cmd.Dir = os.Getenv("HELPER_PROJECT_ROOT")
			if output, err := cmd.CombinedOutput(); err != nil {
				fmt.Fprintln(os.Stderr, string(output))
				return 104
			}
		}
		if os.Getenv("HELPER_ADAPTER_CHILD") == request.CheckID {
			child := exec.Command("sleep", "5")
			if err := child.Start(); err != nil {
				return 105
			}
			if path := os.Getenv("HELPER_ADAPTER_CHILD_PID"); path != "" {
				_ = os.WriteFile(path, []byte(strconv.Itoa(child.Process.Pid)), 0o600)
			}
		}
		if os.Getenv("HELPER_ADAPTER_SLEEP") == request.CheckID {
			if n, _ := strconv.Atoi(os.Getenv("HELPER_ADAPTER_STDOUT")); n > 0 {
				_, _ = os.Stdout.Write([]byte(strings.Repeat("a", n)))
				time.Sleep(time.Second)
				return 0
			}
			time.Sleep(5 * time.Second)
			return 0
		}
		exitCode := 0
		if os.Getenv("HELPER_CHECK_FAILED") == request.CheckID {
			exitCode = 1
		}
		fmt.Printf(`{"schema_version":2,"flow_run_id":%q,"step_id":%q,"attempt_id":%q,"check_id":%q,"result":{"exit_code":%d,"log_path":"logs/%s.log"}}`,
			request.FlowRunID, request.StepID, request.AttemptID, request.CheckID, exitCode, request.CheckID)
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

func TestRunCheckModeFlowOrderAndFailedDomainContinues(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	cfg.CheckAdapter = os.Args[0]
	cfg.CheckAdapterArgs = []string{"adapter"}
	cfg.CheckTerminateGrace = 20 * time.Millisecond
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_CHECKS", `["first","second"]`)
	t.Setenv("HELPER_CHECK_FAILED", "first")
	adapterMark := filepath.Join(cfg.ProjectRoot, "adapter-calls")
	recordMark := filepath.Join(cfg.ProjectRoot, "check-records")
	t.Setenv("HELPER_ADAPTER_MARK", adapterMark)
	t.Setenv("HELPER_CHECK_RECORD_MARK", recordMark)

	got := Run(context.Background(), cfg)
	if got.ExitCode != 0 || got.ResultV3 == nil || got.ResultV3.Status != "stopped" ||
		len(got.ResultV3.Checks) != 2 || got.ResultV3.Checks[0].Passed == nil ||
		*got.ResultV3.Checks[0].Passed || got.ResultV3.Checks[1].Passed == nil ||
		!*got.ResultV3.Checks[1].Passed {
		t.Fatalf("Run = %#v", got)
	}
	for _, path := range []string{adapterMark, recordMark} {
		data, _ := os.ReadFile(path)
		if string(data) != "first\nsecond\n" {
			t.Fatalf("%s = %q", path, data)
		}
	}
}

func TestRunCheckModeAlreadyRecordedSkipsAdapter(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	cfg.CheckAdapter = os.Args[0]
	cfg.CheckAdapterArgs = []string{"adapter"}
	cfg.CheckTerminateGrace = 20 * time.Millisecond
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_CHECKS", `["first"]`)
	t.Setenv("HELPER_ALREADY", "first")
	mark := filepath.Join(cfg.ProjectRoot, "adapter-calls")
	t.Setenv("HELPER_ADAPTER_MARK", mark)

	got := Run(context.Background(), cfg)
	if got.ExitCode != 0 || got.ResultV3 == nil || len(got.ResultV3.Checks) != 1 ||
		got.ResultV3.Checks[0].Status != "already_recorded" || got.ResultV3.Checks[0].Passed != nil {
		t.Fatalf("Run = %#v", got)
	}
	if _, err := os.Stat(mark); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("adapter was called: %v", err)
	}
}

func TestRunCheckModeAdapterFailureStopsFollowingChecks(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	cfg.CheckAdapter = os.Args[0]
	cfg.CheckAdapterArgs = []string{"adapter"}
	cfg.CheckTerminateGrace = 20 * time.Millisecond
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_CHECKS", `["first","second","third"]`)
	t.Setenv("HELPER_ADAPTER_NONZERO", "second")
	mark := filepath.Join(cfg.ProjectRoot, "adapter-calls")
	t.Setenv("HELPER_ADAPTER_MARK", mark)

	got := Run(context.Background(), cfg)
	if got.ExitCode != 4 || got.ResultV3 == nil || got.ResultV3.Status != "failed" ||
		len(got.ResultV3.Checks) != 2 || got.ResultV3.Checks[1].Status != "failed" {
		t.Fatalf("Run = %#v", got)
	}
	data, _ := os.ReadFile(mark)
	if string(data) != "first\nsecond\n" {
		t.Fatalf("adapter calls = %q", data)
	}
}

func TestRunCheckModeAdapterTimeout(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts = true
	cfg.CheckAdapter = os.Args[0]
	cfg.CheckAdapterArgs = []string{"adapter"}
	cfg.CheckTimeout = 20 * time.Millisecond
	cfg.CheckTerminateGrace = 20 * time.Millisecond
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_CHECKS", `["first"]`)
	t.Setenv("HELPER_ADAPTER_SLEEP", "first")

	got := Run(context.Background(), cfg)
	if got.ExitCode != 124 || got.ResultV3 == nil || got.ResultV3.Status != "timed_out" ||
		len(got.ResultV3.Checks) != 1 || got.ResultV3.Checks[0].Status != "failed" ||
		got.ResultV3.Checks[0].AdapterExitCode != nil {
		t.Fatalf("Run = %#v", got)
	}
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

func TestRunCompletionContextResultV4(t *testing.T) {
	for _, outcome := range []string{"completed", "blocked", "failed"} {
		t.Run(outcome, func(t *testing.T) {
			cfg, _ := helperConfig(t)
			cfg.RecordArtifacts, cfg.CheckAdapter, cfg.CheckAdapterArgs, cfg.CompletionContext = true, os.Args[0], []string{"adapter"}, true
			t.Setenv("HELPER_ARTIFACTS", `[]`)
			t.Setenv("HELPER_CHECKS", `["check-a"]`)
			t.Setenv("HELPER_OUTCOME", outcome)
			if outcome == "completed" {
				t.Setenv("HELPER_CHECK_FAILED", "check-a")
			}
			mark := filepath.Join(cfg.ProjectRoot, "completion-called")
			t.Setenv("HELPER_COMPLETION_MARK", mark)
			got := Run(context.Background(), cfg)
			if got.ExitCode != 0 || got.ResultV4 == nil || got.ResultV4.SchemaVersion != 4 || got.ResultV4.Status != "recorded" || got.ResultV4.Error != nil || !bytes.Contains(got.ResultV4.CompletionContext, []byte(`"schema_version":1`)) {
				t.Fatalf("Run = %#v", got)
			}
			data, err := os.ReadFile(mark)
			if err != nil || string(data) != "completion-context --step build --attempt attempt_1" {
				t.Fatalf("completion args=%q err=%v", data, err)
			}
		})
	}
}

func TestRunCompletionContextFailures(t *testing.T) {
	tests := []struct {
		name, mode, category, code string
		exit                       int
	}{
		{"process", "nonzero", "completion_context_process", "failed", 3},
		{"protocol", "malformed", "completion_context_protocol", "invalid_output", 5},
		{"identity", "mismatch", "completion_context_protocol", "invalid_output", 5},
		{"overflow", "oversized", "completion_context_protocol", "oversized_stdout", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := helperConfig(t)
			cfg.RecordArtifacts, cfg.CheckAdapter, cfg.CheckAdapterArgs, cfg.CompletionContext = true, os.Args[0], []string{"adapter"}, true
			t.Setenv("HELPER_ARTIFACTS", `[]`)
			t.Setenv("HELPER_CHECKS", `[]`)
			t.Setenv("HELPER_COMPLETION", tt.mode)
			got := Run(context.Background(), cfg)
			if got.ExitCode != tt.exit || got.ResultV4 == nil || got.ResultV4.CompletionContext == nil || string(got.ResultV4.CompletionContext) != "null" || got.ResultV4.Error == nil || got.ResultV4.Error.Category != tt.category || got.ResultV4.Error.Code != tt.code {
				t.Fatalf("Run = %#v", got)
			}
			encoded, _ := json.Marshal(got.ResultV4)
			if bytes.Contains(encoded, []byte("sensitive core stderr")) {
				t.Fatalf("result leaks Core stderr: %s", encoded)
			}
		})
	}
}

func TestRunCompletionContextSkipsV3Failure(t *testing.T) {
	cfg, _ := helperConfig(t)
	cfg.RecordArtifacts, cfg.CheckAdapter, cfg.CheckAdapterArgs, cfg.CompletionContext = true, os.Args[0], []string{"adapter"}, true
	t.Setenv("HELPER_ARTIFACTS", `[]`)
	t.Setenv("HELPER_CHECKS", `["check-a"]`)
	t.Setenv("HELPER_ADAPTER_NONZERO", "check-a")
	mark := filepath.Join(cfg.ProjectRoot, "completion-called")
	t.Setenv("HELPER_COMPLETION_MARK", mark)
	got := Run(context.Background(), cfg)
	if got.ExitCode == 0 || got.ResultV4 == nil || got.ResultV4.Error == nil {
		t.Fatalf("Run = %#v", got)
	}
	if _, err := os.Stat(mark); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("completion context was called: %v", err)
	}
}

func TestRunCompletionContextTimeoutAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name, wantStatus, wantCode string
		context                    func() (context.Context, context.CancelFunc)
	}{
		{"timeout", "timed_out", "timeout", func() (context.Context, context.CancelFunc) {
			return context.Background(), func() {}
		}},
		{"cancellation", "cancelled", "cancelled", func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := helperConfig(t)
			cfg.RecordArtifacts, cfg.CheckAdapter, cfg.CheckAdapterArgs, cfg.CompletionContext = true, os.Args[0], []string{"adapter"}, true
			if tt.name == "timeout" {
				cfg.Timeout = 100 * time.Millisecond
			}
			t.Setenv("HELPER_ARTIFACTS", `[]`)
			t.Setenv("HELPER_CHECKS", `[]`)
			t.Setenv("HELPER_COMPLETION", "sleep")
			mark := filepath.Join(cfg.ProjectRoot, "completion-called")
			t.Setenv("HELPER_COMPLETION_MARK", mark)
			ctx, cancel := tt.context()
			defer cancel()
			if tt.name == "cancellation" {
				go func() {
					for {
						if _, err := os.Stat(mark); err == nil {
							cancel()
							return
						}
						time.Sleep(time.Millisecond)
					}
				}()
			}
			base := Result{FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1"}
			v3 := &ResultV3{SchemaVersion: 3, Status: "stopped", FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1", Artifacts: []ArtifactResult{}, Checks: []CheckItem{}}
			got := finishV4(ctx, cfg, RunResult{Result: base, ResultV3: v3})
			if got.ExitCode != map[string]int{"timeout": 124, "cancellation": 130}[tt.name] || got.ResultV4 == nil || got.ResultV4.Status != tt.wantStatus || got.ResultV4.Error == nil || got.ResultV4.Error.Code != tt.wantCode {
				t.Fatalf("Run = %#v", got)
			}
		})
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

func TestCheckBoundedIOBoundaries(t *testing.T) {
	t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")

	t.Run("request and adapter stdout exact limit", func(t *testing.T) {
		for _, limit := range []int{MaxCheckRequestBytes, MaxCheckAdapterStdoutBytes} {
			t.Setenv("HELPER_BOUNDED_STDOUT", strconv.Itoa(limit))
			got := runStreamProcess(context.Background(), t.TempDir(), os.Args[0], []string{"bounded"}, nil, limit, MaxCheckAdapterStderrTailBytes, 0, 0)
			if got.startErr != nil || got.waitErr != nil || got.overflow || len(got.stdout) != limit || got.exitCode == nil || *got.exitCode != 0 {
				t.Fatalf("limit=%d result=%#v", limit, got)
			}
		}
	})

	t.Run("request overflow stops before adapter and record", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		cfg.RecordArtifacts = true
		cfg.CheckAdapter = os.Args[0]
		cfg.CheckAdapterArgs = []string{"adapter"}
		cfg.CheckTerminateGrace = 20 * time.Millisecond
		t.Setenv("HELPER_ARTIFACTS", `[]`)
		t.Setenv("HELPER_CHECKS", `["first","second"]`)
		t.Setenv("HELPER_CHECK_REQUEST_STDOUT", strconv.Itoa(MaxCheckRequestBytes+1))
		adapterMark := filepath.Join(cfg.ProjectRoot, "adapter-calls")
		recordMark := filepath.Join(cfg.ProjectRoot, "check-records")
		t.Setenv("HELPER_ADAPTER_MARK", adapterMark)
		t.Setenv("HELPER_CHECK_RECORD_MARK", recordMark)
		tempRoot := t.TempDir()
		t.Setenv("TMPDIR", tempRoot)

		got := Run(context.Background(), cfg)
		if got.ExitCode != 3 || got.ResultV3 == nil || len(got.ResultV3.Checks) != 1 || got.ResultV3.Checks[0].Error == nil || got.ResultV3.Checks[0].Error.Category != "check_request" || got.ResultV3.Checks[0].Error.Code != "request_output_oversized" {
			t.Fatalf("Run=%#v", got)
		}
		assertNoPath(t, adapterMark)
		assertNoPath(t, recordMark)
		assertEmptyDir(t, tempRoot)
		result, err := json.Marshal(got.ResultV3)
		if err != nil || bytes.Contains(result, []byte("rrrr")) {
			t.Fatalf("runtime result exposed request stdout: %v %s", err, result)
		}
	})

	t.Run("adapter overflow stops before record", func(t *testing.T) {
		cfg, _ := helperConfig(t)
		cfg.RecordArtifacts = true
		cfg.CheckAdapter = os.Args[0]
		cfg.CheckAdapterArgs = []string{"adapter"}
		cfg.CheckTerminateGrace = 20 * time.Millisecond
		t.Setenv("HELPER_ARTIFACTS", `[]`)
		t.Setenv("HELPER_CHECKS", `["first","second"]`)
		t.Setenv("HELPER_ADAPTER_STDOUT", strconv.Itoa(MaxCheckAdapterStdoutBytes+1))
		recordMark := filepath.Join(cfg.ProjectRoot, "check-records")
		pidPath := filepath.Join(cfg.ProjectRoot, "adapter-child.pid")
		t.Setenv("HELPER_CHECK_RECORD_MARK", recordMark)
		t.Setenv("HELPER_ADAPTER_CHILD", "first")
		t.Setenv("HELPER_ADAPTER_CHILD_PID", pidPath)
		t.Setenv("HELPER_ADAPTER_SLEEP", "first")
		tempRoot := t.TempDir()
		t.Setenv("TMPDIR", tempRoot)

		got := Run(context.Background(), cfg)
		if got.ExitCode != 5 || got.ResultV3 == nil || len(got.ResultV3.Checks) != 1 || got.ResultV3.Checks[0].Status != "failed" || got.ResultV3.Checks[0].Error == nil || got.ResultV3.Checks[0].Error.Category != "check_adapter_protocol" || got.ResultV3.Checks[0].Error.Code != "output_oversized" {
			t.Fatalf("Run=%#v", got)
		}
		assertNoPath(t, recordMark)
		assertEmptyDir(t, tempRoot)
		result, err := json.Marshal(got.ResultV3)
		if err != nil || bytes.Contains(result, bytes.Repeat([]byte("a"), 128)) {
			t.Fatalf("runtime result exposed adapter stdout: %v %s", err, result)
		}
		assertProcessGone(t, pidPath)
	})

	t.Run("adapter stderr tail boundaries and chunks", func(t *testing.T) {
		for _, tt := range []struct {
			name      string
			size      int
			truncated bool
		}{
			{"below", MaxCheckAdapterStderrTailBytes - 1, false},
			{"exact", MaxCheckAdapterStderrTailBytes, false},
			{"plus one", MaxCheckAdapterStderrTailBytes + 1, true},
			{"large", MaxCheckAdapterStderrTailBytes*4 + 17, true},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Setenv("HELPER_BOUNDED_STDERR", strconv.Itoa(tt.size))
				got := runStreamProcess(context.Background(), t.TempDir(), os.Args[0], []string{"bounded"}, nil, MaxCheckAdapterStdoutBytes, MaxCheckAdapterStderrTailBytes, 0, 0)
				if got.startErr != nil || got.waitErr != nil || got.stderrTruncated != tt.truncated || len(got.stderr) != min(tt.size, MaxCheckAdapterStderrTailBytes) || !bytes.Equal(got.stderr, bytes.Repeat([]byte("e"), len(got.stderr))) {
					t.Fatalf("size=%d result=%#v", tt.size, got)
				}
			})
		}
	})

	t.Run("cancellation wins over stdout overflow", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		t.Setenv("HELPER_BOUNDED_STDOUT", strconv.Itoa(MaxCheckRequestBytes+1))
		got := runStreamProcess(ctx, t.TempDir(), os.Args[0], []string{"bounded"}, nil, MaxCheckRequestBytes, MaxCheckAdapterStderrTailBytes, time.Hour, 0)
		if !got.cancelled || got.timedOut || got.overflow || got.exitCode != nil {
			t.Fatalf("result=%#v", got)
		}
	})
}

func assertNoPath(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected path %q: %v", path, err)
	}
}

func assertEmptyDir(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temp entries=%v err=%v", entries, err)
	}
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

const integrationCheckFlow = `flow: {
  id: "check-runtime-review"
  title: "Check runtime review"
  description: "Check runtime integration fixture."
  steps: [{
    id: "build"
    title: "Build"
    instruction: "Build artifacts."
    artifacts: [{path: "out/a.txt", required: true}, {path: "out/b.txt", required: false}]
    required_checks: ["check-a", "check-b"]
  }, {
    id: "verify"
    title: "Verify"
    instruction: "Verify the build."
  }]
}`

func TestIntegrationRealDevflowCheckMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("devflow call marker fixture is Unix-specific")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	devflow := filepath.Join(binDir, "devflow")
	runner := filepath.Join(binDir, "devflow-runner")
	if output, err := buildCheckIntegrationBinaries(repositoryRoot, devflow, runner); err != nil {
		t.Fatalf("build binaries: %v: %s", err, output)
	}

	t.Run("completed records checks in Flow order", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		result := runCheckRunner(t, runner, fixture)
		assertCheckRun(t, result, []string{"check-a", "check-b"}, []bool{true, true}, []string{"recorded", "recorded"})
		if len(result.ResultV3.Artifacts) != 1 || result.ResultV3.Artifacts[0].Path != "out/a.txt" || artifactEvidence(t, fixture.statePath, fixture.attemptID)["out/a.txt"] == nil {
			t.Fatalf("artifacts=%#v evidence=%#v", result.ResultV3.Artifacts, artifactEvidence(t, fixture.statePath, fixture.attemptID))
		}
		assertCheckState(t, fixture, []string{"check-a", "check-b"})
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a", "check-b"})
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check record", "check request", "check record"})
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check record", "check request", "check record"})
		assertMarkLines(t, fixture.completionMark, nil)
	})

	t.Run("failed domain check continues to following check", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_CHECK_FAILED", "check-a")
		before := checkInvariant(t, fixture)
		result := runCheckRunner(t, runner, fixture)
		assertCheckRun(t, result, []string{"check-a", "check-b"}, []bool{false, true}, []string{"recorded", "recorded"})
		assertCheckState(t, fixture, []string{"check-a", "check-b"})
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a", "check-b"})
	})

	t.Run("blocked report does not run checks", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_OUTCOME", "blocked")
		t.Setenv("HELPER_REFS", `["out/b.txt"]`)
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A",%q:"B"}`, filepath.Join(fixture.root, "out", "a.txt"), filepath.Join(fixture.root, "out", "b.txt")))
		before := checkInvariant(t, fixture)
		result := runCheckRunner(t, runner, fixture)
		if result.ExitCode != 0 || result.ResultV3.Status != "stopped" || len(result.ResultV3.Checks) != 0 {
			t.Fatalf("Run=%#v", result)
		}
		if len(result.ResultV3.Artifacts) != 1 || result.ResultV3.Artifacts[0].Path != "out/b.txt" || artifactEvidence(t, fixture.statePath, fixture.attemptID)["out/b.txt"] == nil {
			t.Fatalf("blocked artifacts=%#v evidence=%#v", result.ResultV3.Artifacts, artifactEvidence(t, fixture.statePath, fixture.attemptID))
		}
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, nil)
		assertMarkLines(t, fixture.cliMark, nil)
	})

	t.Run("failed report does not run artifacts or checks", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_OUTCOME", "failed")
		before := checkInvariant(t, fixture)
		result := runCheckRunner(t, runner, fixture)
		if result.ExitCode != 0 || result.ResultV3.Status != "stopped" || len(result.ResultV3.Artifacts) != 0 || len(result.ResultV3.Checks) != 0 {
			t.Fatalf("Run=%#v", result)
		}
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, nil)
		assertMarkLines(t, fixture.cliMark, nil)
	})

	t.Run("same attempt rerun skips already recorded checks", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		first := runCheckRunner(t, runner, fixture)
		assertCheckRun(t, first, []string{"check-a", "check-b"}, []bool{true, true}, []string{"recorded", "recorded"})
		stateAfterFirst, err := os.ReadFile(fixture.statePath)
		if err != nil {
			t.Fatal(err)
		}
		reportAfterFirst, err := os.ReadFile(fixture.reportPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(fixture.adapterMark); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.Remove(fixture.cliMark); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		t.Setenv("HELPER_ALREADY", "check-a")
		second := runCheckRunner(t, runner, fixture)
		if second.ExitCode != 0 || second.ResultV3.Status != "stopped" || !second.ResultV3.ReportIdempotent || len(second.ResultV3.Checks) != 2 {
			t.Fatalf("second=%#v", second)
		}
		for _, item := range second.ResultV3.Checks {
			if item.Status != "already_recorded" || item.Passed != nil || item.CheckExitCode != nil || item.AdapterExitCode != nil || item.Error != nil {
				t.Fatalf("already recorded item=%#v", item)
			}
		}
		stateAfterSecond, _ := os.ReadFile(fixture.statePath)
		reportAfterSecond, _ := os.ReadFile(fixture.reportPath)
		if !bytes.Equal(stateAfterFirst, stateAfterSecond) || !bytes.Equal(reportAfterFirst, reportAfterSecond) {
			t.Fatal("rerun changed persisted CheckResult or Report")
		}
		assertMarkLines(t, fixture.adapterMark, nil)
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check request"})
	})
}

const integrationCompletionApprovalFlow = `flow: {
  id: "check-runtime-review"
  title: "Check runtime review"
  steps: [{
    id: "build"
    title: "Build"
    instruction: "Build artifacts."
    artifacts: [{path: "out/a.txt", required: true}]
    approval: {required: true}
    required_checks: ["check-a", "check-b"]
  }, {
    id: "verify"
    title: "Verify"
    instruction: "Verify the build."
  }]
}`

func TestIntegrationRealDevflowCompletionContextMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("devflow call marker fixture is Unix-specific")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	devflow := filepath.Join(binDir, "devflow")
	runner := filepath.Join(binDir, "devflow-runner")
	if output, err := buildCheckIntegrationBinaries(repositoryRoot, devflow, runner); err != nil {
		t.Fatalf("build binaries: %v: %s", err, output)
	}

	t.Run("ready without approval", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		got := runCompletionContextRunner(t, runner, fixture)
		assertCompletionResult(t, got, fixture, "ready", "", "not_required")
		if artifactEvidence(t, fixture.statePath, fixture.attemptID)["out/a.txt"] == nil {
			t.Fatal("required ArtifactEvidence was not recorded")
		}
		assertCheckState(t, fixture, []string{"check-a", "check-b"})
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.completionMark, []string{"completion-context --step build --attempt " + fixture.attemptID})
		assertNoCompletionTransition(t, fixture)

		direct := runCompletionContextCLI(t, fixture)
		if !bytes.Equal(got.CompletionContext, direct) {
			t.Fatalf("completion context differs on direct Core retrieval\nrunner=%s\ncore=%s", got.CompletionContext, direct)
		}
	})

	t.Run("pending approval", func(t *testing.T) {
		fixture := setupCheckIntegrationFlow(t, devflow, integrationCompletionApprovalFlow)
		before := checkInvariant(t, fixture)
		got := runCompletionContextRunner(t, runner, fixture)
		context := assertCompletionResult(t, got, fixture, "blocked", "missing_approval", "pending")
		approval := context["approval"].(map[string]any)
		if approval["evidence_set_digest"] == nil || approval["approved_evidence_set_digest"] != nil {
			t.Fatalf("approval digest=%#v", approval)
		}
		assertCheckState(t, fixture, []string{"check-a", "check-b"})
		assertCheckInvariant(t, fixture, before)
		assertNoCompletionTransition(t, fixture)
	})

	t.Run("valid failed check", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_CHECK_FAILED", "check-a")
		before := checkInvariant(t, fixture)
		got := runCompletionContextRunner(t, runner, fixture)
		context := assertCompletionResult(t, got, fixture, "blocked", "failed_check", "not_required")
		checks := context["checks"].([]any)
		if len(checks) != 2 || checks[0].(map[string]any)["id"] != "check-a" || checks[0].(map[string]any)["status"] != "failed" || checks[0].(map[string]any)["exit_code"] != float64(1) || checks[1].(map[string]any)["id"] != "check-b" || checks[1].(map[string]any)["status"] != "passed" || checks[1].(map[string]any)["exit_code"] != float64(0) {
			t.Fatalf("checks=%#v", checks)
		}
		blocker := context["completion"].(map[string]any)["blocker"].(map[string]any)
		if blocker["subject_id"] != "check-a" {
			t.Fatalf("blocker=%#v", blocker)
		}
		assertCheckState(t, fixture, []string{"check-a", "check-b"})
		assertCheckInvariant(t, fixture, before)
		assertNoCompletionTransition(t, fixture)
	})

	t.Run("blocked report still retrieves context", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_OUTCOME", "blocked")
		before := checkInvariant(t, fixture)
		got := runCompletionContextRunner(t, runner, fixture)
		if got.SchemaVersion != 4 || got.ReportOutcome != "blocked" || bytes.Equal(got.CompletionContext, []byte("null")) {
			t.Fatalf("result=%#v", got)
		}
		context := decodeCompletionContext(t, got.CompletionContext)
		if context["completion"].(map[string]any)["status"] != "blocked" {
			t.Fatalf("context=%#v", context)
		}
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertNoCompletionTransition(t, fixture)
	})
}

func runCompletionContextRunner(t *testing.T, runner string, fixture checkIntegration) ResultV4 {
	t.Helper()
	t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
	t.Setenv("HELPER_OUTCOME", envDefault("HELPER_OUTCOME", "completed"))
	if os.Getenv("HELPER_FILES") == "" {
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A"}`, filepath.Join(fixture.root, "out", "a.txt")))
	}
	t.Setenv("HELPER_ADAPTER_MARK", fixture.adapterMark)
	t.Setenv("DEVFLOW_CHECK_CLI_MARK", fixture.cliMark)
	t.Setenv("DEVFLOW_COMPLETION_CONTEXT_CLI_MARK", fixture.completionMark)
	t.Setenv("HELPER_REAL_DEVFLOW", fixture.coreDevflow)
	t.Setenv("HELPER_PROJECT_ROOT", fixture.root)
	args := []string{"execute", "--project-root", fixture.root, "--devflow", fixture.devflow, "--step", fixture.stepID, "--attempt", fixture.attemptID, "--record-artifacts", "--check-adapter", os.Args[0], "--check-adapter-arg", "adapter", "--completion-context", "--", os.Args[0]}
	cmd := exec.Command(runner, args...)
	cmd.Dir = fixture.root
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("runner: %v: %s", err, output)
	}
	var got ResultV4
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("runner JSON: %v: %s", err, output)
	}
	return got
}

func runCompletionContextCLI(t *testing.T, fixture checkIntegration) []byte {
	t.Helper()
	cmd := exec.Command(fixture.devflow, "completion-context", "--step", fixture.stepID, "--attempt", fixture.attemptID)
	cmd.Dir = fixture.root
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("completion-context: %v: %s", err, output)
	}
	return bytes.TrimSpace(output)
}

func decodeCompletionContext(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("completion context: %v: %s", err, raw)
	}
	return got
}

func assertCompletionResult(t *testing.T, got ResultV4, fixture checkIntegration, status, blockerCode, approvalStatus string) map[string]any {
	t.Helper()
	if got.SchemaVersion != 4 || got.FlowRunID != fixture.runID || got.StepID != fixture.stepID || got.AttemptID != fixture.attemptID || got.Error != nil || got.CompletionContext == nil {
		t.Fatalf("result=%#v", got)
	}
	context := decodeCompletionContext(t, got.CompletionContext)
	if context["schema_version"] != float64(1) || context["flow_run_id"] != fixture.runID || context["step_id"] != fixture.stepID || context["attempt_id"] != fixture.attemptID || context["attempt_status"] != "active" || context["is_current_attempt"] != true || context["approval"].(map[string]any)["status"] != approvalStatus || context["completion"].(map[string]any)["status"] != status {
		t.Fatalf("context=%#v", context)
	}
	blocker := context["completion"].(map[string]any)["blocker"]
	if blockerCode == "" {
		if blocker != nil {
			t.Fatalf("blocker=%#v", blocker)
		}
	} else if blocker.(map[string]any)["code"] != blockerCode {
		t.Fatalf("blocker=%#v", blocker)
	}
	return context
}

func assertNoCompletionTransition(t *testing.T, fixture checkIntegration) {
	t.Helper()
	state := readJSONMap(t, fixture.statePath)
	if state["current_step_id"] != fixture.stepID || state["current_attempt_id"] != fixture.attemptID || len(state["attempts"].([]any)) != 1 {
		t.Fatalf("state advanced: %#v", state)
	}
	attempt := checkAttempt(t, fixture)
	if attempt["status"] != "active" || attempt["approval"] != nil {
		t.Fatalf("attempt transitioned: %#v", attempt)
	}
}

func TestIntegrationRealDevflowCheckModeFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("devflow call marker fixture is Unix-specific")
	}
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	devflow := filepath.Join(binDir, "devflow")
	runner := filepath.Join(binDir, "devflow-runner")
	if output, err := buildCheckIntegrationBinaries(repositoryRoot, devflow, runner); err != nil {
		t.Fatalf("build binaries: %v: %s", err, output)
	}

	t.Run("adapter nonzero preserves earlier CheckResult", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		t.Setenv("HELPER_CHECKS", `["check-a","check-b","check-c"]`)
		t.Setenv("HELPER_ADAPTER_NONZERO", "check-b")
		result := runCheckRunner(t, runner, fixture)
		assertCheckFailure(t, result, 4, "failed", []string{"check-a", "check-b"}, []string{"recorded", "failed"}, "check_adapter_process")
		assertCheckState(t, fixture, []string{"check-a"})
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a", "check-b"})
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check record", "check request"})
	})

	t.Run("malformed adapter CheckRecord stops before Core record", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		t.Setenv("HELPER_CHECKS", `["check-a","check-b"]`)
		t.Setenv("HELPER_ADAPTER_MALFORMED", "check-a")
		result := runCheckRunner(t, runner, fixture)
		assertCheckFailure(t, result, 5, "failed", []string{"check-a"}, []string{"failed"}, "check_adapter_protocol")
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a"})
		assertMarkLines(t, fixture.cliMark, []string{"check request"})
	})

	t.Run("adapter timeout terminates process group and stops following checks", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		pidPath := filepath.Join(fixture.root, "adapter-child.pid")
		t.Setenv("HELPER_CHECKS", `["check-a","check-b"]`)
		t.Setenv("HELPER_ADAPTER_SLEEP", "check-a")
		t.Setenv("HELPER_ADAPTER_CHILD", "check-a")
		t.Setenv("HELPER_ADAPTER_CHILD_PID", pidPath)
		result := runCheckRunnerArgs(t, runner, fixture, "--check-timeout", "50ms", "--check-terminate-grace", "50ms")
		assertCheckFailure(t, result, 124, "timed_out", []string{"check-a"}, []string{"failed"}, "check_adapter_process")
		if result.ResultV3.Checks[0].AdapterExitCode != nil {
			t.Fatalf("adapter_exit_code=%v, want nil", *result.ResultV3.Checks[0].AdapterExitCode)
		}
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a"})
		assertMarkLines(t, fixture.cliMark, []string{"check request"})
		assertProcessGone(t, pidPath)
	})

	t.Run("stale Attempt is rejected by Core without reusing CheckResult", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		t.Setenv("HELPER_CHECKS", `["check-a","check-b"]`)
		t.Setenv("HELPER_CLOSE_ATTEMPT", "check-a")
		result := runCheckRunner(t, runner, fixture)
		assertCheckFailure(t, result, 3, "failed", []string{"check-a"}, []string{"failed"}, "check_record")
		if result.ResultV3.Checks[0].Error.Code != "error_stale_attempt" {
			t.Fatalf("stale diagnostic=%#v", result.ResultV3.Checks[0].Error)
		}
		state := readJSONMap(t, fixture.statePath)
		for _, raw := range state["attempts"].([]any) {
			attempt := raw.(map[string]any)
			if attempt["id"] == fixture.attemptID && len(attempt["check_results"].(map[string]any)) != 0 {
				t.Fatalf("stale Attempt got CheckResult: %#v", attempt)
			}
		}
		if _, err := os.Stat(fixture.reportPath); err != nil {
			t.Fatalf("Report was not saved: %v", err)
		}
		if artifactEvidence(t, fixture.statePath, fixture.attemptID)["out/a.txt"] == nil {
			t.Fatal("ArtifactEvidence was not saved")
		}
		assertMarkLines(t, fixture.adapterMark, []string{"check-a"})
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check record"})
	})

	t.Run("independent Core check record failure stops following checks", func(t *testing.T) {
		fixture := setupCheckIntegration(t, devflow)
		before := checkInvariant(t, fixture)
		t.Setenv("HELPER_CHECKS", `["check-a","check-b"]`)
		t.Setenv("HELPER_CHECK_RECORD_FAIL", "1")
		result := runCheckRunner(t, runner, fixture)
		assertCheckFailure(t, result, 3, "failed", []string{"check-a"}, []string{"failed"}, "check_record")
		assertNoCheckState(t, fixture)
		assertCheckInvariant(t, fixture, before)
		assertMarkLines(t, fixture.adapterMark, []string{"check-a"})
		assertMarkLines(t, fixture.cliMark, []string{"check request", "check record"})
	})
}

func buildCheckIntegrationBinaries(repositoryRoot, devflow, runner string) (string, error) {
	for output, target := range map[string]string{devflow: "./cmd/devflow", runner: "./cmd/devflow-runner"} {
		cmd := exec.Command("go", "build", "-o", output, target)
		cmd.Dir = repositoryRoot
		if result, err := cmd.CombinedOutput(); err != nil {
			return string(result), err
		}
	}
	return "", nil
}

type checkIntegration struct {
	root, devflow, coreDevflow, runID, stepID, attemptID, statePath, currentPath, reportPath, adapterMark, cliMark, completionMark string
}

func setupCheckIntegration(t *testing.T, devflow string) checkIntegration {
	return setupCheckIntegrationFlow(t, devflow, integrationCheckFlow)
}

func setupCheckIntegrationFlow(t *testing.T, devflow, flow string) checkIntegration {
	t.Helper()
	root := t.TempDir()
	runCLI := func(args ...string) {
		t.Helper()
		cmd := exec.Command(devflow, args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("devflow %v: %v: %s", args, err, output)
		}
	}
	runCLI("init")
	if err := os.WriteFile(filepath.Join(root, ".devflow", "flows", "check-runtime-review.cue"), []byte(flow), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "task.md"), []byte("Build artifacts.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runCLI("start", "check-runtime-review", "--task-file", "task.md")
	currentPath := filepath.Join(root, ".devflow", "current.json")
	current := readJSONMap(t, currentPath)
	runID := current["flow_run_id"].(string)
	statePath := filepath.Join(root, ".devflow", "runs", runID, "state.json")
	state := readJSONMap(t, statePath)
	stepID := state["current_step_id"].(string)
	attemptID := state["current_attempt_id"].(string)
	cliMark := filepath.Join(root, "check-cli-calls")
	completionMark := filepath.Join(root, "completion-context-cli-calls")
	wrapper := filepath.Join(root, "devflow-wrapper")
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = check ]; then printf '%%s %%s\\n' \"$1\" \"$2\" >> \"$DEVFLOW_CHECK_CLI_MARK\"; fi\nif [ \"$1\" = completion-context ]; then printf '%%s %%s %%s %%s %%s\\n' \"$1\" \"$2\" \"$3\" \"$4\" \"$5\" >> \"$DEVFLOW_COMPLETION_CONTEXT_CLI_MARK\"; fi\nif [ \"$1\" = check ] && [ \"$2\" = record ] && [ -n \"$HELPER_CHECK_RECORD_FAIL\" ]; then echo 'error: error_check_record_fixture' >&2; exit 1; fi\nexec %q \"$@\"\n", devflow)
	if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return checkIntegration{root: root, devflow: wrapper, coreDevflow: devflow, runID: runID, stepID: stepID, attemptID: attemptID, statePath: statePath, currentPath: currentPath, reportPath: filepath.Join(root, ".devflow", "runs", runID, "execution-reports", attemptID+".json"), adapterMark: filepath.Join(root, "adapter-calls"), cliMark: cliMark, completionMark: completionMark}
}

func runCheckRunner(t *testing.T, runner string, fixture checkIntegration) RunResult {
	return runCheckRunnerArgs(t, runner, fixture)
}

func runCheckRunnerArgs(t *testing.T, runner string, fixture checkIntegration, extra ...string) RunResult {
	t.Helper()
	t.Setenv("DEVFLOW_RUNTIME_HELPER", "1")
	t.Setenv("HELPER_OUTCOME", envDefault("HELPER_OUTCOME", "completed"))
	if os.Getenv("HELPER_FILES") == "" {
		t.Setenv("HELPER_FILES", fmt.Sprintf(`{%q:"A"}`, filepath.Join(fixture.root, "out", "a.txt")))
	}
	t.Setenv("HELPER_ADAPTER_MARK", fixture.adapterMark)
	t.Setenv("DEVFLOW_CHECK_CLI_MARK", fixture.cliMark)
	t.Setenv("DEVFLOW_COMPLETION_CONTEXT_CLI_MARK", fixture.completionMark)
	t.Setenv("HELPER_REAL_DEVFLOW", fixture.coreDevflow)
	t.Setenv("HELPER_PROJECT_ROOT", fixture.root)
	args := []string{"execute", "--project-root", fixture.root, "--devflow", fixture.devflow, "--step", fixture.stepID, "--attempt", fixture.attemptID, "--record-artifacts", "--check-adapter", os.Args[0], "--check-adapter-arg", "adapter"}
	args = append(args, extra...)
	args = append(args, "--", os.Args[0])
	cmd := exec.Command(runner, args...)
	cmd.Dir = fixture.root
	output, err := cmd.Output()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("runner: %v: %s", err, output)
		}
		exitCode = exitErr.ExitCode()
	}
	var result ResultV3
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("runner: %v: %s", err, output)
	}
	return RunResult{ResultV3: &result, ExitCode: exitCode}
}

func assertCheckFailure(t *testing.T, got RunResult, exitCode int, status string, ids, statuses []string, category string) {
	t.Helper()
	if got.ExitCode != exitCode || got.ResultV3 == nil || got.ResultV3.Status != status || len(got.ResultV3.Checks) != len(ids) {
		t.Fatalf("Run=%#v", got)
	}
	for i, item := range got.ResultV3.Checks {
		if item.CheckID != ids[i] || item.Status != statuses[i] {
			t.Fatalf("check[%d]=%#v", i, item)
		}
		if item.Status == "failed" && (item.Error == nil || item.Error.Category != category) {
			t.Fatalf("failed check[%d]=%#v", i, item)
		}
	}
}

func assertProcessGone(t *testing.T, pidPath string) {
	t.Helper()
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("adapter child pid: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("adapter child process %d survived timeout: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertCheckRun(t *testing.T, got RunResult, ids []string, passed []bool, statuses []string) {
	t.Helper()
	if got.ExitCode != 0 || got.ResultV3.SchemaVersion != 3 || got.ResultV3.Status != "stopped" || got.ResultV3.Error != nil || len(got.ResultV3.Checks) != len(ids) {
		t.Fatalf("Run=%#v", got)
	}
	for i, item := range got.ResultV3.Checks {
		wantExitCode := 1
		if passed[i] {
			wantExitCode = 0
		}
		if item.CheckID != ids[i] || item.Status != statuses[i] || item.Passed == nil || *item.Passed != passed[i] || item.CheckExitCode == nil || *item.CheckExitCode != wantExitCode || item.AdapterExitCode == nil || *item.AdapterExitCode != 0 || item.Error != nil {
			t.Fatalf("check[%d]=%#v", i, item)
		}
	}
}

func assertCheckState(t *testing.T, fixture checkIntegration, ids []string) {
	t.Helper()
	attempt := checkAttempt(t, fixture)
	results := attempt["check_results"].(map[string]any)
	if len(results) != len(ids) {
		t.Fatalf("check results=%#v", results)
	}
	for _, id := range ids {
		if results[id] == nil {
			t.Fatalf("missing check result %q: %#v", id, results)
		}
	}
}

func assertNoCheckState(t *testing.T, fixture checkIntegration) {
	t.Helper()
	if results := checkAttempt(t, fixture)["check_results"].(map[string]any); len(results) != 0 {
		t.Fatalf("unexpected CheckResult=%#v", results)
	}
}

func checkAttempt(t *testing.T, fixture checkIntegration) map[string]any {
	t.Helper()
	state := readJSONMap(t, fixture.statePath)
	if state["current_attempt_id"] != fixture.attemptID || state["current_step_id"] != fixture.stepID {
		t.Fatalf("current state changed: %#v", state)
	}
	attempts := state["attempts"].([]any)
	if len(attempts) != 1 {
		t.Fatalf("unexpected next Attempt: %#v", attempts)
	}
	for _, raw := range attempts {
		attempt := raw.(map[string]any)
		if attempt["id"] == fixture.attemptID {
			if attempt["status"] != "active" || attempt["approval"] != nil {
				t.Fatalf("attempt changed: %#v", attempt)
			}
			return attempt
		}
	}
	t.Fatalf("attempt %q missing", fixture.attemptID)
	return nil
}

type checkInvariants struct{ current, report, flowSnapshot, taskSnapshot []byte }

func checkInvariant(t *testing.T, fixture checkIntegration) checkInvariants {
	t.Helper()
	current, err := os.ReadFile(fixture.currentPath)
	if err != nil {
		t.Fatal(err)
	}
	state := readJSONMap(t, fixture.statePath)
	flowSnapshot, _ := json.Marshal(state["flow_snapshot"])
	taskSnapshot, _ := json.Marshal(state["task_snapshot"])
	return checkInvariants{current: current, flowSnapshot: flowSnapshot, taskSnapshot: taskSnapshot}
}

func assertCheckInvariant(t *testing.T, fixture checkIntegration, before checkInvariants) {
	t.Helper()
	after := checkInvariant(t, fixture)
	if !bytes.Equal(before.current, after.current) || !bytes.Equal(before.flowSnapshot, after.flowSnapshot) || !bytes.Equal(before.taskSnapshot, after.taskSnapshot) {
		t.Fatal("current pointer or snapshots changed")
	}
	if _, err := os.Stat(fixture.reportPath); err != nil {
		t.Fatalf("Report was not saved: %v", err)
	}
}

func assertMarkLines(t *testing.T, path string, want []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && len(want) == 0 {
		return
	}
	got := []string(nil)
	if err == nil {
		got = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mark %s=%q err=%v want=%v", path, data, err, want)
	}
}
