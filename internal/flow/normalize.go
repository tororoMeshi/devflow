package flow

func Normalize(flow Flow) Flow {
	if flow.Steps == nil {
		flow.Steps = []Step{}
	}

	for stepIndex := range flow.Steps {
		if flow.Steps[stepIndex].Inputs == nil {
			flow.Steps[stepIndex].Inputs = []Artifact{}
		}
		if flow.Steps[stepIndex].Artifacts == nil {
			flow.Steps[stepIndex].Artifacts = []Artifact{}
		}
		if flow.Steps[stepIndex].RequiredChecks == nil {
			flow.Steps[stepIndex].RequiredChecks = []string{}
		}
	}

	return flow
}
