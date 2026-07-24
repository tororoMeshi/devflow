package automationruntime

import (
	"strings"
	"testing"
)

func TestParseArtifactRecordOutputStrict(t *testing.T) {
	target := ArtifactTarget{Path: "out/result.txt", Required: true}
	valid := "Recorded artifact: out/result.txt\nAttempt: attempt_1\nDigest: " + reportDigest + "\nSize: 12\n"
	if err := parseArtifactRecordOutput([]byte(valid), target, "attempt_1"); err != nil {
		t.Fatal(err)
	}
	for i, input := range []string{
		strings.TrimSuffix(valid, "\n"), valid + "\n",
		strings.Replace(valid, "out/result.txt", "other", 1),
		strings.Replace(valid, "attempt_1", "other", 1),
		strings.Replace(valid, reportDigest, "sha256:ABC", 1),
		strings.Replace(valid, "Size: 12", "Size: -1", 1),
		strings.Replace(valid, "Size: 12", "Size: 01", 1),
		strings.ReplaceAll(valid, "\n", "\r\n"),
	} {
		if err := parseArtifactRecordOutput([]byte(input), target, "attempt_1"); err == nil {
			t.Errorf("mutation %d accepted", i)
		}
	}
}
