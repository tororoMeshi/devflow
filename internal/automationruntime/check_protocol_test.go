package automationruntime

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCheckRequestStrictValidation(t *testing.T) {
	pkg := testPackageHeader()
	valid := `{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test"}` + "\n"
	if err := validateCheckRequest([]byte(valid), pkg, "go-test"); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{
		"", "null", "[]", valid + `{}`,
		strings.Replace(valid, `"schema_version":2`, `"schema_version":1`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"other"`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"a","\u0063heck_id":"b"`, 1),
		strings.Replace(valid, `"check_id":`, `"unknown":true,"check_id":`, 1),
	} {
		if err := validateCheckRequest([]byte(input), pkg, "go-test"); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

func TestCheckRecordStrictValidationAndLogPath(t *testing.T) {
	pkg := testPackageHeader()
	base := `{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":0%s}}`
	for _, suffix := range []string{"", `,"log_path":"logs/go-test.log"`} {
		data := []byte(strings.Replace(base, "%s", suffix, 1))
		record, err := parseCheckRecord(data, pkg, "go-test", t.TempDir())
		if err != nil || record.ExitCode != 0 {
			t.Fatalf("suffix=%q record=%#v err=%v", suffix, record, err)
		}
	}
	for _, path := range []string{"", ".", "..", "../x", "/tmp/x", `C:\x`, ".devflow", ".devflow/x", "a\nb", "a\x00b"} {
		data := []byte(strings.Replace(base, "%s", `,"log_path":"`+strings.ReplaceAll(path, `\`, `\\`)+`"`, 1))
		if _, err := parseCheckRecord(data, pkg, "go-test", t.TempDir()); err == nil {
			t.Errorf("accepted log_path=%q", path)
		}
	}
	for _, mutation := range []string{
		strings.Replace(base, "%s", `,"log_path":null`, 1),
		strings.Replace(base, `"exit_code":0`, `"exit_code":-1`, 1),
		strings.Replace(base, `"result":`, `"unknown":true,"result":`, 1),
		strings.Replace(base, `"exit_code":0`, `"exit_code":0,"exit_code":1`, 1),
	} {
		if _, err := parseCheckRecord([]byte(mutation), pkg, "go-test", t.TempDir()); err == nil {
			t.Errorf("accepted %s", mutation)
		}
	}
}

func TestParseCheckRecordSuccessStrict(t *testing.T) {
	record := checkRecordProjection{
		checkIdentity: checkIdentity{FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1", CheckID: "go-test"},
		ExitCode:      1,
	}
	valid := []byte("Recorded check: go-test\nRun: run_1\nStep: build\nAttempt: attempt_1\nExit code: 1\n")
	if err := parseCheckRecordSuccess(valid, record); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{
		bytes.TrimSuffix(valid, []byte("\n")), append(valid, '\n'),
		bytes.Replace(valid, []byte("Exit code: 1"), []byte("Exit code: 01"), 1),
		bytes.Replace(valid, []byte("Exit code: 1"), []byte("Exit code: -1"), 1),
		bytes.ReplaceAll(valid, []byte("\n"), []byte("\r\n")),
		bytes.Replace(valid, []byte("run_1"), []byte("other"), 1),
	} {
		if err := parseCheckRecordSuccess(data, record); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
	if !errors.Is(errCheckIdentityMismatch, errCheckIdentityMismatch) {
		t.Fatal("identity error sentinel changed")
	}
}

func TestCheckRequestRejectsEveryIdentityAndTypeViolation(t *testing.T) {
	pkg := testPackageHeader()
	valid := `{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test"}`
	for _, input := range []string{
		`{"schema_version":2,"flow_run_id":null,"step_id":"build","attempt_id":"attempt_1","check_id":"go-test"}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":null,"attempt_id":"attempt_1","check_id":"go-test"}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":null,"check_id":"go-test"}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":null}`,
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":1`, 1),
		strings.Replace(valid, `"step_id":"build"`, `"step_id":[]`, 1),
		strings.Replace(valid, `"attempt_id":"attempt_1"`, `"attempt_id":{}`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":true`, 1),
		strings.Replace(valid, `"flow_run_id":"run_1"`, `"flow_run_id":"other"`, 1),
		strings.Replace(valid, `"step_id":"build"`, `"step_id":"other"`, 1),
		strings.Replace(valid, `"attempt_id":"attempt_1"`, `"attempt_id":"other"`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"other"`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"go-test","unknown":true`, 1),
		strings.Replace(valid, `"check_id":"go-test"`, `"check_id":"go-test","\\u0063heck_id":"other"`, 1),
	} {
		if err := validateCheckRequest([]byte(input), pkg, "go-test"); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func TestCheckRecordRejectsRequiredFieldsAndPreservesInput(t *testing.T) {
	pkg := testPackageHeader()
	valid := []byte(`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":1}}` + "\n")
	before := append([]byte(nil), valid...)
	record, err := parseCheckRecord(valid, pkg, "go-test", t.TempDir())
	if err != nil || record.ExitCode != 1 || !bytes.Equal(valid, before) {
		t.Fatalf("record=%#v err=%v bytes changed=%t", record, err, !bytes.Equal(valid, before))
	}
	for _, input := range []string{
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test"}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":null}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{}}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":null}}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":"1"}}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":-1}}`,
		`{"schema_version":1,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":0}}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":0},"unknown":true}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":0}} {}`,
		`{"schema_version":2,"flow_run_id":"run_1","step_id":"build","attempt_id":"attempt_1","check_id":"go-test","result":{"exit_code":0,"\\u0065xit_code":1}}`,
	} {
		if _, err := parseCheckRecord([]byte(input), pkg, "go-test", t.TempDir()); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}

func TestRuntimeLogPathContract(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"log.txt", "logs/check.log", `logs\\check.log`} {
		if !validRuntimeLogPath(path, root) {
			t.Errorf("rejected safe path %q", path)
		}
	}
	for _, path := range []string{"", ".", "..", "../x", "nested/../../x", ".devflow", ".devflow/x", "/tmp/x", `C:\\x`, `\\\\server\\share\\x`, "x\ry", "x\ny", "x\x00y"} {
		if validRuntimeLogPath(path, root) {
			t.Errorf("accepted unsafe path %q", path)
		}
	}
}

func TestParseCheckRecordSuccessRejectsExactFormatViolations(t *testing.T) {
	record := checkRecordProjection{checkIdentity: checkIdentity{FlowRunID: "run_1", StepID: "build", AttemptID: "attempt_1", CheckID: "go-test"}, ExitCode: 0}
	valid := []byte("Recorded check: go-test\nRun: run_1\nStep: build\nAttempt: attempt_1\nExit code: 0\n")
	for _, input := range [][]byte{
		[]byte("Recorded check: go-test\nRun: run_1\nStep: build\nAttempt: attempt_1\n"),
		append(append([]byte(nil), valid...), []byte("extra\n")...),
		bytes.Replace(valid, []byte("Recorded check:"), []byte("Recorded Check:"), 1),
		bytes.Replace(valid, []byte("Run: run_1"), []byte("Step: build"), 1),
		bytes.Replace(valid, []byte("Exit code: 0"), []byte("Exit code: +0"), 1),
		bytes.Replace(valid, []byte("Exit code: 0"), []byte("Exit code: 00"), 1),
		bytes.Replace(valid, []byte("Exit code: 0"), []byte("Exit code: 1"), 1),
		bytes.Replace(valid, []byte("Run: run_1"), []byte("Run: run_1 "), 1),
	} {
		if err := parseCheckRecordSuccess(input, record); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
