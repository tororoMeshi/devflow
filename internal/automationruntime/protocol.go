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
	Artifacts         []ArtifactContract
}

type ArtifactContract struct {
	Path     string
	Required bool
}

type reportHeader struct {
	SchemaVersion     int    `json:"schema_version"`
	FlowRunID         string `json:"flow_run_id"`
	StepID            string `json:"step_id"`
	AttemptID         string `json:"attempt_id"`
	WorkPackageDigest string `json:"work_package_digest"`
	Outcome           string `json:"outcome"`
	ArtifactRefs      []string
}

type reportHeaderProjection struct {
	SchemaVersion     *int            `json:"schema_version"`
	FlowRunID         *string         `json:"flow_run_id"`
	StepID            *string         `json:"step_id"`
	AttemptID         *string         `json:"attempt_id"`
	WorkPackageDigest *string         `json:"work_package_digest"`
	Outcome           *string         `json:"outcome"`
	ArtifactRefs      json.RawMessage `json:"artifact_refs"`
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
	return parseWorkPackageForMode(data, stepID, attemptID, false)
}

func parseWorkPackageForMode(data []byte, stepID, attemptID string, requireArtifacts bool) (workPackageHeader, error) {
	var projection struct {
		SchemaVersion     *int            `json:"schema_version"`
		WorkPackageDigest *string         `json:"work_package_digest"`
		FlowRunID         *string         `json:"flow_run_id"`
		StepID            *string         `json:"step_id"`
		AttemptID         *string         `json:"attempt_id"`
		Step              json.RawMessage `json:"step"`
	}
	if err := decodeOne(data, &projection); err != nil {
		return workPackageHeader{}, err
	}
	if projection.SchemaVersion == nil || projection.WorkPackageDigest == nil || projection.FlowRunID == nil ||
		projection.StepID == nil || projection.AttemptID == nil || (requireArtifacts && len(projection.Step) == 0) {
		return workPackageHeader{}, errors.New("invalid WorkPackage header")
	}
	var rawArtifacts []struct {
		Path     *string `json:"path"`
		Required *bool   `json:"required"`
	}
	if requireArtifacts {
		var step struct {
			Artifacts json.RawMessage `json:"artifacts"`
		}
		if err := json.Unmarshal(projection.Step, &step); err != nil || len(step.Artifacts) == 0 || bytes.Equal(step.Artifacts, []byte("null")) {
			return workPackageHeader{}, errors.New("invalid WorkPackage artifacts")
		}
		if err := json.Unmarshal(step.Artifacts, &rawArtifacts); err != nil {
			return workPackageHeader{}, errors.New("invalid WorkPackage artifacts")
		}
	}
	artifacts := make([]ArtifactContract, len(rawArtifacts))
	seen := map[string]struct{}{}
	for i, item := range rawArtifacts {
		if item.Path == nil || *item.Path == "" || item.Required == nil {
			return workPackageHeader{}, errors.New("invalid WorkPackage artifact")
		}
		if _, ok := seen[*item.Path]; ok {
			return workPackageHeader{}, errors.New("duplicate WorkPackage artifact")
		}
		seen[*item.Path] = struct{}{}
		artifacts[i] = ArtifactContract{Path: *item.Path, Required: *item.Required}
	}
	header := workPackageHeader{
		SchemaVersion: *projection.SchemaVersion, WorkPackageDigest: *projection.WorkPackageDigest,
		FlowRunID: *projection.FlowRunID, StepID: *projection.StepID, AttemptID: *projection.AttemptID,
		Artifacts: artifacts,
	}
	if header.SchemaVersion != 1 || header.FlowRunID == "" || header.StepID != stepID ||
		header.AttemptID != attemptID || !digestPattern.MatchString(header.WorkPackageDigest) {
		return header, errors.New("invalid WorkPackage header")
	}
	return header, nil
}

func parseReportHeader(data []byte, pkg workPackageHeader) (reportHeader, error) {
	return parseReportHeaderForMode(data, pkg, false)
}

func parseReportHeaderForMode(data []byte, pkg workPackageHeader, requireArtifactRefs bool) (reportHeader, error) {
	var projection reportHeaderProjection
	if err := decodeOne(data, &projection); err != nil {
		return reportHeader{}, errInvalidReportOutput
	}
	if projection.SchemaVersion == nil || projection.FlowRunID == nil || projection.StepID == nil ||
		projection.AttemptID == nil || projection.WorkPackageDigest == nil || projection.Outcome == nil ||
		(requireArtifactRefs && (len(projection.ArtifactRefs) == 0 || bytes.Equal(projection.ArtifactRefs, []byte("null")))) {
		return reportHeader{}, errInvalidReportOutput
	}
	var refs []string
	if requireArtifactRefs {
		if err := json.Unmarshal(projection.ArtifactRefs, &refs); err != nil {
			return reportHeader{}, errInvalidReportOutput
		}
	}
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if ref == "" {
			return reportHeader{}, errInvalidReportOutput
		}
		if _, ok := seen[ref]; ok {
			return reportHeader{}, errInvalidReportOutput
		}
		seen[ref] = struct{}{}
	}
	header := reportHeader{
		SchemaVersion:     *projection.SchemaVersion,
		FlowRunID:         *projection.FlowRunID,
		StepID:            *projection.StepID,
		AttemptID:         *projection.AttemptID,
		WorkPackageDigest: *projection.WorkPackageDigest,
		Outcome:           *projection.Outcome,
		ArtifactRefs:      refs,
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
