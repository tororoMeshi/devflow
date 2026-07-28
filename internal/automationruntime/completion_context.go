package automationruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
)

type completionContextHeader struct {
	SchemaVersion    *int            `json:"schema_version"`
	FlowRunID        *string         `json:"flow_run_id"`
	StepID           *string         `json:"step_id"`
	AttemptID        *string         `json:"attempt_id"`
	AttemptStatus    *string         `json:"attempt_status"`
	IsCurrentAttempt *bool           `json:"is_current_attempt"`
	Artifacts        json.RawMessage `json:"artifacts"`
	Checks           json.RawMessage `json:"checks"`
	Approval         json.RawMessage `json:"approval"`
	Completion       json.RawMessage `json:"completion"`
}

type completionArtifact struct {
	Path     *string         `json:"path"`
	Required *bool           `json:"required"`
	Status   *string         `json:"status"`
	Digest   json.RawMessage `json:"digest"`
	Size     json.RawMessage `json:"size"`
}

type completionCheck struct {
	ID       *string         `json:"id"`
	Status   *string         `json:"status"`
	ExitCode json.RawMessage `json:"exit_code"`
}

type completionApproval struct {
	Required                  *bool           `json:"required"`
	Status                    *string         `json:"status"`
	EvidenceSetDigest         json.RawMessage `json:"evidence_set_digest"`
	ApprovedEvidenceSetDigest json.RawMessage `json:"approved_evidence_set_digest"`
}

type completionDecision struct {
	Status  *string         `json:"status"`
	Blocker json.RawMessage `json:"blocker"`
}

type completionBlocker struct {
	Code      *string         `json:"code"`
	SubjectID json.RawMessage `json:"subject_id"`
}

func parseCompletionContext(data []byte, flowRunID, stepID, attemptID string) (json.RawMessage, error) {
	var header completionContextHeader
	if err := strictDecode(data, &header); err != nil {
		return nil, err
	}
	if header.SchemaVersion == nil || *header.SchemaVersion != 1 || header.FlowRunID == nil || header.StepID == nil ||
		header.AttemptID == nil || header.AttemptStatus == nil || header.IsCurrentAttempt == nil || header.Artifacts == nil ||
		header.Checks == nil || header.Approval == nil || header.Completion == nil || *header.FlowRunID != flowRunID ||
		*header.StepID != stepID || *header.AttemptID != attemptID || !validIdentifier(*header.FlowRunID) ||
		!validIdentifier(*header.StepID) || !validIdentifier(*header.AttemptID) ||
		(*header.AttemptStatus != "active" && *header.AttemptStatus != "closed") {
		return nil, errors.New("invalid completion context header")
	}
	if err := validateCompletionArtifacts(header.Artifacts); err != nil {
		return nil, err
	}
	if err := validateCompletionChecks(header.Checks); err != nil {
		return nil, err
	}
	if err := validateCompletionApproval(header.Approval); err != nil {
		return nil, err
	}
	if err := validateCompletionDecision(header.Completion); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), data...), nil
}

func validateCompletionArtifacts(data json.RawMessage) error {
	if bytes.Equal(data, []byte("null")) {
		return errors.New("null artifacts")
	}
	var artifacts []completionArtifact
	if err := strictDecode(data, &artifacts); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if artifact.Path == nil || artifact.Required == nil || artifact.Status == nil || artifact.Digest == nil || artifact.Size == nil ||
			!validArtifactPath(*artifact.Path) || !oneOf(*artifact.Status, "recorded", "missing", "unavailable", "mismatch") ||
			!validNullableDigest(artifact.Digest) || !validNullableNonnegativeInt64(artifact.Size) {
			return errors.New("invalid completion artifact")
		}
	}
	return nil
}

func validateCompletionChecks(data json.RawMessage) error {
	if bytes.Equal(data, []byte("null")) {
		return errors.New("null checks")
	}
	var checks []completionCheck
	if err := strictDecode(data, &checks); err != nil {
		return err
	}
	for _, check := range checks {
		if check.ID == nil || check.Status == nil || check.ExitCode == nil || !validIdentifier(*check.ID) ||
			!oneOf(*check.Status, "pending", "passed", "failed") || !validNullableNonnegativeInt(check.ExitCode) {
			return errors.New("invalid completion check")
		}
	}
	return nil
}

func validateCompletionApproval(data json.RawMessage) error {
	var approval completionApproval
	if err := strictDecode(data, &approval); err != nil || approval.Required == nil || approval.Status == nil ||
		approval.EvidenceSetDigest == nil || approval.ApprovedEvidenceSetDigest == nil ||
		!oneOf(*approval.Status, "not_required", "pending", "approved") ||
		!validNullableDigest(approval.EvidenceSetDigest) || !validNullableDigest(approval.ApprovedEvidenceSetDigest) {
		return errors.New("invalid completion approval")
	}
	return nil
}

func validateCompletionDecision(data json.RawMessage) error {
	var decision completionDecision
	if err := strictDecode(data, &decision); err != nil || decision.Status == nil || decision.Blocker == nil ||
		!oneOf(*decision.Status, "ready", "blocked", "not_applicable") {
		return errors.New("invalid completion decision")
	}
	if bytes.Equal(decision.Blocker, []byte("null")) {
		return nil
	}
	var blocker completionBlocker
	if err := strictDecode(decision.Blocker, &blocker); err != nil || blocker.Code == nil || blocker.SubjectID == nil ||
		!validIdentifier(*blocker.Code) || !validNullableIdentifier(blocker.SubjectID) {
		return errors.New("invalid completion blocker")
	}
	return nil
}

func validNullableDigest(data json.RawMessage) bool {
	if bytes.Equal(data, []byte("null")) {
		return true
	}
	var value string
	return strictDecode(data, &value) == nil && digestPattern.MatchString(value)
}

func validNullableNonnegativeInt64(data json.RawMessage) bool {
	if bytes.Equal(data, []byte("null")) {
		return true
	}
	var value int64
	return strictDecode(data, &value) == nil && value >= 0
}

func validNullableNonnegativeInt(data json.RawMessage) bool {
	if bytes.Equal(data, []byte("null")) {
		return true
	}
	var value int
	return strictDecode(data, &value) == nil && value >= 0
}

func validIdentifier(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\r\n\x00")
}

func validNullableIdentifier(data json.RawMessage) bool {
	if bytes.Equal(data, []byte("null")) {
		return true
	}
	var value string
	return strictDecode(data, &value) == nil && validIdentifier(value)
}

func validArtifactPath(path string) bool {
	return validIdentifier(path) && !filepath.IsAbs(path) && filepath.VolumeName(path) == "" && !hasPortableVolumeName(path)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
