package main

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestParseArgs(t *testing.T) {
	args := []string{"execute", "--attempt", "a", "--record-artifacts", "--step", "s", "--timeout", "0", "--project-root", ".", "--devflow", "df", "--terminate-grace", "1s", "--", "exec", "", " x ", "--flag=value"}
	got, ok := parseArgs(args)
	if !ok || got.StepID != "s" || got.AttemptID != "a" || got.Timeout != 0 ||
		got.TerminateGrace != time.Second || !got.RecordArtifacts || !reflect.DeepEqual(got.ExecutorArgs, []string{"", " x ", "--flag=value"}) {
		t.Fatalf("parseArgs = %#v, %t", got, ok)
	}
	defaults, ok := parseArgs([]string{"execute", "--step", "s", "--attempt", "a", "--", "exec"})
	if !ok || defaults.ProjectRoot != "." || defaults.Devflow != "devflow" ||
		defaults.TerminateGrace != 5*time.Second {
		t.Fatalf("defaults = %#v, %t", defaults, ok)
	}
}

func TestParseArgsCheckMode(t *testing.T) {
	args := []string{"execute", "--check-adapter-arg", "one", "--record-artifacts",
		"--check-timeout", "0", "--check-adapter", "adapter", "--check-adapter-arg", "two",
		"--check-terminate-grace", "1s", "--step", "s", "--attempt", "a", "--", "exec",
		"--check-adapter", "preserved"}
	got, ok := parseArgs(args)
	if !ok || got.CheckAdapter != "adapter" || got.CheckTimeout != 0 ||
		got.CheckTerminateGrace != time.Second ||
		!reflect.DeepEqual(got.CheckAdapterArgs, []string{"one", "two"}) ||
		!reflect.DeepEqual(got.ExecutorArgs, []string{"--check-adapter", "preserved"}) {
		t.Fatalf("parseArgs = %#v, %t", got, ok)
	}
}

func TestParseArgsRejectsInvalid(t *testing.T) {
	tests := [][]string{
		nil, {"other"}, {"execute"}, {"execute", "--step", "s", "--", "e"},
		{"execute", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--"},
		{"execute", "--step"}, {"execute", "--unknown", "x", "--step", "s", "--attempt", "a", "--", "e"},
		{"execute", "--step=s", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s", "--step", "s", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--timeout", "0", "--timeout", "0", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--project-root", ".", "--project-root", ".", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--devflow", "d", "--devflow", "d", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--terminate-grace", "0", "--terminate-grace", "0", "--", "e"},
		{"execute", "positional", "--step", "s", "--attempt", "a", "--", "e"},
		{"execute", "--step", " ", "--attempt", "a", "--", "e"},
		{"execute", "--step", "\u3000", "--attempt", "a", "--", "e"},
		{"execute", "--step", " s", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s ", "--attempt", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--timeout", "-1s", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--timeout", "bad", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--record-artifacts", "--record-artifacts", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--record-artifacts=true", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--check-adapter", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--check-adapter-arg", "x", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--check-timeout", "0", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--check-terminate-grace", "0", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--record-artifacts", "--check-adapter", "a", "--check-adapter", "a", "--", "e"},
		{"execute", "--step", "s", "--attempt", "a", "--record-artifacts", "--check-adapter", "a", "--check-timeout", "-1s", "--", "e"},
	}
	for i, args := range tests {
		if _, ok := parseArgs(args); ok {
			t.Errorf("%d accepted: %#v", i, args)
		}
	}
}

func TestUsageExact(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), nil, &stdout, &stderr)
	if exit != 2 || stdout.Len() != 0 || stderr.String() != usage {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestValidInvocationWritesResultJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := run(context.Background(), []string{
		"execute", "--project-root", "/path/that/does/not/exist", "--step", "s", "--attempt", "a", "--", "exec",
	}, &stdout, &stderr)
	if exit != 6 || !bytes.Contains(stdout.Bytes(), []byte(`"category":"runtime_io","code":"invalid_project_root"`)) ||
		stderr.String() != "runtime_io:invalid_project_root\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestResultWriteFailureIsRuntimeIO(t *testing.T) {
	var stderr bytes.Buffer
	exit := run(context.Background(), []string{
		"execute", "--project-root", "/path/that/does/not/exist", "--step", "s", "--attempt", "a", "--", "exec",
	}, failingWriter{}, &stderr)
	if exit != 6 || stderr.String() != "runtime_io:result_write_failed\n" {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
}
