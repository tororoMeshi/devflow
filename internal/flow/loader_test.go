package flow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		cue      string
		wantFlow Flow
	}{
		{
			name:     "loads minimal flow",
			fileName: "test-flow.cue",
			cue: `flow: {
				purpose: "Test flow purpose"
				id: "test-flow"
				title: "Test Flow"
				steps: [{
					id: "first"
					title: "First"
					outcome: "Do first thing."
				}]
			}`,
			wantFlow: Flow{
				Purpose: "Test flow purpose",
				ID:      "test-flow",
				Title:   "Test Flow",
				Steps: []Step{
					{
						ID:        "first",
						Title:     "First",
						Outcome:   "Do first thing.",
						Artifacts: []Artifact{},
					},
				},
			},
		},
		{
			name:     "loads purpose artifacts approvals and preserves step order",
			fileName: "post-task-review.cue",
			cue: `flow: {
				id: "post-task-review"
				title: "Post Task Review"
				purpose: "Review after a task."
				steps: [
					{
						id: "check_changes"
						title: "Check Changes"
						outcome: "Check git status."
						artifacts: [
							{path: "docs/code-review.md"},
							{path: "docs/optional.md", required: false},
						]
					},
					{
						id: "human_approval"
						title: "Human Approval"
						outcome: "Ask for approval."
						approval: {required: true}
					},
				]
			}`,
			wantFlow: Flow{
				ID:      "post-task-review",
				Title:   "Post Task Review",
				Purpose: "Review after a task.",
				Steps: []Step{
					{
						ID:      "check_changes",
						Title:   "Check Changes",
						Outcome: "Check git status.",
						Artifacts: []Artifact{
							{Path: "docs/code-review.md", Required: true},
							{Path: "docs/optional.md", Required: false},
						},
					},
					{
						ID:        "human_approval",
						Title:     "Human Approval",
						Outcome:   "Ask for approval.",
						Artifacts: []Artifact{},
						Approval:  &Approval{Required: true},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFlowFile(t, tt.fileName, tt.cue)

			got, err := LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			assertFlowEqual(t, got, tt.wantFlow)
		})
	}
}

func TestLoadRejectsLegacyInstructionWithoutOutcome(t *testing.T) {
	_, err := Load([]byte(`flow: {
		purpose: "Test flow purpose"
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
		}]
	}`))

	assertValidationErrorCode(t, err, ErrorMissingStepOutcome)
}

func TestLoadFileReturnsErrorForFilenameMismatch(t *testing.T) {
	path := writeFlowFile(t, "actual-file.cue", `flow: {
		purpose: "Test flow purpose"
		id: "different-id"
		title: "Different"
		steps: [{
			id: "first"
			title: "First"
			outcome: "Do first thing."
		}]
	}`)

	_, err := LoadFile(path)

	assertValidationErrorCode(t, err, ErrorFlowIDFilenameMismatch)
}

func TestLoadFileReturnsValidationErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		cue      string
		wantCode ErrorCode
	}{
		{
			name: "missing flow id",
			cue: `flow: {
				purpose: "Test flow purpose"
				title: "Test Flow"
				steps: [{
					id: "first"
					title: "First"
					outcome: "Do first thing."
				}]
			}`,
			wantCode: ErrorMissingFlowID,
		},
		{
			name: "missing flow title",
			cue: `flow: {
				purpose: "Test flow purpose"
				id: "test-flow"
				steps: [{
					id: "first"
					title: "First"
					outcome: "Do first thing."
				}]
			}`,
			wantCode: ErrorMissingFlowTitle,
		},
		{
			name: "missing steps",
			cue: `flow: {
				purpose: "Test flow purpose"
				id: "test-flow"
				title: "Test Flow"
			}`,
			wantCode: ErrorFlowHasNoSteps,
		},
		{
			name: "missing artifact path",
			cue: `flow: {
				purpose: "Test flow purpose"
				id: "test-flow"
				title: "Test Flow"
				steps: [{
					id: "first"
					title: "First"
					outcome: "Do first thing."
					artifacts: [{}]
				}]
			}`,
			wantCode: ErrorMissingArtifactPath,
		},
		{
			name: "invalid artifact path",
			cue: `flow: {
				purpose: "Test flow purpose"
				id: "test-flow"
				title: "Test Flow"
				steps: [{
					id: "first"
					title: "First"
					outcome: "Do first thing."
					artifacts: [{path: "../secret.md"}]
				}]
			}`,
			wantCode: ErrorInvalidArtifactPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFlowFile(t, "test-flow.cue", tt.cue)

			_, err := LoadFile(path)

			assertValidationErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestLoadDirContinuesWhenValidAndInvalidFlowsAreMixed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".devflow", "flows")
	writeFile(t, filepath.Join(dir, "valid-flow.cue"), `flow: {
		purpose: "Test flow purpose"
		id: "valid-flow"
		title: "Valid"
		steps: [{
			id: "first"
			title: "First"
			outcome: "Do first thing."
		}]
	}`)
	writeFile(t, filepath.Join(dir, "invalid-flow.cue"), `flow: {
		purpose: "Test flow purpose"
		id: "invalid-flow"
		title: "Invalid"
		steps: []
	}`)

	results, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	var validCount, invalidCount int
	for _, result := range results {
		switch result.Status {
		case FlowFileValid:
			validCount++
			if result.Flow == nil {
				t.Fatalf("valid result has nil Flow")
			}
		case FlowFileInvalid:
			invalidCount++
			if result.Err == nil {
				t.Fatalf("invalid result has nil Err")
			}
		default:
			t.Fatalf("unexpected status %q", result.Status)
		}
	}
	if validCount != 1 || invalidCount != 1 {
		t.Fatalf("validCount = %d, invalidCount = %d, want 1 and 1", validCount, invalidCount)
	}
}

func writeFlowFile(t *testing.T, fileName string, content string) string {
	t.Helper()

	root := t.TempDir()
	path := filepath.Join(root, ".devflow", "flows", fileName)
	writeFile(t, path, content)
	return path
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
