package automationruntime

import (
	"bytes"
	"testing"
)

func TestWriteResultExactJSON(t *testing.T) {
	exit := 0
	success := Result{
		SchemaVersion: 1, Status: "recorded", FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1",
		WorkPackageDigest: testDigest, ExecutionReportDigest: reportDigest, ReportOutcome: "blocked",
		ReportIdempotent: false, ExecutorExitCode: &exit, StderrTruncated: false, Error: nil,
	}
	var out bytes.Buffer
	if err := WriteResult(&out, success); err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"status":"recorded","flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","work_package_digest":"` + testDigest + `","execution_report_digest":"` + reportDigest + `","report_outcome":"blocked","report_idempotent":false,"executor_exit_code":0,"stderr_truncated":false,"error":null}` + "\n"
	if out.String() != want {
		t.Fatalf("got %q\nwant %q", out.String(), want)
	}
	out.Reset()
	failure := Result{SchemaVersion: 1, Status: "failed", Error: &ErrorInfo{Category: "executor_process", Code: "start_failed"}}
	if err := WriteResult(&out, failure); err != nil {
		t.Fatal(err)
	}
	want = `{"schema_version":1,"status":"failed","flow_run_id":"","step_id":"","attempt_id":"","work_package_digest":"","execution_report_digest":"","report_outcome":"","report_idempotent":false,"executor_exit_code":null,"stderr_truncated":false,"error":{"category":"executor_process","code":"start_failed"}}` + "\n"
	if out.String() != want {
		t.Fatalf("got %q\nwant %q", out.String(), want)
	}
}
