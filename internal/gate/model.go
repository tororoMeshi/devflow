package gate

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
