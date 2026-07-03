package command

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	devflowGitignoreContent = "state.json\n"

	postTaskReviewFlowContent = `flow: {
	id: "post-task-review"
	title: "タスク後レビュー"
	description: "AIによる実装や修正が完了した後に、変更内容、テスト、レビュー、人間承認を確認するFlowです。"

	steps: [
		{
			id: "check_changes"
			title: "変更ファイル確認"
			instruction: "git status と diff を確認し、変更されたファイルを整理してください。"
		},
		{
			id: "summarize_changes"
			title: "変更内容の要約"
			instruction: "変更内容を確認し、依頼内容に対して何を変更したかを要約してください。"
		},
		{
			id: "check_quality"
			title: "品質確認"
			instruction: "テスト、lint、型チェックなど、今回の変更に必要な確認を行い、結果を整理してください。"
		},
		{
			id: "write_review"
			title: "レビュー結果作成"
			instruction: "変更内容、確認結果、懸念点、必要な修正を docs/code-review.md にまとめてください。"
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
			instruction: "レビュー結果を人間に提示し、次へ進んでよいか確認してください。"
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
