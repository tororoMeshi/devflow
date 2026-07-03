package flow

import "testing"

func TestNormalizeMakesMissingArtifactsEmptySlice(t *testing.T) {
	flow := Flow{
		ID:    "test-flow",
		Title: "Test Flow",
		Steps: []Step{
			{
				ID:          "first",
				Title:       "First",
				Instruction: "Do first thing.",
			},
		},
	}

	got := Normalize(flow)

	if got.Steps[0].Artifacts == nil {
		t.Fatalf("Artifacts is nil")
	}
}

func TestLoadDefaultsArtifactRequiredToTrue(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			artifacts: [{path: "README.md"}]
		}]
	}`)

	if !got.Steps[0].Artifacts[0].Required {
		t.Fatalf("Required = false, want true")
	}
}

func TestLoadKeepsArtifactRequiredFalse(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			artifacts: [{path: "README.md", required: false}]
		}]
	}`)

	if got.Steps[0].Artifacts[0].Required {
		t.Fatalf("Required = true, want false")
	}
}

func TestLoadDefaultsApprovalRequiredToFalse(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			approval: {}
		}]
	}`)

	if got.Steps[0].Approval == nil {
		t.Fatalf("Approval is nil")
	}
	if got.Steps[0].Approval.Required {
		t.Fatalf("Approval.Required = true, want false")
	}
}

func TestLoadKeepsMissingApprovalNil(t *testing.T) {
	got := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
		}]
	}`)

	if got.Steps[0].Approval != nil {
		t.Fatalf("Approval = %#v, want nil", got.Steps[0].Approval)
	}
}

func loadFlowFromString(t *testing.T, content string) Flow {
	t.Helper()

	got, err := Load([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func assertFlowEqual(t *testing.T, got Flow, want Flow) {
	t.Helper()

	if got.ID != want.ID {
		t.Fatalf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Title != want.Title {
		t.Fatalf("Title = %q, want %q", got.Title, want.Title)
	}
	if got.Description != want.Description {
		t.Fatalf("Description = %q, want %q", got.Description, want.Description)
	}
	if len(got.Steps) != len(want.Steps) {
		t.Fatalf("len(Steps) = %d, want %d", len(got.Steps), len(want.Steps))
	}
	for i := range want.Steps {
		assertStepEqual(t, got.Steps[i], want.Steps[i])
	}
}

func assertStepEqual(t *testing.T, got Step, want Step) {
	t.Helper()

	if got.ID != want.ID {
		t.Fatalf("Step.ID = %q, want %q", got.ID, want.ID)
	}
	if got.Title != want.Title {
		t.Fatalf("Step.Title = %q, want %q", got.Title, want.Title)
	}
	if got.Instruction != want.Instruction {
		t.Fatalf("Step.Instruction = %q, want %q", got.Instruction, want.Instruction)
	}
	if len(got.Artifacts) != len(want.Artifacts) {
		t.Fatalf("len(Step.Artifacts) = %d, want %d", len(got.Artifacts), len(want.Artifacts))
	}
	for i := range want.Artifacts {
		if got.Artifacts[i] != want.Artifacts[i] {
			t.Fatalf("Artifact[%d] = %#v, want %#v", i, got.Artifacts[i], want.Artifacts[i])
		}
	}
	if (got.Approval == nil) != (want.Approval == nil) {
		t.Fatalf("Approval = %#v, want %#v", got.Approval, want.Approval)
	}
	if got.Approval != nil && *got.Approval != *want.Approval {
		t.Fatalf("Approval = %#v, want %#v", got.Approval, want.Approval)
	}
}
