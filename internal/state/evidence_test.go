package state

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateArtifactEvidence(t *testing.T) {
	valid := ArtifactEvidence{Digest: "sha256:" + strings.Repeat("a", 64), Size: 0}
	if err := ValidateArtifactEvidence(valid); err != nil {
		t.Fatal(err)
	}
	for _, digest := range []string{
		"", strings.Repeat("a", 64), "md5:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("A", 64), "sha256:" + strings.Repeat("a", 63),
		"sha256:" + strings.Repeat("a", 65), "sha256:" + strings.Repeat("g", 64),
		" sha256:" + strings.Repeat("a", 64),
	} {
		if !errors.Is(ValidateArtifactEvidence(ArtifactEvidence{Digest: digest}), ErrInvalidArtifactDigest) {
			t.Fatalf("invalid digest accepted: %q", digest)
		}
	}
	if !errors.Is(ValidateArtifactEvidence(ArtifactEvidence{Digest: valid.Digest, Size: -1}), ErrInvalidArtifactEvidence) {
		t.Fatal("negative size accepted")
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip ArtifactEvidence
	if err := json.Unmarshal(data, &roundTrip); err != nil || roundTrip != valid {
		t.Fatalf("round trip = %#v, %v", roundTrip, err)
	}
}

func TestCloneArtifactEvidenceDoesNotShareMap(t *testing.T) {
	source := map[string]ArtifactEvidence{"out/report.md": {Digest: "sha256:" + strings.Repeat("a", 64), Size: 1}}
	cloned := cloneArtifactEvidence(source)
	delete(cloned, "out/report.md")
	if len(source) != 1 {
		t.Fatal("clone shares map")
	}
}
