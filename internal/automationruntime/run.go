package automationruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func Run(ctx context.Context, cfg Config) RunResult {
	run := runWithFileOps(ctx, cfg, os.CreateTemp, os.Remove)
	if cfg.CheckAdapter != "" && run.ResultV3 == nil {
		var artifacts []ArtifactResult
		if run.ResultV2 != nil {
			artifacts = run.ResultV2.Artifacts
		}
		return finishV3(run, artifacts, nil)
	}
	return finishV2(run, cfg, nil)
}

func finishV2(run RunResult, cfg Config, artifacts []ArtifactResult) RunResult {
	if !cfg.RecordArtifacts {
		return run
	}
	if run.ResultV2 != nil {
		return run
	}
	status := run.Result.Status
	if run.ExitCode == 0 {
		status = "stopped"
	} else if status != "timed_out" && status != "cancelled" {
		status = "failed"
	}
	if artifacts == nil {
		artifacts = []ArtifactResult{}
	}
	run.ResultV2 = &ResultV2{
		SchemaVersion: 2, Status: status, FlowRunID: run.Result.FlowRunID,
		StepID: run.Result.StepID, AttemptID: run.Result.AttemptID,
		WorkPackageDigest:     run.Result.WorkPackageDigest,
		ExecutionReportDigest: run.Result.ExecutionReportDigest,
		ReportOutcome:         run.Result.ReportOutcome, ReportIdempotent: run.Result.ReportIdempotent,
		ExecutorExitCode: run.Result.ExecutorExitCode, StderrTruncated: run.Result.StderrTruncated,
		Artifacts: artifacts, Error: run.Result.Error,
	}
	return run
}

func finishV3(run RunResult, artifacts []ArtifactResult, checks []CheckItem) RunResult {
	status := run.Result.Status
	if run.ExitCode == 0 {
		status = "stopped"
	} else if status != "timed_out" && status != "cancelled" {
		status = "failed"
	}
	if artifacts == nil {
		artifacts = []ArtifactResult{}
	}
	if checks == nil {
		checks = []CheckItem{}
	}
	run.ResultV3 = &ResultV3{
		SchemaVersion: 3, Status: status, FlowRunID: run.Result.FlowRunID,
		StepID: run.Result.StepID, AttemptID: run.Result.AttemptID,
		WorkPackageDigest: run.Result.WorkPackageDigest, ExecutionReportDigest: run.Result.ExecutionReportDigest,
		ReportOutcome: run.Result.ReportOutcome, ReportIdempotent: run.Result.ReportIdempotent,
		ExecutorExitCode: run.Result.ExecutorExitCode, StderrTruncated: run.Result.StderrTruncated,
		Artifacts: artifacts, Checks: checks, Error: run.Result.Error,
	}
	return run
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
	pkg, err := parseWorkPackageForMode(wp.stdout, cfg.StepID, cfg.AttemptID, cfg.RecordArtifacts, cfg.CheckAdapter != "")
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
	report, err := parseReportHeaderForMode(executor.stdout, pkg, cfg.RecordArtifacts)
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
	if cfg.RecordArtifacts {
		targets, resolveErr := ResolveArtifactTargets(report.Outcome, pkg.Artifacts, report.ArtifactRefs)
		if resolveErr != nil {
			failed := finishV2(fail(result, "artifact_contract", "invalid_artifact_targets", 3), cfg, nil)
			if err := cleanup(); err != nil {
				failed.CleanupCode = "temporary_report_cleanup_failed"
			}
			return failed
		}
		artifactResults := make([]ArtifactResult, 0, len(targets))
		for _, target := range targets {
			recorded := runCaptured(ctx, root, cfg.Devflow, []string{
				"artifact", "record", "--step", cfg.StepID, "--attempt", cfg.AttemptID, "--path", target.Path,
			}, MaxExecutorStdoutBytes)
			item := ArtifactResult{Path: target.Path, Required: target.Required}
			if ctx.Err() != nil {
				item.Status = "failed"
				item.Error = &ErrorInfo{Category: "devflow_process", Code: "cancelled"}
				artifactResults = append(artifactResults, item)
				result.Status = "cancelled"
				failed := finishV2(fail(result, "devflow_process", "cancelled", 130), cfg, artifactResults)
				if err := cleanup(); err != nil {
					failed.CleanupCode = "temporary_report_cleanup_failed"
				}
				return failed
			}
			if recorded.startErr != nil {
				item.Status = "failed"
				item.Error = &ErrorInfo{Category: "devflow_process", Code: "start_failed"}
			} else if recorded.overflow {
				item.Status = "unknown"
				item.Error = &ErrorInfo{Category: "artifact_contract", Code: "artifact_output_oversized"}
			} else if recorded.waitErr != nil {
				code := stableDiagnosticCode(recorded.stderr)
				if code == "" {
					code = "artifact_record_failed"
				}
				item.Status = "failed"
				item.Error = &ErrorInfo{Category: "artifact_contract", Code: code}
			} else if err := parseArtifactRecordOutput(recorded.stdout, target, cfg.AttemptID); err != nil {
				item.Status = "unknown"
				item.Error = &ErrorInfo{Category: "artifact_contract", Code: "invalid_artifact_record_output"}
			} else {
				item.Status = "recorded"
			}
			artifactResults = append(artifactResults, item)
			if item.Error != nil {
				failed := finishV2(fail(result, item.Error.Category, item.Error.Code, 3), cfg, artifactResults)
				if err := cleanup(); err != nil {
					failed.CleanupCode = "temporary_report_cleanup_failed"
				}
				return failed
			}
		}
		runResult = finishV2(runResult, cfg, artifactResults)
		if cfg.CheckAdapter != "" {
			runResult = finishV3(runResult, artifactResults, nil)
			if report.Outcome == "completed" {
				runResult = runChecks(ctx, cfg, pkg, result, artifactResults, createTemp, remove)
			}
		}
	}
	if err := cleanup(); err != nil {
		runResult.CleanupCode = "temporary_report_cleanup_failed"
	}
	return runResult
}
