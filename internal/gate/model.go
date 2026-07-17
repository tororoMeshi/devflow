package gate

type GateResult struct {
	MissingInputs    []string
	OK               bool
	MissingArtifacts []string
	MissingApprovals []string
	CheckProblems    []CheckProblem
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
