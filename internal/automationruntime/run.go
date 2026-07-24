package automationruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func Run(ctx context.Context, cfg Config) RunResult {
	return runWithFileOps(ctx, cfg, os.CreateTemp, os.Remove)
}

func runWithFileOps(
	ctx context.Context,
	cfg Config,
	createTemp func(string, string) (*os.File, error),
	remove func(string) error,
) RunResult {
	result := baseResult(cfg)
	root, err := filepath.Abs(cfg.ProjectRoot)
	if err != nil {
		return fail(result, "runtime_io", "invalid_project_root", 6)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return fail(result, "runtime_io", "invalid_project_root", 6)
	}
	cfg.ProjectRoot = root
	if ctx.Err() != nil {
		result.Status = "cancelled"
		return fail(result, "executor_process", "cancelled", 130)
	}

	wp := runCaptured(ctx, root, cfg.Devflow, []string{"work-package", "--step", cfg.StepID, "--attempt", cfg.AttemptID}, MaxWorkPackageBytes)
	if ctx.Err() != nil {
		result.Status = "cancelled"
		return fail(result, "devflow_process", "cancelled", 130)
	}
	if wp.overflow {
		return fail(result, "devflow_process", "oversized_output", 3)
	}
	if wp.startErr != nil {
		return fail(result, "devflow_process", "start_failed", 3)
	}
	if wp.waitErr != nil {
		code := stableDiagnosticCode(wp.stderr)
		if code == "" {
			code = "work_package_failed"
		}
		return fail(result, "devflow_contract", code, 3)
	}
	pkg, err := parseWorkPackage(wp.stdout, cfg.StepID, cfg.AttemptID)
	if err != nil {
		return fail(result, "devflow_process", "invalid_output", 3)
	}
	result.FlowRunID, result.WorkPackageDigest = pkg.FlowRunID, pkg.WorkPackageDigest

	executor := runExecutor(ctx, cfg, wp.stdout)
	result.ExecutorExitCode = executor.exitCode
	result.StderrTruncated = executor.stderrTruncated
	if executor.cancelled || ctx.Err() != nil {
		result.Status = "cancelled"
		return fail(result, "executor_process", "cancelled", 130)
	}
	if executor.timedOut {
		result.Status = "timed_out"
		return fail(result, "executor_process", "timeout", 124)
	}
	if executor.overflow {
		return fail(result, "executor_protocol", "oversized_stdout", 5)
	}
	if executor.startErr != nil {
		return fail(result, "executor_process", "start_failed", 4)
	}
	if executor.waitErr != nil {
		code := "nonzero_exit"
		if executor.signaled {
			code = "signaled"
		}
		return fail(result, "executor_process", code, 4)
	}
	if executor.stdoutErr != nil || executor.stderrErr != nil {
		return fail(result, "runtime_io", "process_output_read_failed", 6)
	}
	if executor.stdinErr != nil {
		return fail(result, "runtime_io", "stdin_write_failed", 6)
	}
	if len(executor.stdout) == 0 {
		return fail(result, "executor_protocol", "empty_stdout", 5)
	}
	report, err := parseReportHeader(executor.stdout, pkg)
	if err != nil {
		if errors.Is(err, errReportIdentityMismatch) {
			return fail(result, "executor_protocol", "report_identity_mismatch", 5)
		}
		return fail(result, "executor_protocol", "invalid_report_output", 5)
	}
	tmp, err := createTemp("", "devflow-execution-report-*.json")
	if err != nil {
		return fail(result, "runtime_io", "temporary_report_failed", 6)
	}
	tmpPath := tmp.Name()
	cleanup := func() error { return remove(tmpPath) }
	chmodErr := tmp.Chmod(0o600)
	written, writeErr := tmp.Write(executor.stdout)
	closeErr := tmp.Close()
	if chmodErr != nil || writeErr != nil || written != len(executor.stdout) || closeErr != nil {
		_ = cleanup()
		return fail(result, "runtime_io", "temporary_report_failed", 6)
	}

	record := runCaptured(ctx, root, cfg.Devflow, []string{"execution-report", "record", "--file", tmpPath}, MaxExecutorStdoutBytes)
	if ctx.Err() != nil {
		_ = cleanup()
		result.Status = "cancelled"
		return fail(result, "devflow_process", "cancelled", 130)
	}
	if record.startErr != nil {
		_ = cleanup()
		return fail(result, "devflow_process", "start_failed", 3)
	}
	if record.overflow {
		_ = cleanup()
		return fail(result, "devflow_process", "oversized_output", 3)
	}
	if record.waitErr != nil {
		_ = cleanup()
		code := stableDiagnosticCode(record.stderr)
		if code == "" {
			code = "report_record_failed"
		}
		return fail(result, "devflow_contract", code, 3)
	}
	parsed, err := parseRecordOutput(record.stdout, pkg)
	if err != nil {
		_ = cleanup()
		return fail(result, "executor_protocol", "invalid_record_output", 5)
	}
	if parsed.Outcome != report.Outcome {
		_ = cleanup()
		return fail(result, "executor_protocol", "invalid_record_output", 5)
	}
	result.Status = "recorded"
	result.ExecutionReportDigest = parsed.ExecutionReportDigest
	result.ReportOutcome = parsed.Outcome
	result.ReportIdempotent = parsed.Idempotent
	result.Error = nil
	runResult := RunResult{Result: result, ExitCode: 0}
	if err := cleanup(); err != nil {
		runResult.CleanupCode = "temporary_report_cleanup_failed"
	}
	return runResult
}
