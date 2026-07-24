package executionreport

import "errors"

const (
	SchemaVersion    = 1
	MaxDocumentBytes = 256 * 1024
	MaxCollection    = 100
)

type Outcome string

const (
	OutcomeCompleted Outcome = "completed"
	OutcomeBlocked   Outcome = "blocked"
	OutcomeFailed    Outcome = "failed"
)

type EvidenceKind string

const (
	EvidenceInput    EvidenceKind = "input"
	EvidenceArtifact EvidenceKind = "artifact"
	EvidenceCheck    EvidenceKind = "check"
)

type Report struct {
	SchemaVersion     int              `json:"schema_version"`
	FlowRunID         string           `json:"flow_run_id"`
	StepID            string           `json:"step_id"`
	AttemptID         string           `json:"attempt_id"`
	WorkPackageDigest string           `json:"work_package_digest"`
	Outcome           Outcome          `json:"outcome"`
	Summary           string           `json:"summary"`
	Decisions         []DecisionRecord `json:"decisions"`
	ArtifactRefs      []string         `json:"artifact_refs"`
	UnresolvedIssues  []string         `json:"unresolved_issues"`
	NextAction        string           `json:"next_action"`
}

type DecisionRecord struct {
	Decision     string              `json:"decision"`
	Rationale    string              `json:"rationale"`
	EvidenceRefs []EvidenceReference `json:"evidence_refs"`
}

type EvidenceReference struct {
	Kind EvidenceKind `json:"kind"`
	ID   string       `json:"id"`
}

var (
	ErrInvalidReport     = errors.New("invalid ExecutionReport")
	ErrUnsupportedSchema = errors.New("unsupported ExecutionReport schema")
	ErrTooLarge          = errors.New("ExecutionReport too large")
	ErrBindingMismatch   = errors.New("WorkPackage digest mismatch")
	ErrUnknownEvidence   = errors.New("unknown evidence reference")
	ErrUnknownArtifact   = errors.New("unknown Artifact reference")
	ErrConflict          = errors.New("conflicting ExecutionReport")
	ErrUnsafeStore       = errors.New("unsafe Report directory or file")
	ErrInvalidExisting   = errors.New("invalid existing Report")
	ErrSave              = errors.New("Report save failure")
	ErrDuplicateJSONKey  = errors.New("duplicate JSON key")
	ErrTrailingJSON      = errors.New("trailing JSON")
	ErrUnknownField      = errors.New("unknown field")
)

type RecordResult struct {
	Digest     string
	Idempotent bool
}
