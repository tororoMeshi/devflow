package flow

import (
	"fmt"
	"testing"
)

func TestLoadPurposeAndOutcome(t *testing.T) {
	tests := []struct {
		name        string
		flowFields  string
		stepFields  string
		stepPurpose string
		wantCode    ErrorCode
	}{
		{
			name:        "flow purpose step purpose and outcome",
			flowFields:  `purpose: "Keep releases reviewable."`,
			stepFields:  `purpose: "Record findings before approval.", outcome: "Review findings are recorded."`,
			stepPurpose: "Record findings before approval.",
		},
		{
			name:       "step purpose is optional",
			flowFields: `purpose: "Keep releases reviewable."`,
			stepFields: `outcome: "Review findings are recorded."`,
		},
		{
			name:       "missing flow purpose",
			stepFields: `outcome: "Review findings are recorded."`,
			wantCode:   ErrorMissingFlowPurpose,
		},
		{
			name:       "blank flow purpose",
			flowFields: `purpose: " \t\n "`,
			stepFields: `outcome: "Review findings are recorded."`,
			wantCode:   ErrorMissingFlowPurpose,
		},
		{
			name:       "missing step outcome",
			flowFields: `purpose: "Keep releases reviewable."`,
			wantCode:   ErrorMissingStepOutcome,
		},
		{
			name:       "blank step outcome",
			flowFields: `purpose: "Keep releases reviewable."`,
			stepFields: `outcome: " \t\n "`,
			wantCode:   ErrorMissingStepOutcome,
		},
		{
			name:       "step purpose does not supply outcome",
			flowFields: `purpose: "Keep releases reviewable."`,
			stepFields: `purpose: "Record findings before approval."`,
			wantCode:   ErrorMissingStepOutcome,
		},
		{
			name:       "legacy description and objective only",
			flowFields: `description: "Review a release."`,
			stepFields: `objective: "Write a review."`,
			wantCode:   ErrorMissingFlowPurpose,
		},
		{
			name:       "description does not supply flow purpose",
			flowFields: `description: "Review a release."`,
			stepFields: `outcome: "Review findings are recorded."`,
			wantCode:   ErrorMissingFlowPurpose,
		},
		{
			name:       "objective does not supply step outcome",
			flowFields: `purpose: "Keep releases reviewable."`,
			stepFields: `objective: "Write a review."`,
			wantCode:   ErrorMissingStepOutcome,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := fmt.Sprintf(`flow: {
				id: "review"
				title: "Release Review"
				%s
				steps: [{id: "findings", title: "Record Findings", %s}]
			}`, tt.flowFields, tt.stepFields)
			got, err := Load([]byte(data))
			if tt.wantCode != "" {
				assertValidationErrorCode(t, err, tt.wantCode)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Purpose != "Keep releases reviewable." {
				t.Fatalf("Flow.Purpose = %q", got.Purpose)
			}
			step := got.Steps[0]
			if step.Purpose != tt.stepPurpose || step.Outcome != "Review findings are recorded." {
				t.Fatalf("Step Purpose/Outcome = %q/%q", step.Purpose, step.Outcome)
			}
		})
	}
}

func TestNormalizePreservesPurposeAndOutcome(t *testing.T) {
	for _, stepPurpose := range []string{" Record findings before approval. ", ""} {
		t.Run(stepPurpose, func(t *testing.T) {
			input := Flow{
				ID: "review", Title: "Release Review", Purpose: " Keep releases reviewable. ",
				Steps: []Step{{ID: "findings", Title: "Record Findings", Purpose: stepPurpose, Outcome: " Review findings are recorded. "}},
			}
			got := Normalize(input)
			assertFlowEqual(t, got, input)
			if err := Validate(got); err != nil {
				t.Fatal(err)
			}
		})
	}
}
