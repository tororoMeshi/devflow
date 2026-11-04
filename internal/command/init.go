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
	purpose: "実装や修正の結果を、変更内容・影響・品質の根拠に基づいて、人間が受け入れてよいか判断できるようにする。"

	steps: [
		{
			id: "describe_changes"
			title: "変更内容の記述"
			purpose: "後続の評価を、実際に何が変わったかという共通の前提に基づいて行えるようにする。"
			outcome: "変更されたファイル・パスと、それぞれで何が変わったかが \u0060docs/code-review.md\u0060 に記録されている。"
			artifacts: [
				{path: "docs/code-review.md", required: true},
			]
		},
		{
			id: "assess_changes"
			title: "変更影響の評価"
			purpose: "変更による影響と懸念を整理し、後続の品質確認と人間の判断がそれらを踏まえて行えるようにする。"
			outcome: "変更がタスクの目的・要求にどう対応しているか、変更が影響する場面や条件、互換性への影響とその維持が必要な範囲、およびそこで生じる影響・懸念が \u0060docs/code-review.md\u0060 に記録されている。"
			inputs: [
				{path: "docs/code-review.md", required: true},
			]
			artifacts: [
				{path: "docs/code-review.md", required: true},
			]
		},
		{
			id: "verify_quality"
			title: "品質確認"
			purpose: "変更を受け入れてよいか判断するために必要な品質上の確認を、前工程の評価に依存せず検討・実施し、その結果に基づくレビュー判断を人間の最終判断の根拠として残す。"
			outcome: "レビュー判断に必要な確認とその結果、判断に影響する未確認・未解決の事項、およびそれらに基づくレビュー判断が \u0060docs/code-review.md\u0060 に記録されている。"
			inputs: [
				{path: "docs/code-review.md", required: true},
			]
			artifacts: [
				{path: "docs/code-review.md", required: true},
			]
		},
		{
			id: "human_approval"
			title: "人間承認"
			purpose: "レビュー結果と実際の運用条件を踏まえ、変更を受け入れるかを人間が最終判断できるようにする。"
			outcome: "\u0060docs/code-review.md\u0060 のレビュー結果を踏まえ、変更を受け入れる判断が人間によって記録されている。"
			inputs: [
				{path: "docs/code-review.md", required: true},
			]
			artifacts: [
				{path: "docs/code-review.md", required: true},
			]
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
