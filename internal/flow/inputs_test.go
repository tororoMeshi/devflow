package flow

import "testing"

func TestNormalizeMakesMissingInputsEmptySlice(t *testing.T) {
	got := Normalize(Flow{Steps: []Step{{}}})
	if got.Steps[0].Inputs == nil {
		t.Fatal("Inputs is nil")
	}
	if len(got.Steps[0].Inputs) != 0 {
		t.Fatalf("len(Inputs) = %d, want 0", len(got.Steps[0].Inputs))
	}
}

func TestLoadInputsDefaultsRequiredToTrue(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			inputs: [{path: "docs/request.md"}]
		}]
	}`)
	if len(got.Steps[0].Inputs) != 1 || !got.Steps[0].Inputs[0].Required {
		t.Fatalf("Inputs = %#v, want one required input", got.Steps[0].Inputs)
	}
}

func TestLoadInputsKeepsRequiredFalse(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			inputs: [{
				path: "docs/optional-context.md"
				required: false
			}]
		}]
	}`)
	if len(got.Steps[0].Inputs) != 1 || got.Steps[0].Inputs[0].Required {
		t.Fatalf("Inputs = %#v, want one optional input", got.Steps[0].Inputs)
	}
}

func TestValidateInputAndArtifactPathsIndependently(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Flow)
		want ErrorCode
	}{
		{
			name: "missing input path",
			edit: func(fl *Flow) { fl.Steps[0].Inputs = []Artifact{{Path: ""}} },
			want: ErrorMissingInputPath,
		},
		{
			name: "invalid input path",
			edit: func(fl *Flow) { fl.Steps[0].Inputs = []Artifact{{Path: "../request.md"}} },
			want: ErrorInvalidInputPath,
		},
		{
			name: "duplicate input path",
			edit: func(fl *Flow) { fl.Steps[0].Inputs = []Artifact{{Path: "docs/request.md"}, {Path: "docs/request.md"}} },
			want: ErrorDuplicateInputPath,
		},
		{
			name: "duplicate artifact path",
			edit: func(fl *Flow) { fl.Steps[0].Artifacts = []Artifact{{Path: "docs/design.md"}, {Path: "docs/design.md"}} },
			want: ErrorDuplicateArtifactPath,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fl := validFlow(nil)
			tt.edit(&fl)
			code, ok := ErrorCodeOf(Validate(fl))
			if !ok || code != tt.want {
				t.Fatalf("Validate code = %q, %t; want %q, true", code, ok, tt.want)
			}
		})
	}
}

func TestValidateAllowsInputAndArtifactWithSamePath(t *testing.T) {
	fl := validFlow(func(fl *Flow) {
		fl.Steps[0].Inputs = []Artifact{{Path: "docs/design.md"}}
		fl.Steps[0].Artifacts = []Artifact{{Path: "docs/design.md"}}
	})
	if err := Validate(fl); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
