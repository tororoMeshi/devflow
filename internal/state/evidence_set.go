package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/tororoMeshi/devflow/internal/pathcheck"
)

const artifactEvidenceSetSchemaVersion = 1

var (
	ErrEvidenceSetDigestMismatch       = errors.New("evidence set digest mismatch")
	ErrMissingRequiredArtifactEvidence = errors.New("missing required artifact evidence")
	ErrUnknownArtifactEvidence         = errors.New("unknown artifact evidence")
	ErrDuplicateRequiredArtifactPath   = errors.New("duplicate required artifact path")
	evidenceSetDigestPattern           = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type canonicalArtifactEvidenceSet struct {
	SchemaVersion int                         `json:"schema_version"`
	Artifacts     []canonicalArtifactEvidence `json:"artifacts"`
}

type canonicalArtifactEvidence struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
	Size   int64  `json:"size"`
}

func ArtifactEvidenceSetDigest(requiredPaths []string, evidence map[string]ArtifactEvidence) (string, error) {
	canonical, err := canonicalArtifactEvidenceSetJSON(requiredPaths, evidence)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalArtifactEvidenceSetJSON(requiredPaths []string, evidence map[string]ArtifactEvidence) ([]byte, error) {
	if evidence == nil {
		return nil, ErrNilArtifactEvidence
	}

	paths := append([]string(nil), requiredPaths...)
	sort.Strings(paths)
	seen := make(map[string]struct{}, len(paths))
	for index, path := range paths {
		if index > 0 && path == paths[index-1] {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateRequiredArtifactPath, path)
		}
		if err := pathcheck.ValidateArtifactPath(path); err != nil {
			return nil, fmt.Errorf("%w: %q", ErrInvalidArtifactEvidence, path)
		}
		seen[path] = struct{}{}
	}

	artifacts := make([]canonicalArtifactEvidence, 0, len(paths))
	for _, path := range paths {
		value, ok := evidence[path]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrMissingRequiredArtifactEvidence, path)
		}
		if err := ValidateArtifactEvidence(value); err != nil {
			return nil, fmt.Errorf("artifact evidence %q: %w", path, err)
		}
		artifacts = append(artifacts, canonicalArtifactEvidence{
			Path: path, Digest: value.Digest, Size: value.Size,
		})
	}
	evidencePaths := make([]string, 0, len(evidence))
	for path := range evidence {
		evidencePaths = append(evidencePaths, path)
	}
	sort.Strings(evidencePaths)
	for _, path := range evidencePaths {
		if _, ok := seen[path]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownArtifactEvidence, path)
		}
	}
	return json.Marshal(canonicalArtifactEvidenceSet{
		SchemaVersion: artifactEvidenceSetSchemaVersion,
		Artifacts:     artifacts,
	})
}

func isValidEvidenceSetDigest(value string) bool {
	return evidenceSetDigestPattern.MatchString(value)
}
