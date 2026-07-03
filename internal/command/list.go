package command

import "github.com/8noki8/devflow/internal/flow"

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
			ID:          result.Flow.ID,
			Title:       result.Flow.Title,
			Description: result.Flow.Description,
			StepCount:   len(result.Flow.Steps),
			FilePath:    result.FilePath,
			Status:      FlowStatusValid,
		}
	}

	return FlowListItem{
		FilePath: result.FilePath,
		Status:   FlowStatusInvalid,
		Err:      result.Err,
	}
}
