// Package task defines the content contract for a Task and its snapshot.
package task

const TaskSnapshotSchemaVersion = 1

// TaskSource identifies the origin of a Task for traceability. It is excluded
// from the snapshot digest.
type TaskSource struct {
	Path string `json:"path,omitempty"`
}

// TaskSnapshot is a normalized, verifiable Task contract.
type TaskSnapshot struct {
	SchemaVersion int        `json:"schema_version"`
	Digest        string     `json:"digest"`
	Source        TaskSource `json:"source"`
	Content       string     `json:"content"`
}
