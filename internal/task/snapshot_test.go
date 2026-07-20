package task

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSnapshotIsReproducible(t *testing.T) {
	const content = "# Task\n\nDo the thing.\n"
	first, err := BuildSnapshot(content, TaskSource{Path: "task.md"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot(content, TaskSource{Path: "task.md"})
	if err != nil {
		t.Fatal(err)
	}
	firstCanonical, err := canonicalJSON(first.SchemaVersion, first.Content)
	if err != nil {
		t.Fatal(err)
	}
	secondCanonical, err := canonicalJSON(second.SchemaVersion, second.Content)
	if err != nil {
		t.Fatal(err)
	}
	if first.Content != second.Content || string(firstCanonical) != string(secondCanonical) || first.Digest != second.Digest {
		t.Fatalf("snapshots are not reproducible:\n%#v\n%#v", first, second)
	}
}

func TestBuildSnapshotDigestExcludesSource(t *testing.T) {
	first, err := BuildSnapshot("Task", TaskSource{Path: "first.md"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSnapshot("Task", TaskSource{Path: "other/second.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest != second.Digest {
		t.Fatalf("digest depends on source: %q != %q", first.Digest, second.Digest)
	}
	if first.Source.Path != "first.md" || second.Source.Path != "other/second.txt" {
		t.Fatalf("sources = %#v, %#v", first.Source, second.Source)
	}
	if err := ValidateSnapshot(first); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(second); err != nil {
		t.Fatal(err)
	}
}

func TestBuildSnapshotNormalizesNewlines(t *testing.T) {
	inputs := []string{
		"first\n\nthird\n",
		"first\r\n\r\nthird\r\n",
		"first\r\rthird\r",
	}
	var want TaskSnapshot
	for i, input := range inputs {
		snapshot, err := BuildSnapshot(input, TaskSource{})
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Content != "first\n\nthird\n" {
			t.Fatalf("Content = %q", snapshot.Content)
		}
		if strings.Contains(snapshot.Content, "\r") {
			t.Fatalf("Content contains CR: %q", snapshot.Content)
		}
		if i == 0 {
			want = snapshot
		} else if snapshot.Content != want.Content || snapshot.Digest != want.Digest {
			t.Fatalf("snapshot %d differs: %#v, want %#v", i, snapshot, want)
		}
	}
}

func TestBuildSnapshotNormalizesMixedNewlines(t *testing.T) {
	snapshot, err := BuildSnapshot("A\r\nB\rC\nD", TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Content != "A\nB\nC\nD" {
		t.Fatalf("Content = %q, want %q", snapshot.Content, "A\nB\nC\nD")
	}
}

func TestBuildSnapshotPreservesContentExceptNewlines(t *testing.T) {
	const input = "  # 見出し  \r\n\titem  e\u0301 é\t  \r\n\r\n\r\n**太字**"
	const want = "  # 見出し  \n\titem  e\u0301 é\t  \n\n\n**太字**"
	snapshot, err := BuildSnapshot(input, TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Content != want {
		t.Fatalf("Content = %q, want %q", snapshot.Content, want)
	}
}

func TestBuildSnapshotTrailingNewlineIsSignificant(t *testing.T) {
	without, err := BuildSnapshot("Task", TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	with, err := BuildSnapshot("Task\n", TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	if without.Content == with.Content || without.Digest == with.Digest {
		t.Fatalf("trailing newline was ignored: %#v, %#v", without, with)
	}
}

func TestBuildSnapshotRejectsInvalidContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    error
	}{
		{"empty", "", ErrEmptyTask},
		{"spaces", "   ", ErrEmptyTask},
		{"newlines", "\r\n\n\r", ErrEmptyTask},
		{"tabs", "\t\t", ErrEmptyTask},
		{"Unicode whitespace", "\u00a0\u3000", ErrEmptyTask},
		{"invalid UTF-8", string([]byte{0xff, 'x'}), ErrInvalidUTF8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildSnapshot(tt.content, TaskSource{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("BuildSnapshot() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestBuildSnapshotDoesNotChangeInput(t *testing.T) {
	content := "Task\r\nnext"
	source := TaskSource{Path: "task.md"}
	wantContent, wantSource := content, source
	if _, err := BuildSnapshot(content, source); err != nil {
		t.Fatal(err)
	}
	if content != wantContent || source != wantSource {
		t.Fatalf("input changed: %q, %#v", content, source)
	}
}

func TestValidateSnapshot(t *testing.T) {
	valid, err := BuildSnapshot("Task\nnext", TaskSource{Path: "task.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateSnapshot(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edit func(*TaskSnapshot)
		want error
	}{
		{"unsupported schema", func(s *TaskSnapshot) { s.SchemaVersion++ }, ErrUnsupportedSnapshotSchema},
		{"invalid UTF-8", func(s *TaskSnapshot) { s.Content = string([]byte{0xff}) }, ErrInvalidUTF8},
		{"empty Task", func(s *TaskSnapshot) { s.Content = " \n\t" }, ErrEmptyTask},
		{"CRLF", func(s *TaskSnapshot) { s.Content = "Task\r\nnext" }, ErrSnapshotNotNormalized},
		{"standalone CR", func(s *TaskSnapshot) { s.Content = "Task\rnext" }, ErrSnapshotNotNormalized},
		{"bad prefix", func(s *TaskSnapshot) { s.Digest = "md5:" + strings.Repeat("0", 64) }, ErrInvalidSnapshotDigest},
		{"short digest", func(s *TaskSnapshot) { s.Digest = "sha256:" + strings.Repeat("0", 63) }, ErrInvalidSnapshotDigest},
		{"long digest", func(s *TaskSnapshot) { s.Digest = "sha256:" + strings.Repeat("0", 65) }, ErrInvalidSnapshotDigest},
		{"uppercase hex", func(s *TaskSnapshot) { s.Digest = "sha256:" + strings.Repeat("A", 64) }, ErrInvalidSnapshotDigest},
		{"non-hex", func(s *TaskSnapshot) { s.Digest = "sha256:" + strings.Repeat("g", 64) }, ErrInvalidSnapshotDigest},
		{"content changed", func(s *TaskSnapshot) { s.Content = "Changed" }, ErrSnapshotDigestMismatch},
		{"digest changed", func(s *TaskSnapshot) { s.Digest = "sha256:" + strings.Repeat("0", 64) }, ErrSnapshotDigestMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valid
			tt.edit(&got)
			err := ValidateSnapshot(got)
			if !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSnapshot() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateSnapshotIgnoresSourceAndDoesNotModifyInput(t *testing.T) {
	snapshot, err := BuildSnapshot("Task", TaskSource{Path: "task.md"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Source.Path = "changed/elsewhere.md"
	before := snapshot
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, before) {
		t.Fatalf("ValidateSnapshot modified input: %#v, want %#v", snapshot, before)
	}
}

func TestCanonicalSchemaVersionAffectsDigest(t *testing.T) {
	canonical, err := canonicalJSON(TaskSnapshotSchemaVersion, "Task")
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != `{"schema_version":1,"content":"Task"}` {
		t.Fatalf("canonical JSON = %s", canonical)
	}
	current, err := snapshotDigest(TaskSnapshotSchemaVersion, "Task")
	if err != nil {
		t.Fatal(err)
	}
	other, err := snapshotDigest(TaskSnapshotSchemaVersion+1, "Task")
	if err != nil {
		t.Fatal(err)
	}
	if current == other {
		t.Fatal("schema version does not affect digest")
	}
}

func TestTaskSnapshotGolden(t *testing.T) {
	const content = "Task本文"
	const wantCanonical = `{"schema_version":1,"content":"Task本文"}`
	const wantDigest = "sha256:1cf0dda5f7447091ac0aa5fbcda9d7690f24c5e56e4724bb60248cf4b54fa188"
	const wantSnapshot = `{"schema_version":1,"digest":"sha256:1cf0dda5f7447091ac0aa5fbcda9d7690f24c5e56e4724bb60248cf4b54fa188","source":{"path":"task.md"},"content":"Task本文"}`

	canonical, err := canonicalJSON(TaskSnapshotSchemaVersion, content)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != wantCanonical {
		t.Fatalf("canonical JSON = %s, want %s", canonical, wantCanonical)
	}
	snapshot, err := BuildSnapshot(content, TaskSource{Path: "task.md"})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != TaskSnapshotSchemaVersion || snapshot.Digest != wantDigest {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != wantSnapshot {
		t.Fatalf("snapshot JSON = %s, want %s", data, wantSnapshot)
	}
}

func TestTaskSnapshotJSONAlwaysContainsSource(t *testing.T) {
	snapshot, err := BuildSnapshot("Task", TaskSource{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"source":{}`) {
		t.Fatalf("snapshot JSON does not contain source: %s", data)
	}
}
