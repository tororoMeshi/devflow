package automationruntime

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var artifactDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func parseArtifactRecordOutput(data []byte, target ArtifactTarget, attemptID string) error {
	lines := strings.Split(string(data), "\n")
	if len(lines) != 5 || lines[4] != "" ||
		lines[0] != "Recorded artifact: "+target.Path ||
		lines[1] != "Attempt: "+attemptID ||
		!strings.HasPrefix(lines[2], "Digest: ") ||
		!artifactDigestPattern.MatchString(strings.TrimPrefix(lines[2], "Digest: ")) ||
		!strings.HasPrefix(lines[3], "Size: ") {
		return errors.New("invalid artifact record output")
	}
	size := strings.TrimPrefix(lines[3], "Size: ")
	if size == "" || (len(size) > 1 && size[0] == '0') {
		return errors.New("invalid artifact size")
	}
	if _, err := strconv.ParseUint(size, 10, 64); err != nil {
		return errors.New("invalid artifact size")
	}
	return nil
}
