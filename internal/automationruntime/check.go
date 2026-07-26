package automationruntime

import (
	"context"
	"errors"
	"os"
)

const alreadyRecordedDiagnostic = "error_check_result_already_recorded"

func runChecks(
	ctx context.Context,
	cfg Config,
	pkg workPackageHeader,
	base Result,
	artifacts []ArtifactResult,
	createTemp func(string, string) (*os.File, error),
	remove func(string) error,
) RunResult {
	checks := make([]CheckItem, 0, len(pkg.RequiredChecks))
	for _, checkID := range pkg.RequiredChecks {
		item := CheckItem{CheckID: checkID}
		request := runStreamProcess(ctx, cfg.ProjectRoot, cfg.Devflow, []string{
			"check", "request", "--step", cfg.StepID, "--attempt", cfg.AttemptID, "--check", checkID,
		}, nil, MaxCheckRequestBytes, MaxExecutorStderrTailBytes, 0, cfg.CheckTerminateGrace)
		if ctx.Err() != nil {
			base.Status = "cancelled"
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_request", "request_failed", nil)), 130)
		}
		if request.overflow {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_request", "request_output_oversized", nil)), 3)
		}
		if request.startErr != nil {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_request", "request_failed", nil)), 3)
		}
		if request.waitErr != nil {
			if stableDiagnosticCode(request.stderr) == alreadyRecordedDiagnostic {
				item.Status = "already_recorded"
				checks = append(checks, item)
				continue
			}
			code := stableDiagnosticCode(request.stderr)
			if code == "" {
				code = "request_failed"
			}
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_request", code, request.exitCode)), 3)
		}
		if err := validateCheckRequest(request.stdout, pkg, checkID); err != nil {
			code := "invalid_request_output"
			if errors.Is(err, errCheckIdentityMismatch) {
				code = "identity_mismatch"
			}
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_request", code, request.exitCode)), 3)
		}

		adapter := runCheckAdapter(ctx, cfg, request.stdout)
		item.StderrTruncated = adapter.stderrTruncated
		if adapter.cancelled || ctx.Err() != nil {
			base.Status = "cancelled"
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_process", "cancelled", nil)), 130)
		}
		if adapter.timedOut {
			base.Status = "timed_out"
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_process", "timeout", nil)), 124)
		}
		if adapter.startErr != nil {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_process", "start_failed", nil)), 4)
		}
		if adapter.overflow {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_protocol", "output_oversized", nil)), 5)
		}
		if adapter.waitErr != nil {
			code := "nonzero_exit"
			if adapter.signaled {
				code = "signaled"
			}
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_process", code, adapter.exitCode)), 4)
		}
		if adapter.stdoutErr != nil || adapter.stderrErr != nil || adapter.stdinErr != nil {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "runtime_io", "process_output_read_failed", adapter.exitCode)), 6)
		}
		if len(adapter.stdout) == 0 {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_protocol", "empty_output", adapter.exitCode)), 5)
		}
		record, err := parseCheckRecord(adapter.stdout, pkg, checkID, cfg.ProjectRoot)
		if err != nil {
			code := "invalid_record_output"
			if errors.Is(err, errCheckIdentityMismatch) {
				code = "identity_mismatch"
			} else if errors.Is(err, errInvalidLogPath) {
				code = "invalid_log_path"
			}
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_adapter_protocol", code, adapter.exitCode)), 5)
		}
		item.CheckExitCode = intPointer(record.ExitCode)
		item.AdapterExitCode = intPointer(0)
		temp, err := createTemp("", "devflow-check-record-*.json")
		if err != nil {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "runtime_io", "temporary_check_record_failed", adapter.exitCode)), 6)
		}
		tempPath := temp.Name()
		chmodErr := temp.Chmod(0o600)
		written, writeErr := temp.Write(adapter.stdout)
		closeErr := temp.Close()
		if chmodErr != nil || writeErr != nil || written != len(adapter.stdout) || closeErr != nil {
			_ = remove(tempPath)
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "runtime_io", "temporary_check_record_failed", adapter.exitCode)), 6)
		}
		recorded := runStreamProcess(ctx, cfg.ProjectRoot, cfg.Devflow,
			[]string{"check", "record", "--file", tempPath}, nil, MaxExecutorStdoutBytes,
			MaxExecutorStderrTailBytes, 0, cfg.CheckTerminateGrace)
		removeErr := remove(tempPath)
		if ctx.Err() != nil {
			base.Status = "cancelled"
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_record", "record_failed", intPointer(0))), 130)
		}
		if recorded.startErr != nil || recorded.waitErr != nil || recorded.overflow {
			code := stableDiagnosticCode(recorded.stderr)
			if code == "" {
				code = "record_failed"
			}
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "check_record", code, intPointer(0))), 3)
		}
		if err := parseCheckRecordSuccess(recorded.stdout, record); err != nil {
			item.Status = "unknown"
			item.Error = &ErrorInfo{Category: "check_record", Code: "invalid_success_output"}
			checks = append(checks, item)
			run := fail(base, item.Error.Category, item.Error.Code, 3)
			return finishV3(run, artifacts, checks)
		}
		if removeErr != nil {
			return checkFailure(base, artifacts, append(checks, failedCheck(item, "runtime_io", "temporary_check_record_cleanup_failed", adapter.exitCode)), 6)
		}
		passed := record.ExitCode == 0
		item.Status = "recorded"
		item.Passed = &passed
		item.Error = nil
		checks = append(checks, item)
	}
	base.Status = "stopped"
	base.Error = nil
	return finishV3(RunResult{Result: base, ExitCode: 0}, artifacts, checks)
}

func failedCheck(item CheckItem, category, code string, adapterExitCode *int) CheckItem {
	item.Status = "failed"
	item.AdapterExitCode = adapterExitCode
	item.Error = &ErrorInfo{Category: category, Code: code}
	return item
}

func checkFailure(base Result, artifacts []ArtifactResult, checks []CheckItem, exitCode int) RunResult {
	last := checks[len(checks)-1]
	run := fail(base, last.Error.Category, last.Error.Code, exitCode)
	return finishV3(run, artifacts, checks)
}

func intPointer(value int) *int { return &value }
