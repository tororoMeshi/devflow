package command

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	devflowGitignoreContent = "state.json\ncurrent.json\nruns/\n"

	postTaskReviewFlowContent = `flow: {
	id: "post-task-review"
	title: "タスク後レビュー"
	description: "AIによる実装や修正が完了した後に、変更内容、テスト、レビュー、人間承認を確認するFlowです。"

	steps: [
		{
			id: "check_changes"
			title: "変更ファイル確認"
			objective: "変更されたファイルを整理する。"
		},
		{
			id: "summarize_changes"
			title: "変更内容の要約"
			objective: "変更内容を要約する。"
		},
		{
			id: "check_quality"
			title: "品質確認"
			objective: "必要な確認結果を整理する。"
		},
		{
			id: "write_review"
			title: "レビュー結果作成"
			objective: "レビュー結果を docs/code-review.md にまとめる。"
			artifacts: [
				{
					path: "docs/code-review.md"
					required: true
				},
			]
		},
		{
			id: "human_approval"
			title: "人間承認"
			objective: "レビュー結果について人間の承認を得る。"
			approval: {
				required: true
			}
		},
	]
}
`
)

func Init(ctx Context) CommandResult {
	result := CommandResult{ExitCode: 0}

	for _, dir := range []string{
		filepath.Join(ctx.ProjectRoot, ".devflow"),
		FlowDir(ctx.ProjectRoot),
		RunsDir(ctx.ProjectRoot),
	} {
		action, err := ensureDir(dir)
		result.Actions = append(result.Actions, action)
		if err != nil {
			result.ExitCode = 1
			return result
		}
	}

	for _, file := range []struct {
		path    string
		content string
	}{
		{
			path:    filepath.Join(ctx.ProjectRoot, ".devflow", ".gitignore"),
			content: devflowGitignoreContent,
		},
		{
			path:    filepath.Join(FlowDir(ctx.ProjectRoot), "post-task-review.cue"),
			content: postTaskReviewFlowContent,
		},
	} {
		action, err := ensureFile(file.path, file.content)
		result.Actions = append(result.Actions, action)
		if err != nil {
			result.ExitCode = 1
			return result
		}
	}

	return result
}

func ensureDir(path string) (CommandAction, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return CommandAction{Path: path, Status: ActionExists}, nil
		}
		return CommandAction{Path: path, Status: ActionExists}, errors.New("path exists and is not a directory")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return CommandAction{Path: path, Status: ActionExists}, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return CommandAction{Path: path, Status: ActionExists}, err
	}
	return CommandAction{Path: path, Status: ActionCreated}, nil
}

func ensureFile(path string, content string) (CommandAction, error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return CommandAction{Path: path, Status: ActionExists}, nil
		}
		return CommandAction{Path: path, Status: ActionExists}, err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return CommandAction{Path: path, Status: ActionCreated}, err
	}
	return CommandAction{Path: path, Status: ActionCreated}, nil
}
