package automationruntime

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var diagnosticPattern = regexp.MustCompile(`^error: (error_[a-z0-9_]+)(?: \([^)]*\))?$`)

type recordOutput struct {
	ExecutionReportDigest string
	Outcome               string
	Idempotent            bool
}

func parseRecordOutput(data []byte, pkg workPackageHeader) (recordOutput, error) {
	lines := strings.Split(string(data), "\n")
	if len(lines) != 9 || lines[8] != "" || lines[0] != "Recorded execution report" {
		return recordOutput{}, errors.New("invalid line count")
	}
	want := []struct {
		prefix string
		value  string
	}{
		{"Run: ", pkg.FlowRunID},
		{"Step: ", pkg.StepID},
		{"Attempt: ", pkg.AttemptID},
		{"Work package: ", pkg.WorkPackageDigest},
	}
	for i, item := range want {
		if lines[i+1] != item.prefix+item.value {
			return recordOutput{}, errors.New("identity mismatch")
		}
	}
	const reportPrefix = "Execution report: "
	if !strings.HasPrefix(lines[5], reportPrefix) {
		return recordOutput{}, errors.New("invalid report digest label")
	}
	digest := strings.TrimPrefix(lines[5], reportPrefix)
	const outcomePrefix = "Outcome: "
	if !digestPattern.MatchString(digest) || !strings.HasPrefix(lines[6], outcomePrefix) {
		return recordOutput{}, errors.New("invalid record result")
	}
	outcome := strings.TrimPrefix(lines[6], outcomePrefix)
	if outcome != "completed" && outcome != "blocked" && outcome != "failed" {
		return recordOutput{}, errors.New("invalid outcome")
	}
	const idempotentPrefix = "Idempotent: "
	if !strings.HasPrefix(lines[7], idempotentPrefix) {
		return recordOutput{}, errors.New("invalid idempotent label")
	}
	boolean := strings.TrimPrefix(lines[7], idempotentPrefix)
	if boolean != "true" && boolean != "false" {
		return recordOutput{}, errors.New("invalid idempotent value")
	}
	idempotent, err := strconv.ParseBool(boolean)
	if err != nil {
		return recordOutput{}, err
	}
	return recordOutput{ExecutionReportDigest: digest, Outcome: outcome, Idempotent: idempotent}, nil
}

func stableDiagnosticCode(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		match := diagnosticPattern.FindStringSubmatch(line)
		if len(match) == 2 {
			return match[1]
		}
	}
	return ""
}
