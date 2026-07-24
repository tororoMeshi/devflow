package workpackage

import "errors"

const SchemaVersion = 1

const WorkingRoot = "."

var (
	ErrInvalidState          = errors.New("invalid State")
	ErrNoActiveFlow          = errors.New("no active flow")
	ErrInvalidAttemptID      = errors.New("invalid Attempt ID")
	ErrAttemptNotFound       = errors.New("Attempt does not exist in Run")
	ErrStaleAttempt          = errors.New("stale Attempt")
	ErrInactiveAttempt       = errors.New("Attempt is not active")
	ErrStepAttemptMismatch   = errors.New("Step and Attempt mismatch")
	ErrInvalidWorkPackage    = errors.New("invalid WorkPackage")
	ErrUnsupportedSchema     = errors.New("unsupported WorkPackage schema version")
	ErrInvalidDigest         = errors.New("invalid WorkPackage digest")
	ErrDigestMismatch        = errors.New("WorkPackage digest mismatch")
	ErrDigestGeneration      = errors.New("generate WorkPackage digest")
	ErrInvalidFlowRunID      = errors.New("invalid Flow Run ID")
	ErrInvalidStepID         = errors.New("invalid Step ID")
	ErrInvalidEntrySequence  = errors.New("invalid entry sequence")
	ErrInvalidSnapshotDigest = errors.New("invalid snapshot digest")
	ErrInvalidWorkingRoot    = errors.New("invalid working root")
	ErrNilContractCollection = errors.New("nil WorkPackage contract collection")
	ErrInvalidStepContract   = errors.New("invalid Step contract")
)

type WorkPackage struct {
	SchemaVersion      int          `json:"schema_version"`
	WorkPackageDigest  string       `json:"work_package_digest"`
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

type StepContract struct {
	Title            string             `json:"title"`
	Instruction      string             `json:"instruction"`
	Inputs           []ArtifactContract `json:"inputs"`
	Artifacts        []ArtifactContract `json:"artifacts"`
	RequiredChecks   []string           `json:"required_checks"`
	ApprovalRequired bool               `json:"approval_required"`
}

type ArtifactContract struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
}
