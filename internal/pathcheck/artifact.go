package pathcheck

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidArtifactPath = errors.New("invalid artifact path")

func ValidateArtifactPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return invalidArtifactPath(path)
	}
	if strings.HasPrefix(path, "/") {
		return invalidArtifactPath(path)
	}
	if strings.HasSuffix(path, "/") {
		return invalidArtifactPath(path)
	}
	if strings.Contains(path, "\\") {
		return invalidArtifactPath(path)
	}
	if strings.Contains(path, ":") {
		return invalidArtifactPath(path)
	}
	if strings.ContainsAny(path, "*?[]") {
		return invalidArtifactPath(path)
	}

	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return invalidArtifactPath(path)
		}
	}

	return nil
}

func invalidArtifactPath(path string) error {
	return fmt.Errorf("%w: %q", ErrInvalidArtifactPath, path)
}
