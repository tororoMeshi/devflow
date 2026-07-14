package flow

import (
	"os"
	"path/filepath"
	"sort"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

type FlowFileStatus string

const (
	FlowFileValid   FlowFileStatus = "valid"
	FlowFileInvalid FlowFileStatus = "invalid"
)

type FlowFileResult struct {
	Flow     *Flow
	FilePath string
	Status   FlowFileStatus
	Err      error
}

func LoadFile(path string) (Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Flow{}, err
	}

	flow, err := Load(data)
	if err != nil {
		return Flow{}, err
	}
	if err := ValidateFilename(flow, path); err != nil {
		return Flow{}, err
	}

	return flow, nil
}

func Load(data []byte) (Flow, error) {
	ctx := cuecontext.New()
	value := ctx.CompileBytes(data)
	if err := value.Err(); err != nil {
		return Flow{}, err
	}

	var raw rawFlow
	if err := value.LookupPath(cue.ParsePath("flow")).Decode(&raw); err != nil {
		return Flow{}, err
	}

	flow := raw.toFlow()
	flow = Normalize(flow)
	if err := Validate(flow); err != nil {
		return Flow{}, err
	}

	return flow, nil
}

func LoadDir(dir string) ([]FlowFileResult, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.cue"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)

	results := make([]FlowFileResult, 0, len(matches))
	for _, path := range matches {
		flow, err := LoadFile(path)
		if err != nil {
			results = append(results, FlowFileResult{
				FilePath: path,
				Status:   FlowFileInvalid,
				Err:      err,
			})
			continue
		}

		flowCopy := flow
		results = append(results, FlowFileResult{
			Flow:     &flowCopy,
			FilePath: path,
			Status:   FlowFileValid,
		})
	}

	return results, nil
}

type rawFlow struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Steps       []rawStep `json:"steps"`
}

type rawStep struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Instruction    string        `json:"instruction"`
	Artifacts      []rawArtifact `json:"artifacts"`
	Approval       *rawApproval  `json:"approval"`
	RequiredChecks []string      `json:"required_checks"`
}

type rawArtifact struct {
	Path     string `json:"path"`
	Required *bool  `json:"required"`
}

type rawApproval struct {
	Required bool `json:"required"`
}

func (f rawFlow) toFlow() Flow {
	steps := make([]Step, len(f.Steps))
	for i, rawStep := range f.Steps {
		steps[i] = rawStep.toStep()
	}

	return Flow{
		ID:          f.ID,
		Title:       f.Title,
		Description: f.Description,
		Steps:       steps,
	}
}

func (s rawStep) toStep() Step {
	artifacts := make([]Artifact, len(s.Artifacts))
	for i, rawArtifact := range s.Artifacts {
		artifacts[i] = rawArtifact.toArtifact()
	}

	var approval *Approval
	if s.Approval != nil {
		approval = &Approval{Required: s.Approval.Required}
	}

	return Step{
		ID:             s.ID,
		Title:          s.Title,
		Instruction:    s.Instruction,
		Artifacts:      artifacts,
		Approval:       approval,
		RequiredChecks: append([]string(nil), s.RequiredChecks...),
	}
}

func (a rawArtifact) toArtifact() Artifact {
	required := true
	if a.Required != nil {
		required = *a.Required
	}

	return Artifact{
		Path:     a.Path,
		Required: required,
	}
}
