package automationruntime

import (
	"encoding/json"
	"io"
	"time"
)

const (
	MaxWorkPackageBytes            = 4 * 1024 * 1024
	MaxExecutorStdoutBytes         = 256 * 1024
	MaxExecutorStderrTailBytes     = 64 * 1024
	MaxCheckRequestBytes           = 256 * 1024
	MaxCheckAdapterStdoutBytes     = 256 * 1024
	MaxCheckAdapterStderrTailBytes = 64 * 1024
)

type Config struct {
	ProjectRoot         string
	Devflow             string
	StepID              string
	AttemptID           string
	Executor            string
	ExecutorArgs        []string
	Timeout             time.Duration
	TerminateGrace      time.Duration
	RecordArtifacts     bool
	CheckAdapter        string
	CheckAdapterArgs    []string
	CheckTimeout        time.Duration
	CheckTerminateGrace time.Duration
}

type ErrorInfo struct {
	Category string `json:"category"`
	Code     string `json:"code"`
}

type Result struct {
	SchemaVersion         int        `json:"schema_version"`
	Status                string     `json:"status"`
	FlowRunID             string     `json:"flow_run_id"`
	StepID                string     `json:"step_id"`
	AttemptID             string     `json:"attempt_id"`
	WorkPackageDigest     string     `json:"work_package_digest"`
	ExecutionReportDigest string     `json:"execution_report_digest"`
	ReportOutcome         string     `json:"report_outcome"`
	ReportIdempotent      bool       `json:"report_idempotent"`
	ExecutorExitCode      *int       `json:"executor_exit_code"`
	StderrTruncated       bool       `json:"stderr_truncated"`
	Error                 *ErrorInfo `json:"error"`
}

type RunResult struct {
	Result      Result
	ResultV2    *ResultV2
	ResultV3    *ResultV3
	ExitCode    int
	CleanupCode string
}

type CheckItem struct {
	CheckID         string     `json:"check_id"`
	Status          string     `json:"status"`
	Passed          *bool      `json:"passed"`
	CheckExitCode   *int       `json:"check_exit_code"`
	AdapterExitCode *int       `json:"adapter_exit_code"`
	StderrTruncated bool       `json:"stderr_truncated"`
	Error           *ErrorInfo `json:"error"`
}

type ResultV3 struct {
	SchemaVersion         int              `json:"schema_version"`
	Status                string           `json:"status"`
	FlowRunID             string           `json:"flow_run_id"`
	StepID                string           `json:"step_id"`
	AttemptID             string           `json:"attempt_id"`
	WorkPackageDigest     string           `json:"work_package_digest"`
	ExecutionReportDigest string           `json:"execution_report_digest"`
	ReportOutcome         string           `json:"report_outcome"`
	ReportIdempotent      bool             `json:"report_idempotent"`
	ExecutorExitCode      *int             `json:"executor_exit_code"`
	StderrTruncated       bool             `json:"stderr_truncated"`
	Artifacts             []ArtifactResult `json:"artifacts"`
	Checks                []CheckItem      `json:"checks"`
	Error                 *ErrorInfo       `json:"error"`
}

type ArtifactResult struct {
	Path     string     `json:"path"`
	Required bool       `json:"required"`
	Status   string     `json:"status"`
	Error    *ErrorInfo `json:"error"`
}

type ResultV2 struct {
	SchemaVersion         int              `json:"schema_version"`
	Status                string           `json:"status"`
	FlowRunID             string           `json:"flow_run_id"`
	StepID                string           `json:"step_id"`
	AttemptID             string           `json:"attempt_id"`
	WorkPackageDigest     string           `json:"work_package_digest"`
	ExecutionReportDigest string           `json:"execution_report_digest"`
	ReportOutcome         string           `json:"report_outcome"`
	ReportIdempotent      bool             `json:"report_idempotent"`
	ExecutorExitCode      *int             `json:"executor_exit_code"`
	StderrTruncated       bool             `json:"stderr_truncated"`
	Artifacts             []ArtifactResult `json:"artifacts"`
	Error                 *ErrorInfo       `json:"error"`
}

func baseResult(cfg Config) Result {
	return Result{SchemaVersion: 1, Status: "failed", StepID: cfg.StepID, AttemptID: cfg.AttemptID}
}

func fail(result Result, category, code string, exit int) RunResult {
	result.Error = &ErrorInfo{Category: category, Code: code}
	return RunResult{Result: result, ExitCode: exit}
}

func WriteResult(w io.Writer, result Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

func WriteResultV2(w io.Writer, result ResultV2) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

func WriteResultV3(w io.Writer, result ResultV3) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}
