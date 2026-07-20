package flow

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestBuildSnapshotIsReproducible(t *testing.T) {
	flow := snapshotTestFlow()
	first, err := BuildSnapshot(flow, FlowSource{Path: "flows/test.cue"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot(flow, FlowSource{Path: "flows/test.cue"})
	if err != nil {
		t.Fatal(err)
	}

	firstCanonical, err := canonicalJSON(first.Flow)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := canonicalJSON(second.Flow)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstCanonical) != string(secondCanonical) {
		t.Fatalf("canonical payloads differ:\n%s\n%s", firstCanonical, secondCanonical)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digest = %q, want %q", first.Digest, second.Digest)
	}
}

func TestBuildSnapshotDigestExcludesSource(t *testing.T) {
	flow := snapshotTestFlow()
	first, err := BuildSnapshot(flow, FlowSource{Path: "flows/first.cue"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot(flow, FlowSource{Path: "other/second.cue"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Digest != second.Digest {
		t.Fatalf("digest depends on source: %q != %q", first.Digest, second.Digest)
	}
	if first.Source.Path != "flows/first.cue" || second.Source.Path != "other/second.cue" {
		t.Fatalf("sources = %#v, %#v", first.Source, second.Source)
	}
}

func TestValidateSnapshot(t *testing.T) {
	snapshot, err := BuildSnapshot(snapshotTestFlow(), FlowSource{Path: "flows/test.cue"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("ValidateSnapshot() error = %v", err)
	}
	before := CloneSnapshot(snapshot)
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("ValidateSnapshot() second error = %v", err)
	}
	if !reflect.DeepEqual(snapshot, before) {
		t.Fatal("ValidateSnapshot modified its input")
	}

	tests := []struct {
		name string
		edit func(*FlowSnapshot)
		want error
	}{
		{"schema", func(s *FlowSnapshot) { s.SchemaVersion++ }, ErrUnsupportedSnapshotSchema},
		{"digest format", func(s *FlowSnapshot) { s.Digest = "sha256:BAD" }, ErrInvalidSnapshotDigest},
		{"digest mismatch", func(s *FlowSnapshot) { s.Digest = "sha256:" + strings.Repeat("0", 64) }, ErrSnapshotDigestMismatch},
		{"flow changed", func(s *FlowSnapshot) { s.Flow.Title = "changed" }, ErrSnapshotDigestMismatch},
		{"not normalized", func(s *FlowSnapshot) { s.Flow.Steps[0].Inputs = nil }, ErrSnapshotNotNormalized},
		{"invalid flow", func(s *FlowSnapshot) { s.Flow.ID = "" }, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshot
			got.Flow = copyFlow(snapshot.Flow)
			tt.edit(&got)
			err := ValidateSnapshot(got)
			if err == nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSnapshot() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCloneSnapshotDoesNotShareFlowMemory(t *testing.T) {
	snapshot, err := BuildSnapshot(snapshotTestFlow(), FlowSource{Path: "flows/test.cue"})
	if err != nil {
		t.Fatal(err)
	}
	clone := CloneSnapshot(snapshot)
	if !reflect.DeepEqual(clone, snapshot) {
		t.Fatalf("CloneSnapshot = %#v, want %#v", clone, snapshot)
	}
	clone.Flow.Steps[0].Inputs[0].Path = "changed-input"
	clone.Flow.Steps[0].Approval.Required = false
	clone.Flow.Steps[0].RequiredChecks[0] = "changed-check"
	if snapshot.Flow.Steps[0].Inputs[0].Path == "changed-input" || !snapshot.Flow.Steps[0].Approval.Required || snapshot.Flow.Steps[0].RequiredChecks[0] == "changed-check" {
		t.Fatal("CloneSnapshot shares Flow memory")
	}
}

func TestBuildSnapshotTreatsNilAndEmptySlicesEqually(t *testing.T) {
	nilSlices := Flow{
		ID: "test-flow", Title: "Test Flow",
		Steps: []Step{{ID: "first", Title: "First", Instruction: "Do first thing."}},
	}
	emptySlices := Flow{
		ID: "test-flow", Title: "Test Flow",
		Steps: []Step{{
			ID: "first", Title: "First", Instruction: "Do first thing.",
			Inputs: []Artifact{}, Artifacts: []Artifact{}, RequiredChecks: []string{},
		}},
	}

	first, err := BuildSnapshot(nilSlices, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot(emptySlices, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digest = %q, want %q", first.Digest, second.Digest)
	}
}

func TestBuildSnapshotDoesNotModifyInput(t *testing.T) {
	flow := Flow{
		ID: "test-flow", Title: "Test Flow",
		Steps: []Step{{ID: "first", Title: "First", Instruction: "Do first thing."}},
	}

	if _, err := BuildSnapshot(flow, FlowSource{}); err != nil {
		t.Fatal(err)
	}
	if flow.Steps[0].Inputs != nil {
		t.Fatalf("input Inputs was modified: %#v", flow.Steps[0].Inputs)
	}
	if flow.Steps[0].Artifacts != nil {
		t.Fatalf("input Artifacts was modified: %#v", flow.Steps[0].Artifacts)
	}
	if flow.Steps[0].RequiredChecks != nil {
		t.Fatalf("input RequiredChecks was modified: %#v", flow.Steps[0].RequiredChecks)
	}
}

func TestBuildSnapshotCUEDefaultMatchesExplicitValue(t *testing.T) {
	omitted := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			artifacts: [{path: "result.txt"}]
		}]
	}`)
	explicit := loadFlowFromString(t, `flow: {
		id: "test-flow"
		title: "Test Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first thing."
			artifacts: [{path: "result.txt", required: true}]
		}]
	}`)

	first, err := BuildSnapshot(omitted, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot(explicit, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digest = %q, want %q", first.Digest, second.Digest)
	}
}

func TestBuildSnapshotDigestChangesWithContractFields(t *testing.T) {
	base := snapshotTestFlow()
	for _, tt := range []struct {
		name   string
		mutate func(*Flow)
	}{
		{"flow ID", func(flow *Flow) { flow.ID = "other-flow" }},
		{"flow title", func(flow *Flow) { flow.Title = "Other Flow" }},
		{"flow description", func(flow *Flow) { flow.Description = "Other description." }},
		{"step ID", func(flow *Flow) { flow.Steps[0].ID = "other-first" }},
		{"step title", func(flow *Flow) { flow.Steps[0].Title = "Other First" }},
		{"step instruction", func(flow *Flow) { flow.Steps[0].Instruction = "Other instruction." }},
		{"step order", func(flow *Flow) { flow.Steps[0], flow.Steps[1] = flow.Steps[1], flow.Steps[0] }},
		{"input path", func(flow *Flow) { flow.Steps[0].Inputs[0].Path = "other-input.txt" }},
		{"input required", func(flow *Flow) { flow.Steps[0].Inputs[0].Required = false }},
		{"artifact path", func(flow *Flow) { flow.Steps[0].Artifacts[0].Path = "other-artifact.txt" }},
		{"artifact required", func(flow *Flow) { flow.Steps[0].Artifacts[0].Required = false }},
		{"approval", func(flow *Flow) { flow.Steps[0].Approval.Required = false }},
		{"required check", func(flow *Flow) { flow.Steps[0].RequiredChecks[0] = "go-vet" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original, err := BuildSnapshot(base, FlowSource{})
			if err != nil {
				t.Fatal(err)
			}
			changed := copyFlow(base)
			tt.mutate(&changed)
			updated, err := BuildSnapshot(changed, FlowSource{})
			if err != nil {
				t.Fatal(err)
			}
			if original.Digest == updated.Digest {
				t.Fatalf("digest did not change: %q", original.Digest)
			}
		})
	}
}

func TestBuildSnapshotDeepCopiesInput(t *testing.T) {
	flow := snapshotTestFlow()
	wantInput, err := json.Marshal(flow)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := BuildSnapshot(flow, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	gotInput, err := json.Marshal(flow)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotInput) != string(wantInput) {
		t.Fatalf("input Flow changed while building snapshot: %s, want %s", gotInput, wantInput)
	}
	wantFlow := copyFlow(snapshot.Flow)
	wantDigest := snapshot.Digest

	flow.Steps[0].Title = "Changed title"
	flow.Steps[0].Instruction = "Changed instruction"
	flow.Steps[0].Inputs[0] = Artifact{Path: "changed-input.txt", Required: false}
	flow.Steps[0].Artifacts[0] = Artifact{Path: "changed-artifact.txt", Required: false}
	flow.Steps[0].RequiredChecks[0] = "changed-check"
	flow.Steps[0].Approval.Required = false

	if !reflect.DeepEqual(snapshot.Flow, wantFlow) {
		t.Fatalf("snapshot Flow changed: %#v", snapshot.Flow)
	}
	if snapshot.Digest != wantDigest {
		t.Fatalf("snapshot digest changed: %q", snapshot.Digest)
	}
}

func TestBuildSnapshotRejectsInvalidFlow(t *testing.T) {
	for _, tt := range []struct {
		name     string
		flow     Flow
		wantCode ErrorCode
	}{
		{"invalid ID", Flow{ID: "invalid id", Title: "Test Flow", Steps: []Step{{ID: "first", Title: "First", Instruction: "Do first thing."}}}, ErrorInvalidFlowID},
		{"no steps", Flow{ID: "test-flow", Title: "Test Flow"}, ErrorFlowHasNoSteps},
		{"duplicate step ID", Flow{ID: "test-flow", Title: "Test Flow", Steps: []Step{{ID: "first", Title: "First", Instruction: "Do first thing."}, {ID: "first", Title: "Second", Instruction: "Do second thing."}}}, ErrorDuplicateStepID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildSnapshot(tt.flow, FlowSource{})
			if err == nil {
				t.Fatal("BuildSnapshot succeeded, want error")
			}
			if gotCode, ok := ErrorCodeOf(err); !ok || gotCode != tt.wantCode {
				t.Fatalf("ErrorCodeOf(err) = %q, %t, want %q, true", gotCode, ok, tt.wantCode)
			}
		})
	}
}

func TestBuildSnapshotSchemaVersionAndDigestFormat(t *testing.T) {
	snapshot, err := BuildSnapshot(snapshotTestFlow(), FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != FlowSnapshotSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", snapshot.SchemaVersion, FlowSnapshotSchemaVersion)
	}
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(snapshot.Digest) {
		t.Fatalf("Digest = %q, want sha256:<64 lowercase hex characters>", snapshot.Digest)
	}
	canonical, err := canonicalJSON(snapshot.Flow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(canonical), `"schema_version":1`) {
		t.Fatalf("canonical payload does not include schema version: %s", canonical)
	}
}

func TestFlowSnapshotJSON(t *testing.T) {
	flow := Flow{
		ID: "test-flow", Title: "Test Flow",
		Steps: []Step{{ID: "first", Title: "First", Instruction: "Do first thing."}},
	}
	snapshot, err := BuildSnapshot(flow, FlowSource{Path: ".devflow/flows/test-flow.cue"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"digest":"` + snapshot.Digest + `","source":{"path":".devflow/flows/test-flow.cue"},"flow":{"id":"test-flow","title":"Test Flow","description":"","steps":[{"id":"first","title":"First","instruction":"Do first thing.","inputs":[],"artifacts":[],"approval":null,"required_checks":[]}]}}`
	if string(data) != want {
		t.Fatalf("snapshot JSON = %s\nwant %s", data, want)
	}
}

func TestCanonicalJSON(t *testing.T) {
	flow := Flow{
		ID: "test-flow", Title: "Test Flow", Description: "Test description.",
		Steps: []Step{{
			ID: "first", Title: "First", Instruction: "Do first thing.",
			Inputs:    []Artifact{{Path: "input.txt", Required: true}},
			Artifacts: []Artifact{{Path: "result.txt", Required: false}},
			Approval:  &Approval{Required: true}, RequiredChecks: []string{"go-test"},
		}},
	}

	canonical, err := canonicalJSON(Normalize(copyFlow(flow)))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"flow":{"id":"test-flow","title":"Test Flow","description":"Test description.","steps":[{"id":"first","title":"First","instruction":"Do first thing.","inputs":[{"path":"input.txt","required":true}],"artifacts":[{"path":"result.txt","required":false}],"approval":{"required":true},"required_checks":["go-test"]}]}}`
	if string(canonical) != want {
		t.Fatalf("canonical JSON = %s\nwant %s", canonical, want)
	}
	snapshot, err := BuildSnapshot(flow, FlowSource{})
	if err != nil {
		t.Fatal(err)
	}
	const wantDigest = "sha256:d2ce416540a41193f90915615848d633a8be58adce711e5e28dcba815980c113"
	if snapshot.Digest != wantDigest {
		t.Fatalf("Digest = %q, want %q", snapshot.Digest, wantDigest)
	}
}

func snapshotTestFlow() Flow {
	return Flow{
		ID: "test-flow", Title: "Test Flow", Description: "Test description.",
		Steps: []Step{
			{
				ID: "first", Title: "First", Instruction: "Do first thing.",
				Inputs:    []Artifact{{Path: "input.txt", Required: true}},
				Artifacts: []Artifact{{Path: "artifact.txt", Required: true}},
				Approval:  &Approval{Required: true}, RequiredChecks: []string{"go-test"},
			},
			{ID: "second", Title: "Second", Instruction: "Do second thing."},
		},
	}
}
