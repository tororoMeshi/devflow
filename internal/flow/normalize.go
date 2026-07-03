package flow

func Normalize(flow Flow) Flow {
	if flow.Steps == nil {
		flow.Steps = []Step{}
	}

	for stepIndex := range flow.Steps {
		if flow.Steps[stepIndex].Artifacts == nil {
			flow.Steps[stepIndex].Artifacts = []Artifact{}
		}
	}

	return flow
}
