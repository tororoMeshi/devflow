package command

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8noki8/devflow/internal/flow"
)

func TestInitCreatesDevflowFiles(t *testing.T) {
	root := t.TempDir()

	got := Init(Context{ProjectRoot: root})

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	assertActionStatuses(t, got.Actions, []string{
		ActionCreated,
		ActionCreated,
		ActionCreated,
		ActionCreated,
	})
	assertDir(t, filepath.Join(root, ".devflow"))
	assertDir(t, FlowDir(root))
	assertFileContent(t, filepath.Join(root, ".devflow", ".gitignore"), devflowGitignoreContent)
	assertFileExists(t, filepath.Join(FlowDir(root), "post-task-review.cue"))
	assertNoFile(t, StatePath(root))

	loaded, err := flow.LoadFile(filepath.Join(FlowDir(root), "post-task-review.cue"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "post-task-review" {
		t.Fatalf("flow ID = %q", loaded.ID)
	}
	if len(loaded.Steps) != 5 {
		t.Fatalf("len(steps) = %d, want 5", len(loaded.Steps))
	}
	for _, step := range loaded.Steps {
		if len(step.RequiredChecks) != 0 {
			t.Fatalf("standard flow step %q has required checks", step.ID)
		}
	}
}

func TestInitDoesNotOverwriteExistingFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(FlowDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	gitignorePath := filepath.Join(root, ".devflow", ".gitignore")
	flowPath := filepath.Join(FlowDir(root), "post-task-review.cue")
	writeCommandTestFile(t, gitignorePath, "custom-ignore\n")
	writeCommandTestFile(t, flowPath, "custom-flow\n")

	got := Init(Context{ProjectRoot: root})

	if got.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", got.ExitCode)
	}
	assertFileContent(t, gitignorePath, "custom-ignore\n")
	assertFileContent(t, flowPath, "custom-flow\n")
	assertNoFile(t, StatePath(root))
	assertActionStatuses(t, got.Actions, []string{
		ActionExists,
		ActionExists,
		ActionExists,
		ActionExists,
	})
}

func assertActionStatuses(t *testing.T, actions []CommandAction, want []string) {
	t.Helper()

	if len(actions) != len(want) {
		t.Fatalf("len(actions) = %d, want %d: %#v", len(actions), len(want), actions)
	}
	for i, action := range actions {
		if action.Status != want[i] {
			t.Fatalf("actions[%d].Status = %q, want %q", i, action.Status, want[i])
		}
	}
}

func assertDir(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file", path)
	}
}

func assertNoFile(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists or stat failed unexpectedly: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s content = %q, want %q", path, string(got), want)
	}
}

func writeCommandTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
