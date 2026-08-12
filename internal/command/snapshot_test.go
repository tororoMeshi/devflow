package command

import (
	"fmt"
	"github.com/tororoMeshi/devflow/internal/flow"
	"github.com/tororoMeshi/devflow/internal/task"
	"os"
	"path/filepath"
	"testing"
)

func testSnapshot(id string) flow.FlowSnapshot {
	return testSnapshotForStep(id, "first")
}

func startWithTestTask(t testing.TB, root, flowID string) CommandResult {
	t.Helper()
	path := filepath.Join(root, "tasks", "task.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Test task\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Start(Context{ProjectRoot: root}, flowID, "tasks/task.md")
}

func testTaskSnapshot() task.TaskSnapshot {
	snapshot, err := task.BuildSnapshot("Test task\n", task.TaskSource{Path: "tasks/task.md"})
	if err != nil {
		panic(err)
	}
	return snapshot
}

func testSnapshotForStep(id, stepID string) flow.FlowSnapshot {
	steps := []flow.Step{{ID: stepID, Title: stepID, Objective: "test objective"}}
	snapshot, err := flow.BuildSnapshot(flow.Flow{ID: id, Title: fmt.Sprintf("%s title", id), Steps: steps}, flow.FlowSource{})
	if err != nil {
		panic(err)
	}
	return snapshot
}
