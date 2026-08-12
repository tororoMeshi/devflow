package flow

import "testing"

func TestValidateRejectsDevflowArtifactButDoesNotChangeInputContract(t *testing.T) {
	base := Flow{ID: "flow", Title: "Flow", Steps: []Step{{ID: "step", Title: "Step", Objective: "Do."}}}
	for _, path := range []string{".devflow", ".devflow/state.json", ".devflow/runs/out"} {
		withArtifact := base
		withArtifact.Steps = append([]Step(nil), base.Steps...)
		withArtifact.Steps[0].Artifacts = []Artifact{{Path: path, Required: true}}
		if err := Validate(withArtifact); err == nil {
			t.Fatalf("%q Artifact accepted", path)
		}
	}
	nested := base
	nested.Steps = append([]Step(nil), base.Steps...)
	nested.Steps[0].Artifacts = []Artifact{{Path: "x/.devflow/file", Required: true}, {Path: ".DEVFLOW/file", Required: true}}
	if err := Validate(nested); err != nil {
		t.Fatalf("non-leading or case-distinct .devflow rejected: %v", err)
	}
	withInput := base
	withInput.Steps = append([]Step(nil), base.Steps...)
	withInput.Steps[0].Inputs = []Artifact{{Path: ".devflow/input.txt", Required: true}}
	if err := Validate(withInput); err != nil {
		t.Fatalf(".devflow input contract changed: %v", err)
	}
}
