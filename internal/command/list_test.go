package command

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListIncludesInitFlow(t *testing.T) {
	root := t.TempDir()
	initResult := Init(Context{ProjectRoot: root})
	if initResult.ExitCode != 0 {
		t.Fatalf("init ExitCode = %d", initResult.ExitCode)
	}

	got := List(Context{ProjectRoot: root})

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if len(got.Flows) != 1 {
		t.Fatalf("len(Flows) = %d, want 1", len(got.Flows))
	}
	item := got.Flows[0]
	if item.Status != FlowStatusValid {
		t.Fatalf("Status = %q, want valid", item.Status)
	}
	if item.ID != "post-task-review" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Title != "タスク後レビュー" {
		t.Fatalf("Title = %q", item.Title)
	}
	if item.Description == "" {
		t.Fatalf("Description is empty")
	}
	if item.StepCount != 5 {
		t.Fatalf("StepCount = %d, want 5", item.StepCount)
	}
}

func TestListReturnsValidFlows(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "minimal-flow", `flow: {
		id: "minimal-flow"
		title: "Minimal Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}]
	}`)

	got := List(Context{ProjectRoot: root})

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if len(got.Flows) != 1 {
		t.Fatalf("len(Flows) = %d, want 1", len(got.Flows))
	}
	item := got.Flows[0]
	if item.ID != "minimal-flow" {
		t.Fatalf("ID = %q", item.ID)
	}
	if item.Description != "" {
		t.Fatalf("Description = %q, want empty", item.Description)
	}
	if item.StepCount != 1 {
		t.Fatalf("StepCount = %d, want 1", item.StepCount)
	}
	if item.Status != FlowStatusValid {
		t.Fatalf("Status = %q, want valid", item.Status)
	}
}

func TestListKeepsValidFlowsWhenInvalidFlowExists(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "valid-flow", `flow: {
		id: "valid-flow"
		title: "Valid Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}]
	}`)
	writeCommandFlow(t, root, "broken-flow", `flow: {
		id: "broken-flow"
		title: "Broken Flow"
		steps: []
	}`)

	got := List(Context{ProjectRoot: root})

	if got.ExitCode == 0 {
		t.Fatalf("ExitCode = 0, want non-zero")
	}
	if len(got.Flows) != 2 {
		t.Fatalf("len(Flows) = %d, want 2", len(got.Flows))
	}
	assertHasFlowStatus(t, got.Flows, "valid-flow", FlowStatusValid)
	assertHasInvalidFlow(t, got.Flows, filepath.Join(FlowDir(root), "broken-flow.cue"))
}

func TestListDoesNotDependOnState(t *testing.T) {
	root := t.TempDir()
	writeCommandFlow(t, root, "valid-flow", `flow: {
		id: "valid-flow"
		title: "Valid Flow"
		steps: [{
			id: "first"
			title: "First"
			instruction: "Do first."
		}]
	}`)

	gotWithoutState := List(Context{ProjectRoot: root})
	if gotWithoutState.ExitCode != 0 {
		t.Fatalf("ExitCode without state = %d, want 0", gotWithoutState.ExitCode)
	}

	writeCommandTestFile(t, StatePath(root), `{"not":"valid state"}`)
	gotWithBrokenState := List(Context{ProjectRoot: root})
	if gotWithBrokenState.ExitCode != 0 {
		t.Fatalf("ExitCode with broken state = %d, want 0", gotWithBrokenState.ExitCode)
	}
	if len(gotWithBrokenState.Flows) != len(gotWithoutState.Flows) {
		t.Fatalf("list depended on state.json")
	}
}

func writeCommandFlow(t *testing.T, root string, id string, content string) {
	t.Helper()

	writeCommandTestFile(t, filepath.Join(FlowDir(root), id+".cue"), content)
}

func assertHasFlowStatus(t *testing.T, items []FlowListItem, id string, status string) {
	t.Helper()

	for _, item := range items {
		if item.ID == id {
			if item.Status != status {
				t.Fatalf("flow %q status = %q, want %q", id, item.Status, status)
			}
			return
		}
	}
	t.Fatalf("flow %q not found in %#v", id, items)
}

func assertHasInvalidFlow(t *testing.T, items []FlowListItem, filePath string) {
	t.Helper()

	for _, item := range items {
		if item.FilePath == filePath {
			if item.Status != FlowStatusInvalid {
				t.Fatalf("flow %q status = %q, want invalid", filePath, item.Status)
			}
			if item.Err == nil {
				t.Fatalf("flow %q Err is nil", filePath)
			}
			return
		}
	}
	t.Fatalf("invalid flow %q not found in %#v", filePath, items)
}

func TestListReturnsZeroWhenFlowDirIsMissing(t *testing.T) {
	root := t.TempDir()

	got := List(Context{ProjectRoot: root})

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	if len(got.Flows) != 0 {
		t.Fatalf("len(Flows) = %d, want 0", len(got.Flows))
	}
	if _, err := os.Stat(StatePath(root)); !os.IsNotExist(err) {
		t.Fatalf("state.json was read or created: %v", err)
	}
}
