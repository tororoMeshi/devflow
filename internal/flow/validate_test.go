package flow

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		flow     Flow
		wantCode ErrorCode
	}{
		{
			name:     "missing flow id",
			flow:     validFlow(func(flow *Flow) { flow.ID = "" }),
			wantCode: ErrorMissingFlowID,
		},
		{
			name:     "blank flow id",
			flow:     validFlow(func(flow *Flow) { flow.ID = "   " }),
			wantCode: ErrorMissingFlowID,
		},
		{
			name:     "invalid flow id",
			flow:     validFlow(func(flow *Flow) { flow.ID = "invalid id" }),
			wantCode: ErrorInvalidFlowID,
		},
		{
			name:     "missing flow title",
			flow:     validFlow(func(flow *Flow) { flow.Title = "" }),
			wantCode: ErrorMissingFlowTitle,
		},
		{
			name:     "blank flow title",
			flow:     validFlow(func(flow *Flow) { flow.Title = "   " }),
			wantCode: ErrorMissingFlowTitle,
		},
		{
			name:     "missing flow purpose",
			flow:     validFlow(func(flow *Flow) { flow.Purpose = "" }),
			wantCode: ErrorMissingFlowPurpose,
		},
		{
			name:     "blank flow purpose",
			flow:     validFlow(func(flow *Flow) { flow.Purpose = " \t\n " }),
			wantCode: ErrorMissingFlowPurpose,
		},
		{
			name:     "flow has no steps",
			flow:     validFlow(func(flow *Flow) { flow.Steps = nil }),
			wantCode: ErrorFlowHasNoSteps,
		},
		{
			name: validFlowName("empty steps"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps = []Step{}
			}),
			wantCode: ErrorFlowHasNoSteps,
		},
		{
			name: validFlowName("missing step id"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].ID = ""
			}),
			wantCode: ErrorMissingStepID,
		},
		{
			name: validFlowName("blank step id"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].ID = "   "
			}),
			wantCode: ErrorMissingStepID,
		},
		{
			name: validFlowName("invalid step id"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].ID = "invalid/id"
			}),
			wantCode: ErrorInvalidStepID,
		},
		{
			name: validFlowName("missing step title"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].Title = ""
			}),
			wantCode: ErrorMissingStepTitle,
		},
		{
			name: validFlowName("missing step outcome"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].Outcome = ""
			}),
			wantCode: ErrorMissingStepOutcome,
		},
		{
			name:     "blank step outcome",
			flow:     validFlow(func(flow *Flow) { flow.Steps[0].Outcome = " \t\n " }),
			wantCode: ErrorMissingStepOutcome,
		},
		{
			name: validFlowName("duplicate step id"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps = append(flow.Steps, Step{
					ID:        flow.Steps[0].ID,
					Title:     "Duplicate",
					Outcome:   "Duplicate outcome.",
					Artifacts: []Artifact{},
				})
			}),
			wantCode: ErrorDuplicateStepID,
		},
		{
			name: validFlowName("missing artifact path"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].Artifacts = []Artifact{{}}
			}),
			wantCode: ErrorMissingArtifactPath,
		},
		{
			name: validFlowName("blank artifact path"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].Artifacts = []Artifact{{Path: "   ", Required: true}}
			}),
			wantCode: ErrorMissingArtifactPath,
		},
		{
			name: validFlowName("invalid artifact path"),
			flow: validFlow(func(flow *Flow) {
				flow.Steps[0].Artifacts = []Artifact{{Path: "../secret.md", Required: true}}
			}),
			wantCode: ErrorInvalidArtifactPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.flow)
			assertValidationErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestValidateAcceptsValidFlow(t *testing.T) {
	for _, purpose := range []string{"Record review findings separately from approval.", "", " \t\n "} {
		t.Run(purpose, func(t *testing.T) {
			flow := validFlow(func(flow *Flow) { flow.Steps[0].Purpose = purpose })
			if err := Validate(flow); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	flow := validFlow(nil)

	if err := ValidateFilename(flow, "/tmp/test-flow.cue"); err != nil {
		t.Fatal(err)
	}

	err := ValidateFilename(flow, "/tmp/other-flow.cue")
	assertValidationErrorCode(t, err, ErrorFlowIDFilenameMismatch)
}

func validFlow(mutator func(*Flow)) Flow {
	flow := Flow{
		Purpose: "Test flow purpose",
		ID:      "test-flow",
		Title:   "Test Flow",
		Steps: []Step{
			{
				ID:        "first_step",
				Title:     "First Step",
				Outcome:   "Do first thing.",
				Artifacts: []Artifact{},
			},
		},
	}
	if mutator != nil {
		mutator(&flow)
	}
	return flow
}

func validFlowName(name string) string {
	return name
}

func assertValidationErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()

	got, ok := ErrorCodeOf(err)
	if !ok {
		t.Fatalf("error = %v, want validation error %q", err, want)
	}
	if got != want {
		t.Fatalf("error code = %q, want %q", got, want)
	}
}
