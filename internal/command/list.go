package command

import "github.com/tororoMeshi/devflow/internal/flow"

func List(ctx Context) CommandResult {
	results, err := flow.LoadDir(FlowDir(ctx.ProjectRoot))
	if err != nil {
		return CommandResult{
			ExitCode: 1,
			Flows: []FlowListItem{
				{
					FilePath: FlowDir(ctx.ProjectRoot),
					Status:   FlowStatusInvalid,
					Err:      err,
				},
			},
		}
	}

	commandResult := CommandResult{ExitCode: 0}
	for _, result := range results {
		item := flowListItem(result)
		if item.Status == FlowStatusInvalid {
			commandResult.ExitCode = 1
		}
		commandResult.Flows = append(commandResult.Flows, item)
	}

	return commandResult
}

func flowListItem(result flow.FlowFileResult) FlowListItem {
	if result.Status == flow.FlowFileValid && result.Flow != nil {
		return FlowListItem{
			ID:        result.Flow.ID,
			Title:     result.Flow.Title,
			Purpose:   result.Flow.Purpose,
			StepCount: len(result.Flow.Steps),
			FilePath:  result.FilePath,
			Status:    FlowStatusValid,
		}
	}

	return FlowListItem{
		FilePath: result.FilePath,
		Status:   FlowStatusInvalid,
		Err:      result.Err,
	}
}
