package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/8noki8/devflow/internal/command"
)

const usage = `Usage:
  devflow init
  devflow list
  devflow start <flow>
  devflow status
  devflow prompt
  devflow approve [--step <step>] [--note <note>]
  devflow done
  devflow back --reason <reason>
  devflow skip --reason <reason>
  devflow finish --reason <reason>
`

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], projectRoot, os.Stdout, os.Stderr))
}

func run(args []string, projectRoot string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 1
	}

	ctx := command.Context{
		ProjectRoot: projectRoot,
		Stdout:      stdout,
		Stderr:      stderr,
	}

	var result command.CommandResult
	switch args[0] {
	case "init":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Init(ctx)
	case "list":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.List(ctx)
	case "start":
		if len(args) != 2 {
			writeUsage(stderr)
			return 1
		}
		result = command.Start(ctx, args[1])
	case "status":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Status(ctx)
	case "prompt":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Prompt(ctx)
	case "approve":
		stepID, note, ok := parseApproveArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Approve(ctx, stepID, note)
	case "done":
		if len(args) != 1 {
			writeUsage(stderr)
			return 1
		}
		result = command.Done(ctx)
	case "back":
		reason, ok := parseReasonArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Back(ctx, reason)
	case "skip":
		reason, ok := parseReasonArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Skip(ctx, reason)
	case "finish":
		reason, ok := parseReasonArgs(args[1:])
		if !ok {
			writeUsage(stderr)
			return 1
		}
		result = command.Finish(ctx, reason)
	default:
		writeUsage(stderr)
		return 1
	}

	writeResult(ctx, result)
	return result.ExitCode
}

func parseApproveArgs(args []string) (string, string, bool) {
	var stepID string
	var note string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--step":
			if i+1 >= len(args) {
				return "", "", false
			}
			stepID = args[i+1]
			i++
		case "--note":
			if i+1 >= len(args) {
				return "", "", false
			}
			note = args[i+1]
			i++
		default:
			return "", "", false
		}
	}
	return stepID, note, true
}

func parseReasonArgs(args []string) (string, bool) {
	if len(args) != 2 || args[0] != "--reason" {
		return "", false
	}
	return args[1], true
}

func writeUsage(stderr io.Writer) {
	_, _ = io.WriteString(stderr, usage)
}

func writeResult(ctx command.Context, result command.CommandResult) {
	writeActions(ctx.Stdout, result.Actions)
	writeFlows(ctx.Stdout, result.Flows)
	if result.Success != nil {
		writeSuccess(ctx.Stdout, *result.Success)
	}
	if result.Status != nil {
		writeStatus(ctx.Stdout, *result.Status)
	}
	if result.Prompt != nil {
		writePrompt(ctx.Stdout, *result.Prompt)
	}
	command.WriteDiagnostics(ctx, result.Diagnostics)
}

func writeSuccess(stdout io.Writer, success command.SuccessResult) {
	if success.StartedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Started flow: %s\n", success.StartedFlowID)
	}
	if success.CurrentStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Current step: %s\n", success.CurrentStepID)
	}
	if success.CompletedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Completed step: %s\n", success.CompletedStepID)
	}
	if success.ApprovedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Approved step: %s\n", success.ApprovedStepID)
	}
	if success.MovedBackToID != "" {
		_, _ = fmt.Fprintf(stdout, "Moved back to: %s\n", success.MovedBackToID)
	}
	if success.SkippedStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Skipped step: %s\n", success.SkippedStepID)
	}
	if success.NextStepID != "" {
		_, _ = fmt.Fprintf(stdout, "Next step: %s\n", success.NextStepID)
	}
	if success.CompletedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Flow completed: %s\n", success.CompletedFlowID)
	}
	if success.FinishedFlowID != "" {
		_, _ = fmt.Fprintf(stdout, "Finished flow: %s\n", success.FinishedFlowID)
	}
}

func writeActions(stdout io.Writer, actions []command.CommandAction) {
	for _, action := range actions {
		_, _ = fmt.Fprintf(stdout, "%s %s\n", action.Status, action.Path)
	}
}

func writeFlows(stdout io.Writer, flows []command.FlowListItem) {
	for _, flow := range flows {
		if flow.Status == command.FlowStatusInvalid {
			_, _ = fmt.Fprintf(stdout, "file: %s\nstatus: %s\n", flow.FilePath, flow.Status)
			if flow.Err != nil {
				_, _ = fmt.Fprintf(stdout, "error: %v\n", flow.Err)
			}
			_, _ = fmt.Fprintln(stdout)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "id: %s\n", flow.ID)
		_, _ = fmt.Fprintf(stdout, "title: %s\n", flow.Title)
		_, _ = fmt.Fprintf(stdout, "description: %s\n", flow.Description)
		_, _ = fmt.Fprintf(stdout, "steps: %d\n", flow.StepCount)
		_, _ = fmt.Fprintf(stdout, "status: %s\n\n", flow.Status)
	}
}

func writeStatus(stdout io.Writer, status command.StatusResult) {
	_, _ = fmt.Fprintf(stdout, "Flow: %s - %s\n", status.FlowID, status.FlowTitle)
	_, _ = fmt.Fprintf(stdout, "Current step: %s - %s\n", status.CurrentStepID, status.CurrentStepTitle)
	writeStringList(stdout, "Completed steps", status.CompletedSteps)

	_, _ = fmt.Fprintln(stdout, "Skipped steps:")
	for _, stepID := range sortedSkippedStepKeys(status.SkippedSteps) {
		_, _ = fmt.Fprintf(stdout, "- %s: %s\n", stepID, status.SkippedSteps[stepID].Reason)
	}
	if len(status.SkippedSteps) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
	}

	_, _ = fmt.Fprintln(stdout, "Approvals:")
	for _, stepID := range sortedApprovalKeys(status.Approvals) {
		approval := status.Approvals[stepID]
		_, _ = fmt.Fprintf(stdout, "- %s: approved=%t note=%s\n", stepID, approval.Approved, approval.Note)
	}
	if len(status.Approvals) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
	}
}

func writePrompt(stdout io.Writer, prompt command.PromptResult) {
	_, _ = fmt.Fprintf(stdout, "Flow: %s\n", prompt.FlowID)
	_, _ = fmt.Fprintf(stdout, "Step: %s - %s\n", prompt.CurrentStepID, prompt.CurrentStepTitle)
	_, _ = fmt.Fprintf(stdout, "Instruction:\n%s\n", prompt.CurrentStepInstruction)
	writeArtifactList(stdout, "Required artifacts", prompt.RequiredArtifacts)
	if len(prompt.OptionalArtifacts) > 0 {
		writeArtifactList(stdout, "Optional artifacts", prompt.OptionalArtifacts)
	}
	_, _ = fmt.Fprintln(stdout, "Required approval:")
	if prompt.RequiredApproval == nil {
		_, _ = fmt.Fprintln(stdout, "- none")
	} else {
		_, _ = fmt.Fprintf(stdout, "- %s\n", prompt.RequiredApproval.StepID)
	}
	writeStringList(stdout, "After completing", prompt.AfterCompleting.Commands)
}

func writeArtifactList(stdout io.Writer, label string, artifacts []command.ArtifactResult) {
	_, _ = fmt.Fprintf(stdout, "%s:\n", label)
	if len(artifacts) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
		return
	}
	for _, artifact := range artifacts {
		_, _ = fmt.Fprintf(stdout, "- %s\n", artifact.Path)
	}
}

func writeStringList(stdout io.Writer, label string, values []string) {
	_, _ = fmt.Fprintf(stdout, "%s:\n", label)
	if len(values) == 0 {
		_, _ = fmt.Fprintln(stdout, "- none")
		return
	}
	for _, value := range values {
		_, _ = fmt.Fprintf(stdout, "- %s\n", value)
	}
}

func sortedSkippedStepKeys(values map[string]command.SkippedStepResult) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedApprovalKeys(values map[string]command.ApprovalResult) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
