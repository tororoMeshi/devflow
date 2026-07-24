package automationruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var (
	errInvalidReportOutput    = errors.New("invalid report output")
	errReportIdentityMismatch = errors.New("report identity mismatch")
)

type workPackageHeader struct {
	SchemaVersion     int    `json:"schema_version"`
	WorkPackageDigest string `json:"work_package_digest"`
	FlowRunID         string `json:"flow_run_id"`
	StepID            string `json:"step_id"`
	AttemptID         string `json:"attempt_id"`
}

type reportHeader struct {
	SchemaVersion     int    `json:"schema_version"`
	FlowRunID         string `json:"flow_run_id"`
	StepID            string `json:"step_id"`
	AttemptID         string `json:"attempt_id"`
	WorkPackageDigest string `json:"work_package_digest"`
	Outcome           string `json:"outcome"`
}

type reportHeaderProjection struct {
	SchemaVersion     *int    `json:"schema_version"`
	FlowRunID         *string `json:"flow_run_id"`
	StepID            *string `json:"step_id"`
	AttemptID         *string `json:"attempt_id"`
	WorkPackageDigest *string `json:"work_package_digest"`
	Outcome           *string `json:"outcome"`
}

func decodeOne(data []byte, target any) error {
	if len(data) == 0 {
		return errors.New("empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func parseWorkPackage(data []byte, stepID, attemptID string) (workPackageHeader, error) {
	var header workPackageHeader
	if err := decodeOne(data, &header); err != nil {
		return header, err
	}
	if header.SchemaVersion != 1 || header.FlowRunID == "" || header.StepID != stepID ||
		header.AttemptID != attemptID || !digestPattern.MatchString(header.WorkPackageDigest) {
		return header, errors.New("invalid WorkPackage header")
	}
	return header, nil
}

func parseReportHeader(data []byte, pkg workPackageHeader) (reportHeader, error) {
	var projection reportHeaderProjection
	if err := decodeOne(data, &projection); err != nil {
		return reportHeader{}, errInvalidReportOutput
	}
	if projection.SchemaVersion == nil || projection.FlowRunID == nil || projection.StepID == nil ||
		projection.AttemptID == nil || projection.WorkPackageDigest == nil || projection.Outcome == nil {
		return reportHeader{}, errInvalidReportOutput
	}
	header := reportHeader{
		SchemaVersion:     *projection.SchemaVersion,
		FlowRunID:         *projection.FlowRunID,
		StepID:            *projection.StepID,
		AttemptID:         *projection.AttemptID,
		WorkPackageDigest: *projection.WorkPackageDigest,
		Outcome:           *projection.Outcome,
	}
	if header.SchemaVersion != 1 ||
		(header.Outcome != "completed" && header.Outcome != "blocked" && header.Outcome != "failed") {
		return header, errInvalidReportOutput
	}
	if header.FlowRunID != pkg.FlowRunID || header.StepID != pkg.StepID ||
		header.AttemptID != pkg.AttemptID || header.WorkPackageDigest != pkg.WorkPackageDigest {
		return header, errReportIdentityMismatch
	}
	return header, nil
}
