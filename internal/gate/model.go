package gate

import "github.com/8noki8/devflow/internal/state"

type GateResult struct {
	MissingInputs       []string
	OK                  bool
	MissingEvidence     []string
	MissingArtifacts    []string
	UnsafeArtifacts     []string
	MismatchedArtifacts []string
	MissingApprovals    []string
	CheckProblems       []CheckProblem
	ArtifactProblems    []ArtifactProblem
}

type ArtifactProblemKind string

const (
	ArtifactEvidenceMissing ArtifactProblemKind = "evidence_missing"
	ArtifactFileMissing     ArtifactProblemKind = "file_missing"
	ArtifactUnsafe          ArtifactProblemKind = "unsafe"
	ArtifactMismatch        ArtifactProblemKind = "mismatch"
)

type ArtifactProblem struct {
	Path string
	Kind ArtifactProblemKind
}

// ArtifactInspection is the current filesystem state of a flow artifact.
// It is intentionally transient: it is derived from the active attempt and
// never written back to State.
type ArtifactInspection struct {
	Path            string
	Required        bool
	Exists          bool
	Evidence        *state.ArtifactEvidence
	MatchesEvidence *bool
	Problem         ArtifactProblemKind
}

type CheckProblemKind string

const (
	CheckMissing CheckProblemKind = "missing"
	CheckFailed  CheckProblemKind = "failed"
)

type CheckProblem struct {
	CheckID string
	Kind    CheckProblemKind
}

type Result = GateResult
