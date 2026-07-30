package workpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type digestPayload struct {
	SchemaVersion      int          `json:"schema_version"`
	FlowRunID          string       `json:"flow_run_id"`
	StepID             string       `json:"step_id"`
	AttemptID          string       `json:"attempt_id"`
	EntrySequence      uint64       `json:"entry_sequence"`
	FlowSnapshotDigest string       `json:"flow_snapshot_digest"`
	TaskSnapshotDigest string       `json:"task_snapshot_digest"`
	TaskContent        string       `json:"task_content"`
	WorkingRoot        string       `json:"working_root"`
	Step               StepContract `json:"step"`
}

func canonicalJSON(pkg WorkPackage) ([]byte, error) {
	data, err := json.Marshal(payload(pkg))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDigestGeneration, err)
	}
	return data, nil
}

func payload(pkg WorkPackage) digestPayload {
	return digestPayload{
		SchemaVersion:      pkg.SchemaVersion,
		FlowRunID:          pkg.FlowRunID,
		StepID:             pkg.StepID,
		AttemptID:          pkg.AttemptID,
		EntrySequence:      pkg.EntrySequence,
		FlowSnapshotDigest: pkg.FlowSnapshotDigest,
		TaskSnapshotDigest: pkg.TaskSnapshotDigest,
		TaskContent:        pkg.TaskContent,
		WorkingRoot:        pkg.WorkingRoot,
		Step:               pkg.Step,
	}
}

// Digest recalculates the digest from the canonical payload. It deliberately
// ignores WorkPackageDigest.
func Digest(pkg WorkPackage) (string, error) {
	data, err := canonicalJSON(pkg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// Validate verifies the complete external WorkPackage contract without
// changing pkg or any collection reachable from it.
func Validate(pkg WorkPackage) error {
	if pkg.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedSchema, pkg.SchemaVersion)
	}
	if !digestPattern.MatchString(pkg.WorkPackageDigest) {
		return ErrInvalidDigest
	}
	if !state.IsValidFlowRunID(pkg.FlowRunID) {
		return ErrInvalidFlowRunID
	}
	if !flow.IsValidID(pkg.StepID) {
		return ErrInvalidStepID
	}
	if !state.IsValidStepAttemptID(pkg.AttemptID) {
		return ErrInvalidAttemptID
	}
	if pkg.EntrySequence == 0 {
		return ErrInvalidEntrySequence
	}
	expectedAttemptID, err := state.StepAttemptID(pkg.EntrySequence)
	if err != nil || expectedAttemptID != pkg.AttemptID {
		return ErrInvalidAttemptID
	}
	if !digestPattern.MatchString(pkg.FlowSnapshotDigest) || !digestPattern.MatchString(pkg.TaskSnapshotDigest) {
		return ErrInvalidSnapshotDigest
	}
	if err := task.ValidateSnapshot(task.TaskSnapshot{
		SchemaVersion: task.TaskSnapshotSchemaVersion,
		Digest:        pkg.TaskSnapshotDigest,
		Content:       pkg.TaskContent,
	}); err != nil {
		return fmt.Errorf("%w: TaskSnapshot binding", ErrInvalidSnapshotDigest)
	}
	if pkg.WorkingRoot != WorkingRoot {
		return ErrInvalidWorkingRoot
	}
	if pkg.Step.Inputs == nil || pkg.Step.Artifacts == nil || pkg.Step.RequiredChecks == nil {
		return ErrNilContractCollection
	}
	if err := validateStepContract(pkg.StepID, pkg.Step); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidStepContract, err)
	}
	digest, err := Digest(pkg)
	if err != nil {
		return err
	}
	if pkg.WorkPackageDigest != digest {
		return fmt.Errorf("%w: got %q, want %q", ErrDigestMismatch, pkg.WorkPackageDigest, digest)
	}
	return nil
}

func validateStepContract(stepID string, contract StepContract) error {
	inputs := make([]flow.Artifact, len(contract.Inputs))
	for i, input := range contract.Inputs {
		inputs[i] = flow.Artifact{Path: input.Path, Required: input.Required}
	}
	artifacts := make([]flow.Artifact, len(contract.Artifacts))
	for i, artifact := range contract.Artifacts {
		artifacts[i] = flow.Artifact{Path: artifact.Path, Required: artifact.Required}
	}
	approval := &flow.Approval{Required: contract.ApprovalRequired}
	candidate := flow.Flow{
		ID:    "work-package-validation",
		Title: "WorkPackage",
		Steps: []flow.Step{{
			ID:             stepID,
			Title:          contract.Title,
			Instruction:    contract.Instruction,
			Inputs:         inputs,
			Artifacts:      artifacts,
			Approval:       approval,
			RequiredChecks: append([]string{}, contract.RequiredChecks...),
		}},
	}
	if err := flow.Validate(candidate); err != nil {
		return err
	}
	return nil
}

func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidWorkPackage) ||
		errors.Is(err, ErrUnsupportedSchema) ||
		errors.Is(err, ErrInvalidDigest) ||
		errors.Is(err, ErrDigestMismatch) ||
		errors.Is(err, ErrInvalidFlowRunID) ||
		errors.Is(err, ErrInvalidStepID) ||
		errors.Is(err, ErrInvalidAttemptID) ||
		errors.Is(err, ErrInvalidEntrySequence) ||
		errors.Is(err, ErrInvalidSnapshotDigest) ||
		errors.Is(err, ErrInvalidWorkingRoot) ||
		errors.Is(err, ErrNilContractCollection) ||
		errors.Is(err, ErrInvalidStepContract)
}
