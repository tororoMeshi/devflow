package command

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tororoMeshi/devflow/internal/flow"
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
		ActionCreated,
	})
	assertDir(t, filepath.Join(root, ".devflow"))
	assertDir(t, FlowDir(root))
	assertDir(t, RunsDir(root))
	assertFileContent(t, filepath.Join(root, ".devflow", ".gitignore"), devflowGitignoreContent)
	assertFileExists(t, filepath.Join(FlowDir(root), "post-task-review.cue"))
	assertNoFile(t, LegacyStatePath(root))
	assertNoFile(t, CurrentPath(root))

	loaded, err := flow.LoadFile(filepath.Join(FlowDir(root), "post-task-review.cue"))
	if err != nil {
		t.Fatal(err)
	}
	want := flow.Normalize(flow.Flow{
		ID:      "post-task-review",
		Title:   "タスク後レビュー",
		Purpose: "実装や修正の結果を、変更内容・影響・品質の根拠に基づいて、人間が受け入れてよいか判断できるようにする。",
		Steps: []flow.Step{
			{
				ID:        "describe_changes",
				Title:     "変更内容の記述",
				Purpose:   "後続の評価を、実際に何が変わったかという共通の前提に基づいて行えるようにする。",
				Outcome:   "変更されたファイル・パスと、それぞれで何が変わったかが `docs/code-review.md` に記録されている。",
				Artifacts: []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
			},
			{
				ID:        "assess_changes",
				Title:     "変更影響の評価",
				Purpose:   "変更による影響と懸念を整理し、後続の品質確認と人間の判断がそれらを踏まえて行えるようにする。",
				Outcome:   "変更がタスクの目的・要求にどう対応しているか、変更が影響する場面や条件、互換性への影響とその維持が必要な範囲、およびそこで生じる影響・懸念が `docs/code-review.md` に記録されている。",
				Inputs:    []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
				Artifacts: []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
			},
			{
				ID:        "verify_quality",
				Title:     "品質確認",
				Purpose:   "変更を受け入れてよいか判断するために必要な品質上の確認を、前工程の評価に依存せず検討・実施し、その結果に基づくレビュー判断を人間の最終判断の根拠として残す。",
				Outcome:   "レビュー判断に必要な確認とその結果、判断に影響する未確認・未解決の事項、およびそれらに基づくレビュー判断が `docs/code-review.md` に記録されている。",
				Inputs:    []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
				Artifacts: []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
			},
			{
				ID:        "human_approval",
				Title:     "人間承認",
				Purpose:   "レビュー結果と実際の運用条件を踏まえ、変更を受け入れるかを人間が最終判断できるようにする。",
				Outcome:   "`docs/code-review.md` のレビュー結果を踏まえ、変更を受け入れる判断が人間によって記録されている。",
				Inputs:    []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
				Artifacts: []flow.Artifact{{Path: "docs/code-review.md", Required: true}},
				Approval:  &flow.Approval{Required: true},
			},
		},
	})
	if !reflect.DeepEqual(loaded, want) {
		t.Fatalf("standard flow = %#v, want %#v", loaded, want)
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
	assertNoFile(t, LegacyStatePath(root))
	assertActionStatuses(t, got.Actions, []string{
		ActionExists,
		ActionExists,
		ActionCreated,
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
