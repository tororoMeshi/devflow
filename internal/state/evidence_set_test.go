package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const emptyEvidenceSetDigest = "sha256:d65728983c6fe0d4f09c0c18ad90370ea86c8b7e63e3367413abc99d88bda60f"

func TestArtifactEvidenceSetDigestEmptyLiteralVector(t *testing.T) {
	canonical, err := canonicalArtifactEvidenceSetJSON([]string{}, map[string]ArtifactEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(canonical), `{"schema_version":1,"artifacts":[]}`; got != want {
		t.Fatalf("canonical = %q, want %q", got, want)
	}
	digest, err := ArtifactEvidenceSetDigest([]string{}, map[string]ArtifactEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	if digest != emptyEvidenceSetDigest {
		t.Fatalf("digest = %q, want %q", digest, emptyEvidenceSetDigest)
	}
}

func TestArtifactEvidenceSetDigestOneArtifactExactVector(t *testing.T) {
	evidence := map[string]ArtifactEvidence{
		"out/report.md": {
			Digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			Size:   12,
		},
	}
	canonical, err := canonicalArtifactEvidenceSetJSON([]string{"out/report.md"}, evidence)
	if err != nil {
		t.Fatal(err)
	}
	const wantCanonical = `{"schema_version":1,"artifacts":[{"path":"out/report.md","digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111","size":12}]}`
	if string(canonical) != wantCanonical {
		t.Fatalf("canonical = %s, want %s", canonical, wantCanonical)
	}
	sum := sha256.Sum256([]byte(wantCanonical))
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	got, err := ArtifactEvidenceSetDigest([]string{"out/report.md"}, evidence)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantDigest {
		t.Fatalf("digest = %q, want %q", got, wantDigest)
	}
}

func TestArtifactEvidenceSetCanonicalJSONAndDeterminism(t *testing.T) {
	a := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("1", 64), Size: 12}
	b := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("2", 64), Size: 34}
	evidence := map[string]ArtifactEvidence{"出力/β.json": b, "out/a<&>.md": a}
	canonical, err := canonicalArtifactEvidenceSetJSON(
		[]string{"出力/β.json", "out/a<&>.md"}, evidence,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"schema_version":1,"artifacts":[{"path":"out/a\u003c\u0026\u003e.md","digest":"sha256:1111111111111111111111111111111111111111111111111111111111111111","size":12},{"path":"出力/β.json","digest":"sha256:2222222222222222222222222222222222222222222222222222222222222222","size":34}]}`
	if string(canonical) != want {
		t.Fatalf("canonical = %s\nwant = %s", canonical, want)
	}
	sum := sha256.Sum256([]byte(want))
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	first, err := ArtifactEvidenceSetDigest([]string{"出力/β.json", "out/a<&>.md"}, evidence)
	if err != nil {
		t.Fatal(err)
	}
	secondEvidence := map[string]ArtifactEvidence{"out/a<&>.md": a, "出力/β.json": b}
	second, err := ArtifactEvidenceSetDigest([]string{"out/a<&>.md", "出力/β.json"}, secondEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if first != wantDigest || second != first {
		t.Fatalf("digests = %q, %q; want %q", first, second, wantDigest)
	}
	for i := 0; i < 10; i++ {
		got, err := ArtifactEvidenceSetDigest([]string{"出力/β.json", "out/a<&>.md"}, evidence)
		if err != nil || got != first {
			t.Fatalf("iteration %d = %q, %v", i, got, err)
		}
	}
}

func TestArtifactEvidenceSetDigestBindsFields(t *testing.T) {
	baseEvidence := map[string]ArtifactEvidence{
		"out/a": {Digest: "sha256:" + strings.Repeat("1", 64), Size: 1},
	}
	base, err := ArtifactEvidenceSetDigest([]string{"out/a"}, baseEvidence)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		paths    []string
		evidence map[string]ArtifactEvidence
	}{
		{[]string{"out/b"}, map[string]ArtifactEvidence{"out/b": baseEvidence["out/a"]}},
		{[]string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: "sha256:" + strings.Repeat("2", 64), Size: 1}}},
		{[]string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: baseEvidence["out/a"].Digest, Size: 2}}},
	}
	for i, tt := range cases {
		got, err := ArtifactEvidenceSetDigest(tt.paths, tt.evidence)
		if err != nil {
			t.Fatal(err)
		}
		if got == base {
			t.Fatalf("case %d did not change digest", i)
		}
	}
}

func TestArtifactEvidenceSetDigestRejectsInvalidInputs(t *testing.T) {
	valid := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("1", 64), Size: 1}
	tests := []struct {
		name     string
		paths    []string
		evidence map[string]ArtifactEvidence
		want     error
	}{
		{"missing", []string{"out/a"}, map[string]ArtifactEvidence{}, ErrMissingRequiredArtifactEvidence},
		{"unknown", []string{}, map[string]ArtifactEvidence{"out/a": valid}, ErrUnknownArtifactEvidence},
		{"duplicate", []string{"out/a", "out/a"}, map[string]ArtifactEvidence{"out/a": valid}, ErrDuplicateRequiredArtifactPath},
		{"invalid digest", []string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: "bad", Size: 1}}, ErrInvalidArtifactDigest},
		{"negative size", []string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: valid.Digest, Size: -1}}, ErrInvalidArtifactEvidence},
		{"empty path", []string{""}, map[string]ArtifactEvidence{"": valid}, ErrInvalidArtifactEvidence},
		{"unicode whitespace path", []string{"\u3000\u2003"}, map[string]ArtifactEvidence{"\u3000\u2003": valid}, ErrInvalidArtifactEvidence},
		{"nil evidence empty set", []string{}, nil, ErrNilArtifactEvidence},
		{"nil evidence required", []string{"out/a"}, nil, ErrNilArtifactEvidence},
		{"digest leading whitespace", []string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: " " + valid.Digest, Size: 1}}, ErrInvalidArtifactDigest},
		{"digest unicode whitespace", []string{"out/a"}, map[string]ArtifactEvidence{"out/a": {Digest: "\u3000" + valid.Digest, Size: 1}}, ErrInvalidArtifactDigest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ArtifactEvidenceSetDigest(tt.paths, tt.evidence); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want errors.Is(_, %v)", err, tt.want)
			}
		})
	}
}

func TestArtifactEvidenceSetDigestUnknownEvidenceErrorIsDeterministic(t *testing.T) {
	valid := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("1", 64), Size: 1}
	for i := 0; i < 20; i++ {
		_, err := ArtifactEvidenceSetDigest([]string{}, map[string]ArtifactEvidence{
			"out/z": valid,
			"out/a": valid,
		})
		if !errors.Is(err, ErrUnknownArtifactEvidence) || !strings.Contains(err.Error(), `"out/a"`) {
			t.Fatalf("iteration %d error = %v, want sorted first unknown path", i, err)
		}
	}
}

func TestArtifactEvidenceSetDigestRequiredPathErrorIsDeterministic(t *testing.T) {
	valid := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("1", 64), Size: 1}
	for _, paths := range [][]string{
		{"out/z", "out/z", "out/a", "out/a"},
		{"out/a", "out/z", "out/a", "out/z"},
	} {
		_, err := ArtifactEvidenceSetDigest(paths, map[string]ArtifactEvidence{
			"out/a": valid,
			"out/z": valid,
		})
		if !errors.Is(err, ErrDuplicateRequiredArtifactPath) || !strings.Contains(err.Error(), `"out/a"`) {
			t.Fatalf("error = %v, want sorted first duplicate path", err)
		}
	}
}

func TestArtifactEvidenceSetDigestDoesNotMutateInputs(t *testing.T) {
	paths := []string{"out/b", "out/a"}
	evidence := map[string]ArtifactEvidence{
		"out/a": {Digest: "sha256:" + strings.Repeat("1", 64), Size: 1},
		"out/b": {Digest: "sha256:" + strings.Repeat("2", 64), Size: 2},
	}
	wantPaths := append([]string(nil), paths...)
	wantEvidence := cloneArtifactEvidence(evidence)
	if _, err := ArtifactEvidenceSetDigest(paths, evidence); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(paths, wantPaths) || !reflect.DeepEqual(evidence, wantEvidence) {
		t.Fatal("inputs mutated")
	}
}

func TestEvidenceSetDigestFormat(t *testing.T) {
	valid := emptyEvidenceSetDigest
	if !isValidEvidenceSetDigest(valid) {
		t.Fatal("valid digest rejected")
	}
	for _, value := range []string{
		"", strings.TrimPrefix(valid, "sha256:"), "sha512:" + valid[7:],
		"sha256:" + strings.Repeat("a", 63), "sha256:" + strings.Repeat("a", 65),
		"sha256:" + strings.Repeat("A", 64), "sha256:" + strings.Repeat("g", 64),
		" " + valid, valid + " ", "\u3000" + valid,
	} {
		if isValidEvidenceSetDigest(value) {
			t.Fatalf("invalid digest accepted: %q", value)
		}
	}
}
