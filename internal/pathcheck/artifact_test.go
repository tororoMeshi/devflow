package pathcheck

import (
	"errors"
	"testing"
)

func TestValidateArtifactPathValid(t *testing.T) {
	tests := []string{
		"docs/code-review.md",
		"docs/review/result.md",
		"README.md",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			if err := ValidateArtifactPath(tt); err != nil {
				t.Fatalf("ValidateArtifactPath(%q) returned error: %v", tt, err)
			}
		})
	}
}

func TestValidateArtifactPathInvalid(t *testing.T) {
	tests := []string{
		"/tmp/result.md",
		"../result.md",
		"docs/../secret.md",
		"https://example.com/result.md",
		"http://example.com/result.md",
		"docs/*.md",
		"docs/",
		"",
		"   ",
		`C:\tmp\result.md`,
		`..\result.md`,
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			err := ValidateArtifactPath(tt)
			if !errors.Is(err, ErrInvalidArtifactPath) {
				t.Fatalf("ValidateArtifactPath(%q) error = %v, want ErrInvalidArtifactPath", tt, err)
			}
		})
	}
}
