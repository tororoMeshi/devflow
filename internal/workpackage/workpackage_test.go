package workpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/state"
	"github.com/tororoMeshi/devflow/internal/task"
)

func TestWorkPackageJSONFieldContract(t *testing.T) {
	assertFields := func(typ reflect.Type, want []string) {
		t.Helper()
		if typ.NumField() != len(want) {
			t.Fatalf("%s has %d fields, want %d", typ, typ.NumField(), len(want))
		}
		for i, name := range want {
			tag := typ.Field(i).Tag.Get("json")
			if tag != name || strings.Contains(tag, "omitempty") || typ.Field(i).Type.Kind() == reflect.Pointer || typ.Field(i).Type.Kind() == reflect.Map {
				t.Fatalf("%s field %d tag/type = %q/%s, want required %q", typ, i, tag, typ.Field(i).Type, name)
			}
		}
	}
	assertFields(reflect.TypeOf(WorkPackage{}), []string{
		"schema_version", "work_package_digest", "flow_run_id", "step_id", "attempt_id",
		"entry_sequence", "flow_snapshot_digest", "task_snapshot_digest", "task_content",
		"working_root", "step",
	})
	assertFields(reflect.TypeOf(StepContract{}), []string{
		"title", "instruction", "inputs", "artifacts", "required_checks", "approval_required",
	})
	assertFields(reflect.TypeOf(ArtifactContract{}), []string{"path", "required"})
}

func TestCanonicalPayloadAndDigestGolden(t *testing.T) {
	pkg := testPackage(t)
	const want = `{"schema_version":1,"flow_run_id":"run_0123456789abcdef0123456789abcdef","step_id":"build","attempt_id":"attempt_00000000000000000003","entry_sequence":3,"flow_snapshot_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","task_snapshot_digest":"sha256:86c83594f69ce69c5586c05ba69cfe02668bbad87e28acedc05a324d30b5319a","task_content":"Build \u003ccomponent\u003e.\n日本語","working_root":".","step":{"title":"Build","instruction":"Implement \u0026 verify.","inputs":[{"path":"docs/spec.md","required":true},{"path":"docs/optional.md","required":false}],"artifacts":[{"path":"out/component.bin","required":true}],"required_checks":["go-test","go-vet"],"approval_required":true}}`
	canonical, err := canonicalJSON(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != want {
		t.Fatalf("canonical JSON:\n%s\nwant:\n%s", canonical, want)
	}
	if strings.Contains(string(canonical), "work_package_digest") || strings.HasSuffix(string(canonical), "\n") {
		t.Fatalf("invalid canonical payload %q", canonical)
	}
	sum := sha256.Sum256([]byte(want))
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	got, err := Digest(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantDigest {
		t.Fatalf("Digest() = %q, want independently calculated %q", got, wantDigest)
	}
	const wantDigestLiteral = "sha256:c630020bfbdb9e88609c8eedaaba3f8a09733c2e9ec5680941dc491458f42cd8"
	if got != wantDigestLiteral {
		t.Fatalf("Digest() = %q, want literal vector %q", got, wantDigestLiteral)
	}
	pkg.WorkPackageDigest = strings.Repeat("x", 71)
	again, err := Digest(pkg)
	if err != nil || again != got {
		t.Fatalf("digest field affected Digest(): %q, %v", again, err)
	}
	pkg.Step.Instruction += " changed"
	changed, err := Digest(pkg)
	if err != nil || changed == got {
		t.Fatalf("payload change did not change digest: %q, %v", changed, err)
	}
}

func TestWorkPackageFullJSONGolden(t *testing.T) {
	pkg := testPackage(t)
	pkg.WorkPackageDigest = "sha256:c630020bfbdb9e88609c8eedaaba3f8a09733c2e9ec5680941dc491458f42cd8"
	const want = `{"schema_version":1,"work_package_digest":"sha256:c630020bfbdb9e88609c8eedaaba3f8a09733c2e9ec5680941dc491458f42cd8","flow_run_id":"run_0123456789abcdef0123456789abcdef","step_id":"build","attempt_id":"attempt_00000000000000000003","entry_sequence":3,"flow_snapshot_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","task_snapshot_digest":"sha256:86c83594f69ce69c5586c05ba69cfe02668bbad87e28acedc05a324d30b5319a","task_content":"Build \u003ccomponent\u003e.\n日本語","working_root":".","step":{"title":"Build","instruction":"Implement \u0026 verify.","inputs":[{"path":"docs/spec.md","required":true},{"path":"docs/optional.md","required":false}],"artifacts":[{"path":"out/component.bin","required":true}],"required_checks":["go-test","go-vet"],"approval_required":true}}`
	data, err := json.Marshal(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("full JSON:\n%s\nwant:\n%s", data, want)
	}
}

func TestCanonicalEmptySlicesHTMLUnicodeAndDeterminism(t *testing.T) {
	st := testState(t, 1, "build")
	st.FlowSnapshot.Flow.Steps[0].Inputs = []flow.Artifact{}
	st.FlowSnapshot.Flow.Steps[0].Artifacts = []flow.Artifact{}
	st.FlowSnapshot.Flow.Steps[0].RequiredChecks = []string{}
	st.FlowSnapshot, _ = flow.BuildSnapshot(st.FlowSnapshot.Flow, st.FlowSnapshot.Source)
	first, err := Generate(st, "build", st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	second, _ := Generate(st, "build", st.CurrentAttemptID)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("repeated generation is not byte-identical")
	}
	text := string(a)
	for _, want := range []string{`"inputs":[]`, `"artifacts":[]`, `"required_checks":[]`, `\u003ccomponent\u003e`, "日本語"} {
		if !strings.Contains(text, want) {
			t.Fatalf("JSON %q does not contain %q", text, want)
		}
	}
	if strings.Contains(text, ":null") {
		t.Fatalf("JSON contains null: %s", text)
	}
}

func TestValidateWorkPackage(t *testing.T) {
	valid := testPackage(t)
	digest, _ := Digest(valid)
	valid.WorkPackageDigest = digest
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		edit func(*WorkPackage)
		want error
	}{
		{"schema", func(p *WorkPackage) { p.SchemaVersion = 2 }, ErrUnsupportedSchema},
		{"digest format", func(p *WorkPackage) { p.WorkPackageDigest = "bad" }, ErrInvalidDigest},
		{"digest mismatch", func(p *WorkPackage) { p.TitleForTest() }, ErrDigestMismatch},
		{"Run ID", func(p *WorkPackage) { p.FlowRunID = "run_bad" }, ErrInvalidFlowRunID},
		{"Step ID", func(p *WorkPackage) { p.StepID = "bad id" }, ErrInvalidStepID},
		{"Attempt ID", func(p *WorkPackage) { p.AttemptID = "attempt_bad" }, ErrInvalidAttemptID},
		{"entry sequence", func(p *WorkPackage) { p.EntrySequence = 0 }, ErrInvalidEntrySequence},
		{"flow digest", func(p *WorkPackage) { p.FlowSnapshotDigest = "bad" }, ErrInvalidSnapshotDigest},
		{"task digest", func(p *WorkPackage) { p.TaskSnapshotDigest = "sha256:" + strings.Repeat("0", 64) }, ErrInvalidSnapshotDigest},
		{"working root", func(p *WorkPackage) { p.WorkingRoot = "/tmp" }, ErrInvalidWorkingRoot},
		{"nil inputs", func(p *WorkPackage) { p.Step.Inputs = nil }, ErrNilContractCollection},
		{"nil artifacts", func(p *WorkPackage) { p.Step.Artifacts = nil }, ErrNilContractCollection},
		{"nil checks", func(p *WorkPackage) { p.Step.RequiredChecks = nil }, ErrNilContractCollection},
		{"artifact path", func(p *WorkPackage) { p.Step.Artifacts[0].Path = "../bad" }, ErrInvalidStepContract},
		{"check ID", func(p *WorkPackage) { p.Step.RequiredChecks[0] = "bad id" }, ErrInvalidStepContract},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clonePackage(valid)
			tt.edit(&got)
			beforeValidation, _ := json.Marshal(got)
			err := Validate(got)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
			afterValidation, _ := json.Marshal(got)
			if string(beforeValidation) != string(afterValidation) {
				t.Fatal("validation mutated input")
			}
		})
	}
}

func (p *WorkPackage) TitleForTest() { p.Step.Title = "Changed" }

func TestGenerateCurrentAttemptAndMutableStateIndependence(t *testing.T) {
	st := testState(t, 1, "build")
	before := st.Clone()
	first, err := Generate(st, "build", st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(st, before) {
		t.Fatal("Generate mutated State")
	}
	mutated := st.Clone()
	attempt := &mutated.Attempts[0]
	attempt.ArtifactEvidence["out/component.bin"] = state.ArtifactEvidence{
		Digest: "sha256:" + strings.Repeat("c", 64),
		Size:   42,
	}
	attempt.CheckResults["go-test"] = state.CheckResult{ExitCode: 0, LogPath: "secret.log"}
	evidenceDigest, err := state.ArtifactEvidenceSetDigest([]string{"out/component.bin"}, attempt.ArtifactEvidence)
	if err != nil {
		t.Fatal(err)
	}
	attempt.Approval = &state.ApprovalRecord{Note: "private note", EvidenceSetDigest: evidenceDigest}
	if err := state.Validate(mutated); err != nil {
		t.Fatal(err)
	}
	second, err := Generate(mutated, "build", mutated.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) || first.WorkPackageDigest != second.WorkPackageDigest {
		t.Fatalf("mutable Attempt state changed package:\n%s\n%s", a, b)
	}
	mutated.Attempts[0].Approval.Note = "different private note"
	third, err := Generate(mutated, "build", mutated.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := json.Marshal(third)
	if string(a) != string(c) || first.WorkPackageDigest != third.WorkPackageDigest {
		t.Fatal("Approval Note changed WorkPackage")
	}
	for _, excluded := range []string{"artifact_evidence", "check_results", `"approval":`, "private note", "secret.log", `"source":`, "/private/"} {
		if strings.Contains(string(a), excluded) {
			t.Fatalf("package leaked %q: %s", excluded, a)
		}
	}
}

func TestGenerateDoesNotShareSnapshotSlices(t *testing.T) {
	st := testState(t, 1, "build")
	st.FlowSnapshot.Flow.Steps[0].Inputs = []flow.Artifact{{Path: "docs/input.md", Required: true}}
	var err error
	st.FlowSnapshot, err = flow.BuildSnapshot(st.FlowSnapshot.Flow, st.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Generate(st, "build", st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}

	pkg.Step.Inputs[0].Path = "changed/input"
	pkg.Step.Artifacts[0].Path = "changed/artifact"
	pkg.Step.RequiredChecks[0] = "changed-check"
	step := st.FlowSnapshot.Flow.Steps[0]
	if step.Inputs[0].Path != "docs/input.md" || step.Artifacts[0].Path != "out/component.bin" || step.RequiredChecks[0] != "go-test" {
		t.Fatal("WorkPackage slices share storage with FlowSnapshot")
	}

	st.FlowSnapshot.Flow.Steps[0].Inputs[0].Path = "state/input"
	st.FlowSnapshot.Flow.Steps[0].Artifacts[0].Path = "state/artifact"
	st.FlowSnapshot.Flow.Steps[0].RequiredChecks[0] = "state-check"
	if pkg.Step.Inputs[0].Path != "changed/input" || pkg.Step.Artifacts[0].Path != "changed/artifact" || pkg.Step.RequiredChecks[0] != "changed-check" {
		t.Fatal("generated WorkPackage changed after State slice mutation")
	}
}

func TestGenerateDoesNotInspectMissingInputOrArtifact(t *testing.T) {
	st := testState(t, 1, "build")
	st.FlowSnapshot.Flow.Steps[0].Inputs = []flow.Artifact{{Path: "missing/input.md", Required: true}}
	st.FlowSnapshot.Flow.Steps[0].Artifacts = []flow.Artifact{{Path: "missing/output.bin", Required: true}}
	var err error
	st.FlowSnapshot, err = flow.BuildSnapshot(st.FlowSnapshot.Flow, st.FlowSnapshot.Source)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Generate(st, "build", st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Step.Inputs[0].Path != "missing/input.md" || pkg.Step.Artifacts[0].Path != "missing/output.bin" {
		t.Fatalf("projection = %#v", pkg.Step)
	}
}

func TestGenerateAttemptClassificationAndReentry(t *testing.T) {
	st := testState(t, 2, "review")
	tests := []struct {
		name, step, attempt string
		edit                func(*state.State)
		want                error
	}{
		{"malformed", "review", "bad", nil, ErrInvalidAttemptID},
		{"nonexistent", "review", "attempt_00000000000000000003", nil, ErrAttemptNotFound},
		{"stale", "build", st.Attempts[0].ID, nil, ErrStaleAttempt},
		{"step mismatch", "build", st.CurrentAttemptID, nil, ErrStepAttemptMismatch},
		{"terminal", "review", st.CurrentAttemptID, func(s *state.State) {
			s.Status = state.StatusFinished
			s.Attempts[1], _ = state.CloseStepAttempt(s.Attempts[1], state.StepAttemptExitFinish, "stop")
			s.Finish = &state.Finish{Reason: "stop"}
			s.CurrentAttemptID = ""
		}, ErrNoActiveFlow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := st.Clone()
			if tt.edit != nil {
				tt.edit(&input)
			}
			before := input.Clone()
			_, err := Generate(input, tt.step, tt.attempt)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Generate() error = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("failed Generate mutated State")
			}
		})
	}
	closedCurrent := st.Clone()
	closedCurrent.Attempts[1], _ = state.CloseStepAttempt(closedCurrent.Attempts[1], state.StepAttemptExitBack, "closed")
	if _, err := Generate(closedCurrent, "review", closedCurrent.CurrentAttemptID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("closed current Attempt error = %v, want invalid State rejection", err)
	}

	review, err := Generate(st, "review", st.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if review.Step.ApprovalRequired || len(review.Step.Inputs) != 0 || len(review.Step.Artifacts) != 0 ||
		review.Step.Inputs == nil || review.Step.Artifacts == nil || review.Step.RequiredChecks == nil {
		t.Fatalf("review projection = %#v", review.Step)
	}

	reentered := testState(t, 3, "build")
	old, err := Generate(testState(t, 1, "build"), "build", "attempt_00000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	current, err := Generate(reentered, "build", reentered.CurrentAttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if old.AttemptID == current.AttemptID || old.EntrySequence == current.EntrySequence || old.WorkPackageDigest == current.WorkPackageDigest {
		t.Fatal("A→B→A reentry did not produce a distinct binding")
	}
}

func testPackage(t *testing.T) WorkPackage {
	t.Helper()
	taskSnapshot, err := task.BuildSnapshot("Build <component>.\n日本語", task.TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	return WorkPackage{
		SchemaVersion:      1,
		FlowRunID:          "run_0123456789abcdef0123456789abcdef",
		StepID:             "build",
		AttemptID:          "attempt_00000000000000000003",
		EntrySequence:      3,
		FlowSnapshotDigest: "sha256:" + strings.Repeat("a", 64),
		TaskSnapshotDigest: taskSnapshot.Digest,
		TaskContent:        taskSnapshot.Content,
		WorkingRoot:        ".",
		Step: StepContract{
			Title:       "Build",
			Instruction: "Implement & verify.",
			Inputs: []ArtifactContract{
				{Path: "docs/spec.md", Required: true},
				{Path: "docs/optional.md", Required: false},
			},
			Artifacts:        []ArtifactContract{{Path: "out/component.bin", Required: true}},
			RequiredChecks:   []string{"go-test", "go-vet"},
			ApprovalRequired: true,
		},
	}
}

func testState(t *testing.T, attempts int, current string) state.State {
	t.Helper()
	fl := flow.Flow{
		ID: "flow", Title: "Flow",
		Steps: []flow.Step{
			{ID: "build", Title: "Build", Instruction: "Build <it>", Inputs: []flow.Artifact{}, Artifacts: []flow.Artifact{{Path: "out/component.bin", Required: true}}, RequiredChecks: []string{"go-test"}, Approval: &flow.Approval{Required: true}},
			{ID: "review", Title: "Review", Instruction: "Review", Inputs: []flow.Artifact{}, Artifacts: []flow.Artifact{}, RequiredChecks: []string{}},
		},
	}
	fs, err := flow.BuildSnapshot(fl, flow.FlowSource{Path: "/private/flow.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	ts, err := task.BuildSnapshot("Build <component>.\n日本語", task.TaskSource{Path: "/private/task.md"})
	if err != nil {
		t.Fatal(err)
	}
	st := state.State{
		SchemaVersion:  state.CurrentSchemaVersion,
		FlowSnapshot:   fs,
		TaskSnapshot:   ts,
		Status:         state.StatusRunning,
		CompletedSteps: []string{},
		SkippedSteps:   map[string]state.SkippedStep{},
		BackHistory:    []state.BackHistory{},
		FlowRunID:      "run_0123456789abcdef0123456789abcdef",
		Attempts:       []state.StepAttempt{},
	}
	sequence := []string{"build", "review", "build"}
	for i := 0; i < attempts; i++ {
		attempt, err := state.NewStepAttempt(sequence[i], uint64(i+1))
		if err != nil {
			t.Fatal(err)
		}
		if i < attempts-1 {
			attempt, err = state.CloseStepAttempt(attempt, state.StepAttemptExitBack, "move")
			if err != nil {
				t.Fatal(err)
			}
		}
		st.Attempts = append(st.Attempts, attempt)
	}
	st.CurrentStepID = current
	st.CurrentAttemptID = st.Attempts[len(st.Attempts)-1].ID
	if err := state.Validate(st); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	return st
}

func clonePackage(pkg WorkPackage) WorkPackage {
	clone := pkg
	clone.Step.Inputs = append([]ArtifactContract{}, pkg.Step.Inputs...)
	clone.Step.Artifacts = append([]ArtifactContract{}, pkg.Step.Artifacts...)
	clone.Step.RequiredChecks = append([]string{}, pkg.Step.RequiredChecks...)
	return clone
}
