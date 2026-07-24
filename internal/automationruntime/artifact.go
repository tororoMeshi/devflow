package automationruntime

import "errors"

type ArtifactTarget struct {
	Path     string
	Required bool
}

func ResolveArtifactTargets(outcome string, artifacts []ArtifactContract, artifactRefs []string) ([]ArtifactTarget, error) {
	known := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		if known[artifact.Path] {
			return nil, errors.New("duplicate artifact")
		}
		known[artifact.Path] = true
	}
	refs := make(map[string]bool, len(artifactRefs))
	for _, ref := range artifactRefs {
		if refs[ref] {
			return nil, errors.New("duplicate artifact ref")
		}
		if !known[ref] {
			return nil, errors.New("unknown artifact ref")
		}
		refs[ref] = true
	}
	targets := []ArtifactTarget{}
	for _, artifact := range artifacts {
		selected := false
		switch outcome {
		case "completed":
			selected = artifact.Required || refs[artifact.Path]
		case "blocked":
			selected = refs[artifact.Path]
		case "failed":
		default:
			return nil, errors.New("unknown outcome")
		}
		if selected {
			targets = append(targets, ArtifactTarget(artifact))
		}
	}
	return targets, nil
}
