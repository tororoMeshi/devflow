package gate

type GateResult struct {
	OK               bool
	MissingArtifacts []string
	MissingApprovals []string
}

type Result = GateResult
