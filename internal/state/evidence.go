package state

import (
	"errors"
	"regexp"
	"sort"

	"github.com/8noki8/devflow/internal/pathcheck"
)

var (
	ErrNilArtifactEvidence     = errors.New("nil artifact evidence")
	ErrInvalidArtifactEvidence = errors.New("invalid artifact evidence")
	ErrInvalidArtifactDigest   = errors.New("invalid artifact digest")
)

var artifactDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type ArtifactEvidence struct {
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

func ValidateArtifactEvidence(value ArtifactEvidence) error {
	if !artifactDigestPattern.MatchString(value.Digest) {
		return ErrInvalidArtifactDigest
	}
	if value.Size < 0 {
		return ErrInvalidArtifactEvidence
	}
	return nil
}

func cloneArtifactEvidence(source map[string]ArtifactEvidence) map[string]ArtifactEvidence {
	if source == nil {
		return nil
	}
	cloned := make(map[string]ArtifactEvidence, len(source))
	for path, evidence := range source {
		cloned[path] = evidence
	}
	return cloned
}

func validateArtifactEvidenceMap(values map[string]ArtifactEvidence) error {
	if values == nil {
		return ErrNilArtifactEvidence
	}
	paths := make([]string, 0, len(values))
	for path := range values {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		evidence := values[path]
		if err := pathcheck.ValidateArtifactPath(path); err != nil {
			return ErrInvalidArtifactEvidence
		}
		if err := ValidateArtifactEvidence(evidence); err != nil {
			return err
		}
	}
	return nil
}
