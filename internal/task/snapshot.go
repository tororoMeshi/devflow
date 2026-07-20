package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnsupportedSnapshotSchema = errors.New("unsupported task snapshot schema version")
	ErrInvalidUTF8               = errors.New("task content is not valid UTF-8")
	ErrEmptyTask                 = errors.New("task content is empty")
	ErrSnapshotNotNormalized     = errors.New("task snapshot content is not normalized")
	ErrInvalidSnapshotDigest     = errors.New("invalid task snapshot digest")
	ErrSnapshotDigestMismatch    = errors.New("task snapshot digest mismatch")
	ErrCanonicalJSON             = errors.New("generate canonical task snapshot JSON")

	taskSnapshotDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// BuildSnapshot creates a normalized and verifiable TaskSnapshot.
func BuildSnapshot(content string, source TaskSource) (TaskSnapshot, error) {
	if !utf8.ValidString(content) {
		return TaskSnapshot{}, ErrInvalidUTF8
	}

	normalized := normalizeContent(content)
	if strings.TrimSpace(normalized) == "" {
		return TaskSnapshot{}, ErrEmptyTask
	}

	digest, err := snapshotDigest(TaskSnapshotSchemaVersion, normalized)
	if err != nil {
		return TaskSnapshot{}, err
	}
	return TaskSnapshot{
		SchemaVersion: TaskSnapshotSchemaVersion,
		Digest:        digest,
		Source:        source,
		Content:       normalized,
	}, nil
}

// ValidateSnapshot verifies a persisted TaskSnapshot without changing it.
func ValidateSnapshot(snapshot TaskSnapshot) error {
	if snapshot.SchemaVersion != TaskSnapshotSchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedSnapshotSchema, snapshot.SchemaVersion)
	}
	if !utf8.ValidString(snapshot.Content) {
		return ErrInvalidUTF8
	}
	if strings.TrimSpace(snapshot.Content) == "" {
		return ErrEmptyTask
	}
	if normalizeContent(snapshot.Content) != snapshot.Content {
		return ErrSnapshotNotNormalized
	}
	if !taskSnapshotDigestPattern.MatchString(snapshot.Digest) {
		return fmt.Errorf("%w: want sha256:<64 lowercase hex characters>", ErrInvalidSnapshotDigest)
	}

	digest, err := snapshotDigest(snapshot.SchemaVersion, snapshot.Content)
	if err != nil {
		return err
	}
	if snapshot.Digest != digest {
		return fmt.Errorf("%w: got %q, want %q", ErrSnapshotDigestMismatch, snapshot.Digest, digest)
	}
	return nil
}

func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.ReplaceAll(content, "\r", "\n")
}

type canonicalTaskSnapshot struct {
	SchemaVersion int    `json:"schema_version"`
	Content       string `json:"content"`
}

func canonicalJSON(schemaVersion int, content string) ([]byte, error) {
	payload, err := json.Marshal(canonicalTaskSnapshot{
		SchemaVersion: schemaVersion,
		Content:       content,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCanonicalJSON, err)
	}
	return payload, nil
}

func snapshotDigest(schemaVersion int, content string) (string, error) {
	payload, err := canonicalJSON(schemaVersion, content)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
