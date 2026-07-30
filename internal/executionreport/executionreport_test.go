package executionreport

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tororoMeshi/devflow/internal/workpackage"
)

const (
	testRun     = "run_0123456789abcdef0123456789abcdef"
	testAttempt = "attempt_00000000000000000003"
	testDigest  = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

func validReport() Report {
	return Report{
		SchemaVersion: SchemaVersion, FlowRunID: testRun, StepID: "build",
		AttemptID: testAttempt, WorkPackageDigest: testDigest, Outcome: OutcomeCompleted,
		Summary: "Implemented component.",
		Decisions: []DecisionRecord{{
			Decision: "Use storage.", Rationale: "It preserves boundaries.",
			EvidenceRefs: []EvidenceReference{{Kind: EvidenceInput, ID: "docs/spec.md"}},
		}},
		ArtifactRefs: []string{"out/component.bin"}, UnresolvedIssues: []string{},
		NextAction: "Run checks.",
	}
}

func TestCanonicalExactAndDigestVector(t *testing.T) {
	report := validReport()
	got, err := CanonicalBytes(report)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"flow_run_id":"run_0123456789abcdef0123456789abcdef","step_id":"build","attempt_id":"attempt_00000000000000000003","work_package_digest":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","outcome":"completed","summary":"Implemented component.","decisions":[{"decision":"Use storage.","rationale":"It preserves boundaries.","evidence_refs":[{"kind":"input","id":"docs/spec.md"}]}],"artifact_refs":["out/component.bin"],"unresolved_issues":[],"next_action":"Run checks."}`
	if string(got) != want {
		t.Fatalf("canonical=%s", got)
	}
	if bytes.Contains(got, []byte("\n")) {
		t.Fatal("canonical has newline")
	}
	digest, err := Digest(report)
	if err != nil {
		t.Fatal(err)
	}
	if digest != "sha256:e622733b3251113e76554cbb5721d93da188ac4192268ebc551deee4f589592e" {
		t.Fatalf("digest=%s", digest)
	}
}

func TestValidateOutcomesTextCollectionsAndDuplicates(t *testing.T) {
	for _, outcome := range []Outcome{OutcomeCompleted, OutcomeBlocked, OutcomeFailed} {
		report := validReport()
		report.Outcome = outcome
		if err := Validate(report); err != nil {
			t.Fatalf("%s: %v", outcome, err)
		}
	}
	tests := []struct {
		name string
		edit func(*Report)
	}{
		{"unknown outcome", func(r *Report) { r.Outcome = "other" }},
		{"empty summary", func(r *Report) { r.Summary = "" }},
		{"space summary", func(r *Report) { r.Summary = " x" }},
		{"empty decision", func(r *Report) { r.Decisions[0].Decision = "" }},
		{"empty rationale", func(r *Report) { r.Decisions[0].Rationale = "" }},
		{"space next", func(r *Report) { r.NextAction = " " }},
		{"nul", func(r *Report) { r.Summary = "a\x00b" }},
		{"nil decisions", func(r *Report) { r.Decisions = nil }},
		{"nil refs", func(r *Report) { r.Decisions[0].EvidenceRefs = nil }},
		{"nil artifacts", func(r *Report) { r.ArtifactRefs = nil }},
		{"nil issues", func(r *Report) { r.UnresolvedIssues = nil }},
		{"duplicate evidence", func(r *Report) {
			r.Decisions[0].EvidenceRefs = append(r.Decisions[0].EvidenceRefs, r.Decisions[0].EvidenceRefs[0])
		}},
		{"duplicate artifact", func(r *Report) { r.ArtifactRefs = append(r.ArtifactRefs, r.ArtifactRefs[0]) }},
		{"invalid kind", func(r *Report) { r.Decisions[0].EvidenceRefs[0].Kind = "url" }},
		{"too many", func(r *Report) {
			r.UnresolvedIssues = make([]string, 101)
			for i := range r.UnresolvedIssues {
				r.UnresolvedIssues[i] = "x"
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := validReport()
			tt.edit(&report)
			if err := Validate(report); err == nil {
				t.Fatal("accepted invalid report")
			}
		})
	}
	report := validReport()
	report.NextAction = ""
	report.Decisions = make([]DecisionRecord, 100)
	for i := range report.Decisions {
		report.Decisions[i] = DecisionRecord{Decision: "x", Rationale: "y", EvidenceRefs: []EvidenceReference{}}
	}
	if err := Validate(report); err != nil {
		t.Fatalf("100/empty next: %v", err)
	}
}

func TestDecodeStrict(t *testing.T) {
	canonical, _ := CanonicalBytes(validReport())
	if _, err := Decode(canonical); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		err  error
	}{
		{"empty", nil, ErrInvalidReport},
		{"whitespace", []byte(" \n\t"), ErrInvalidReport},
		{"scalar", []byte(`1`), ErrInvalidReport},
		{"array root", []byte(`[]`), ErrInvalidReport},
		{"schema missing", bytes.Replace(canonical, []byte(`"schema_version":1,`), nil, 1), ErrInvalidReport},
		{"schema null", bytes.Replace(canonical, []byte(`"schema_version":1`), []byte(`"schema_version":null`), 1), ErrInvalidReport},
		{"schema wrong type", bytes.Replace(canonical, []byte(`"schema_version":1`), []byte(`"schema_version":"1"`), 1), ErrInvalidReport},
		{"unsupported schema", bytes.Replace(canonical, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1), ErrUnsupportedSchema},
		{"missing", bytes.Replace(canonical, []byte(`"summary":"Implemented component.",`), nil, 1), ErrInvalidReport},
		{"null", bytes.Replace(canonical, []byte(`"summary":"Implemented component."`), []byte(`"summary":null`), 1), ErrInvalidReport},
		{"wrong type", bytes.Replace(canonical, []byte(`"summary":"Implemented component."`), []byte(`"summary":1`), 1), ErrInvalidReport},
		{"unknown", bytes.Replace(canonical, []byte(`"schema_version":1`), []byte(`"schema_version":1,"extra":1`), 1), ErrUnknownField},
		{"duplicate", []byte(`{"summary":"a","\u0073ummary":"b"}`), ErrDuplicateJSONKey},
		{"nested duplicate", bytes.Replace(canonical, []byte(`"decision":"Use storage."`), []byte(`"decision":"x","decision":"y"`), 1), ErrDuplicateJSONKey},
		{"trailing", append(append([]byte{}, canonical...), []byte(` {}`)...), ErrTrailingJSON},
		{"too large", bytes.Repeat([]byte("x"), MaxDocumentBytes+1), ErrTooLarge},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.data); !errors.Is(err, tt.err) {
				t.Fatalf("error=%v want %v", err, tt.err)
			}
		})
	}
}

func TestDocumentSizeBoundary(t *testing.T) {
	report := validReport()
	base, err := CanonicalBytes(report)
	if err != nil {
		t.Fatal(err)
	}
	report.Summary = strings.Repeat("x", MaxDocumentBytes-len(base)+len(report.Summary))
	atLimit, err := CanonicalBytes(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(atLimit) != MaxDocumentBytes {
		t.Fatalf("canonical size=%d", len(atLimit))
	}
	if _, err := Decode(atLimit); err != nil {
		t.Fatalf("exact limit rejected: %v", err)
	}
	if _, err := Decode(append(append([]byte{}, atLimit...), ' ')); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("limit+1 error=%v", err)
	}
}

func TestAllCollectionBoundaries(t *testing.T) {
	t.Run("decisions", func(t *testing.T) {
		report := validReport()
		report.Decisions = make([]DecisionRecord, MaxCollection)
		for i := range report.Decisions {
			report.Decisions[i] = DecisionRecord{Decision: "d", Rationale: "r", EvidenceRefs: []EvidenceReference{}}
		}
		if err := Validate(report); err != nil {
			t.Fatal(err)
		}
		report.Decisions = append(report.Decisions, report.Decisions[0])
		if err := Validate(report); err == nil {
			t.Fatal("101 decisions accepted")
		}
	})
	t.Run("evidence refs", func(t *testing.T) {
		report := validReport()
		report.Decisions[0].EvidenceRefs = make([]EvidenceReference, MaxCollection)
		for i := range report.Decisions[0].EvidenceRefs {
			report.Decisions[0].EvidenceRefs[i] = EvidenceReference{Kind: EvidenceInput, ID: "in/" + strings.Repeat("a", i+1)}
		}
		if err := Validate(report); err != nil {
			t.Fatal(err)
		}
		report.Decisions[0].EvidenceRefs = append(report.Decisions[0].EvidenceRefs, EvidenceReference{Kind: EvidenceInput, ID: "extra"})
		if err := Validate(report); err == nil {
			t.Fatal("101 evidence refs accepted")
		}
	})
	t.Run("artifact refs", func(t *testing.T) {
		report := validReport()
		report.ArtifactRefs = make([]string, MaxCollection)
		for i := range report.ArtifactRefs {
			report.ArtifactRefs[i] = "out/" + strings.Repeat("a", i+1)
		}
		if err := Validate(report); err != nil {
			t.Fatal(err)
		}
		report.ArtifactRefs = append(report.ArtifactRefs, "out/extra")
		if err := Validate(report); err == nil {
			t.Fatal("101 artifact refs accepted")
		}
	})
	t.Run("unresolved issues", func(t *testing.T) {
		report := validReport()
		report.UnresolvedIssues = make([]string, MaxCollection)
		for i := range report.UnresolvedIssues {
			report.UnresolvedIssues[i] = "issue " + strings.Repeat("a", i+1)
		}
		if err := Validate(report); err != nil {
			t.Fatal(err)
		}
		report.UnresolvedIssues = append(report.UnresolvedIssues, "extra")
		if err := Validate(report); err == nil {
			t.Fatal("101 unresolved issues accepted")
		}
	})
}

func TestDecodeCanonicalIgnoresInputWhitespaceAndFieldOrder(t *testing.T) {
	canonical, _ := CanonicalBytes(validReport())
	reordered := `{"next_action":"Run checks.","unresolved_issues":[],"artifact_refs":["out/component.bin"],"decisions":[{"evidence_refs":[{"id":"docs/spec.md","kind":"input"}],"rationale":"It preserves boundaries.","decision":"Use storage."}],"summary":"Implemented component.","outcome":"completed","work_package_digest":"` + testDigest + `","attempt_id":"` + testAttempt + `","step_id":"build","flow_run_id":"` + testRun + `","schema_version":1}`
	decoded, err := Decode([]byte(" \n" + reordered + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := CanonicalBytes(decoded)
	if !bytes.Equal(got, canonical) {
		t.Fatal("canonical differs")
	}
	decoded.ArtifactRefs = []string{}
	other, _ := CanonicalBytes(decoded)
	if bytes.Equal(other, canonical) {
		t.Fatal("array difference ignored")
	}
}

func TestValidateBindingReferences(t *testing.T) {
	report := validReport()
	pkg := workpackage.WorkPackage{
		FlowRunID: testRun, StepID: "build", AttemptID: testAttempt, WorkPackageDigest: testDigest,
		Step: workpackage.StepContract{
			Inputs:         []workpackage.ArtifactContract{{Path: "docs/spec.md"}},
			Artifacts:      []workpackage.ArtifactContract{{Path: "out/component.bin"}, {Path: "out/optional.bin"}},
			RequiredChecks: []string{"go-test"},
		},
	}
	report.Decisions[0].EvidenceRefs = []EvidenceReference{
		{Kind: EvidenceInput, ID: "docs/spec.md"},
		{Kind: EvidenceArtifact, ID: "out/optional.bin"},
		{Kind: EvidenceCheck, ID: "go-test"},
	}
	if err := ValidateBinding(report, pkg); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []EvidenceReference{
		{Kind: EvidenceInput, ID: "no.md"}, {Kind: EvidenceArtifact, ID: "no.bin"}, {Kind: EvidenceCheck, ID: "no"},
	} {
		bad := report
		bad.Decisions = append([]DecisionRecord{}, report.Decisions...)
		bad.Decisions[0].EvidenceRefs = []EvidenceReference{ref}
		if !errors.Is(ValidateBinding(bad, pkg), ErrUnknownEvidence) {
			t.Fatalf("accepted %#v", ref)
		}
	}
	report.ArtifactRefs = []string{"unknown"}
	if !errors.Is(ValidateBinding(report, pkg), ErrUnknownArtifact) {
		t.Fatal("accepted unknown artifact")
	}
}

func TestStoreImmutableIdempotentConflictAndModes(t *testing.T) {
	root := storeRoot(t)
	report := validReport()
	first, err := Record(root, report)
	if err != nil || first.Idempotent {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	path, _ := ReportPath(root, testRun, testAttempt)
	wantPath := filepath.Join(root, ".devflow", "runs", testRun, "execution-reports", testAttempt+".json")
	if path != wantPath {
		t.Fatalf("path=%q want=%q", path, wantPath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := CanonicalBytes(report)
	if !bytes.Equal(data, append(canonical, '\n')) {
		t.Fatal("stored bytes not canonical")
	}
	fileInfo, _ := os.Stat(path)
	dirInfo, _ := os.Stat(filepath.Dir(path))
	if fileInfo.Mode().Perm() != 0o600 || dirInfo.Mode().Perm() != 0o755 {
		t.Fatalf("modes file=%o dir=%o", fileInfo.Mode().Perm(), dirInfo.Mode().Perm())
	}
	second, err := Record(root, report)
	if err != nil || !second.Idempotent {
		t.Fatalf("second=%#v err=%v", second, err)
	}
	conflict := report
	conflict.Summary = "Different."
	if _, err := Record(root, conflict); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict error=%v", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, data) {
		t.Fatal("conflict overwrote file")
	}
	assertNoReportTemps(t, filepath.Dir(path))
}

func TestStoreRejectsSymlinksAndCorruption(t *testing.T) {
	t.Run("report directory symlink", func(t *testing.T) {
		root := storeRoot(t)
		runDir := filepath.Join(root, ".devflow", "runs", testRun)
		if err := os.Symlink(t.TempDir(), filepath.Join(runDir, "execution-reports")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := Record(root, validReport()); !errors.Is(err, ErrUnsafeStore) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("target symlink", func(t *testing.T) {
		root := storeRoot(t)
		path, _ := ReportPath(root, testRun, testAttempt)
		if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "elsewhere"), path); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := Record(root, validReport()); !errors.Is(err, ErrUnsafeStore) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("run directory symlink", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".devflow", "runs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), filepath.Join(root, ".devflow", "runs", testRun)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := Record(root, validReport()); !errors.Is(err, ErrUnsafeStore) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("existing noncanonical", func(t *testing.T) {
		root := storeRoot(t)
		path, _ := ReportPath(root, testRun, testAttempt)
		if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		canonical, _ := CanonicalBytes(validReport())
		if err := os.WriteFile(path, append([]byte(" "), append(canonical, '\n')...), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Record(root, validReport()); !errors.Is(err, ErrInvalidExisting) {
			t.Fatalf("error=%v", err)
		}
	})
	for _, fixture := range []struct {
		name string
		data func([]byte) []byte
	}{
		{"malformed", func([]byte) []byte { return []byte("{\n") }},
		{"no newline", func(data []byte) []byte { return data }},
		{"two newlines", func(data []byte) []byte { return append(append(data, '\n'), '\n') }},
		{"pretty prefix", func(data []byte) []byte { return append([]byte(" "), append(data, '\n')...) }},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			root := storeRoot(t)
			path, _ := ReportPath(root, testRun, testAttempt)
			if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			canonical, _ := CanonicalBytes(validReport())
			original := fixture.data(append([]byte{}, canonical...))
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Record(root, validReport()); !errors.Is(err, ErrInvalidExisting) {
				t.Fatalf("error=%v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatal("corrupt existing file was modified")
			}
		})
	}
}

func TestStoreConcurrentSameAndDifferent(t *testing.T) {
	t.Run("same", func(t *testing.T) {
		root := storeRoot(t)
		results := concurrentRecord(root, validReport(), validReport())
		var fresh, idem int
		for _, result := range results {
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.value.Idempotent {
				idem++
			} else {
				fresh++
			}
		}
		if fresh != 1 || idem != 1 {
			t.Fatalf("fresh=%d idem=%d", fresh, idem)
		}
		path, _ := ReportPath(root, testRun, testAttempt)
		assertNoReportTemps(t, filepath.Dir(path))
	})
	t.Run("different", func(t *testing.T) {
		root := storeRoot(t)
		other := validReport()
		other.Summary = "Different."
		results := concurrentRecord(root, validReport(), other)
		var success, conflict int
		for _, result := range results {
			if result.err == nil {
				success++
			}
			if errors.Is(result.err, ErrConflict) {
				conflict++
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatalf("results=%#v", results)
		}
		path, _ := ReportPath(root, testRun, testAttempt)
		data, err := os.ReadFile(path)
		if err != nil || !strings.HasSuffix(string(data), "\n") {
			t.Fatalf("final invalid: %v", err)
		}
		if _, err := Decode(data[:len(data)-1]); err != nil {
			t.Fatal(err)
		}
		assertNoReportTemps(t, filepath.Dir(path))
	})
}

func assertNoReportTemps(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".execution-report-") {
			t.Fatalf("temporary file remains: %s", entry.Name())
		}
	}
}

type recordCall struct {
	value RecordResult
	err   error
}

func concurrentRecord(root string, reports ...Report) []recordCall {
	start := make(chan struct{})
	results := make([]recordCall, len(reports))
	var wg sync.WaitGroup
	for i := range reports {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i].value, results[i].err = Record(root, reports[i])
		}(i)
	}
	close(start)
	wg.Wait()
	return results
}

func storeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runDir := filepath.Join(root, ".devflow", "runs", testRun)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
