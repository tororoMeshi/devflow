package automationruntime

import (
	"errors"
	"strings"
	"testing"
)

const testDigest = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
const reportDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func testPackageHeader() workPackageHeader {
	return workPackageHeader{SchemaVersion: 1, FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1", WorkPackageDigest: testDigest}
}

func TestParseRecordOutputStrict(t *testing.T) {
	pkg := testPackageHeader()
	valid := "Recorded execution report\nRun: run_1\nStep: build\nAttempt: attempt_1\nWork package: " +
		testDigest + "\nExecution report: " + reportDigest + "\nOutcome: blocked\nIdempotent: true\n"
	got, err := parseRecordOutput([]byte(valid), pkg)
	if err != nil || got.ExecutionReportDigest != reportDigest || !got.Idempotent {
		t.Fatalf("parse valid = %#v, %v", got, err)
	}
	mutations := []string{
		strings.TrimSuffix(valid, "Idempotent: true\n"),
		valid + "\n",
		strings.Replace(valid, "Recorded execution report", "Recorded report", 1),
		strings.Replace(valid, "Run: run_1", "Step: run_1", 1),
		strings.Replace(valid, "Run: run_1", "Run: other", 1),
		strings.Replace(valid, "Step: build", "Step: other", 1),
		strings.Replace(valid, "Attempt: attempt_1", "Attempt: other", 1),
		strings.Replace(valid, testDigest, reportDigest, 1),
		strings.Replace(valid, reportDigest, "sha256:ABC", 1),
		strings.Replace(valid, "Outcome: blocked", "Outcome: unknown", 1),
		strings.Replace(valid, "Idempotent: true", "Idempotent: yes", 1),
		strings.Replace(valid, "Idempotent: true", "Idempotent: TRUE", 1),
		strings.Replace(valid, "Run: run_1", "Run: run_1 ", 1),
		strings.ReplaceAll(valid, "\n", "\r\n"),
	}
	for i, input := range mutations {
		if _, err := parseRecordOutput([]byte(input), pkg); err == nil {
			t.Errorf("mutation %d accepted", i)
		}
	}
}

func TestHeaders(t *testing.T) {
	wp := []byte(`{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","work_package_digest":"` + testDigest + `"}` + "\n")
	pkg, err := parseWorkPackage(wp, "build", "attempt_1")
	if err != nil {
		t.Fatal(err)
	}
	report := []byte(`{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","work_package_digest":"` + testDigest + `","outcome":"failed"}`)
	if _, err := parseReportHeader(report, pkg); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, append(wp, []byte(`{}`)...), []byte(`[]`)} {
		if _, err := parseWorkPackage(bad, "build", "attempt_1"); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestWorkPackageHeaderRejectsInvalid(t *testing.T) {
	valid := `{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","work_package_digest":"` + testDigest + `"}`
	tests := []string{
		"", "null", "1", `"value"`, "[]", "{}", valid + `{}`,
		strings.Replace(valid, `"schema_version":1`, `"schema_version":0`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":null`, 1),
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":""`, 1),
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":null`, 1),
		strings.Replace(valid, `"step_id":"build"`, `"step_id":"other"`, 1),
		strings.Replace(valid, `"attempt_id":"attempt_1"`, `"attempt_id":"other"`, 1),
		strings.Replace(valid, testDigest, "sha256:ABC", 1),
	}
	for i, input := range tests {
		if _, err := parseWorkPackage([]byte(input), "build", "attempt_1"); err == nil {
			t.Errorf("%d accepted: %s", i, input)
		}
	}
}

func TestReportHeaderValidation(t *testing.T) {
	pkg := testPackageHeader()
	base := `{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","work_package_digest":"` + testDigest + `","outcome":"%s","extra":"allowed"}`
	for _, outcome := range []string{"completed", "blocked", "failed"} {
		if _, err := parseReportHeader([]byte(strings.Replace(base, "%s", outcome, 1)), pkg); err != nil {
			t.Fatalf("outcome %q: %v", outcome, err)
		}
	}
	valid := strings.Replace(base, "%s", "completed", 1)
	invalidOutput := []string{
		"", "null", "[]", "{}", valid + `{}`,
		strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		strings.Replace(valid, `"schema_version":1`, `"schema_version":null`, 1),
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":null`, 1),
		strings.Replace(valid, `"step_id":"build"`, `"step_id":1`, 1),
		strings.Replace(valid, `"outcome":"completed"`, `"outcome":"unknown"`, 1),
		strings.Replace(valid, `"outcome":"completed"`, `"outcome":null`, 1),
	}
	for i, input := range invalidOutput {
		if _, err := parseReportHeader([]byte(input), pkg); !errors.Is(err, errInvalidReportOutput) {
			t.Errorf("invalid output %d error=%v", i, err)
		}
	}
	identityMismatch := []string{
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":"other"`, 1),
		strings.Replace(valid, `"step_id":"build"`, `"step_id":"other"`, 1),
		strings.Replace(valid, `"attempt_id":"attempt_1"`, `"attempt_id":"other"`, 1),
		strings.Replace(valid, testDigest, reportDigest, 1),
	}
	for i, input := range identityMismatch {
		if _, err := parseReportHeader([]byte(input), pkg); !errors.Is(err, errReportIdentityMismatch) {
			t.Errorf("identity mismatch %d error=%v", i, err)
		}
	}
}
